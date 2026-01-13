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
			s.logger.Info("protection id", zap.String("protection_id", hdr.Value))
			val, err := strconv.ParseUint(string(hdr.RawValue), 10, 32)
			if err != nil {
				fmt.Println("Error:", err)
				return 0
			}
			return uint32(val)
		}
	}
	return 0
}

func (s *ExternalProcessor) Process(stream extproc.ExternalProcessor_ProcessServer) error {
	var evalRequest *modsec.EvalRequest
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
		// Request Headers Evaluation
		case *extproc.ProcessingRequest_RequestHeaders:
			log.Println("Processing request headers")
			// get protection ID
			protectionId := s.getProtectionId(r.RequestHeaders.Headers.Headers)
			if protectionId == 0 {
				s.logger.Warn("protection id is undefined")
			}
			// init eval request
			evalRequest = s.modsec.InitEvalRequest(
				req.Attributes["envoy.filters.http.ext_proc"].GetFields(),
				r.RequestHeaders.Headers.Headers,
				protectionId,
			)
			// process transaction
			intervened := s.modsec.EvaluateHeaders(evalRequest)
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
		case *extproc.ProcessingRequest_ResponseHeaders:
			resp = &extproc.ProcessingResponse{
				Response: &extproc.ProcessingResponse_ResponseHeaders{
					ResponseHeaders: &extproc.HeadersResponse{},
				},
			}
		// Request Body Evaluation
		case *extproc.ProcessingRequest_RequestBody:
			log.Println("Processing request body")
			s.modsec.SetEvalRequestBody(r.RequestBody.String(), evalRequest)
			intervened := s.modsec.EvaluateBody(evalRequest)
			if intervened {
				resp = s.interventionResponse(evalRequest)
			} else {
				resp = &extproc.ProcessingResponse{
					Response: &extproc.ProcessingResponse_RequestBody{
						RequestBody: &extproc.BodyResponse{},
					},
				}
			}
		case *extproc.ProcessingRequest_ResponseBody:
			resp = &extproc.ProcessingResponse{
				Response: &extproc.ProcessingResponse_ResponseBody{
					ResponseBody: &extproc.BodyResponse{},
				},
			}

		}

		if err := stream.Send(resp); err != nil {
			return err
		}
	}
}

func (s *ExternalProcessor) interventionResponse(evalRequest *modsec.EvalRequest) *extproc.ProcessingResponse {
	headers := s.modsec.InterventionContext(evalRequest)
	headers = append(headers, &core.HeaderValueOption{
		Header: &core.HeaderValue{
			Key:      "content-type",
			RawValue: []byte("text/html"),
		},
	})
	return &extproc.ProcessingResponse{
		Response: &extproc.ProcessingResponse_ImmediateResponse{
			ImmediateResponse: &extproc.ImmediateResponse{
				Status: &typev3.HttpStatus{Code: typev3.StatusCode_Unauthorized},
				Headers: &extproc.HeaderMutation{
					SetHeaders: headers,
				},
				Body: s.assets.BlockPage(),
			},
		},
	}
}
