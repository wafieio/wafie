package modsec

/*
#cgo LDFLAGS: -lwafie
#include <stdlib.h>
#include <wafie/wafielib.h>
*/
import "C"
import (
	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	"go.uber.org/zap"
	"unsafe"
)

type ModeSec struct {
	logger *zap.Logger
}

func NewModSec(logger *zap.Logger) *ModeSec {
	return &ModeSec{logger: logger}
}

func (s *ModeSec) EvaluationRequestHeaders(reqHeaders []*corev3.HeaderValue) *C.EvaluationRequestHeader {
	var evalRequestHeaders = (*C.EvaluationRequestHeader)(
		C.malloc(
			C.size_t(unsafe.Sizeof(C.EvaluationRequestHeader{})) * C.size_t(len(reqHeaders)),
		),
	)
	for i, reqHeader := range reqHeaders {
		hdr := (*C.EvaluationRequestHeader)(
			unsafe.Pointer(uintptr(unsafe.Pointer(evalRequestHeaders)) + uintptr(i)*
				unsafe.Sizeof(C.EvaluationRequestHeader{})))
		hdr.key = (*C.uchar)(unsafe.Pointer(C.CString(reqHeader.Key)))
		hdr.value = (*C.uchar)(unsafe.Pointer(C.CString(reqHeader.Value)))
	}
	return evalRequestHeaders
}

func (s *ModeSec) NewEvaluationRequest(reqHeaders []*corev3.HeaderValue) {
	C.wafie_library_init(C.CString("/config"))
	var evalRequest C.EvaluationRequest
	//var clientIp, httpVersion string
	//ip := "1.2.3.4"

	evalRequest.client_ip = C.CString("1.2.3.4")
	evalRequest.uri = C.CString("foo/bar")
	evalRequest.http_method = C.CString("method")
	evalRequest.http_version = C.CString("httpVersion")
	evalRequest.headers_count = C.size_t(len(reqHeaders))
	//evalRequest.headers = s.EvaluationRequestHeaders(reqHeaders)
	evalRequest.headers = (*C.EvaluationRequestHeader)(
		C.malloc(C.size_t(unsafe.Sizeof(C.EvaluationRequestHeader{})) * C.size_t(len(reqHeaders))),
	)

	for idx, hdr := range reqHeaders {
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
	evalRequest.body = nil
	C.wafie_init_request_transaction(&evalRequest)
	s.logger.Info("new evaluation request",
		zap.Any("client_ip", evalRequest.client_ip),
		zap.Any("uri", evalRequest.uri),
		zap.Any("method", evalRequest.http_method),
		zap.Any("version", evalRequest.http_version),
		zap.Any("headers_count", evalRequest.headers_count),
		zap.Any("headers", evalRequest.headers),
	)
	if C.wafie_process_request_headers(&evalRequest) != 0 {
		s.logger.Info("violation on headers")
	}
}
