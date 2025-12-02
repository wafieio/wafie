package ingresscache

import (
	"context"
	"net/http"

	"connectrpc.com/connect"
	"connectrpc.com/grpchealth"
	healthv1 "github.com/wafieio/wafie/api/gen/grpc/health/v1"
	"github.com/wafieio/wafie/api/gen/grpc/health/v1/healthv1connect"
	"github.com/wafieio/wafie/logger"
	"go.uber.org/zap"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
)

type Server struct {
	logger     *zap.Logger
	listenAddr string
	apiAddr    string
}

func NewHealthCheckServer(listenAddr, apiAddr string) *Server {
	return &Server{
		logger:     logger.NewLogger(),
		listenAddr: listenAddr,
		apiAddr:    apiAddr,
	}
}

func (s *Server) Serve() {
	go func() {
		s.logger.Info("starting health check server", zap.String("address", s.listenAddr))
		mux := http.NewServeMux()
		mux.Handle(grpchealth.NewHandler(s))
		go func() {
			if err := http.ListenAndServe(s.listenAddr, h2c.NewHandler(mux, &http2.Server{})); err != nil {
				s.logger.Error("failed to start health check server", zap.Error(err))
			}
		}()
	}()
}

func (s *Server) Check(context.Context, *grpchealth.CheckRequest) (*grpchealth.CheckResponse, error) {
	apiSrvHealthCheck := healthv1connect.NewHealthClient(http.DefaultClient, s.apiAddr)
	resp, err := apiSrvHealthCheck.Check(context.Background(), connect.NewRequest(&healthv1.HealthCheckRequest{}))
	if err != nil {
		s.logger.Error("failed to connect to API Server health service", zap.Error(err))
		return &grpchealth.CheckResponse{Status: grpchealth.StatusNotServing}, nil
	}
	if resp.Msg.GetStatus() == healthv1.HealthCheckResponse_SERVING {
		return &grpchealth.CheckResponse{Status: grpchealth.StatusServing}, nil
	} else {
		return &grpchealth.CheckResponse{Status: grpchealth.StatusNotServing}, nil
	}
}
