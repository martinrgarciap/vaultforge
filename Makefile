SHELL := /bin/bash

API_DIR := apps/api
WEB_DIR := apps/web
HASH_SERVICE_DIR := services/hash-service
COMPOSE_FILE := deployments/compose.yaml
MIGRATIONS_DIR := apps/api/migrations

POSTGRES_SERVICE := postgres
POSTGRES_USER := vaultforge
POSTGRES_DATABASE := vaultforge
POSTGRES_TEST_DATABASE := vaultforge_test
POSTGRES_E2E_DATABASE := vaultforge_e2e
REDIS_SERVICE := redis
OTEL_COLLECTOR_SERVICE := otel-collector
JAEGER_SERVICE := jaeger

E2E_DATABASE_URL ?= postgres://vaultforge:vaultforge_local_dev@127.0.0.1:5433/vaultforge_e2e?sslmode=disable
E2E_REDIS_URL ?= redis://127.0.0.1:6380/2
API_BUILD_PACKAGE := github.com/martinrgarciap/vaultforge/apps/api/internal/buildinfo
API_BUILD_VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo development)
API_BUILD_COMMIT ?= $(shell git rev-parse --short=12 HEAD 2>/dev/null || echo unknown)
API_BUILD_LDFLAGS := -X $(API_BUILD_PACKAGE).Version=$(API_BUILD_VERSION) \
	-X $(API_BUILD_PACKAGE).Commit=$(API_BUILD_COMMIT)

.PHONY: \
	setup \
	setup-api \
	setup-web \
	setup-e2e \
	dev \
	dev-api \
	dev-web \
	test \
	test-api \
	test-web \
	test-e2e \
	verify-e2e \
	db-reset-e2e \
	lint \
	lint-api \
	lint-web \
	format \
	format-api \
	format-web \
	format-check \
	format-check-api \
	format-check-web \
	typecheck-web \
	build-api \
	build-web \
	format-rust \
	format-check-rust \
	lint-rust \
	test-rust \
	build-rust \
	verify-rust \
	mod-verify \
	verify \
	verify-api \
	verify-web \
	compose-up \
	compose-stop \
	compose-down \
	compose-logs \
	compose-ps \
	observability-up \
	observability-stop \
	observability-logs \
	db-setup \
	db-create-test \
	db-shell \
	redis-shell \
	migrate-create \
	migrate-up \
	migrate-down \
	migrate-version

setup: setup-api setup-web setup-e2e

setup-api:
	cd $(API_DIR) && go mod download

setup-web:
	cd $(WEB_DIR) && npm ci

setup-e2e:
	cd $(WEB_DIR) && npx playwright install chromium

dev: dev-api

dev-api:
	cd $(API_DIR) && \
		go run \
			-ldflags "$(API_BUILD_LDFLAGS)" \
			./cmd/api

dev-web:
	cd $(WEB_DIR) && npm run dev

test: test-api test-web

test-api:
	@test -n "$$TEST_DATABASE_URL" || \
		(echo "TEST_DATABASE_URL is required" && exit 1)
	@test -n "$$TEST_REDIS_URL" || \
		(echo "TEST_REDIS_URL is required" && exit 1)
	cd $(API_DIR) && go test -race -count=1 ./...

test-web:
	cd $(WEB_DIR) && npm run test

test-e2e: db-reset-e2e
	cd $(WEB_DIR) && \
		E2E_DATABASE_URL="$(E2E_DATABASE_URL)" \
		E2E_REDIS_URL="$(E2E_REDIS_URL)" \
		npm run test:e2e

lint: lint-api lint-web

lint-api:
	cd $(API_DIR) && go vet ./...
	cd $(API_DIR) && staticcheck ./...

lint-web:
	cd $(WEB_DIR) && npm run lint

format: format-api format-web

format-api:
	cd $(API_DIR) && gofmt -w .

format-web:
	cd $(WEB_DIR) && npm run format

format-check: format-check-api format-check-web

format-check-api:
	test -z "$$(gofmt -l $(API_DIR))"

format-check-web:
	cd $(WEB_DIR) && npm run format:check

typecheck-web:
	cd $(WEB_DIR) && npm run typecheck

build-api:
	cd $(API_DIR) && \
		output="$$(mktemp)" && \
		trap 'rm -f "$$output"' EXIT && \
		go build \
			-trimpath \
			-ldflags "$(API_BUILD_LDFLAGS)" \
			-o "$$output" \
			./cmd/api

build-web:
	cd $(WEB_DIR) && npm run build

format-rust:
	cd $(HASH_SERVICE_DIR) && cargo fmt

format-check-rust:
	cd $(HASH_SERVICE_DIR) && cargo fmt --check

lint-rust:
	cd $(HASH_SERVICE_DIR) && cargo clippy -- -D warnings

test-rust:
	cd $(HASH_SERVICE_DIR) && cargo test

build-rust:
	cd $(HASH_SERVICE_DIR) && cargo build

verify-rust: format-check-rust lint-rust test-rust build-rust

mod-verify:
	cd $(API_DIR) && go mod verify

verify: verify-api verify-web verify-rust verify-e2e

verify-api: format-check-api mod-verify lint-api test-api build-api

verify-web: format-check-web lint-web typecheck-web test-web build-web

verify-e2e: test-e2e

compose-up:
	docker compose -f $(COMPOSE_FILE) up -d --wait \
		$(POSTGRES_SERVICE) $(REDIS_SERVICE)

compose-stop:
	docker compose -f $(COMPOSE_FILE) stop \
		$(POSTGRES_SERVICE) $(REDIS_SERVICE)

compose-down:
	docker compose -f $(COMPOSE_FILE) down

compose-logs:
	docker compose -f $(COMPOSE_FILE) logs -f \
		$(POSTGRES_SERVICE) $(REDIS_SERVICE)

compose-ps:
	docker compose -f $(COMPOSE_FILE) ps

observability-up:
	docker compose -f $(COMPOSE_FILE) up -d \
		$(JAEGER_SERVICE) $(OTEL_COLLECTOR_SERVICE)

observability-stop:
	docker compose -f $(COMPOSE_FILE) stop \
		$(OTEL_COLLECTOR_SERVICE) $(JAEGER_SERVICE)

observability-logs:
	docker compose -f $(COMPOSE_FILE) logs -f \
		$(OTEL_COLLECTOR_SERVICE) $(JAEGER_SERVICE)

db-setup:
	$(MAKE) compose-up
	$(MAKE) db-create-test
	$(MAKE) migrate-up

db-create-test:
	@echo "Waiting for PostgreSQL..."
	@until docker compose -f $(COMPOSE_FILE) exec -T $(POSTGRES_SERVICE) \
		pg_isready \
		-U $(POSTGRES_USER) \
		-d $(POSTGRES_DATABASE) >/dev/null 2>&1; do \
			sleep 1; \
	done
	@docker compose -f $(COMPOSE_FILE) exec -T $(POSTGRES_SERVICE) \
		psql \
		-U $(POSTGRES_USER) \
		-d postgres \
		-tAc "SELECT 1 FROM pg_database WHERE datname='$(POSTGRES_TEST_DATABASE)'" \
		| grep -q 1 || \
	docker compose -f $(COMPOSE_FILE) exec -T $(POSTGRES_SERVICE) \
		createdb \
		-U $(POSTGRES_USER) \
		$(POSTGRES_TEST_DATABASE)
	@echo "Test database is ready."

db-reset-e2e:
	$(MAKE) compose-up
	@echo "Waiting for PostgreSQL..."
	@until docker compose -f $(COMPOSE_FILE) exec -T $(POSTGRES_SERVICE) \
		pg_isready \
		-U $(POSTGRES_USER) \
		-d $(POSTGRES_DATABASE) >/dev/null 2>&1; do \
			sleep 1; \
	done
	@docker compose -f $(COMPOSE_FILE) exec -T $(POSTGRES_SERVICE) \
		dropdb \
		-U $(POSTGRES_USER) \
		--if-exists \
		--force \
		$(POSTGRES_E2E_DATABASE)
	@docker compose -f $(COMPOSE_FILE) exec -T $(POSTGRES_SERVICE) \
		createdb \
		-U $(POSTGRES_USER) \
		$(POSTGRES_E2E_DATABASE)
	@for migration in $$(find $(MIGRATIONS_DIR) \
		-maxdepth 1 \
		-type f \
		-name '*.up.sql' \
		| sort); do \
			echo "Applying $$migration"; \
			docker compose -f $(COMPOSE_FILE) exec -T $(POSTGRES_SERVICE) \
				psql \
				-v ON_ERROR_STOP=1 \
				-U $(POSTGRES_USER) \
				-d $(POSTGRES_E2E_DATABASE) \
				< "$$migration"; \
	done
	@echo "E2E database is ready."

db-shell:
	docker compose -f $(COMPOSE_FILE) exec $(POSTGRES_SERVICE) \
		psql \
		-U $(POSTGRES_USER) \
		-d $(POSTGRES_DATABASE)

redis-shell:
	docker compose -f $(COMPOSE_FILE) exec $(REDIS_SERVICE) redis-cli

migrate-create:
	@test -n "$(name)" || \
		(echo "usage: make migrate-create name=create_example" && exit 1)
	migrate create \
		-ext sql \
		-dir $(MIGRATIONS_DIR) \
		-seq \
		"$(name)"

migrate-up:
	@test -n "$$DATABASE_URL" || \
		(echo "DATABASE_URL is required" && exit 1)
	migrate \
		-path $(MIGRATIONS_DIR) \
		-database "$$DATABASE_URL" \
		up

migrate-down:
	@test -n "$$DATABASE_URL" || \
		(echo "DATABASE_URL is required" && exit 1)
	migrate \
		-path $(MIGRATIONS_DIR) \
		-database "$$DATABASE_URL" \
		down 1

migrate-version:
	@test -n "$$DATABASE_URL" || \
		(echo "DATABASE_URL is required" && exit 1)
	migrate \
		-path $(MIGRATIONS_DIR) \
		-database "$$DATABASE_URL" \
		version
