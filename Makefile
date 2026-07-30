ifeq ($(OS),Windows_NT)
SHELL := cmd.exe
else
SHELL := /bin/bash
export PATH := /usr/local/go/bin:$(PATH)
endif
.DEFAULT_GOAL := help

# Load .env if present so make targets can use the same vars as the apps.
ifneq (,$(wildcard .env))
include .env
export
endif

GOOSE := go run github.com/pressly/goose/v3/cmd/goose@v3.22.0
# Migrations bypass PgBouncer and go directly to Postgres (DDL needs session mode, not transaction mode)
PG_DSN := postgres://$(POSTGRES_USER):$(POSTGRES_PASSWORD)@$(POSTGRES_HOST):5436/$(POSTGRES_DB)?sslmode=$(POSTGRES_SSLMODE)
PNPM := $(shell if command -v pnpm >/dev/null 2>&1; then printf 'pnpm'; elif command -v corepack >/dev/null 2>&1; then printf 'corepack pnpm'; else printf 'npx -y pnpm@9.10.0'; fi)

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
.PHONY: migrate up down status new migrate-up migrate-down migrate-status migrate-new
migrate: ## Compatibility alias for `make migrate up/down/status/new`
	@:

up: migrate-up ## Compatibility alias for `make migrate up`
down: migrate-down ## Compatibility alias for `make migrate down`
status: migrate-status ## Compatibility alias for `make migrate status`
new: migrate-new ## Compatibility alias for `make migrate new name=...`

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
api: ## Run the Go API (auto-reloads on code changes via air)
	cd apps/api && $(HOME)/go/bin/air

web: ## Run the Next.js web app (admin + public + auth, single app)
	cd apps/web && $(PNPM) dev

wa-web: ## Run the WhatsApp Web QR service
	cd apps/wa-web && node src/index.js

tg-web: ## Run the Telegram QR-login service (reports "not configured" without TELEGRAM_API_ID/HASH in .env)
	cd apps/tg-web && node src/index.js

dev: ## Run API + web + wa-web + tg-web concurrently (tg-web degrades to "not configured" without TELEGRAM_API_ID/HASH — see docs/SETUP_TELEGRAM_QR.md)
	$(PNPM) dlx concurrently -n api,web,wa-web,tg-web -c blue,magenta,green,cyan \
		"\"$(MAKE)\" api" "\"$(MAKE)\" web" "\"$(MAKE)\" wa-web" "\"$(MAKE)\" tg-web"

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
	$(PNPM) install
