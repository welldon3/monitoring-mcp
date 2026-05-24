BINARY := bin/monitoring-mcp

.PHONY: build test lint run tidy docker-up docker-down

build:
	go build -o $(BINARY) .

test:
	go test ./...

lint:
	golangci-lint run

run:
	go run .

tidy:
	go mod tidy

docker-up:
	docker compose -f examples/docker-compose.yml up -d

docker-down:
	docker compose -f examples/docker-compose.yml down -v

.DEFAULT_GOAL := build