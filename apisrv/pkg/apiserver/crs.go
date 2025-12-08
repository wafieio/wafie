package apiserver

import (
	"connectrpc.com/connect"
	"context"
	wv1 "github.com/wafieio/wafie/api/gen/wafie/v1"
	v1 "github.com/wafieio/wafie/api/gen/wafie/v1/wafiev1connect"
	"github.com/wafieio/wafie/apisrv/internal/models"
	"go.uber.org/zap"
)

type CrsService struct {
	v1.UnimplementedCrsServiceHandler
	logger *zap.Logger
}

func NewCrsService(logger *zap.Logger) *CrsService {
	return &CrsService{
		logger: logger,
	}
}

func (s *CrsService) CreateCrsVersion(ctx context.Context,
	req *connect.Request[wv1.CreateCrsVersionRequest]) (
	*connect.Response[wv1.CreateCrsVersionResponse], error) {
	repo := models.NewCrsRepository(nil, s.logger)
	v := &models.CrsVersion{}
	v.FromProto(req.Msg.Version)
	err := repo.CreateCrsVersion(v)
	if err != nil {
		s.logger.Error(err.Error())
		return connect.NewResponse(&wv1.CreateCrsVersionResponse{}), connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&wv1.CreateCrsVersionResponse{Version: v.ToProto()}), nil
}

func (s *CrsService) CreateCrsRuleSet(ctx context.Context,
	req *connect.Request[wv1.CreateCrsRuleSetRequest]) (
	*connect.Response[wv1.CreateCrsRuleSetResponse], error) {
	repo := models.NewCrsRepository(nil, s.logger)
	if err := repo.CloneCrsProfileToCrsRuleSet(req.Msg.ProfileName, uint(req.Msg.CrsVersionId)); err != nil {
		s.logger.Error(err.Error())
		return connect.NewResponse(&wv1.CreateCrsRuleSetResponse{}), connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&wv1.CreateCrsRuleSetResponse{}), nil
}
