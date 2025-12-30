package apiserver

import (
	"context"

	"connectrpc.com/connect"
	wv1 "github.com/wafieio/wafie/api/gen/wafie/v1"
	v1 "github.com/wafieio/wafie/api/gen/wafie/v1/wafiev1connect"
	"github.com/wafieio/wafie/apisrv/internal/models"
	"go.uber.org/zap"
)

type EventService struct {
	v1.UnimplementedEventServiceHandler
	logger *zap.Logger
}

func NewEventService(log *zap.Logger) *EventService {
	return &EventService{
		logger: log,
	}
}

func (s *EventService) ListEvents(
	ctx context.Context,
	req *connect.Request[wv1.ListEventsRequest]) (
	*connect.Response[wv1.ListEventsResponse], error) {
	s.logger.Info("listing events",
		zap.Uint32p("protectionId", req.Msg.ProtectionId),
		zap.Uint32p("limit", req.Msg.Limit),
		zap.Uint32p("offset", req.Msg.Offset),
	)
	defer s.logger.Info("events listed")

	repo := models.NewEventRepository(nil, s.logger)
	events, total, err := repo.ListEvents(req.Msg)
	if err != nil {
		s.logger.Error("failed to list events", zap.Error(err))
		return connect.NewResponse(&wv1.ListEventsResponse{}), err
	}

	var protoEvents []*wv1.Event
	for _, event := range events {
		protoEvents = append(protoEvents, event.ToProto())
	}

	return connect.NewResponse(&wv1.ListEventsResponse{
		Events: protoEvents,
		Total:  total,
	}), nil
}

func (s *EventService) GetEventStats(
	ctx context.Context,
	req *connect.Request[wv1.GetEventStatsRequest]) (
	*connect.Response[wv1.GetEventStatsResponse], error) {
	s.logger.Info("getting event stats",
		zap.Uint32p("protectionId", req.Msg.ProtectionId),
	)
	defer s.logger.Info("event stats retrieved")

	repo := models.NewEventRepository(nil, s.logger)
	stats, err := repo.GetEventStats(req.Msg)
	if err != nil {
		s.logger.Error("failed to get event stats", zap.Error(err))
		return connect.NewResponse(&wv1.GetEventStatsResponse{}), err
	}

	return connect.NewResponse(&wv1.GetEventStatsResponse{
		Stats: stats,
	}), nil
}
