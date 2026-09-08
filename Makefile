SHELL := /bin/sh
GO ?= go
BIN := bin

# Tool versions are pinned so CI and every developer machine agree.
GOLANGCI_VERSION := v2.13.2
GOOSE_VERSION    := v3.28.0
SQLC_VERSION     := v1.31.1
TYGO_VERSION     := v0.2.21

ifeq ($(OS),Windows_NT)
RM_DIR = cmd //c "if exist $(1) rmdir /s /q $(1)"
else
RM_DIR = rm -rf $(1)
endif

POSTGRES_USER ?= tennis
POSTGRES_DB   ?= tennis

# The go command walks into web/node_modules, which ships the odd vendored Go
# file inside an npm package, and it has no flag for skipping a directory.
# Evaluated inside the recipes that need it rather than on every make run.
GO_PACKAGES = $$($(GO) list ./... | grep -v '/web/node_modules/')

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
# Named explicitly: `docker compose up` also builds and runs the API and the
# frontend, which is the whole site but a slow inner loop for Go work.
up:
	docker compose up -d --wait postgres redis

## site: build and run the whole stack, frontend included, on :5174
site:
	docker compose up -d --build --wait

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

# Only the two targets below use this. It is deliberately not exported: the
# tests start their own Postgres, and an exported default would silently point
# them at the compose stack instead.
TEST_DATABASE_URL ?= postgres://tennis:tennis@localhost:5433/tennis_test?sslmode=disable

## testdb: create a database for TEST_DATABASE_URL (optional; tests start their own)
testdb:
	-docker compose exec -T postgres psql -U $(POSTGRES_USER) -d $(POSTGRES_DB) -c "CREATE DATABASE tennis_test"
	$(MAKE) migrate-test

## migrate-test: apply migrations to the test database
# Split out of testdb because CI gets its database from a service container and
# so needs the migration step without the compose call above.
migrate-test:
	$(GO) run github.com/pressly/goose/v3/cmd/goose@$(GOOSE_VERSION) -dir $(MIGRATIONS) postgres "$(TEST_DATABASE_URL)" up

## test: run the test suite
# Integration tests start their own Postgres per package via testcontainers, so
# they need Docker but no setup. Set TEST_DATABASE_URL to use a server you
# already have instead; the database must be named *test*.
test:
	$(GO) test $(GO_PACKAGES)

## test-race: run the test suite with the race detector (used by CI)
# The race detector is unsupported on windows/arm64, so this is a separate
# target rather than the default. CI runs on linux/amd64 and always uses it.
test-race:
	$(GO) test -race $(GO_PACKAGES)

## fmt: format and tidy
fmt:
	$(GO) fmt $(GO_PACKAGES)
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

## seed: start the stack, migrate, and load the seed fixture
seed: up migrate-up ingest

## api: run the HTTP API on port 8080
api:
	$(GO) run ./cmd/api

# The seasons the seed fixture covers. Narrowing the plan keeps the run quiet
# rather than reporting three hundred files the fixture was never going to have.
SEED_SEASONS := 1975,2015,2019,2026

## ingest: load the small seed fixture (fast, for local development)
ingest:
	$(GO) run ./cmd/ingest --local-path ./testdata --seasons $(SEED_SEASONS)

## ingest-full: load the complete dataset (skips files unchanged since the last run)
ingest-full:
	$(GO) run ./cmd/ingest

## ingest-force: as ingest-full, but re-read every file regardless
ingest-force:
	$(GO) run ./cmd/ingest --force

## dataqual: report data-quality anomalies
dataqual:
	$(GO) run ./cmd/dataqual

## prune: repair rows that predate a rule the ingest now enforces
prune:
	$(GO) run ./cmd/ingest --stage prune

## rate: recompute Elo ratings from scratch over every match
rate:
	$(GO) run ./cmd/rate

## web: run the frontend dev server against the local API
web:
	cd web && npm install && npm run dev

## web-build: type-check, lint, test and build the frontend
web-build:
	cd web && npm install && npm run typecheck && npm run lint && npm run test && npm run build

## web-types: regenerate the TypeScript types from the Go response structs
web-types:
	$(GO) run github.com/gzuidhof/tygo@$(TYGO_VERSION) generate

## validate: report the rating-engine validation checks
validate:
	$(GO) run ./cmd/validate

## clean: remove build artefacts
clean:
	$(call RM_DIR,$(BIN))

.PHONY: help up down reset psql testdb migrate-test build test test-race fmt lint migrate-up migrate-down migrate-reset sqlc seed api ingest ingest-full ingest-force prune dataqual rate validate site web web-build web-types clean

# print-VAR: echo a make variable, so CI can read the pinned tool versions
# from here rather than duplicating them in a workflow file.
print-%:
	@echo $($*)
