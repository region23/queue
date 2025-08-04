.PHONY: help build run ngrok webhook clean dev stop

# Default target
help:
	@echo "Available commands:"
	@echo "  build     - Build the bot binary"
	@echo "  run       - Run the bot with environment variables"
	@echo "  ngrok     - Start ngrok tunnel"
	@echo "  webhook   - Set webhook URL"
	@echo "  dev       - Start development environment (ngrok + bot)"
	@echo "  stop      - Stop all running processes (ngrok, bot)"
	@echo "  clean     - Clean build artifacts"
	@echo "  env       - Show environment variables"

# Build the bot
build:
	go build -o queue_bot .

# Run the bot with environment variables
run: build
	./load_env.sh ./queue_bot

# Start ngrok tunnel
ngrok:
	ngrok http --domain=frankly-wanted-polliwog.ngrok-free.app 8080

# Set webhook URL using the ngrok domain
webhook:
	./load_env.sh bash -c 'curl -X POST "https://api.telegram.org/bot$$TELEGRAM_TOKEN/setWebhook" \
		-H "Content-Type: application/json" \
		-d "{\"url\": \"$$WEBHOOK_URL\"}" && echo ""'

# Development mode: start ngrok in background and run bot
dev: build
	@echo "Starting development environment..."
	@echo "1. Starting ngrok tunnel..."
	@ngrok http --domain=frankly-wanted-polliwog.ngrok-free.app 8080 --log=stdout > ngrok.log 2>&1 &
	@echo "2. Waiting for ngrok to start..."
	@sleep 3
	@echo "3. Setting webhook and starting bot..."
	@./load_env.sh bash -c '\
		echo "Setting webhook to $$WEBHOOK_URL"; \
		curl -s -X POST "https://api.telegram.org/bot$$TELEGRAM_TOKEN/setWebhook" \
			-H "Content-Type: application/json" \
			-d "{\"url\": \"$$WEBHOOK_URL\"}" > /dev/null && \
		echo "Webhook configured successfully" && \
		echo "Starting bot on port $$PORT..." && \
		./queue_bot'

# Clean build artifacts
clean:
	rm -f queue_bot ngrok.log queue.db

# Show environment variables
env:
	@./load_env.sh bash -c 'echo "TELEGRAM_TOKEN: $${TELEGRAM_TOKEN:0:10}..."; echo "WEBHOOK_URL: $$WEBHOOK_URL"; echo "PORT: $$PORT"'
