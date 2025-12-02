package apisrv

import (
	"context"
	"fmt"
	"net/http"

	"connectrpc.com/connect"
	"connectrpc.com/grpchealth"
	"connectrpc.com/grpcreflect"
	wv1 "github.com/wafieio/wafie/api/gen/wafie/v1"
	"github.com/wafieio/wafie/api/gen/wafie/v1/wafiev1connect"
	"github.com/wafieio/wafie/relay/pkg/relay"
	"go.uber.org/zap"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
)

const (
	ApiListeningPort = 57812
)

type Server struct {
	wafiev1connect.UnimplementedRelayServiceHandler
	logger     *zap.Logger
	listenAddr string
	relay      relay.Relay
}

func NewServer(logger *zap.Logger, r relay.Relay) *Server {
	return &Server{
		logger:     logger,
		listenAddr: fmt.Sprintf("localhost:%d", ApiListeningPort),
		relay:      r,
	}
}

func (s *Server) Start() {
	go func() {
		s.logger.Info("starting health check server", zap.String("address", s.listenAddr))
		mux := http.NewServeMux()
		mux.Handle(grpchealth.NewHandler(s))
		mux.Handle(wafiev1connect.NewRelayServiceHandler(s))
		reflector := grpcreflect.NewStaticReflector(
			wafiev1connect.RelayServiceName,
			grpchealth.HealthV1ServiceName,
		)
		mux.Handle(grpcreflect.NewHandlerV1(reflector))
		mux.Handle(grpcreflect.NewHandlerV1Alpha(reflector))
		go func() {
			if err := http.ListenAndServe(s.listenAddr, h2c.NewHandler(mux, &http2.Server{})); err != nil {
				s.logger.Error("failed to start health check server", zap.Error(err))
			}
		}()
	}()
}

func (s *Server) StartRelay(
	ctx context.Context,
	req *connect.Request[wv1.StartRelayRequest]) (
	*connect.Response[wv1.StartRelayResponse], error) {
	s.logger.Debug("starting relay instance")
	startRelay, _ := s.relay.Configure(req.Msg.Options)
	startRelay()
	resp := connect.NewResponse(
		&wv1.StartRelayResponse{
			TcpRelayStatus: "ok",
			NftStatus:      "ok",
		},
	)
	return resp, nil
}

func (s *Server) Check(context.Context, *grpchealth.CheckRequest) (*grpchealth.CheckResponse, error) {
	s.logger.Debug("health check request received")
	return &grpchealth.CheckResponse{Status: grpchealth.StatusServing}, nil
}

func (s *Server) StopRelay(
	ctx context.Context,
	req *connect.Request[wv1.StopRelayRequest]) (
	*connect.Response[wv1.StopRelayResponse], error) {
	s.logger.Debug("terminating relay instance")
	_, stopRelay := s.relay.Configure(nil)
	stopRelay()
	s.logger.Debug("relay instance terminated")
	return connect.NewResponse(&wv1.StopRelayResponse{}), nil
}
