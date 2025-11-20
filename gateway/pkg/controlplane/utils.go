package controlplane

import (
	"fmt"

	wv1 "github.com/Dimss/wafie/api/gen/wafie/v1"
)

func shouldSkipProtection(protection *wv1.Protection) bool {
	if protection.Application == nil {
		return true // skip if no application is associated
	}
	if len(protection.Application.Ingress) == 0 {
		return true // skip if no ingress is associated
	}
	return false
}

func protectionContainerPort(protection *wv1.Protection) (*wv1.Port, error) {
	for _, port := range protection.Application.Ingress[0].Upstream.Ports {
		if port.PortType == wv1.PortType_PORT_TYPE_CONTAINER_PORT {
			return port, nil
		}
	}
	return nil, fmt.Errorf("protectoin [%d] does not have container ports", protection.Id)
}
