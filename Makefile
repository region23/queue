# Simple Makefile for Telegram Queue Bot

.PHONY: build run test clean docker-build docker-run

# Build the application
build:
	go build -o bin/bot .

# Run locally
run:
	go run .

# Run tests
test:
	go test ./...

# Clean build artifacts
clean:
	rm -rf bin/

# Docker commands
docker-build:
	docker build -t queue-bot .

docker-run:
	docker-compose up -d

docker-stop:
	docker-compose down

# Development helpers
dev:
	air -c .air.toml

fmt:
	go fmt ./...

lint:
	golangci-lint run