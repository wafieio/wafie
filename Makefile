SHELL := /usr/bin/env bash

shell:
	@$(RUN) /bin/bash

# Control plane build
build.cp:
	go build \
      -ldflags="-X 'github.com/Dimss/wafie/apisrv/cmd/apiserver/cmd.Build=$$(git rev-parse --short HEAD)'" \
      -o .bin/api-server apisrv/cmd/apiserver/main.go

build.cp.image:
	podman buildx build -t docker.io/dimssss/wafie-control-plane --platform linux/arm64 -f dockerfiles/controlplane/Dockerfile .
	podman push docker.io/dimssss/wafie-control-plane


# AppSecGw build
build.appsecgw:
	go build \
		-ldflags="-X 'github.com/Dimss/wafie/appsecgw/cmd.Build=$$(git rev-parse --short HEAD)'" \
		-o .bin/appsecgw appsecgw/cmd/main.go

build.appsecgw.image:
	podman buildx build --build-arg ARCH=arm64 -t docker.io/dimssss/wafie-appsecgw --platform linux/arm64 -f appsecgw/Dockerfile .
	podman push docker.io/dimssss/wafie-appsecgw

build.appsecgw.image.dev:
	podman buildx build --build-arg ARCH=arm64 -t docker.io/dimssss/wafie-appsecgw-dev --platform linux/arm64 -f appsecgw/Dockerfile.dev .
	podman push docker.io/dimssss/wafie-appsecgw-dev

xproc.image:
	podman buildx build --build-arg ARCH=arm64 -t docker.io/dimssss/wafie-xproc --platform linux/arm64 -f xproc/Containerfile .
	podman push docker.io/dimssss/wafie-xproc


# Discovery build
build.discovery:
	go build \
      -ldflags="-X 'github.com/Dimss/wafie/discovery/cmd/discovery/cmd.Build=$$(git rev-parse --short HEAD)'" \
      -o .bin/discovery-agent discovery/cmd/discovery/main.go

# Relay build
build.relay:
	go build -o .bin/wafie-relay relay/cmd/main.go

build.relay.image:
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