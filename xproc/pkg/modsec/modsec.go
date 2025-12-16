package modsec

/*
#cgo LDFLAGS: -lwafie
#include <stdlib.h>
#include <stdint.h>
#include <wafie/wafielib.h>
*/
import "C"
import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unsafe"

	"connectrpc.com/connect"
	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	structpb "github.com/golang/protobuf/ptypes/struct"
	wv1 "github.com/wafieio/wafie/api/gen/wafie/v1"
	wv1c "github.com/wafieio/wafie/api/gen/wafie/v1/wafiev1connect"
	"go.uber.org/zap"
)

type EvalRequest C.WafieEvaluationRequest
type RuleSetConfig C.WafieRuleSetConfig

type EnvoyRequestAttributes map[string]string

type ModeSec struct {
	logger            *zap.Logger
	protectionClient  wv1c.ProtectionServiceClient
	ruleSetBaseConfig string
	auditLogFile      string
}

func NewModSec(apiAddr string, logger *zap.Logger) *ModeSec {
	// init wafie library
	C.wafie_init()
	// init mod sec instance
	modSec := &ModeSec{
		protectionClient:  wv1c.NewProtectionServiceClient(http.DefaultClient, apiAddr),
		logger:            logger,
		ruleSetBaseConfig: "/rules",
		auditLogFile:      "/data/audit/modsec.log", // statically configured, must be the same as in modsecurity.conf
	}
	// start the ruleset watcher
	modSec.runRulesetWatcher()
	// start audit log rotation
	modSec.runAuditLogRotation()
	// return modeSec instance
	return modSec
}

func NewEnvoyRequestAttributes(envoyProcessingAttributes map[string]*structpb.Value) EnvoyRequestAttributes {
	attributes := map[string]string{
		"request.path":     "",
		"source.address":   "",
		"request.protocol": "",
		"request.method":   "",
	}
	for attributeKey, _ := range attributes {
		if attrVal, ok := envoyProcessingAttributes[attributeKey]; ok {
			attributes[attributeKey] = attrVal.GetStringValue()
		}
	}
	return attributes
}

func (a EnvoyRequestAttributes) RequestPath() string {
	return a["request.path"]
}

func (a EnvoyRequestAttributes) SourceAddress() string {
	// envoy stores both ip:port in the source.address
	envoySourceAddrAttribute := strings.Split(a["source.address"], ":")
	return envoySourceAddrAttribute[0]
}

func (a EnvoyRequestAttributes) RequestProtocol() string {
	return a["request.protocol"]
}

func (a EnvoyRequestAttributes) RequestMethod() string {
	return a["request.method"]
}

func (s *ModeSec) listProtections() []*wv1.Protection {
	includeApps := false
	listReq := connect.NewRequest(&wv1.ListProtectionsRequest{
		Options: &wv1.ListProtectionsOptions{
			IncludeApps: &includeApps,
		},
	})
	lst, err := s.protectionClient.ListProtections(context.Background(), listReq)
	if err != nil {
		s.logger.Error("error listing ruleset", zap.Error(err))
		return []*wv1.Protection{} // in case of error return empty list
	}
	return lst.Msg.Protections
}

func (s *ModeSec) getProtectionRules(id uint32) (*wv1.Protection, error) {
	activeRules := wv1.GetProtectionOptionsIncludeCrsRules_GET_PROTECTION_OPTIONS_INCLUDE_CRS_RULES_ACTIVE
	getReq := connect.NewRequest(&wv1.GetProtectionRequest{
		Id: id,
		Options: &wv1.GetProtectionOptions{
			IncludeCrsRules: &activeRules,
		},
	})
	if p, err := s.protectionClient.GetProtection(context.Background(), getReq); err != nil {
		return nil, err
	} else {
		return p.Msg.Protection, nil
	}
}

func (s *ModeSec) ruleSetMD5(ruleSets []*wv1.CrsRuleSet) (string, error) {
	h := md5.New()
	for _, rules := range ruleSets {
		if _, err := io.WriteString(h, rules.Md5); err != nil {
			return "", err
		}
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func (s *ModeSec) shouldWrite(rulesDir string) (bool, error) {
	_, err := os.Stat(rulesDir)
	// if directory not exists, write it
	if os.IsNotExist(err) {
		return true, nil
	}
	// in case of any other error, return it
	if err != nil {
		return false, err
	}
	// check directory entries
	entries, err := os.ReadDir(rulesDir)
	if err != nil {
		return false, err
	}
	// if no rules files found, write it
	if len(entries) == 0 {
		return true, nil
	}
	//  has rule files, do not write and return
	return false, nil
}

func (s *ModeSec) writeRules(protectionId uint32, ruleSetMd5 string, ruleSets []*wv1.CrsRuleSet) (reloadRequire bool, err error) {
	// check for a changes in a rule set
	rulesDir := s.rulesDir(protectionId, ruleSetMd5)
	shouldWrite, err := s.shouldWrite(rulesDir)
	if err != nil {
		return false, err
	}
	// write rules not required, return
	if !shouldWrite {
		return false, nil
	}
	// rules write is required, write the rules,
	// set reloadRequire=true for further reloading in libwafie
	s.logger.Info("rules write required", zap.String("rulesDir", rulesDir))
	reloadRequire = true
	for _, rules := range ruleSets {
		ruleFile := fmt.Sprintf("%s/%s", rulesDir, rules.CrsFileName)
		dirPath, _ := filepath.Split(ruleFile)
		if err := os.MkdirAll(dirPath, 0755); err != nil {
			return false, err
		}
		if err := os.WriteFile(ruleFile, []byte(rules.CrsFileContent), 0700); err != nil {
			return false, err
		}
	}
	return reloadRequire, nil
}

func (s *ModeSec) reloadCRSRules(ruleSetConfigForReload map[string]uint32) {
	if len(ruleSetConfigForReload) == 0 {
		s.logger.Info("rule set config for reload is empty, no crs rules reload required")
		return
	}
	ruleSetConfig := make([]RuleSetConfig, len(ruleSetConfigForReload))
	defer func() {
		for i := range ruleSetConfig {
			C.free(unsafe.Pointer(ruleSetConfig[i].config_path)) // free all the C strings
		}
	}()
	idx := 0
	for configPath, protectionId := range ruleSetConfigForReload {
		ruleSetConfig[idx].config_path = C.CString(configPath)
		ruleSetConfig[idx].protection_id = C.uint32_t(protectionId)
		idx++
	}
	C.wafie_load_rule_sets((*C.WafieRuleSetConfig)(&ruleSetConfig[0]), C.int(len(ruleSetConfigForReload)))
}

func (s *ModeSec) cleanupRules(ruleSetConfigForReload map[string]uint32) {
	if len(ruleSetConfigForReload) == 0 {
		s.logger.Info("rule set config for reload is empty, no crs rules reload required")
		return
	}
	for activeProtectionPath, protectionId := range ruleSetConfigForReload {
		entries, err := os.ReadDir(s.protectionBaseDir(protectionId))
		if err != nil {
			s.logger.Error("error reading rules directory",
				zap.String("activeProtectionPath", activeProtectionPath), zap.Error(err))
			continue
		}
		for _, entry := range entries {
			inactiveProtectionPath := filepath.Join(s.protectionBaseDir(protectionId), entry.Name())
			// not active protection path - delete it
			if activeProtectionPath != inactiveProtectionPath {
				s.logger.Info("removing inactive protection path",
					zap.String("inactiveProtectionPath", inactiveProtectionPath))
				if err := os.RemoveAll(inactiveProtectionPath); err != nil {
					s.logger.Error("error removing rules directory",
						zap.String("protectionPath", inactiveProtectionPath), zap.Error(err))
				}
			}
		}
	}
}

func (s *ModeSec) protectionBaseDir(protectionId uint32) string {
	return fmt.Sprintf("%s/%d", s.ruleSetBaseConfig, protectionId)
}

func (s *ModeSec) rulesDir(protectionId uint32, ruleSetMd5 string) string {
	return fmt.Sprintf("%s/%s", s.protectionBaseDir(protectionId), ruleSetMd5)
}

func (s *ModeSec) runRulesetWatcher() {
	firstRun := true
	go func() {
		ticker := time.NewTicker(3 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			reloadRequire := false
			ruleSetConfigForReload := map[string]uint32{}

			for _, p := range s.listProtections() {
				rules, err := s.getProtectionRules(p.Id)
				if err != nil {
					s.logger.Error("error getting ruleset", zap.Error(err), zap.Uint32("protectionId", p.Id))
					continue
				}
				if rules.CrsVersions == nil {
					continue
				}
				if len(rules.CrsVersions) != 1 {
					s.logger.Error("got more than one active rule set",
						zap.Int("count", len(rules.CrsVersions)), zap.Uint32("protectionId", p.Id))
					continue
				}
				if len(rules.CrsVersions[0].CrsRuleSets) == 0 {
					continue
				}
				// calculate rule set md5
				ruleSetMd5, err := s.ruleSetMD5(rules.CrsVersions[0].CrsRuleSets)
				if err != nil {
					s.logger.Error("error getting rule set md5", zap.Error(err))
					continue
				}
				if reload, err := s.writeRules(p.Id, ruleSetMd5, rules.CrsVersions[0].CrsRuleSets); err != nil {
					s.logger.Error("error writing ruleset", zap.Error(err), zap.Uint32("protectionId", p.Id))
				} else if reload {
					reloadRequire = true
				}
				ruleSetConfigForReload[s.rulesDir(p.Id, ruleSetMd5)] = p.Id
			}
			if reloadRequire || firstRun {
				firstRun = false
				s.reloadCRSRules(ruleSetConfigForReload)
				s.cleanupRules(ruleSetConfigForReload)
			}
		}
	}()
}

func (s *ModeSec) DestroyTransaction(evalRequest *EvalRequest) {
	// log and cleanup transaction
	C.wafie_cleanup((*C.WafieEvaluationRequest)(evalRequest))
	// free allocated evaluation request
	C.free(unsafe.Pointer(evalRequest.client_ip))
	C.free(unsafe.Pointer(evalRequest.uri))
	C.free(unsafe.Pointer(evalRequest.http_method))
	C.free(unsafe.Pointer(evalRequest.http_version))
	C.free(unsafe.Pointer(evalRequest.body))
	for i := 0; i < int(evalRequest.headers_count); i++ {
		hdr := (*C.WafieEvaluationRequestHeader)(
			unsafe.Pointer(uintptr(unsafe.Pointer(evalRequest.headers)) + uintptr(i)*
				unsafe.Sizeof(C.WafieEvaluationRequestHeader{})))
		C.free(unsafe.Pointer(hdr.key))
		C.free(unsafe.Pointer(hdr.value))
	}
	C.free(unsafe.Pointer(evalRequest.headers))
}

func (s *ModeSec) requestAttributes(envoyProcessingAttributes map[string]*structpb.Value) map[string]string {
	attributes := map[string]string{
		"request.path":     "",
		"source.address":   "",
		"request.protocol": "",
		"request.method":   "",
	}
	for attributeKey, _ := range attributes {
		if attrVal, ok := envoyProcessingAttributes[attributeKey]; ok {
			attributes[attributeKey] = attrVal.GetStringValue()
		}
	}
	return attributes
}

func (s *ModeSec) InitEvalRequest(
	envoyProcessingAttributes map[string]*structpb.Value,
	hdrs []*corev3.HeaderValue, protectionId uint32) *EvalRequest {
	evalRequest := EvalRequest{}
	attributes := NewEnvoyRequestAttributes(envoyProcessingAttributes)
	// set basic intervention parameters
	evalRequest.client_ip = C.CString(attributes.SourceAddress())
	evalRequest.uri = C.CString(attributes.RequestPath())
	evalRequest.http_method = C.CString(attributes.RequestMethod())
	evalRequest.http_version = C.CString(s.getHttpProtocolVersion(attributes.RequestProtocol()))
	evalRequest.protection_id = C.uint32_t(protectionId)
	// envoy by default will be using :authority for a host header
	// ModSecurity need host header
	hdrs = append(hdrs, &corev3.HeaderValue{Key: "host", RawValue: s.getAuthorityHeader(hdrs)})
	// set headers size
	headersCount := len(hdrs)
	evalRequest.headers_count = C.size_t(headersCount)
	evalRequest.headers = (*C.WafieEvaluationRequestHeader)(
		C.malloc(C.size_t(unsafe.Sizeof(C.WafieEvaluationRequestHeader{})) * C.size_t(headersCount)),
	)
	// add headers into evaluation request
	for idx, hdr := range hdrs {
		headerPtr := (*C.WafieEvaluationRequestHeader)(
			unsafe.Pointer(
				uintptr(unsafe.Pointer(evalRequest.headers)) +
					uintptr(idx)*unsafe.Sizeof(C.WafieEvaluationRequestHeader{}),
			),
		)
		headerPtr.key = (*C.uchar)(unsafe.Pointer(C.CString(hdr.Key)))
		if hdr.Value != "" {
			headerPtr.value = (*C.uchar)(unsafe.Pointer(C.CString(hdr.Value)))
		} else {
			headerPtr.value = (*C.uchar)(unsafe.Pointer(C.CString(string(hdr.RawValue))))
		}
	}
	// init modsec transaction
	C.wafie_init_transaction((*C.WafieEvaluationRequest)(&evalRequest))
	return &evalRequest
}

func (s *ModeSec) SetEvalRequestBody(body string, evalRequest *EvalRequest) {
	evalRequest.body = C.CString(body)
}

func (s *ModeSec) getHttpProtocolVersion(protocol string) string {
	protocolSlice := strings.Split(protocol, "/")
	if len(protocolSlice) > 0 {
		return protocolSlice[len(protocolSlice)-1]
	}
	return protocol
}

func (s *ModeSec) getAuthorityHeader(hdrs []*corev3.HeaderValue) []byte {
	for _, hdr := range hdrs {
		if hdr.Key == ":authority" {
			return hdr.RawValue
		}
	}
	return []byte{}
}

func (s *ModeSec) EvaluateHeaders(evalRequest *EvalRequest) (intervened bool) {
	if C.wafie_process_request_headers((*C.WafieEvaluationRequest)(evalRequest)) != 0 {
		s.logger.Debug("intervention on headers evaluation")
		return true
	}

	return false
}

func (s *ModeSec) EvaluateBody(evalRequest *EvalRequest) (intervened bool) {
	if C.wafie_process_request_body((*C.WafieEvaluationRequest)(evalRequest)) != 0 {
		s.logger.Debug("intervention on body evaluation")
		return true
	}
	return false
}

func (s *ModeSec) runAuditLogRotation() {
	go func() {
		var maxAuditLogSize int64 = 5 * 1024 * 1024 // 5 MB
		ticker := time.NewTicker(1 * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			info, err := os.Stat(s.auditLogFile)
			if os.IsNotExist(err) {
				continue
			}
			if err != nil {
				s.logger.Warn("failed to stat audit log file")
				continue
			}
			if info.Size() >= maxAuditLogSize {
				err := os.Truncate(s.auditLogFile, 0)
				if err != nil {
					s.logger.Error("failed to truncate audit log file", zap.Error(err))
				} else {
					s.logger.Info("audit log file has been rotated", zap.Int64("size", info.Size()))
				}
			}
		}
	}()
}
