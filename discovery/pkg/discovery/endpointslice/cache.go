package endpointslice

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"connectrpc.com/connect"
	wv1 "github.com/wafieio/wafie/api/gen/wafie/v1"
	v1 "github.com/wafieio/wafie/api/gen/wafie/v1/wafiev1connect"
	"go.uber.org/zap"
	discoveryv1 "k8s.io/api/discovery/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
)

type Cache struct {
	EpsCh                 chan *discoveryv1.EndpointSlice
	svcFqdnCacheUpdaterCh chan []string
	svcFqdnCache          []string
	routeSvcClient        v1.RouteServiceClient
	mu                    sync.RWMutex
	logger                *zap.Logger
}

func NewCache(apiAddr string, logger *zap.Logger) *Cache {
	return &Cache{
		EpsCh:                 make(chan *discoveryv1.EndpointSlice, 5),
		svcFqdnCacheUpdaterCh: make(chan []string, 5),
		routeSvcClient: v1.NewRouteServiceClient(
			http.DefaultClient,
			apiAddr,
		),
		logger: logger,
	}
}

func (c *Cache) Run() {
	// start svc fqdn cacher
	c.runUpstreamSvcFqdnCache()
	// start upstream api syncer
	c.RunUpstreamIPsSyncer()
	// start endpointslice K8s informer
	c.RunInformer()
}

func (c *Cache) RunUpstreamIPsSyncer() {
	go func() {
		for {
			select {
			case eps := <-c.EpsCh:
				svcFqdn := fmt.Sprintf("%s.%s.svc", eps.Labels["kubernetes.io/service-name"], eps.Namespace)
				if !c.shouldProtect(svcFqdn) {
					continue
				}
				//setContainerIpsOnly := true
				req := &wv1.UpdateRouteRequest{
					Upstream: &wv1.Upstream{
						SvcFqdn:   svcFqdn,
						Endpoints: c.endpoints(eps),
					},
				}
				if _, err := c.routeSvcClient.UpdateRoute(
					context.Background(),
					connect.NewRequest(req)); err != nil {
					c.logger.Info("update route failed", zap.Error(err))
				}
				c.logger.Info("endpoints were updated", zap.String("svcFqdn", svcFqdn))

			case c.svcFqdnCache = <-c.svcFqdnCacheUpdaterCh:
				c.logger.Info("svc fqdn got updated")
			}
		}
	}()
}

func (c *Cache) endpoints(eps *discoveryv1.EndpointSlice) (endpoints []*wv1.Endpoint) {
	endpoints = make([]*wv1.Endpoint, len(eps.Endpoints))
	for idx, ep := range eps.Endpoints {
		if len(ep.Addresses) == 0 {
			continue
		}
		endpoints[idx] = &wv1.Endpoint{
			Ip:        ep.Addresses[0], // not sure yet what to do when CNI allocates more than one ip to container
			NodeName:  *ep.NodeName,
			Kind:      ep.TargetRef.Kind,
			Name:      ep.TargetRef.Name,
			Namespace: ep.TargetRef.Namespace,
		}
	}
	return endpoints
}

func (c *Cache) runUpstreamSvcFqdnCache() {
	go func() {
		for {
			req := connect.NewRequest(&wv1.ListRoutesRequest{})
			upstreams, err := c.routeSvcClient.ListRoutes(context.Background(), req)
			if err != nil {
				c.logger.Error("error listing upstreams", zap.Error(err))
			} else {
				uc := make([]string, len(upstreams.Msg.Upstreams))
				for i, u := range upstreams.Msg.Upstreams {
					uc[i] = u.SvcFqdn
				}
				// update the cache
				c.svcFqdnCacheUpdaterCh <- uc
			}
			time.Sleep(10 * time.Second)
		}
	}()
}

func (c *Cache) shouldProtect(svcName string) bool {
	for _, cachedSvcName := range c.svcFqdnCache {
		if cachedSvcName == svcName {
			return true
		}
	}
	return false
}

func (c *Cache) RunInformer() {
	go func() {
		var informerStartError error
		c.logger.Info("starting endpoints slice informer")
		for {
			if informerStartError != nil {
				c.logger.Error("informer start error", zap.Error(informerStartError))
				c.logger.Info("restarting informer after error...")
				informerStartError = nil
				time.Sleep(3 * time.Second)
			}
			rc, err := config.GetConfig()
			if err != nil {
				informerStartError = err
				continue
			}
			clientset, err := kubernetes.NewForConfig(rc)
			if err != nil {
				informerStartError = err
				continue
			}
			ctx, cancel := context.WithCancel(context.Background())
			informerFactory := informers.NewSharedInformerFactory(clientset, 30*time.Second)
			endpointSliceInformer := informerFactory.Discovery().V1().EndpointSlices()
			_, informerStartError = endpointSliceInformer.
				Informer().
				AddEventHandler(
					cache.ResourceEventHandlerFuncs{
						AddFunc: func(obj interface{}) {
							c.EpsCh <- obj.(*discoveryv1.EndpointSlice)
						},
						UpdateFunc: func(oldObj, newObj interface{}) {
							c.EpsCh <- newObj.(*discoveryv1.EndpointSlice)
						},
						DeleteFunc: func(obj interface{}) {
							// TODO: implement delete logic
							eps := obj.(*discoveryv1.EndpointSlice)
							serviceName := eps.Labels["kubernetes.io/service-name"]
							fmt.Printf("EndpointSlice deleted for service %s\n", serviceName)
						},
					},
				)
			// make sure handlers successfully added
			if informerStartError != nil {
				cancel()
				continue
			}
			// Start informer
			informerFactory.Start(ctx.Done())

			// Wait for cache sync
			if !cache.WaitForCacheSync(ctx.Done(), endpointSliceInformer.Informer().HasSynced) {
				fmt.Println("Failed to sync cache")
				cancel()
				continue
			}

			fmt.Println("endpointslice informer running...")
			<-ctx.Done()
		}
	}()
}
