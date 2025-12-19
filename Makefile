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
	podman build \
      --manifest api \
      --platform linux/arm64,linux/amd64 \
      -f apisrv/Containerfile .
	podman manifest push api $(REGISTRY)/api:latest

# gateway build
.PHONY: gateway
gateway:
	go build \
		-ldflags="-X 'github.com/wafieio/wafie/gateway/cmd.Build=$$(git rev-parse --short HEAD)'" \
		-o .bin/gateway gateway/cmd/main.go

gateway.image:
	podman build \
        --manifest gateway \
        --platform linux/arm64,linux/amd64 \
        -f gateway/Containerfile .
	podman manifest push gateway $(REGISTRY)/gateway:latest

.PHONY: xproc
xproc:
	go build -o .bin/xproc xproc/cmd/main.go

xproc.image:
	podman build \
        --manifest xproc \
        --platform linux/arm64,linux/amd64 \
        -f xproc/Containerfile .
	podman manifest push xproc $(REGISTRY)/xproc:latest


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
	podman build \
      --manifest relay \
      --secret id=buf_auth_token,src=./api/buf_auth_token \
      --platform linux/arm64,linux/amd64 \
      -f relay/Containerfile .
	podman manifest push relay $(REGISTRY)/relay:latest


helm:
	helm package chart
	charts_pod=$$(kubectl get pod -lapp=nginx -ncharts --kubeconfig ~/.kube/wafie-staging -o jsonpath="{.items[0].metadata.name}"); \
	kubectl cp wafie-0.0.1.tgz $$charts_pod:/usr/share/nginx/html -c helm -ncharts --kubeconfig ~/.kube/wafie-staging; \
	kubectl exec $$charts_pod -c helm -ncharts --kubeconfig ~/.kube/wafie-staging -- /bin/bash -c "cd /usr/share/nginx/html && helm repo index ."
	rm wafie-0.0.1.tgz

.PHONY: proto
proto:
	cd api \
	&& buf dep update \
	&& buf export buf.build/googleapis/googleapis --output vendor \
	&& buf export buf.build/bufbuild/protovalidate --output vendor \
	&& buf lint \
	&& buf generate