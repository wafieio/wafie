package controlplane

import (
	"fmt"
	mutation_rulesv3 "github.com/envoyproxy/go-control-plane/envoy/config/common/mutation_rules/v3"
	"time"

	wv1 "github.com/Dimss/wafie/api/gen/wafie/v1"
	applogger "github.com/Dimss/wafie/logger"
	accesslog "github.com/envoyproxy/go-control-plane/envoy/config/accesslog/v3"
	cluster "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	core "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	endpoint "github.com/envoyproxy/go-control-plane/envoy/config/endpoint/v3"
	v3listener "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
	route "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"
	stream "github.com/envoyproxy/go-control-plane/envoy/extensions/access_loggers/stream/v3"
	extproc "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/ext_proc/v3"
	router "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/router/v3"
	hcm "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/network/http_connection_manager/v3"
	upstreams "github.com/envoyproxy/go-control-plane/envoy/extensions/upstreams/http/v3"
	"github.com/envoyproxy/go-control-plane/pkg/cache/types"
	"github.com/envoyproxy/go-control-plane/pkg/resource/v3"
	"github.com/envoyproxy/go-control-plane/pkg/wellknown"
	"github.com/golang/protobuf/ptypes/wrappers"
	"go.uber.org/zap"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

const wafieExtProcClusterName = "wafie_xproc_cluster"

type state struct {
	xprocSocket string
	logger      *zap.Logger
}

func newState(xprocSocket string) *state {
	return &state{
		xprocSocket: xprocSocket,
		logger:      applogger.NewLogger(),
	}
}

func (s *state) wafieXprocFilter() *extproc.ExternalProcessor {
	return &extproc.ExternalProcessor{
		GrpcService: &core.GrpcService{
			TargetSpecifier: &core.GrpcService_EnvoyGrpc_{
				EnvoyGrpc: &core.GrpcService_EnvoyGrpc{
					ClusterName: wafieExtProcClusterName,
				},
			},
		},
		FailureModeAllow: false,
		ProcessingMode: &extproc.ProcessingMode{
			RequestHeaderMode:  extproc.ProcessingMode_SEND,
			ResponseHeaderMode: extproc.ProcessingMode_SEND,
			RequestBodyMode:    extproc.ProcessingMode_BUFFERED,
			ResponseBodyMode:   extproc.ProcessingMode_NONE,
		},
		RequestAttributes: []string{
			"request.protocol",
			"request.method",
			"request.path",
			"source.address",
		},
		MutationRules: &mutation_rulesv3.HeaderMutationRules{
			AllowAllRouting: &wrappers.BoolValue{Value: true},
			AllowEnvoy:      &wrappers.BoolValue{Value: true},
		},
	}
}

func (s *state) wafieXprocCluster() *cluster.Cluster {
	// Explicitly enforce HTTP/2 configuration
	httpProtocolOptions, err := anypb.New(
		&upstreams.HttpProtocolOptions{
			UpstreamProtocolOptions: &upstreams.HttpProtocolOptions_ExplicitHttpConfig_{
				ExplicitHttpConfig: &upstreams.HttpProtocolOptions_ExplicitHttpConfig{
					ProtocolConfig: &upstreams.HttpProtocolOptions_ExplicitHttpConfig_Http2ProtocolOptions{},
				},
			},
		},
	)
	if err != nil {
		s.logger.Error("error defining a cluster for wafie xproc filter", zap.Error(err))
		return nil
	}
	return &cluster.Cluster{
		Name:                 wafieExtProcClusterName,
		ConnectTimeout:       durationpb.New(5 * time.Second),
		ClusterDiscoveryType: &cluster.Cluster_Type{Type: cluster.Cluster_STATIC},
		TypedExtensionProtocolOptions: map[string]*anypb.Any{
			"envoy.extensions.upstreams.http.v3.HttpProtocolOptions": httpProtocolOptions,
		},
		LoadAssignment: &endpoint.ClusterLoadAssignment{
			ClusterName: wafieExtProcClusterName,
			Endpoints: []*endpoint.LocalityLbEndpoints{
				{
					LbEndpoints: []*endpoint.LbEndpoint{
						{
							HostIdentifier: &endpoint.LbEndpoint_Endpoint{
								Endpoint: &endpoint.Endpoint{
									Address: &core.Address{
										Address: &core.Address_Pipe{
											Pipe: &core.Pipe{
												Path: s.xprocSocket,
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
}

func (s *state) httpFilters(protection *wv1.Protection) []*hcm.HttpFilter {
	var filters []*hcm.HttpFilter
	// wafie modsec filter
	if protection.DesiredState.ModeSec.ProtectionMode == wv1.ProtectionMode_PROTECTION_MODE_ON {
		if wafieExtProcFilter, err := anypb.New(s.wafieXprocFilter()); err == nil {
			filters = append(filters, &hcm.HttpFilter{
				Name: "envoy.filters.http.ext_proc",
				ConfigType: &hcm.HttpFilter_TypedConfig{
					TypedConfig: wafieExtProcFilter,
				},
			})
		} else {
			s.logger.Error("failed to create wafie ext proc filter", zap.Error(err))
		}
	}
	// http filter
	routerConfig, err := anypb.New(&router.Router{})
	if err != nil {
		s.logger.Error("failed to create router config", zap.Error(err))
	}
	filters = append(filters, &hcm.HttpFilter{
		Name: wellknown.Router,
		ConfigType: &hcm.HttpFilter_TypedConfig{
			TypedConfig: routerConfig,
		},
	})
	return filters
}

func (s *state) mirroredClusterName(appName string) string {
	return fmt.Sprintf("%s-mirrored", appName)
}

func (s *state) routes(protection *wv1.Protection) []*route.Route {
	routes := make([]*route.Route, 0)
	routeAction := &route.RouteAction{
		Timeout: durationpb.New(0 * time.Second), // zero meaning disabled
		ClusterSpecifier: &route.RouteAction_Cluster{
			Cluster: protection.Application.Name,
		},
		HostRewriteSpecifier: &route.RouteAction_AutoHostRewrite{
			AutoHostRewrite: &wrapperspb.BoolValue{Value: true},
		},
	}

	mirrorPolicy := protection.Application.Ingress[0].Upstream.MirrorPolicy
	if mirrorPolicy != nil && mirrorPolicy.Status == wv1.MirrorPolicyStatus_MIRROR_POLICY_STATUS_ENABLED {
		routeAction.RequestMirrorPolicies = append(
			routeAction.RequestMirrorPolicies,
			&route.RouteAction_RequestMirrorPolicy{
				Cluster: s.mirroredClusterName(protection.Application.Name),
			},
		)
	}

	return append(routes, &route.Route{
		Name: protection.Application.Name,
		Match: &route.RouteMatch{
			PathSpecifier: &route.RouteMatch_Prefix{
				Prefix: "/",
			},
		},
		Action: &route.Route_Route{
			Route: routeAction,
		},
	})
}

func (s *state) httpConnectionManager(protection *wv1.Protection) *hcm.HttpConnectionManager {
	stdoutLogs, _ := anypb.New(&stream.StdoutAccessLog{})
	return &hcm.HttpConnectionManager{
		CodecType:  hcm.HttpConnectionManager_AUTO,
		StatPrefix: "http",
		GenerateRequestId: &wrappers.BoolValue{
			Value: true,
		},
		AccessLog: []*accesslog.AccessLog{
			{
				Name: "envoy.access_loggers.stdout",
				ConfigType: &accesslog.AccessLog_TypedConfig{
					TypedConfig: stdoutLogs,
				},
			},
		},
		HttpFilters: s.httpFilters(protection),
		UpgradeConfigs: []*hcm.HttpConnectionManager_UpgradeConfig{
			{
				UpgradeType: "websocket",
			},
		},
		RouteSpecifier: &hcm.HttpConnectionManager_RouteConfig{
			RouteConfig: &route.RouteConfiguration{
				Name: "local_route",
				VirtualHosts: []*route.VirtualHost{
					{
						Name: protection.Application.Name,
						// TODO: when route by virtual host, real app domain should be used, i.e protection.Application.Ingress[0].Host
						Domains: []string{"*"},
						Routes:  s.routes(protection),
					},
				},
			},
		},
	}
}

func (s *state) listeners(protections []*wv1.Protection) []types.Resource {
	var listeners = make([]types.Resource, len(protections))
	for i := 0; i < len(protections); i++ {
		httpConnectionMgr, _ := anypb.New(s.httpConnectionManager(protections[i]))
		port, err := protectionContainerPort(protections[i])
		if err != nil {
			s.logger.Error("unable detect proxy listening port", zap.Error(err))
			continue
		}
		listeners[i] = &v3listener.Listener{
			Name: fmt.Sprintf("listener-%d", i),
			Address: &core.Address{
				Address: &core.Address_SocketAddress{
					SocketAddress: &core.SocketAddress{
						Protocol: core.SocketAddress_TCP,
						Address:  "0.0.0.0",
						PortSpecifier: &core.SocketAddress_PortValue{
							PortValue: port.ProxyListeningPort,
						},
					},
				},
			},
			FilterChains: []*v3listener.FilterChain{
				{
					Filters: []*v3listener.Filter{
						{
							Name: wellknown.HTTPConnectionManager,
							ConfigType: &v3listener.Filter_TypedConfig{
								TypedConfig: httpConnectionMgr,
							},
						},
					},
				},
			}}
	}
	return listeners
}

func (s *state) lbEndpoint(ip string, port uint32) *endpoint.LbEndpoint {
	return &endpoint.LbEndpoint{
		HostIdentifier: &endpoint.LbEndpoint_Endpoint{
			Endpoint: &endpoint.Endpoint{
				Address: &core.Address{
					Address: &core.Address_SocketAddress{
						SocketAddress: &core.SocketAddress{
							Protocol: core.SocketAddress_TCP,
							Address:  ip,
							PortSpecifier: &core.SocketAddress_PortValue{
								PortValue: port,
							},
						},
					},
				},
			},
		},
	}
}

func (s *state) cluster(name string, lbep []*endpoint.LbEndpoint, t cluster.Cluster_DiscoveryType) *cluster.Cluster {
	return &cluster.Cluster{
		Name:                 name,
		ClusterDiscoveryType: &cluster.Cluster_Type{Type: t},
		ConnectTimeout:       durationpb.New(20 * time.Second),
		LbPolicy:             cluster.Cluster_ROUND_ROBIN,
		DnsLookupFamily:      cluster.Cluster_V4_ONLY,
		LoadAssignment: &endpoint.ClusterLoadAssignment{
			ClusterName: name,
			Endpoints:   []*endpoint.LocalityLbEndpoints{{LbEndpoints: lbep}},
		},
	}
}

func (s *state) containerPortsEndpoints(
	upstreamEp []*wv1.Endpoint, upstreamPorts []*wv1.Port) (lbEndpoints []*endpoint.LbEndpoint) {
	for _, uep := range upstreamEp {
		for _, port := range upstreamPorts {
			if port.PortType == wv1.PortType_PORT_TYPE_CONTAINER_PORT {
				lbEndpoints = append(lbEndpoints, s.lbEndpoint(uep.Ip, port.Number))
			}
		}
	}
	return lbEndpoints
}

func (s *state) clusters(protections []*wv1.Protection) (clusters []types.Resource) {
	clusters = make([]types.Resource, 0, len(protections))
	for _, protection := range protections {
		// if protection disabled, skip it
		if shouldSkipProtection(protection) {
			continue
		}
		// first generate container port endpoints, a.k.a routing by dedicated listener port
		lbEndpoints := s.containerPortsEndpoints(
			protection.Application.Ingress[0].Upstream.Endpoints,
			protection.Application.Ingress[0].Upstream.Ports,
		)
		clusters = append(clusters, s.cluster(protection.Application.Name, lbEndpoints, cluster.Cluster_STATIC))
		// check for mirror policy, if enabled create a mirroring cluster
		mirrorPolicy := protection.Application.Ingress[0].Upstream.MirrorPolicy
		if mirrorPolicy != nil && mirrorPolicy.Status == wv1.MirrorPolicyStatus_MIRROR_POLICY_STATUS_ENABLED {
			address := mirrorPolicy.Ip
			// always use dns if set
			if mirrorPolicy.Dns != "" {
				address = mirrorPolicy.Dns
			}
			// mirrored endpoints
			mirroEndpoints := []*endpoint.LbEndpoint{s.lbEndpoint(address, mirrorPolicy.Port)}
			// mirrored cluster
			mirroredCluster := s.cluster(s.mirroredClusterName(protection.Application.Name),
				mirroEndpoints,
				cluster.Cluster_STRICT_DNS,
			)
			clusters = append(clusters, mirroredCluster)
		}
	}
	// add wafie ext proc cluster
	clusters = append(clusters, s.wafieXprocCluster())
	return clusters
}

func (s *state) buildResources(protections []*wv1.Protection) map[resource.Type][]types.Resource {
	return map[resource.Type][]types.Resource{
		resource.ListenerType: s.listeners(protections),
		resource.ClusterType:  s.clusters(protections),
	}
}
