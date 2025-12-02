package apiserver

import (
	"context"

	"connectrpc.com/connect"
	wv1 "github.com/wafieio/wafie/api/gen/wafie/v1"
	v1 "github.com/wafieio/wafie/api/gen/wafie/v1/wafiev1connect"
	"github.com/wafieio/wafie/apisrv/internal/models"
	"go.uber.org/zap"
)

type ProtectionService struct {
	v1.UnimplementedApplicationServiceHandler
	logger *zap.Logger
}

func NewProtectionService(log *zap.Logger) *ProtectionService {
	return &ProtectionService{
		logger: log,
	}
}

func (s *ProtectionService) CreateProtection(
	ctx context.Context,
	req *connect.Request[wv1.CreateProtectionRequest]) (
	*connect.Response[wv1.CreateProtectionResponse], error) {
	l := s.logger.With(zap.Uint32("applicationId", req.Msg.ApplicationId))
	l.Info("creating new protection entry")
	defer l.Info("protection entry created")
	repo := models.NewProtectionRepository(nil, l)
	protection, err := repo.CreateProtection(req.Msg)
	if err != nil {
		l.Error("failed to create protection entry", zap.Error(err))
		return connect.NewResponse(&wv1.CreateProtectionResponse{}), connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&wv1.CreateProtectionResponse{
		Protection: protection.ToProto(),
	}), nil
}

func (s *ProtectionService) GetProtection(
	ctx context.Context,
	req *connect.Request[wv1.GetProtectionRequest]) (
	*connect.Response[wv1.GetProtectionResponse], error) {
	l := s.logger.With(zap.Uint32("protectionId", req.Msg.Id))
	l.Info("getting protection entry")
	defer l.Info("protection entry retrieved")
	repo := models.NewProtectionRepository(nil, l)
	protection, err := repo.GetProtection(uint(req.Msg.GetId()), req.Msg.GetOptions())
	if err != nil {
		l.Error("failed to get protection entry", zap.Error(err))
		return connect.NewResponse(&wv1.GetProtectionResponse{}), err
	}
	return connect.NewResponse(&wv1.GetProtectionResponse{
		Protection: protection.ToProto(),
	}), nil
}

func (s *ProtectionService) PutProtection(
	ctx context.Context,
	req *connect.Request[wv1.PutProtectionRequest]) (
	*connect.Response[wv1.PutProtectionResponse], error) {
	l := s.logger.With(zap.Uint32("protectionId", req.Msg.Id))
	l.Info("updating protection entry")
	defer l.Info("protection entry updated")
	repo := models.NewProtectionRepository(nil, l)
	protection, err := repo.UpdateProtection(req.Msg)
	if err != nil {
		return connect.NewResponse(&wv1.PutProtectionResponse{}), err
	}
	return connect.NewResponse(&wv1.PutProtectionResponse{
		Protection: protection.ToProto(),
	}), nil
}

func (s *ProtectionService) ListProtections(
	ctx context.Context,
	req *connect.Request[wv1.ListProtectionsRequest]) (
	*connect.Response[wv1.ListProtectionsResponse], error) {
	s.logger.Info("listing protections")
	defer s.logger.Info("protections listed")
	repo := models.NewProtectionRepository(nil, s.logger)
	protections, err := repo.ListProtections(req.Msg.Options)
	if err != nil {
		s.logger.Error("failed to list protections", zap.Error(err))
		return connect.NewResponse(&wv1.ListProtectionsResponse{}), err
	}
	var cwafv1Protections []*wv1.Protection
	for _, protection := range protections {
		cwafv1Protections = append(cwafv1Protections, protection.ToProto())
	}
	return connect.NewResponse(&wv1.ListProtectionsResponse{
		Protections: cwafv1Protections,
	}), nil
}

func (s *ProtectionService) DeleteProtection(
	ctx context.Context,
	req *connect.Request[wv1.DeleteProtectionRequest]) (
	*connect.Response[wv1.DeleteProtectionResponse], error) {
	l := s.logger.With(zap.Uint32("protectionId", req.Msg.Id))
	l.Info("deleting protection entry")
	defer l.Info("protection entry deleted")
	protectionModelSvc := models.NewProtectionRepository(nil, l)
	err := protectionModelSvc.DeleteProtection(req.Msg.Id)
	if err != nil {
		l.Error("failed to delete protection entry", zap.Error(err))
		return connect.NewResponse(&wv1.DeleteProtectionResponse{}), err
	}
	return connect.NewResponse(&wv1.DeleteProtectionResponse{}), nil
}
