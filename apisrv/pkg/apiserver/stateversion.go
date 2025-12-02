package apiserver

import (
	"context"

	"connectrpc.com/connect"
	wv1 "github.com/wafieio/wafie/api/gen/wafie/v1"
	"github.com/wafieio/wafie/api/gen/wafie/v1/wafiev1connect"
	"github.com/wafieio/wafie/apisrv/internal/models"
	"go.uber.org/zap"
)

type StateVersionService struct {
	wafiev1connect.UnimplementedStateVersionServiceHandler
	logger *zap.Logger
}

func NewStateVersionService(log *zap.Logger) *StateVersionService {
	return &StateVersionService{
		logger: log,
	}
}

func (s *StateVersionService) GetStateVersion(
	ctx context.Context,
	req *connect.Request[wv1.GetStateVersionRequest]) (
	*connect.Response[wv1.GetStateVersionResponse], error) {
	version, err := models.
		NewStateRepository(nil, s.logger).
		GetVersionByTypeId(uint32(req.Msg.TypeId))
	if err != nil {
		s.logger.Error("error getting protection version", zap.Error(err))
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(version.ToProto()), nil
}
