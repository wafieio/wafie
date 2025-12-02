package apiserver

import (
	"connectrpc.com/connect"
	cwafv1 "github.com/wafieio/wafie/api/gen/wafie/v1"
	v1 "github.com/wafieio/wafie/api/gen/wafie/v1/wafiev1connect"
	"github.com/wafieio/wafie/apisrv/internal/models"
	"go.uber.org/zap"
	"golang.org/x/net/context"
)

type ApplicationService struct {
	v1.UnimplementedApplicationServiceHandler
	logger *zap.Logger
}

func NewApplicationService(log *zap.Logger) *ApplicationService {
	return &ApplicationService{
		logger: log,
	}
}

func (s *ApplicationService) CreateApplication(
	ctx context.Context, req *connect.Request[cwafv1.CreateApplicationRequest]) (
	*connect.Response[cwafv1.CreateApplicationResponse], error) {
	s.logger.With(
		zap.String("name", req.Msg.Name)).
		Info("creating new application entry")
	defer s.logger.Info("application entry created")
	applicationModelSvc := models.NewApplicationRepository(nil, s.logger)
	if app, err := applicationModelSvc.CreateApplication(req.Msg); err != nil {
		// ToDo: verify if the application already exists
		return connect.NewResponse(&cwafv1.CreateApplicationResponse{}), err
	} else {
		return connect.NewResponse(&cwafv1.CreateApplicationResponse{Id: uint32(app.ID)}), nil
	}
}

func (s *ApplicationService) GetApplication(
	ctx context.Context, req *connect.Request[cwafv1.GetApplicationRequest]) (
	*connect.Response[cwafv1.GetApplicationResponse], error) {
	s.logger.With(
		zap.Uint32("id", req.Msg.GetId())).
		Info("getting application entry")
	applicationModelSvc := models.NewApplicationRepository(nil, s.logger)
	app, err := applicationModelSvc.GetApplication(req.Msg)
	if err != nil {
		return connect.NewResponse(&cwafv1.GetApplicationResponse{}), err
	}
	return connect.NewResponse(&cwafv1.GetApplicationResponse{
		Application: app.ToProto(),
	}), err
}

func (s *ApplicationService) ListApplications(
	ctx context.Context, req *connect.Request[cwafv1.ListApplicationsRequest]) (
	*connect.Response[cwafv1.ListApplicationsResponse], error) {
	s.logger.Info("start applications listing")
	defer s.logger.Info("end applications listing")
	appRepository := models.NewApplicationRepository(nil, s.logger)
	apps, err := appRepository.ListApplications(req.Msg.Options)
	if err != nil {
		return nil, err
	}
	var cwafv1Apps []*cwafv1.Application
	for _, app := range apps {
		cwafv1Apps = append(cwafv1Apps, app.ToProto())
	}
	return connect.NewResponse(&cwafv1.ListApplicationsResponse{Applications: cwafv1Apps}), nil
}

func (s *ApplicationService) PutApplication(
	ctx context.Context, req *connect.Request[cwafv1.PutApplicationRequest]) (
	*connect.Response[cwafv1.PutApplicationResponse], error) {
	var app *models.Application
	var err error
	applicationModelSvc := models.NewApplicationRepository(nil, s.logger)
	if app, err = applicationModelSvc.UpdateApplication(req.Msg.Application); err != nil {
		return connect.NewResponse(&cwafv1.PutApplicationResponse{}), err
	}
	return connect.NewResponse(&cwafv1.PutApplicationResponse{
		Application: app.ToProto(),
	}), nil
}
