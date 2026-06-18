SHELL := /bin/bash

.PHONY: setup dev test lint format format-check generate migrate compose-up

setup:
	cd apps/api && go mod download
	cd apps/web && npm ci

dev:
	@echo "API runtime begins in Step 2."
	cd apps/web && npm run dev

test:
	cd apps/api && go test -race ./...
	cd apps/web && npm run test

lint:
	cd apps/api && go vet ./...
	cd apps/api && staticcheck ./...
	cd apps/web && npm run typecheck
	cd apps/web && npm run lint

format:
	cd apps/api && gofmt -w .
	cd apps/web && npm run format

format-check:
	test -z "$$(gofmt -l apps/api)"
	cd apps/web && npm run format:check

generate:
	@echo "Code generation is not required yet."

migrate:
	@echo "Database migrations begin in Step 3."

compose-up:
	@echo "Docker Compose begins after the services exist."