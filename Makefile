SHELL := /usr/bin/env bash
REGISTRY ?= docker.io/wafieio
shell:
	@$(RUN) /bin/bash

.PHONY: api
api:
	go build \
      -ldflags="-X 'github.com/wafieio/wafie/apisrv/cmd/apiserver/cmd.Build=$$(git rev-parse --short HEAD)'" \
      -o .bin/api-server apisrv/cmd/apiserver/main.go

api.image:
	podman buildx build -t $(REGISTRY)/api --platform linux/arm64 -f apisrv/Containerfile .
	podman push $(REGISTRY)/api

# gateway build
.PHONY: gateway
gateway:
	go build \
		-ldflags="-X 'github.com/wafieio/wafie/gateway/cmd.Build=$$(git rev-parse --short HEAD)'" \
		-o .bin/gateway gateway/cmd/main.go

gateway.image:
	podman buildx build --build-arg ARCH=arm64 -t $(REGISTRY)/gateway --platform linux/arm64 -f gateway/Containerfile .
	podman push $(REGISTRY)/gateway

.PHONY: xproc
xproc:
	go build -o .bin/xproc xproc/cmd/main.go

xproc.image:
	podman buildx build --build-arg ARCH=arm64 -t $(REGISTRY)/xproc --platform linux/arm64 -f xproc/Containerfile .
	podman push $(REGISTRY)/xproc


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
	podman buildx build -t $(REGISTRY)/relay --platform linux/arm64 -f relay/Containerfile .
	podman push $(REGISTRY)/relay


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