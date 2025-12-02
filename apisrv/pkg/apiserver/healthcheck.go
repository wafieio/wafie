package apiserver

import (
	"context"

	"connectrpc.com/grpchealth"
	"github.com/wafieio/wafie/apisrv/internal/models"
	"go.uber.org/zap"
)

type HealthCheckService struct {
	logger *zap.Logger
}

func NewHealthCheckService(log *zap.Logger) *HealthCheckService {
	return &HealthCheckService{
		logger: log,
	}
}

func (s *HealthCheckService) Check(context.Context, *grpchealth.CheckRequest) (*grpchealth.CheckResponse, error) {
	healthCheckModelSvc := models.NewHealthCheckSvc(nil, s.logger)
	if err := healthCheckModelSvc.Ping(); err != nil {
		s.logger.Error("database ping failed", zap.Error(err))
		return &grpchealth.CheckResponse{Status: grpchealth.StatusNotServing}, nil
	}
	return &grpchealth.CheckResponse{Status: grpchealth.StatusServing}, nil
}
