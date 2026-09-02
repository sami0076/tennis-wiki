SHELL := /bin/sh
GO ?= go
BIN := bin

# Tool versions are pinned so CI and every developer machine agree.
GOLANGCI_VERSION := v2.13.2
GOOSE_VERSION    := v3.28.0
SQLC_VERSION     := v1.31.1

MIGRATIONS := ./migrations
DATABASE_URL ?= postgres://tennis:tennis@localhost:5432/tennis?sslmode=disable

.DEFAULT_GOAL := help

## help: list available targets
help:
	@grep -E '^## ' $(MAKEFILE_LIST) | sed 's/## //' | awk -F':' '{printf "  \033[36m%-16s\033[0m %s\n", $$1, $$2}'

## build: compile every binary into bin/
build:
	$(GO) build -o $(BIN)/ ./cmd/...

## test: run the full test suite with the race detector
test:
	$(GO) test -race ./...

## fmt: format and tidy
fmt:
	$(GO) fmt ./...
	$(GO) mod tidy

## lint: run golangci-lint
lint:
	$(GO) run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_VERSION) run

## migrate-up: apply all migrations
migrate-up:
	$(GO) run github.com/pressly/goose/v3/cmd/goose@$(GOOSE_VERSION) -dir $(MIGRATIONS) postgres "$(DATABASE_URL)" up

## migrate-down: roll back one migration
migrate-down:
	$(GO) run github.com/pressly/goose/v3/cmd/goose@$(GOOSE_VERSION) -dir $(MIGRATIONS) postgres "$(DATABASE_URL)" down

## sqlc: regenerate typed queries from migrations
sqlc:
	$(GO) run github.com/sqlc-dev/sqlc/cmd/sqlc@$(SQLC_VERSION) generate

## ingest: load the small seed fixture (fast, for local development)
ingest:
	$(GO) run ./cmd/ingest --local-path ./testdata

## ingest-full: load the complete dataset from the configured sources
ingest-full:
	$(GO) run ./cmd/ingest

## dataqual: report data-quality anomalies
dataqual:
	$(GO) run ./cmd/dataqual

## validate: report the rating-engine validation checks
validate:
	$(GO) run ./cmd/validate

## clean: remove build artefacts
clean:
	rm -rf $(BIN)

.PHONY: help build test fmt lint migrate-up migrate-down sqlc ingest ingest-full dataqual validate clean
