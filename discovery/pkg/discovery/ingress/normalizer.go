package ingress

import (
	wv1 "github.com/wafieio/wafie/api/gen/wafie/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

const (
	VsIngressType    IngressType = "istio"
	K8sIngressType   IngressType = "ingress"
	RouteIngressType IngressType = "openshift"
)

type normalizer interface {
	gvr() schema.GroupVersionResource
	normalize(*unstructured.Unstructured) (*wv1.CreateRouteRequest, error)
}
