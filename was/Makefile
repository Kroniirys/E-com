GO ?= go
-include ../.env
export

.PHONY: build run worker migrate-up seed test vet check format test-integration reset-test

build:
	mkdir -p bin
	CGO_ENABLED=0 $(GO) build -trimpath -o bin/api ./cmd/api
	CGO_ENABLED=0 $(GO) build -trimpath -o bin/worker ./cmd/worker
	CGO_ENABLED=0 $(GO) build -trimpath -o bin/migrate ./cmd/migrate
	CGO_ENABLED=0 $(GO) build -trimpath -o bin/seed ./cmd/seed
	CGO_ENABLED=0 $(GO) build -trimpath -o bin/healthcheck ./cmd/healthcheck

run:
	$(GO) run ./cmd/api

worker:
	$(GO) run ./cmd/worker

migrate-up:
	$(GO) run ./cmd/migrate up

seed:
	$(GO) run ./cmd/seed

test:
	$(GO) test ./...

vet:
	$(GO) vet ./...

check: test vet

format:
	gofmt -w cmd internal migrations tests api/embed.go

test-integration:
	GO="$(GO)" bash scripts/test-integration.sh

reset-test:
	APP_ENV=test DATABASE_URL="$(TEST_DATABASE_URL)" ALLOW_TEST_RESET="$(ALLOW_TEST_RESET)" $(GO) run ./cmd/migrate down-one
