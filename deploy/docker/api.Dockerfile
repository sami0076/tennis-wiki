# Multi-stage so the shipped image is the binary and its certificates, nothing
# else: no toolchain, no source, no module cache.
#
# The compiler runs on the builder's own architecture and cross-compiles for
# the target. A Go build has no reason to run under emulation, and on an Arm
# machine building the amd64 image CI publishes, that is the whole difference.
FROM --platform=$BUILDPLATFORM golang:1.23-alpine AS build

WORKDIR /src

# Dependencies first, so a source-only change does not re-download them.
COPY go.mod go.sum ./
RUN go mod download

COPY . .
ARG TARGETARCH
RUN CGO_ENABLED=0 GOOS=linux GOARCH=$TARGETARCH \
    go build -trimpath -ldflags="-s -w" -o /out/api ./cmd/api

FROM alpine:3.21

# The API talks to Postgres over TLS in any deployment that is not this
# compose file, and needs the root certificates to verify it.
RUN apk add --no-cache ca-certificates && adduser -D -u 10001 api

COPY --from=build /out/api /usr/local/bin/api

USER api
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/api"]
