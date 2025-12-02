package apiserver

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	wafiev1 "github.com/wafieio/wafie/api/gen/wafie/v1"
	applogger "github.com/wafieio/wafie/logger"
)

func TestCreateIngressWithNoneExistingApp(t *testing.T) {
	svc := NewIngressService(applogger.NewLogger())
	req := connect.NewRequest(
		&wafiev1.CreateIngressRequest{
			Ingress: &wafiev1.Ingress{
				Name:         randomString(),
				Host:         randomString(),
				Port:         80,
				Path:         "",
				UpstreamHost: randomString(),
				UpstreamPort: 90,
			},
		},
	)
	_, err := svc.CreateIngress(context.Background(), req)
	assert.Nil(t, err)
}

func TestCreateIngressWithExistingApp(t *testing.T) {
	appSvc := NewApplicationService(applogger.NewLogger())
	app, err := appSvc.CreateApplication(
		context.Background(),
		connect.NewRequest(
			&wafiev1.CreateApplicationRequest{
				Name: randomString(),
			},
		),
	)
	assert.Nil(t, err)
	svc := NewIngressService(applogger.NewLogger())
	_, err = svc.CreateIngress(context.Background(),
		connect.NewRequest(
			&wafiev1.CreateIngressRequest{
				Ingress: &wafiev1.Ingress{
					Name:          randomString(),
					Host:          randomString(),
					Port:          80,
					Path:          "",
					UpstreamHost:  randomString(),
					UpstreamPort:  90,
					ApplicationId: int32(app.Msg.Id),
				},
			},
		),
	)

	assert.Nil(t, err)

}
