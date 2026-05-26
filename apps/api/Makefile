.PHONY: help dev build run test lint migrate-up migrate-down migrate-create

DATABASE_URL ?= postgres://hotel:hotel@localhost:5432/hotel_booking?sslmode=disable

help:
	@echo "Targets:"
	@echo "  dev              - run API with hot reload (requires air)"
	@echo "  build            - build api and worker binaries to bin/"
	@echo "  run              - run API once (no hot reload)"
	@echo "  test             - run tests"
	@echo "  lint             - run linters"
	@echo "  migrate-up       - apply pending migrations"
	@echo "  migrate-down     - revert last migration"
	@echo "  migrate-create N=<name> - create new migration file"

dev:
	@command -v air >/dev/null 2>&1 || { echo "air not installed. run: go install github.com/air-verse/air@latest"; exit 1; }
	air

build:
	mkdir -p bin
	go build -o bin/api  ./cmd/api
	go build -o bin/worker ./cmd/worker

run:
	go run ./cmd/api

test:
	go test ./... -race -cover

lint:
	go vet ./...
	@command -v golangci-lint >/dev/null 2>&1 && golangci-lint run || echo "golangci-lint not installed (optional)"

migrate-up:
	@command -v migrate >/dev/null 2>&1 || { echo "migrate not installed. brew install golang-migrate"; exit 1; }
	migrate -path migrations -database "$(DATABASE_URL)" up

migrate-down:
	migrate -path migrations -database "$(DATABASE_URL)" down 1

migrate-create:
	@if [ -z "$(N)" ]; then echo "Usage: make migrate-create N=<name>"; exit 1; fi
	migrate create -ext sql -dir migrations -seq $(N)
