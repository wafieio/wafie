SHELL := /usr/bin/env bash

shell:
	@$(RUN) /bin/bash

.PHONY: api
api:
	go build \
      -ldflags="-X 'github.com/wafieio/wafie/apisrv/cmd/apiserver/cmd.Build=$$(git rev-parse --short HEAD)'" \
      -o .bin/api-server apisrv/cmd/apiserver/main.go

api.image:
	podman buildx build -t docker.io/dimssss/wafie-control-plane --platform linux/arm64 -f dockerfiles/controlplane/Dockerfile .
	podman push docker.io/dimssss/wafie-control-plane


# gateway build
.PHONY: gateway
gateway:
	go build \
		-ldflags="-X 'github.com/wafieio/wafie/gateway/cmd.Build=$$(git rev-parse --short HEAD)'" \
		-o .bin/gateway gateway/cmd/main.go

gateway.image:
	podman buildx build --build-arg ARCH=arm64 -t docker.io/dimssss/wafie-gateway --platform linux/arm64 -f gateway/Dockerfile .
	podman push docker.io/dimssss/wafie-gateway

gateway.image.dev:
	podman buildx build --build-arg ARCH=arm64 -t docker.io/dimssss/wafie-gateway-dev --platform linux/arm64 -f gateway/Dockerfile.dev .
	podman push docker.io/dimssss/wafie-gateway-dev

.PHONY: xproc
xproc:
	go build -o .bin/xproc xproc/cmd/main.go

xproc.image:
	podman buildx build --build-arg ARCH=arm64 -t docker.io/dimssss/wafie-xproc --platform linux/arm64 -f xproc/Containerfile .
	podman push docker.io/dimssss/wafie-xproc


.PHONY: discovery
discovery:
	go build \
      -ldflags="-X 'github.com/wafieio/wafie/discovery/cmd/discovery/cmd.Build=$$(git rev-parse --short HEAD)'" \
      -o .bin/discovery-agent discovery/cmd/discovery/main.go

# Relay build
.PHONY: relay
relay:
	go build -o .bin/wafie-relay relay/cmd/main.go

relay.image:
	podman buildx build -t docker.io/dimssss/wafie-relay --platform linux/arm64 -f relay/Containerfile .
	podman push docker.io/dimssss/wafie-relay

build.httpfilter.debug:
	go build \
	  -buildmode=c-shared \
	  -gcflags="all=-N -l" \
	  -ldflags="-compressdwarf=false -X main.debugMode=true" \
	  -tags="debug" \
	  -race \
	  -o ./wafie-modsec.so \
	  ./modsecfilter

helm:
	helm package chart
	scp wafie-0.0.1.tgz root@charts.wafie.io:/var/www/charts
	rm wafie-0.0.1.tgz
	ssh root@charts.wafie.io "helm repo index /var/www/charts"

.PHONY: proto
proto:
	cd api \
	&& buf dep update \
	&& buf export buf.build/googleapis/googleapis --output vendor \
	&& buf export buf.build/bufbuild/protovalidate --output vendor \
	&& buf lint \
	&& buf generate