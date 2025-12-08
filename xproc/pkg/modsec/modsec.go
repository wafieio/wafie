package modsec

/*
#cgo LDFLAGS: -lwafie
#include <stdlib.h>
#include <wafie/wafielib.h>
*/
import "C"
import (
	"connectrpc.com/connect"
	"context"
	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	structpb "github.com/golang/protobuf/ptypes/struct"
	wv1 "github.com/wafieio/wafie/api/gen/wafie/v1"
	wv1c "github.com/wafieio/wafie/api/gen/wafie/v1/wafiev1connect"
	"go.uber.org/zap"
	"net/http"
	"strings"
	"time"
	"unsafe"
)

type EvalRequest C.WafieEvaluationRequest
type RuleSetConfig C.WafieRuleSetConfig

type ModeSec struct {
	logger           *zap.Logger
	protectionClient wv1c.ProtectionServiceClient
}

func NewModSec(apiAddr string, logger *zap.Logger) *ModeSec {
	// init wafie library
	C.wafie_init()
	// init ruleset
	ruleSetConfig := []RuleSetConfig{
		{
			config_path:   C.CString("/config"),
			protection_id: C.int(1),
		},
	}
	C.wafie_load_rule_sets((*C.WafieRuleSetConfig)(&ruleSetConfig[0]), 1)
	// init mod sec instance
	modSec := &ModeSec{
		protectionClient: wv1c.NewProtectionServiceClient(http.DefaultClient, apiAddr),
		logger:           logger,
	}
	// start the ruleset watcher
	modSec.RunRulesetWatcher()
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

func (s *ModeSec) writeRules(pId uint32, ruleSets []*wv1.CrsRuleSet) error {

	return nil
}

func (s *ModeSec) RunRulesetWatcher() {
	for {
		time.Sleep(3 * time.Second)
		s.logger.Debug("fetching current ruleset")
		for _, p := range s.listProtections() {
			rules, err := s.getProtectionRules(p.Id)
			if err != nil {
				s.logger.Error("error getting ruleset", zap.Error(err), zap.Uint32("protectionId", p.Id))
				continue
			}
			if len(rules.CrsVersions) != 1 {
				s.logger.Error("got more than one active rule set",
					zap.Int("count", len(rules.CrsVersions)), zap.Uint32("protectionId", p.Id))
				continue
			}
			if err := s.writeRules(p.Id, rules.CrsVersions[0].CrsRuleSets); err != nil {
				s.logger.Error("error writing ruleset", zap.Error(err), zap.Uint32("protectionId", p.Id))
			}
		}
	}
}

func (s *ModeSec) DestroyTransaction(evalRequest *EvalRequest) {
	// log and cleanup transaction
	C.wafie_cleanup((*C.WafieEvaluationRequest)(evalRequest))
	// free allocated evaluation request
	C.free(unsafe.Pointer(evalRequest.client_ip))
	C.free(unsafe.Pointer(evalRequest.uri))
	C.free(unsafe.Pointer(evalRequest.http_method))
	C.free(unsafe.Pointer(evalRequest.http_version))
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
	hdrs []*corev3.HeaderValue) *EvalRequest {
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
	evalRequest.protection_id = C.int(1)
	// envoy by default will be using :authority for a host header
	// ModSecurity need host header
	hdrs = append(hdrs, &corev3.HeaderValue{Key: "host", RawValue: s.getAuthorityHeader(hdrs)})
	// set headers size
	headersCount := len(hdrs)
	evalRequest.headers_count = C.size_t(headersCount)
	evalRequest.headers = (*C.WafieEvaluationRequestHeader)(
		C.malloc(C.size_t(unsafe.Sizeof(C.WafieEvaluationRequestHeader{})) * C.size_t(headersCount)),
	)
	//if body not set to empty string,
	//it will be NULL and will be skipped from evaluation
	evalRequest.body = C.CString("")
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
	s.logger.Info("new evaluation request",
		zap.Any("client_ip", evalRequest.client_ip),
		zap.Any("uri", evalRequest.uri),
		zap.Any("method", evalRequest.http_method),
		zap.Any("version", evalRequest.http_version),
		zap.Any("headers_count", evalRequest.headers_count),
		zap.Any("headers", evalRequest.headers),
	)

	if C.wafie_process_request_headers((*C.WafieEvaluationRequest)(evalRequest)) != 0 {
		s.logger.Debug("intervention on headers evaluation")
		return true
	}
	if C.wafie_process_request_body((*C.WafieEvaluationRequest)(evalRequest)) != 0 {
		s.logger.Debug("intervention on body evaluation")
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
