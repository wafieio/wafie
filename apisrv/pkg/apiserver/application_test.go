package apiserver

import (
	"connectrpc.com/connect"
	"context"
	"github.com/stretchr/testify/assert"
	cwafv1 "github.com/wafieio/wafie/api/gen/wafie/v1"
	applogger "github.com/wafieio/wafie/logger"
	"testing"
)

func TestCreateApplication(t *testing.T) {
	svc := NewApplicationService(applogger.NewLogger())
	req := connect.NewRequest(&cwafv1.CreateApplicationRequest{
		Name: randomString(),
	})
	_, err := svc.CreateApplication(context.Background(), req)
	assert.Nil(t, err)
}

func TestGetApplication(t *testing.T) {
	svc := NewApplicationService(applogger.NewLogger())
	req := connect.NewRequest(&cwafv1.CreateApplicationRequest{
		Name: randomString(),
	})
	createAppResp, err := svc.CreateApplication(context.Background(), req)
	assert.Nil(t, err)
	getReq := connect.NewRequest(&cwafv1.GetApplicationRequest{Id: createAppResp.Msg.Id})
	getAppResp, err := svc.GetApplication(context.Background(), getReq)
	assert.Nil(t, err)
	assert.Equal(t, req.Msg.Name, getAppResp.Msg.Application.Name)
}
