package ingress

import (
	wv1 "github.com/wafieio/wafie/api/gen/wafie/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type route struct{}

func (r *route) gvr() schema.GroupVersionResource {
	return schema.GroupVersionResource{
		Group:    "route.openshift.io",
		Version:  "v1",
		Resource: "routes",
	}
}
func (r *route) normalize(obj *unstructured.Unstructured) (*wv1.CreateRouteRequest, error) {
	return nil, nil
}
