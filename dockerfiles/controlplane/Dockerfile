FROM bufbuild/buf AS protobuf-builder
WORKDIR /app
ADD ../api ./api
ADD ../Makefile ./
RUN cd api \
    && buf dep update \
    && buf lint \
    && buf generate

FROM golang:1.25.4-bookworm AS builder
WORKDIR /app
COPY ../../logger ./logger
COPY ../../go.mod go.sum ./
COPY ../../Makefile ./
COPY --from=protobuf-builder /app/api ./api
COPY ../../apisrv ./apisrv
COPY ../../discovery ./discovery
RUN make api discovery

FROM debian:bookworm-slim
COPY --from=builder /app/.bin/discovery-agent /usr/local/bin/discovery-agent
COPY --from=builder /app/.bin/api-server /usr/local/bin/api-server


