package ingress

import (
	wv1 "github.com/wafieio/wafie/api/gen/wafie/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type vs struct{}

func (s *vs) gvr() schema.GroupVersionResource {
	return schema.GroupVersionResource{
		Group:    "networking.istio.io",
		Version:  "v1beta1",
		Resource: "virtualservices",
	}
}
func (s *vs) normalize(obj *unstructured.Unstructured) (*wv1.CreateRouteRequest, error) {
	return nil, nil
}
