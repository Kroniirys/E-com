GO ?= go
-include .env
export

.PHONY: up down build run worker migrate-up seed test test-integration test-e2e test-db-up test-db-down contract viewer-dev viewer-check frontend-check was-check check format reset-test

up:
	docker compose up --build

down:
	docker compose down

build run worker migrate-up seed test reset-test format test-integration:
	$(MAKE) -C was GO="$(GO)" $@

was-check:
	$(MAKE) -C was GO="$(GO)" check

contract:
	npm --prefix viewer run validate:api
	npm --prefix viewer run generate:api

viewer-dev:
	npm --prefix viewer run dev

viewer-check: contract
	npm --prefix viewer run lint
	npm --prefix viewer run typecheck
	npm --prefix viewer test
	npm --prefix viewer run build

frontend-check: viewer-check

check: was-check viewer-check

test-db-up:
	docker compose --env-file .env.example --profile test up -d --wait postgres-test

test-db-down:
	docker compose --env-file .env.example --profile test stop postgres-test

test-e2e: build
	$(MAKE) -C was GO="$(GO)" test-helper
	python3 scripts/run-e2e.py
