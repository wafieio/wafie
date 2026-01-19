package processor

import "C"
import (
	"fmt"
	"io"
	"log"
	"strconv"

	core "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	extproc "github.com/envoyproxy/go-control-plane/envoy/service/ext_proc/v3"
	typev3 "github.com/envoyproxy/go-control-plane/envoy/type/v3"
	"github.com/wafieio/wafie/xproc/pkg/assets"
	"github.com/wafieio/wafie/xproc/pkg/modsec"
	"go.uber.org/zap"
)

const (
	wafieProtectionIdHeader = "x-wafie-protection-id"
)

type ExternalProcessor struct {
	logger *zap.Logger
	modsec *modsec.ModeSec
	assets *assets.Assets
	extproc.UnimplementedExternalProcessorServer
}

func NewExternalProcessor(apiAddr string, logger *zap.Logger) *ExternalProcessor {
	return &ExternalProcessor{
		modsec: modsec.NewModSec(apiAddr, logger),
		assets: assets.New(logger),
		logger: logger,
	}
}

func (s *ExternalProcessor) getProtectionId(hdrs []*core.HeaderValue) uint32 {
	for _, hdr := range hdrs {
		if hdr.Key == wafieProtectionIdHeader {
			val, err := strconv.ParseUint(string(hdr.RawValue), 10, 32)
			if err != nil {
				fmt.Println("Error:", err)
				return 0
			}
			s.logger.Info("protection", zap.Uint32("id", uint32(val)))
			return uint32(val)
		}
	}
	return 0
}

func (s *ExternalProcessor) Process(stream extproc.ExternalProcessor_ProcessServer) error {
	var evalRequest *modsec.EvalRequest
	var protectionId uint32
	defer func() {
		if evalRequest != nil {
			s.modsec.DestroyTransaction(evalRequest)
		}
	}()
	for {
		req, err := stream.Recv()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		var resp *extproc.ProcessingResponse
		switch r := req.Request.(type) {
		// Request beaders evaluation
		case *extproc.ProcessingRequest_RequestHeaders:
			log.Println("processing request headers")
			// get protection ID
			protectionId = s.getProtectionId(r.RequestHeaders.Headers.Headers)
			if protectionId == 0 {
				s.logger.Warn("protection id is undefined")
			}
			// init eval request
			evalRequest = s.modsec.InitEvalRequest(
				modsec.NewEnvoyRequestAttributes(req.Attributes["envoy.filters.http.ext_proc"].GetFields()),
				r.RequestHeaders.Headers.Headers,
				protectionId,
			)
			// process transaction
			intervened := s.modsec.ProcessRequestHeaders(evalRequest)
			// if intervened, block request
			if intervened {
				resp = s.interventionResponse(evalRequest)
			} else {
				resp = &extproc.ProcessingResponse{
					Response: &extproc.ProcessingResponse_RequestHeaders{
						RequestHeaders: &extproc.HeadersResponse{},
					},
				}
			}
		// Request body evaluation
		case *extproc.ProcessingRequest_RequestBody:
			log.Println("processing request body")
			s.modsec.SetRequestBody(r.RequestBody.String(), evalRequest)
			intervened := s.modsec.ProcessRequestBody(evalRequest)
			if intervened {
				resp = s.interventionResponse(evalRequest)
			} else {
				resp = &extproc.ProcessingResponse{
					Response: &extproc.ProcessingResponse_RequestBody{
						RequestBody: &extproc.BodyResponse{},
					},
				}
			}
		// Response headers evaluation
		case *extproc.ProcessingRequest_ResponseHeaders:
			s.modsec.SetResponseHeaders(r.ResponseHeaders.Headers.Headers, evalRequest)
			// process transaction
			intervened := s.modsec.ProcessResponseHeaders(evalRequest)
			if intervened {
				resp = s.interventionResponse(evalRequest)
			} else {
				resp = &extproc.ProcessingResponse{
					Response: &extproc.ProcessingResponse_ResponseHeaders{
						ResponseHeaders: &extproc.HeadersResponse{},
					},
				}
			}
		// Response body evaluation
		case *extproc.ProcessingRequest_ResponseBody:
			s.modsec.SetResponseBody(r.ResponseBody.String(), evalRequest)
			intervened := s.modsec.ProcessRequestBody(evalRequest)
			if intervened {
				resp = s.interventionResponse(evalRequest)
			} else {
				resp = &extproc.ProcessingResponse{
					Response: &extproc.ProcessingResponse_ResponseBody{
						ResponseBody: &extproc.BodyResponse{},
					},
				}
			}
		}

		if err := stream.Send(resp); err != nil {
			return err
		}
	}
}

func (s *ExternalProcessor) interventionResponse(evalRequest *modsec.EvalRequest) *extproc.ProcessingResponse {
	immediateResponse := &extproc.ImmediateResponse{
		Status: &typev3.HttpStatus{Code: typev3.StatusCode_Forbidden},
		Headers: &extproc.HeaderMutation{
			SetHeaders: []*core.HeaderValueOption{
				{
					Header: &core.HeaderValue{
						Key:      "content-type",
						RawValue: []byte("text/html"),
					},
				},
			},
		},
		Body: s.assets.BlockPage(),
	}
	// enrich with context
	s.modsec.EnrichWithInterventionContext(evalRequest, immediateResponse, s.assets)
	// return optionally enriched response
	return &extproc.ProcessingResponse{
		Response: &extproc.ProcessingResponse_ImmediateResponse{
			ImmediateResponse: immediateResponse,
		},
	}
}
