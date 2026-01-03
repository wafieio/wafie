package apiserver

import (
	"connectrpc.com/connect"
	"context"
	"encoding/base64"
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

func (s *CrsService) CreateRule(ctx context.Context, req *connect.Request[wv1.CreateRuleRequest]) (
	*connect.Response[wv1.CreateRuleResponse], error) {
	repo := models.NewCrsRepository(nil, s.logger)
	var decodedRules = make([]string, len(req.Msg.Rule.Base64Rule))
	for idx, base64Rule := range req.Msg.Rule.Base64Rule {
		// decode base64 rule to string
		decodedRule, err := base64.StdEncoding.DecodeString(base64Rule)
		if err != nil {
			s.logger.Error("failed to decode base64 rule", zap.Error(err))
			return connect.NewResponse(&wv1.CreateRuleResponse{}), connect.NewError(connect.CodeInvalidArgument, err)
		}
		decodedRules[idx] = string(decodedRule)
	}
	if err := repo.CreateRule(
		uint(req.Msg.Rule.RuleSetId),
		decodedRules,
		req.Msg.Rule.ApplicationName,
	); err != nil {
		return connect.NewResponse(&wv1.CreateRuleResponse{}), connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&wv1.CreateRuleResponse{}), nil
}
