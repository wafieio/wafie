package processor

import (
	"fmt"
	"github.com/Dimss/wafie/xproc/pkg/modsec"
	core "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	extproc "github.com/envoyproxy/go-control-plane/envoy/service/ext_proc/v3"
	typev3 "github.com/envoyproxy/go-control-plane/envoy/type/v3"
	"go.uber.org/zap"
	"io"
	"log"
)

type ExternalProcessor struct {
	logger *zap.Logger
	extproc.UnimplementedExternalProcessorServer
}

func NewExternalProcessor(logger *zap.Logger) *ExternalProcessor {
	return &ExternalProcessor{
		logger: logger,
	}
}

func (s *ExternalProcessor) Process(stream extproc.ExternalProcessor_ProcessServer) error {
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
		case *extproc.ProcessingRequest_RequestHeaders:
			log.Println("Processing request headers")
			modSec := modsec.NewModSec(s.logger)
			modSec.NewEvaluationRequest(r.RequestHeaders.Headers.Headers)

			attributes := []string{"request.path", "source.address", "request.protocol", "request.method"}

			for _, attribute := range attributes {
				if attrVal, ok := req.Attributes["envoy.filters.http.ext_proc"].GetFields()[attribute]; ok {
					s.logger.Info("request attributes", zap.String(attribute, attrVal.GetStringValue()))
				}
			}
			//for attrName, attrValue := range req.Attributes {
			//	s.logger.Info("request attributes",
			//		zap.String(attrName, attrValue.String()))
			//}
			//if val, ok := req.Attributes[attrName]; ok {
			//	s.logger.Info("request attributes",
			//		zap.String("path", attr.String()))
			//}
			//}

			//modsec.EvaluationRequestHeaders(r.RequestHeaders.Headers.Headers)
			for _, header := range r.RequestHeaders.Headers.Headers {
				if header.Key == "foo" {
					if err := stream.Send(immediateResponse()); err != nil {
						fmt.Println(err)

					}
					return nil
				}

			}

			resp = &extproc.ProcessingResponse{
				Response: &extproc.ProcessingResponse_RequestHeaders{
					RequestHeaders: &extproc.HeadersResponse{},
				},
			}

		case *extproc.ProcessingRequest_ResponseHeaders:
			log.Println("Processing response headers")
			resp = &extproc.ProcessingResponse{
				Response: &extproc.ProcessingResponse_ResponseHeaders{
					ResponseHeaders: &extproc.HeadersResponse{},
				},
			}

		case *extproc.ProcessingRequest_RequestBody:
			log.Println("Processing request body")
			resp = &extproc.ProcessingResponse{
				Response: &extproc.ProcessingResponse_RequestBody{
					RequestBody: &extproc.BodyResponse{},
				},
			}

		case *extproc.ProcessingRequest_ResponseBody:
			log.Println("Processing response body")
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

func immediateResponse() *extproc.ProcessingResponse {
	body := "blocked by wafie.io"
	return &extproc.ProcessingResponse{
		Response: &extproc.ProcessingResponse_ImmediateResponse{
			ImmediateResponse: &extproc.ImmediateResponse{
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
				Body: []byte(body),
			},
		},
	}
}
