package modsec

/*
#cgo LDFLAGS: -lwafie
#include <stdlib.h>
#include <wafie/wafielib.h>
*/
import "C"
import (
	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	structpb "github.com/golang/protobuf/ptypes/struct"
	"go.uber.org/zap"
	"strings"
	"unsafe"
)

type EvalRequest C.EvaluationRequest

type ModeSec struct {
	logger *zap.Logger
}

func NewModSec(logger *zap.Logger) *ModeSec {
	// init modsecurity library
	C.wafie_library_init(C.CString("/config"))
	// init mod sec instance
	return &ModeSec{
		logger: logger,
	}
}

func (s *ModeSec) DestroyTransaction(evalRequest *EvalRequest) {
	// log and cleanup transaction
	C.wafie_transaction_cleanup((*C.EvaluationRequest)(evalRequest))
	// free allocated evaluation request
	C.free(unsafe.Pointer(evalRequest.client_ip))
	C.free(unsafe.Pointer(evalRequest.uri))
	C.free(unsafe.Pointer(evalRequest.http_method))
	C.free(unsafe.Pointer(evalRequest.http_version))
	for i := 0; i < int(evalRequest.headers_count); i++ {
		hdr := (*C.EvaluationRequestHeader)(
			unsafe.Pointer(uintptr(unsafe.Pointer(evalRequest.headers)) + uintptr(i)*
				unsafe.Sizeof(C.EvaluationRequestHeader{})))
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
	// envoy by default will be using :authority for a host header
	// ModSecurity need host header
	hdrs = append(hdrs, &corev3.HeaderValue{Key: "host", RawValue: s.getAuthorityHeader(hdrs)})
	// set headers size
	headersCount := len(hdrs)
	evalRequest.headers_count = C.size_t(headersCount)
	evalRequest.headers = (*C.EvaluationRequestHeader)(
		C.malloc(C.size_t(unsafe.Sizeof(C.EvaluationRequestHeader{})) * C.size_t(headersCount)),
	)
	//if body not set to empty string,
	//it will be NULL and will be skipped from evaluation
	evalRequest.body = C.CString("")

	for idx, hdr := range hdrs {
		headerPtr := (*C.EvaluationRequestHeader)(
			unsafe.Pointer(
				uintptr(unsafe.Pointer(evalRequest.headers)) +
					uintptr(idx)*unsafe.Sizeof(C.EvaluationRequestHeader{}),
			),
		)
		headerPtr.key = (*C.uchar)(unsafe.Pointer(C.CString(hdr.Key)))
		if hdr.Value != "" {
			headerPtr.value = (*C.uchar)(unsafe.Pointer(C.CString(hdr.Value)))
		} else {
			headerPtr.value = (*C.uchar)(unsafe.Pointer(C.CString(string(hdr.RawValue))))
		}
	}
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
	C.wafie_init_request_transaction((*C.EvaluationRequest)(evalRequest))
	s.logger.Info("new evaluation request",
		zap.Any("client_ip", evalRequest.client_ip),
		zap.Any("uri", evalRequest.uri),
		zap.Any("method", evalRequest.http_method),
		zap.Any("version", evalRequest.http_version),
		zap.Any("headers_count", evalRequest.headers_count),
		zap.Any("headers", evalRequest.headers),
	)

	if C.wafie_process_request_headers((*C.EvaluationRequest)(evalRequest)) != 0 {
		s.logger.Debug("intervention on headers evaluation")
		return true
	}
	if C.wafie_process_request_body((*C.EvaluationRequest)(evalRequest)) != 0 {
		s.logger.Debug("intervention on body evaluation")
		return true
	}
	return false
}

func (s *ModeSec) EvaluateBody(evalRequest *EvalRequest) (intervened bool) {
	if C.wafie_process_request_body((*C.EvaluationRequest)(evalRequest)) != 0 {
		s.logger.Debug("intervention on body evaluation")
		return true
	}
	return false
}
