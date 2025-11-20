package controlplane

import (
	"context"
	"fmt"
	"math/rand"
	"net"
	"net/http"
	"time"

	"connectrpc.com/connect"
	wafiev1 "github.com/Dimss/wafie/api/gen/wafie/v1"
	"github.com/Dimss/wafie/api/gen/wafie/v1/wafiev1connect"
	applogger "github.com/Dimss/wafie/logger"
	clusterservice "github.com/envoyproxy/go-control-plane/envoy/service/cluster/v3"
	discoverygrpc "github.com/envoyproxy/go-control-plane/envoy/service/discovery/v3"
	endpointservice "github.com/envoyproxy/go-control-plane/envoy/service/endpoint/v3"
	listenerservice "github.com/envoyproxy/go-control-plane/envoy/service/listener/v3"
	routeservice "github.com/envoyproxy/go-control-plane/envoy/service/route/v3"
	runtimeservice "github.com/envoyproxy/go-control-plane/envoy/service/runtime/v3"
	secretservice "github.com/envoyproxy/go-control-plane/envoy/service/secret/v3"
	"github.com/envoyproxy/go-control-plane/pkg/cache/types"
	"github.com/envoyproxy/go-control-plane/pkg/cache/v3"
	"github.com/envoyproxy/go-control-plane/pkg/resource/v3"
	"github.com/envoyproxy/go-control-plane/pkg/server/v3"
	"github.com/envoyproxy/go-control-plane/pkg/test/v3"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/keepalive"
)

type EnvoyControlPlane struct {
	state                 *state
	cache                 cache.SnapshotCache
	logger                *zap.Logger
	resourcesCh           chan map[resource.Type][]types.Resource
	stateVersion          string
	namespace             string
	protectionSvcClient   wafiev1connect.ProtectionServiceClient
	stateVersionSvcClient wafiev1connect.StateVersionServiceClient
}

func NewEnvoyControlPlane(apiAddr, namespace, xprocSocket string) *EnvoyControlPlane {

	cp := &EnvoyControlPlane{
		state:       newState(xprocSocket),
		logger:      applogger.NewLogger(),
		resourcesCh: make(chan map[resource.Type][]types.Resource, 1),
		namespace:   namespace,
		cache: cache.NewSnapshotCache(
			false, cache.IDHash{}, applogger.NewLogger().Sugar(),
		),
		protectionSvcClient: wafiev1connect.NewProtectionServiceClient(
			http.DefaultClient, apiAddr,
		),
		stateVersionSvcClient: wafiev1connect.NewStateVersionServiceClient(
			http.DefaultClient, apiAddr,
		),
	}
	// start control plane data watcher
	cp.startApiIngressWatcher()
	// start envoy snapshot generator
	cp.startSnapshotGenerator()
	return cp
}

func (p *EnvoyControlPlane) Start() {
	envoySrv := server.NewServer(context.Background(), p.cache, &test.Callbacks{})
	grpcSrv := grpc.NewServer([]grpc.ServerOption{
		grpc.KeepaliveParams(keepalive.ServerParameters{
			MaxConnectionIdle:     5 * time.Minute,
			MaxConnectionAge:      30 * time.Minute,
			MaxConnectionAgeGrace: 5 * time.Minute,
			Time:                  2 * time.Hour,
			Timeout:               20 * time.Second,
		}),
		grpc.KeepaliveEnforcementPolicy(keepalive.EnforcementPolicy{
			MinTime:             5 * time.Second,
			PermitWithoutStream: false,
		})}...,
	)
	// register the gRPC services
	discoverygrpc.RegisterAggregatedDiscoveryServiceServer(grpcSrv, envoySrv)
	endpointservice.RegisterEndpointDiscoveryServiceServer(grpcSrv, envoySrv)
	clusterservice.RegisterClusterDiscoveryServiceServer(grpcSrv, envoySrv)
	routeservice.RegisterRouteDiscoveryServiceServer(grpcSrv, envoySrv)
	listenerservice.RegisterListenerDiscoveryServiceServer(grpcSrv, envoySrv)
	secretservice.RegisterSecretDiscoveryServiceServer(grpcSrv, envoySrv)
	runtimeservice.RegisterRuntimeDiscoveryServiceServer(grpcSrv, envoySrv)
	lis, err := net.Listen("tcp", fmt.Sprintf("%s:%d", "0.0.0.0", 18000))
	if err != nil {
		p.logger.Error("failed to listen", zap.Error(err))
	}
	p.logger.Info("Envoy control plane listening started", zap.String("address", lis.Addr().String()))
	if err = grpcSrv.Serve(lis); err != nil {
		zap.S().Fatal(err)
	}
}

func (p *EnvoyControlPlane) stateVersionChanged() bool {
	stateVersionResponse, err := p.stateVersionSvcClient.GetStateVersion(
		context.Background(),
		connect.NewRequest(
			&wafiev1.GetStateVersionRequest{
				TypeId: wafiev1.StateTypeId_STATE_TYPE_ID_PROTECTION,
			},
		),
	)
	if err != nil {
		p.logger.Error("failed to get protection state version", zap.Error(err))
		return false
	}
	// check if the protection state has changed since last iteration
	if stateVersionResponse.Msg.StateVersionId == p.stateVersion {
		return false
	}
	p.logger.Info("protection state version has changed",
		zap.String("versionId", stateVersionResponse.Msg.StateVersionId))
	p.stateVersion = stateVersionResponse.Msg.StateVersionId
	return true
}

func (p *EnvoyControlPlane) startApiIngressWatcher() {
	p.logger.Info("starting api ingress watcher")
	go func() {
		for {
			time.Sleep(1 * time.Second)
			if !p.stateVersionChanged() {
				continue
			}
			mode := wafiev1.ProtectionMode_PROTECTION_MODE_ON
			includeApps := true
			req := connect.NewRequest(&wafiev1.ListProtectionsRequest{
				Options: &wafiev1.ListProtectionsOptions{
					ProtectionMode: &mode,
					IncludeApps:    &includeApps,
				},
			})
			listProtectionResp, err := p.protectionSvcClient.ListProtections(context.Background(), req)
			if err != nil {
				p.logger.Error("failed to list protections", zap.Error(err))
				continue
			}
			p.logger.Info("data version has changed, building new resources")
			p.resourcesCh <- p.state.buildResources(listProtectionResp.Msg.Protections)

		}
	}()
}

func (p *EnvoyControlPlane) startSnapshotGenerator() {
	p.logger.Info("starting envoy snapshot generator")
	go func() {
		for resources := range p.resourcesCh {
			p.logger.Info("state has been changed, generating new snapshot...")
			snap, _ := cache.NewSnapshot(fmt.Sprintf("%d", rand.Int()), resources)
			if err := snap.Consistent(); err != nil {
				p.logger.Error("snapshot inconsistency", zap.Error(err))
				continue
			}
			if err := p.cache.SetSnapshot(context.Background(), "node-1", snap); err != nil {
				p.logger.Error("failed to set snapshot", zap.Error(err))
				continue
			}
		}
	}()
}
