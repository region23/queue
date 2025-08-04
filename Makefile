.PHONY: help build run ngrok webhook clean dev stop delete-webhook env

# Default target
help:
	@echo "Available commands:"
	@echo "  build          - Build the bot binary"
	@echo "  run            - Run the bot (loads .env automatically)"
	@echo "  ngrok          - Start ngrok tunnel"
	@echo "  webhook        - Set webhook URL"
	@echo "  delete-webhook - Remove webhook from Telegram"
	@echo "  dev            - Start development environment (ngrok + bot)"
	@echo "  stop           - Stop all running processes (ngrok, bot)"
	@echo "  clean          - Clean build artifacts"
	@echo "  env            - Show environment variables from .env"

# Build the bot
build:
	go build -o queue_bot .

# Run the bot (now loads .env automatically)
run: build
	./queue_bot

# Start ngrok tunnel
ngrok:
	ngrok http --domain=frankly-wanted-polliwog.ngrok-free.app 8080

# Set webhook URL using environment variables from .env
webhook:
	@if [ -f .env ]; then \
		export $$(grep -v '^#' .env | xargs) && \
		curl -X POST "https://api.telegram.org/bot$$TELEGRAM_TOKEN/setWebhook" \
			-H "Content-Type: application/json" \
			-d "{\"url\": \"$$WEBHOOK_URL\"}" && echo ""; \
	else \
		echo "Error: .env file not found"; \
		exit 1; \
	fi

# Remove webhook from Telegram
delete-webhook:
	@if [ -f .env ]; then \
		export $$(grep -v '^#' .env | xargs) && \
		curl -X POST "https://api.telegram.org/bot$$TELEGRAM_TOKEN/deleteWebhook" && echo ""; \
	else \
		echo "Error: .env file not found"; \
		exit 1; \
	fi

# Development mode: start ngrok in background and run bot
dev: build
	@echo "Starting development environment..."
	@echo "1. Starting ngrok tunnel..."
	@ngrok http --domain=frankly-wanted-polliwog.ngrok-free.app 8080 --log=stdout > ngrok.log 2>&1 &
	@echo "2. Waiting for ngrok to start..."
	@sleep 3
	@echo "3. Setting webhook and starting bot..."
	@./queue_bot

# Clean build artifacts
clean:
	rm -f queue_bot ngrok.log queue.db

# Stop all running processes
stop:
	@echo "Stopping all processes..."
	@pkill -f "ngrok http" || true
	@pkill -f "queue_bot" || true
	@echo "All processes stopped"

# Show environment variables
env:
	@if [ -f .env ]; then \
		export $$(grep -v '^#' .env | xargs) && \
		echo "TELEGRAM_TOKEN: $${TELEGRAM_TOKEN:0:10}..." && \
		echo "WEBHOOK_URL: $$WEBHOOK_URL" && \
		echo "PORT: $$PORT"; \
	else \
		echo "Error: .env file not found"; \
		exit 1; \
	fi
