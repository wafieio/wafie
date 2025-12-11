package modsec

/*
#cgo LDFLAGS: -lwafie
#include <stdlib.h>
#include <stdint.h>
#include <wafie/wafielib.h>
*/
import "C"
import (
	"connectrpc.com/connect"
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	structpb "github.com/golang/protobuf/ptypes/struct"
	wv1 "github.com/wafieio/wafie/api/gen/wafie/v1"
	wv1c "github.com/wafieio/wafie/api/gen/wafie/v1/wafiev1connect"
	"go.uber.org/zap"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unsafe"
)

type EvalRequest C.WafieEvaluationRequest
type RuleSetConfig C.WafieRuleSetConfig

type ModeSec struct {
	logger            *zap.Logger
	protectionClient  wv1c.ProtectionServiceClient
	ruleSetBaseConfig string
}

func NewModSec(apiAddr string, logger *zap.Logger) *ModeSec {
	// init wafie library
	C.wafie_init()
	// init ruleset
	//ruleSetConfig := []RuleSetConfig{
	//	{
	//		config_path:   C.CString("/config"),
	//		protection_id: C.int(1),
	//	},
	//}
	//C.wafie_load_rule_sets((*C.WafieRuleSetConfig)(&ruleSetConfig[0]), 0)
	// init mod sec instance
	modSec := &ModeSec{
		protectionClient:  wv1c.NewProtectionServiceClient(http.DefaultClient, apiAddr),
		logger:            logger,
		ruleSetBaseConfig: "/config",
	}
	// start the ruleset watcher
	modSec.runRulesetWatcher()
	// return modeSec instance
	return modSec
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
	rulesDir := fmt.Sprintf("%s/%s/%d", s.ruleSetBaseConfig, ruleSetMd5, protectionId)
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

func (s *ModeSec) runRulesetWatcher() {
	firstRun := true
	go func() {
		for {
			time.Sleep(3 * time.Second)
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
				ruleSetConfigForReload[fmt.Sprintf("%s/%s/%d", s.ruleSetBaseConfig, ruleSetMd5, p.Id)] = p.Id
			}
			if reloadRequire || firstRun {
				firstRun = false
				s.reloadCRSRules(ruleSetConfigForReload)
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

func (s *ModeSec) InitEvalRequest(
	envoyProcessingAttributes map[string]*structpb.Value,
	hdrs []*corev3.HeaderValue, protectionId uint32) *EvalRequest {
	evalRequest := EvalRequest{}
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
	// set basic intervention parameters
	evalRequest.client_ip = C.CString(attributes["request.address"])
	evalRequest.uri = C.CString(attributes["request.path"])
	evalRequest.http_method = C.CString(attributes["request.method"])
	evalRequest.http_version = C.CString(s.getHttpProtocolVersion(attributes["request.protocol"]))
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
