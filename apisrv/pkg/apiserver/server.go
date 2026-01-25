package apiserver

import (
	"net/http"

	"connectrpc.com/connect"
	connectcors "connectrpc.com/cors"
	"connectrpc.com/grpchealth"
	"connectrpc.com/grpcreflect"
	"github.com/rs/cors"
	v1 "github.com/wafieio/wafie/api/gen/wafie/v1/wafiev1connect"
	"go.uber.org/zap"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
)

type ApiServer struct {
	logger *zap.Logger
}

func NewApiServer(log *zap.Logger) *ApiServer {

	return &ApiServer{logger: log}
}

func (s *ApiServer) Start() {
	s.logger.Info("starting API server")
	mux := http.NewServeMux()
	s.enableReflection(mux)
	s.registerHandlers(mux)
	handler := s.cors(mux)
	go func() {
		if err := http.ListenAndServe(":8080", h2c.NewHandler(handler, &http2.Server{})); err != nil {
			s.logger.Error("failed to start API server", zap.Error(err))
		}
	}()
	s.logger.Info("server running on 0.0.0.0:8080")
}

func (s *ApiServer) cors(mux *http.ServeMux) http.Handler {
	// Add Authorization header to the allowed headers for auth support
	allowedHeaders := append(connectcors.AllowedHeaders(), "Authorization")

	middleware := cors.New(cors.Options{
		AllowedOrigins:   []string{"http://localhost:5174", "https://localhost:5174"},
		AllowedMethods:   connectcors.AllowedMethods(),
		AllowedHeaders:   allowedHeaders,
		ExposedHeaders:   connectcors.ExposedHeaders(),
		AllowCredentials: true, // Required for Authorization headers
		MaxAge:           7200, // Cache preflight for 2 hours
	})
	return middleware.Handler(mux)
}

func (s *ApiServer) registerHandlers(mux *http.ServeMux) {
	s.logger.Info("registering handlers")
	compress1KB := connect.WithCompressMinBytes(1024)
	mux.Handle(
		grpchealth.NewHandler(
			NewHealthCheckService(s.logger),
			compress1KB,
		),
	)
	mux.Handle(
		v1.NewApplicationServiceHandler(
			NewApplicationService(s.logger),
			compress1KB,
		),
	)
	mux.Handle(
		v1.NewProtectionServiceHandler(
			NewProtectionService(s.logger),
			compress1KB,
		),
	)
	mux.Handle(
		v1.NewStateVersionServiceHandler(
			NewStateVersionService(s.logger),
			compress1KB,
		),
	)
	mux.Handle(
		v1.NewRouteServiceHandler(
			NewRouteService(s.logger),
			compress1KB,
		),
	)
	mux.Handle(
		v1.NewCrsServiceHandler(
			NewCrsService(s.logger),
			compress1KB,
		),
	)
	mux.Handle(
		v1.NewEventServiceHandler(
			NewEventService(s.logger),
			compress1KB,
		),
	)
}

func (s *ApiServer) enableReflection(mux *http.ServeMux) {
	reflector := grpcreflect.NewStaticReflector(
		v1.RouteServiceName,
		v1.AuthServiceName,
		v1.ApplicationServiceName,
		v1.ProtectionServiceName,
		v1.StateVersionServiceName,
		v1.CrsServiceName,
		v1.EventServiceName,
	)
	mux.Handle(grpcreflect.NewHandlerV1(reflector))
	mux.Handle(grpcreflect.NewHandlerV1Alpha(reflector))
}
