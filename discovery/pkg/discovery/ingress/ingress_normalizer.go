package ingress

import (
	"fmt"

	wv1 "github.com/wafieio/wafie/api/gen/wafie/v1"
	applogger "github.com/wafieio/wafie/logger"
	"go.uber.org/zap"
	v1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/intstr"
)

type ingress struct {
	logger *zap.Logger
}

func newIngress() *ingress {
	return &ingress{
		logger: applogger.NewLogger().With(zap.String("type", "ingressNormalizer")),
	}
}

func (i *ingress) gvr() schema.GroupVersionResource {
	return schema.GroupVersionResource{
		Group:    "networking.k8s.io",
		Version:  "v1",
		Resource: "ingresses",
	}
}

func (i *ingress) normalizedWithError(req *wv1.CreateRouteRequest, err error) (*wv1.CreateRouteRequest, error) {
	req.Ingress.DiscoveryStatus = wv1.DiscoveryStatusType_DISCOVERY_STATUS_TYPE_INCOMPLETE
	req.Ingress.DiscoveryMessage = err.Error()
	return req, err
}

func (i *ingress) normalize(obj *unstructured.Unstructured) (createRouteReq *wv1.CreateRouteRequest, err error) {
	createRouteReq = &wv1.CreateRouteRequest{
		Upstream: &wv1.Upstream{},
		Ingress:  &wv1.Ingress{},
		Ports:    []*wv1.Port{},
	}
	k8sIngress := &v1.Ingress{}
	if err := runtime.
		DefaultUnstructuredConverter.
		FromUnstructured(obj.Object, k8sIngress); err != nil {
		return i.normalizedWithError(createRouteReq, err)
	}
	if len(k8sIngress.Spec.Rules) > 0 && len(k8sIngress.Spec.Rules[0].HTTP.Paths) > 0 {
		//TODO: check what will happen when the host will be empty, i.e ingress with wildcard scenario
		if k8sIngress.Spec.Rules[0].Host == "" {
			i.logger.Info("skipping ingress due to wildcard '*' hostname",
				zap.String("ingress", k8sIngress.Name+"."+k8sIngress.Namespace))
			return nil, nil
		}
		// set upstream ingress
		createRouteReq.Ingress = &wv1.Ingress{
			Name:            k8sIngress.Name,
			Namespace:       k8sIngress.Namespace,
			Port:            80, // TODO: add support for TLS passthroughs and other protocols later on
			Path:            k8sIngress.Spec.Rules[0].HTTP.Paths[0].Path,
			Host:            k8sIngress.Spec.Rules[0].Host,
			Scheme:          i.discoverScheme(k8sIngress),
			IngressType:     wv1.IngressType_INGRESS_TYPE_NGINX,
			DiscoveryStatus: wv1.DiscoveryStatusType_DISCOVERY_STATUS_TYPE_SUCCESS,
		}
		// set upstream service fqdn
		createRouteReq.Upstream.SvcFqdn = fmt.Sprintf("%s.%s.svc",
			k8sIngress.Spec.Rules[0].HTTP.Paths[0].Backend.Service.Name,
			k8sIngress.Namespace)
		// set upstream services ports
		if err := i.discoverSvcPorts(k8sIngress, &createRouteReq.Ports); err != nil {
			return i.normalizedWithError(createRouteReq, err)
		}
		// set upstream containers port
		if err := i.discoverContainerPorts(k8sIngress, &createRouteReq.Ports); err != nil {
			return i.normalizedWithError(createRouteReq, err)
		}
		// set upstream container IPs
		if err := discoverEndpoints(
			k8sIngress.Spec.Rules[0].HTTP.Paths[0].Backend.Service.Name,
			k8sIngress.Namespace,
			&createRouteReq.Upstream.Endpoints,
		); err != nil {
			return i.normalizedWithError(createRouteReq, err)
		}
		return createRouteReq, nil
	}
	return nil, nil
}

// discoverSvcPorts is in use when envoy making routing by virtual host
func (i *ingress) discoverSvcPorts(ing *v1.Ingress, ports *[]*wv1.Port) error {
	//
	if ing.Spec.Rules[0].HTTP.Paths[0].Backend.Service.Port.Number != 0 {
		*ports = append(*ports, &wv1.Port{
			Number:   uint32(ing.Spec.Rules[0].HTTP.Paths[0].Backend.Service.Port.Number),
			Name:     ing.Spec.Rules[0].HTTP.Paths[0].Backend.Service.Port.Name,
			Status:   wv1.PortStatusType_PORT_STATUS_TYPE_ENABLED,
			PortType: wv1.PortType_PORT_TYPE_SVC_PORT,
		})
		return nil
	}
	// get service port number by service port name
	if port, err := getSvcPortNumberBySvcPortName(
		ing.Spec.Rules[0].HTTP.Paths[0].Backend.Service.Port.Name,
		ing.Spec.Rules[0].HTTP.Paths[0].Backend.Service.Name,
		ing.Namespace,
	); err != nil {
		return err
	} else {
		*ports = append(*ports, &wv1.Port{
			Number:   uint32(port),
			Name:     ing.Spec.Rules[0].HTTP.Paths[0].Backend.Service.Port.Name,
			Status:   wv1.PortStatusType_PORT_STATUS_TYPE_ENABLED,
			PortType: wv1.PortType_PORT_TYPE_SVC_PORT,
		})
	}
	return nil
}

// discoverContainerPorts in use when envoy making routing by listeners port
func (i *ingress) discoverContainerPorts(ing *v1.Ingress, ports *[]*wv1.Port) error {
	if portNumber, portName, err := getContainerPortBySvcPort(
		intstr.IntOrString{
			IntVal: ing.Spec.Rules[0].HTTP.Paths[0].Backend.Service.Port.Number,
			StrVal: ing.Spec.Rules[0].HTTP.Paths[0].Backend.Service.Port.Name,
		},
		ing.Spec.Rules[0].HTTP.Paths[0].Backend.Service.Name,
		ing.Namespace,
	); err != nil {
		return err
	} else {
		*ports = append(*ports, &wv1.Port{
			Number:   uint32(portNumber),
			Name:     portName,
			Status:   wv1.PortStatusType_PORT_STATUS_TYPE_ENABLED,
			PortType: wv1.PortType_PORT_TYPE_CONTAINER_PORT,
		})
	}
	return nil
}

func (i *ingress) discoverScheme(ing *v1.Ingress) string {
	if ing.Spec.TLS != nil {
		return "https"
	}
	return "http"
}
