.PHONY: run build test lint db-up db-down

run:
	go run ./cmd/sift

build:
	go build -o bin/sift ./cmd/sift

test:
	go test ./... -race -count=1

lint:
	golangci-lint run

db-up:
	docker compose up -d postgres

db-down:
	docker compose down
