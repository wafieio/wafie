package main

/*
#cgo LDFLAGS: -lwafie
#include <stdlib.h>
#include <wafie/wafielib.h>
*/
import "C"
import (
	"github.com/envoyproxy/envoy/contrib/golang/common/go/api"
	"github.com/envoyproxy/envoy/contrib/golang/filters/http/source/go/pkg/http"
	applogger "github.com/wafieio/wafie/logger"
	"google.golang.org/protobuf/types/known/anypb"
)

func init() {
	C.wafie_library_init(C.CString("/config"))
	c := config{}
	http.RegisterHttpFilterFactoryAndConfigParser("wafie", wafieFilterFactory, c)

}

type config struct {
}

func (c config) Parse(any *anypb.Any, callbacks api.ConfigCallbackHandler) (interface{}, error) {
	return nil, nil
}

func (c config) Merge(parentConfig interface{}, childConfig interface{}) interface{} {
	return nil
}

func wafieFilterFactory(config interface{}, callbacks api.FilterCallbackHandler) api.StreamFilter {
	return &filter{
		callbacks: callbacks,
		logger:    applogger.NewLogger(),
	}
}

func main() {
	// wafie ModSecurity Envoy HTTP filter
	// compiled as a shared object (.so) for use with Envoy.
	// depends on the wafie (wafie.so) library and wafie/wafielib.h files
	// to build: go build -ldflags='-s -w' -o ./wafie-modsec.so -buildmode=c-shared ./cmd/modsecfilter
}
