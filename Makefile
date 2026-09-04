SHELL := /bin/sh
GO ?= go
BIN := bin

# Tool versions are pinned so CI and every developer machine agree.
GOLANGCI_VERSION := v2.13.2
GOOSE_VERSION    := v3.28.0
SQLC_VERSION     := v1.31.1

ifeq ($(OS),Windows_NT)
RM_DIR = cmd //c "if exist $(1) rmdir /s /q $(1)"
else
RM_DIR = rm -rf $(1)
endif

POSTGRES_USER ?= tennis
POSTGRES_DB   ?= tennis

MIGRATIONS := ./migrations
# Matches the published compose port; 5432 is usually already taken.
DATABASE_URL ?= postgres://tennis:tennis@localhost:5433/tennis?sslmode=disable
# Exported so the cmd/ binaries see it; they read DATABASE_URL from the
# environment and every target below would otherwise need it passed by hand.
export DATABASE_URL

.DEFAULT_GOAL := help

## help: list available targets
help:
	@grep -E '^## ' $(MAKEFILE_LIST) | sed 's/## //' | awk -F':' '{printf "  \033[36m%-16s\033[0m %s\n", $$1, $$2}'

## up: start Postgres and Redis, wait until healthy
up:
	docker compose up -d --wait

## down: stop the stack, keeping data
down:
	docker compose down

## reset: stop the stack and delete all data
reset:
	docker compose down -v

## psql: open a shell on the database
psql:
	docker compose exec postgres psql -U $(POSTGRES_USER) -d $(POSTGRES_DB)

## build: compile every binary into bin/
build:
	$(GO) build -o $(BIN)/ ./cmd/...

TEST_DATABASE_URL ?= postgres://tennis:tennis@localhost:5433/tennis_test?sslmode=disable
export TEST_DATABASE_URL

## testdb: create the disposable database the integration tests truncate
testdb:
	-docker compose exec -T postgres psql -U $(POSTGRES_USER) -d $(POSTGRES_DB) -c "CREATE DATABASE tennis_test"
	$(MAKE) migrate-test

## migrate-test: apply migrations to the test database
# Split out of testdb because CI gets its database from a service container and
# so needs the migration step without the compose call above.
migrate-test:
	$(GO) run github.com/pressly/goose/v3/cmd/goose@$(GOOSE_VERSION) -dir $(MIGRATIONS) postgres "$(TEST_DATABASE_URL)" up

## test: run the test suite
test:
	$(GO) test ./...

## test-race: run the test suite with the race detector (used by CI)
# The race detector is unsupported on windows/arm64, so this is a separate
# target rather than the default. CI runs on linux/amd64 and always uses it.
test-race:
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

## migrate-reset: roll every migration back, dropping every table
migrate-reset:
	$(GO) run github.com/pressly/goose/v3/cmd/goose@$(GOOSE_VERSION) -dir $(MIGRATIONS) postgres "$(DATABASE_URL)" down-to 0

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
	$(call RM_DIR,$(BIN))

.PHONY: help up down reset psql testdb migrate-test build test test-race fmt lint migrate-up migrate-down migrate-reset sqlc ingest ingest-full dataqual validate clean

# print-VAR: echo a make variable, so CI can read the pinned tool versions
# from here rather than duplicating them in a workflow file.
print-%:
	@echo $($*)
