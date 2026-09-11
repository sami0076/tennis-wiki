# Everything a cluster needs that the API deliberately is not: the schema, the
# ingest, the rating rebuild and the two report commands.
#
# One image rather than five. They are one module and one compile pass, they
# share almost all of their compiled code, and five images would be five copies
# of the same layers in the registry for no isolation anyone benefits from. A
# Job picks a binary by command, not by image.
FROM golang:1.23-alpine AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

# goose is a third-party binary, not one of ours, so it is pinned on its own.
# The Makefile holds the version and passes it; the build stops here rather
# than quietly resolving something else.
#
# GOTOOLCHAIN=auto for this step alone: goose asks for a newer Go than go.mod
# declares, and the base image is pinned to go.mod deliberately. Our own
# binaries below still compile with the toolchain the module names. CI's
# migrations job sets the same variable for the same reason.
#
# The build tags drop the drivers goose ships for databases this project does
# not have. Unstripped and with all of them, the one binary was 57 MB, more
# than the four of ours put together.
ARG GOOSE_VERSION
RUN : "${GOOSE_VERSION:?run make images, or pass --build-arg GOOSE_VERSION}" \
    && CGO_ENABLED=0 GOOS=linux GOTOOLCHAIN=auto \
       go install -trimpath -ldflags="-s -w" \
       -tags='no_clickhouse no_libsql no_mssql no_mysql no_sqlite3 no_vertica no_ydb' \
       github.com/pressly/goose/v3/cmd/goose@${GOOSE_VERSION}

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/ \
    ./cmd/ingest ./cmd/rate ./cmd/dataqual ./cmd/validate

FROM alpine:3.21

# The ingest reads the mirrors over HTTPS, and every command here talks to
# Postgres over TLS in any deployment that is not the compose file.
RUN apk add --no-cache ca-certificates && adduser -D -u 10001 tools

COPY --from=build /go/bin/goose /usr/local/bin/goose
COPY --from=build /out/ /usr/local/bin/

# cmd/ingest resolves configs/sources.json and configs/player_overrides.json
# relative to the working directory, and goose is pointed at ./migrations.
WORKDIR /app
COPY migrations ./migrations
COPY configs ./configs

USER tools

# No entrypoint: there is no one thing this image does. Failing loudly beats a
# Job that forgot its command exiting zero and reading as a success.
CMD ["sh", "-c", "echo 'give this image a command: goose, ingest, rate, dataqual or validate' >&2; exit 64"]
