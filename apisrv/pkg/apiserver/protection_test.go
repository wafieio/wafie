package apiserver

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	wafiev1 "github.com/wafieio/wafie/api/gen/wafie/v1"
	applogger "github.com/wafieio/wafie/logger"
)

func createProtectionDependencies(t *testing.T) (appId uint32) {
	// create new application
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
	// create new ingress
	ingSvc := NewIngressService(applogger.NewLogger())
	_, err = ingSvc.CreateIngress(context.Background(),
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
	//create new protection
	_ = &wafiev1.CreateProtectionRequest{
		ApplicationId:  app.Msg.Id,
		ProtectionMode: wafiev1.ProtectionMode_PROTECTION_MODE_OFF,
		DesiredState: &wafiev1.ProtectionDesiredState{
			ModeSec: &wafiev1.ModSec{
				ProtectionMode: wafiev1.ProtectionMode_PROTECTION_MODE_OFF,
				ParanoiaLevel:  wafiev1.ParanoiaLevel_PARANOIA_LEVEL_4,
			},
		},
	}
	return app.Msg.Id
}
