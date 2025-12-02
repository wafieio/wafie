package ingress

import (
	"context"
	"fmt"

	wv1 "github.com/wafieio/wafie/api/gen/wafie/v1"
	v1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
)

func getSvc(svcName, namespace string) (*v1.Service, error) {
	rc, err := config.GetConfig()
	if err != nil {
		return nil, err
	}
	clientset, err := kubernetes.NewForConfig(rc)
	if err != nil {
		return nil, err
	}
	return clientset.CoreV1().
		Services(namespace).
		Get(context.TODO(),
			svcName,
			metav1.GetOptions{},
		)
}

func getSvcPortNumberBySvcPortName(portName, svcName, namespace string) (int32, error) {
	service, err := getSvc(svcName, namespace)
	if err != nil {
		return 0, err
	}
	for _, p := range service.Spec.Ports {
		if p.Name == portName {
			return p.Port, nil
		}
	}
	return 0, nil
}

func getContainerPortBySvcPort(kPort intstr.IntOrString, svcName, namespace string) (portNumber int32, portName string, err error) {
	service, err := getSvc(svcName, namespace)
	if err != nil {
		return 0, "", err
	}
	for _, p := range service.Spec.Ports {
		// equal either by port name or port number and target port number is set
		if (p.Name == kPort.StrVal || p.Port == kPort.IntVal) && p.TargetPort.IntVal != 0 {
			return p.TargetPort.IntVal, "", nil
		}
		// equal either by port name or port number and target port name is set
		// in that case further discovery required with endpoints slices
		if (p.Name == kPort.StrVal || p.Port == kPort.IntVal) && p.TargetPort.StrVal != "" {
			endpointSlice, err := getEndpointSliceBySvcName(svcName, namespace)
			if err != nil {
				return 0, "", err
			}
			for _, port := range endpointSlice.Ports {
				if *port.Port == kPort.IntVal || *port.Name == kPort.StrVal {
					return *port.Port, *port.Name, nil
				}
			}
		}
	}
	return 0, "", fmt.Errorf("can not find container port for service: %s", svcName)
}

func discoverEndpoints(svcName, namespace string, endpoints *[]*wv1.Endpoint) error {
	eps, err := getEndpointSliceBySvcName(svcName, namespace)
	if err != nil {
		return err
	}
	*endpoints = make([]*wv1.Endpoint, len(eps.Endpoints))
	for idx, ep := range eps.Endpoints {
		if len(ep.Addresses) == 0 {
			continue
		}
		(*endpoints)[idx] = &wv1.Endpoint{
			Ip:        ep.Addresses[0], // not sure yet what to do when CNI allocates more than one ip to container
			NodeName:  *ep.NodeName,
			Kind:      ep.TargetRef.Kind,
			Name:      ep.TargetRef.Name,
			Namespace: ep.TargetRef.Namespace,
		}
	}
	return nil
}

func getEndpointSliceBySvcName(svcName, namespace string) (*discoveryv1.EndpointSlice, error) {
	rc, err := config.GetConfig()
	if err != nil {
		return nil, err
	}
	clientset, err := kubernetes.NewForConfig(rc)
	if err != nil {
		return nil, err
	}
	labelSelector := fmt.Sprintf("kubernetes.io/service-name=%s", svcName)
	endpoints, err := clientset.DiscoveryV1().EndpointSlices(namespace).List(
		context.Background(),
		metav1.ListOptions{
			LabelSelector: labelSelector,
		},
	)
	if err != nil {
		return nil, err
	}
	if len(endpoints.Items) == 0 {
		return nil, fmt.Errorf("no endpointslice found for service %s", svcName)
	}
	return &endpoints.Items[0], nil
}
