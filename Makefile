ifeq ($(OS),Windows_NT)
SHELL := cmd.exe
else
SHELL := /bin/bash
endif
.DEFAULT_GOAL := help

# Load .env if present so make targets can use the same vars as the apps.
ifneq (,$(wildcard .env))
include .env
export
endif

GOOSE := go run github.com/pressly/goose/v3/cmd/goose@v3.22.0
PG_DSN := postgres://$(POSTGRES_USER):$(POSTGRES_PASSWORD)@$(POSTGRES_HOST):$(POSTGRES_PORT)/$(POSTGRES_DB)?sslmode=$(POSTGRES_SSLMODE)

.PHONY: help
help: ## Show this help
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage: make \033[36m<target>\033[0m\n\nTargets:\n"} /^[a-zA-Z_-]+:.*?##/ { printf "  \033[36m%-22s\033[0m %s\n", $$1, $$2 }' $(MAKEFILE_LIST)

# ---------- environment ----------
.PHONY: db-up db-down db-logs
db-up: ## Start Postgres (Docker)
	docker compose up -d postgres
	@echo "Waiting for Postgres to be healthy..."
	@until docker inspect --format='{{.State.Health.Status}}' projectx-postgres 2>/dev/null | grep -q healthy; do sleep 1; done
	@echo "Postgres is ready."

db-down: ## Stop Postgres
	docker compose down

db-logs: ## Tail Postgres logs
	docker compose logs -f postgres

# ---------- migrations ----------
.PHONY: migrate-up migrate-down migrate-status migrate-new
migrate-up: ## Apply all pending migrations
	cd apps/api && $(GOOSE) -dir migrations postgres "$(PG_DSN)" up

migrate-down: ## Roll back the most recent migration
	cd apps/api && $(GOOSE) -dir migrations postgres "$(PG_DSN)" down

migrate-status: ## Show migration status
	cd apps/api && $(GOOSE) -dir migrations postgres "$(PG_DSN)" status

migrate-new: ## Create a new migration. Usage: make migrate-new name=add_something
	cd apps/api && $(GOOSE) -dir migrations create $(name) sql

# ---------- seed ----------
.PHONY: seed-admin
seed-admin: ## Seed the super-admin user from .env (idempotent)
	cd apps/api && go run ./cmd/seed

# ---------- dev ----------
.PHONY: api web dev
api: ## Run the Go API
	cd apps/api && go run ./cmd/server

web: ## Run the Next.js web app (admin + public + auth, single app)
	cd apps/web && corepack pnpm dev

dev: ## Run API + web concurrently (requires `npx`)
	npx -y concurrently -k -n api,web -c blue,magenta \
		"\"$(MAKE)\" api" "\"$(MAKE)\" web"

# ---------- quality ----------
.PHONY: test lint fmt
test: ## Run tests
	cd apps/api && go test ./...

lint: ## Lint all code
	cd apps/api && go vet ./...
	cd . && pnpm -r lint

fmt: ## Format Go code
	cd apps/api && go fmt ./...

# ---------- bootstrap ----------
.PHONY: install
install: ## Install JS deps
	corepack pnpm install
