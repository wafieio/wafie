package apiserver

import (
	"context"

	"connectrpc.com/connect"

	"buf.build/go/protovalidate"
	wv1 "github.com/wafieio/wafie/api/gen/wafie/v1"
	"github.com/wafieio/wafie/api/gen/wafie/v1/wafiev1connect"
	"github.com/wafieio/wafie/apisrv/internal/models"
	"go.uber.org/zap"
)

type RouteService struct {
	wafiev1connect.UnimplementedRouteServiceHandler
	logger *zap.Logger
}

func NewRouteService(log *zap.Logger) *RouteService {
	return &RouteService{
		logger: log,
	}
}

func (s *RouteService) CreateRoute(
	ctx context.Context,
	req *connect.Request[wv1.CreateRouteRequest]) (
	*connect.Response[wv1.CreateRouteResponse], error) {
	s.logger.Debug("create route", zap.String("upstream", req.Msg.Upstream.SvcFqdn))
	if err := protovalidate.Validate(req.Msg); err != nil {
		return connect.NewResponse(&wv1.CreateRouteResponse{}), connect.NewError(connect.CodeInternal, err)
	}
	// save upstream
	u, err := models.NewUpstreamRepository(nil, s.logger).
		Save(models.NewUpstreamFromRequest(req.Msg.Upstream))
	if err != nil {
		return connect.NewResponse(&wv1.CreateRouteResponse{}), connect.NewError(connect.CodeInternal, err)
	}
	// save ingress
	i := models.NewIngressFromProto(req.Msg.Ingress)
	i.UpstreamID = u.ID // set foreign key upstream id
	if err := models.NewIngressRepository(nil, s.logger).Save(i); err != nil {
		return connect.NewResponse(&wv1.CreateRouteResponse{}), err
	}
	// save ports
	err = models.NewPortRepository(u.ID, i.ID, nil, s.logger).
		Save(models.NewPortsFromProto(req.Msg.Ports))
	return connect.NewResponse(&wv1.CreateRouteResponse{}), err
}

func (s *RouteService) UpdateRoute(
	ctx context.Context,
	req *connect.Request[wv1.UpdateRouteRequest]) (
	*connect.Response[wv1.UpdateRouteResponse], error) {
	s.logger.Debug("update route", zap.String("upstream", req.Msg.Upstream.SvcFqdn))
	if err := protovalidate.Validate(req.Msg); err != nil {
		return connect.NewResponse(&wv1.UpdateRouteResponse{}), connect.NewError(connect.CodeInternal, err)
	}
	_, err := models.NewUpstreamRepository(nil, s.logger).
		Save(models.NewUpstreamFromRequest(req.Msg.Upstream))
	if err != nil {
		return connect.NewResponse(&wv1.UpdateRouteResponse{}), connect.NewError(connect.CodeInternal, err)
	}
	//models.NewPortRepository(nil, s.logger)
	return connect.NewResponse(&wv1.UpdateRouteResponse{}), nil
}

func (s *RouteService) ListRoutes(
	ctx context.Context,
	req *connect.Request[wv1.ListRoutesRequest]) (
	*connect.Response[wv1.ListRoutesResponse], error) {
	if err := protovalidate.Validate(req.Msg); err != nil {
		return connect.NewResponse(&wv1.ListRoutesResponse{}), connect.NewError(connect.CodeInternal, err)
	}
	upstreams, err := models.
		NewUpstreamRepository(nil, s.logger).
		List(req.Msg.Options)
	if err != nil {
		return connect.NewResponse(&wv1.ListRoutesResponse{}), connect.NewError(connect.CodeInternal, err)
	}
	var upstreamList = make([]*wv1.Upstream, len(upstreams))
	for i, upstream := range upstreams {
		upstreamList[i] = upstream.ToProto()
	}
	return connect.NewResponse(&wv1.ListRoutesResponse{Upstreams: upstreamList}), nil
}
