SHELL := /bin/bash

API_DIR := apps/api
COMPOSE_FILE := deployments/compose.yaml
MIGRATIONS_DIR := apps/api/migrations

POSTGRES_SERVICE := postgres
POSTGRES_USER := vaultforge
POSTGRES_DATABASE := vaultforge
POSTGRES_TEST_DATABASE := vaultforge_test

.PHONY: \
	setup \
	dev \
	test \
	lint \
	format \
	format-check \
	mod-verify \
	verify \
	compose-up \
	compose-stop \
	compose-down \
	compose-logs \
	compose-ps \
	db-setup \
	db-create-test \
	db-shell \
	migrate-create \
	migrate-up \
	migrate-down \
	migrate-version

setup:
	cd $(API_DIR) && go mod download

dev:
	cd $(API_DIR) && go run ./cmd/api

test:
	@test -n "$$TEST_DATABASE_URL" || \
		(echo "TEST_DATABASE_URL is required" && exit 1)
	cd $(API_DIR) && go test -race -count=1 ./...

lint:
	cd $(API_DIR) && go vet ./...
	cd $(API_DIR) && staticcheck ./...

format:
	cd $(API_DIR) && gofmt -w .

format-check:
	test -z "$$(gofmt -l $(API_DIR))"

mod-verify:
	cd $(API_DIR) && go mod verify

verify: format-check mod-verify lint test

compose-up:
	docker compose -f $(COMPOSE_FILE) up -d $(POSTGRES_SERVICE)

compose-stop:
	docker compose -f $(COMPOSE_FILE) stop $(POSTGRES_SERVICE)

compose-down:
	docker compose -f $(COMPOSE_FILE) down

compose-logs:
	docker compose -f $(COMPOSE_FILE) logs -f $(POSTGRES_SERVICE)

compose-ps:
	docker compose -f $(COMPOSE_FILE) ps

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

db-shell:
	docker compose -f $(COMPOSE_FILE) exec $(POSTGRES_SERVICE) \
		psql \
		-U $(POSTGRES_USER) \
		-d $(POSTGRES_DATABASE)

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