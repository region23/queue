#!/bin/bash

# Load environment variables from .env file
if [ -f .env ]; then
    export $(grep -v '^#' .env | xargs)
    echo "Environment variables loaded from .env"
    echo "TELEGRAM_TOKEN: ${TELEGRAM_TOKEN:0:10}..."
    echo "WEBHOOK_URL: $WEBHOOK_URL"
    echo "PORT: $PORT"
else
    echo ".env file not found"
    exit 1
fi

# Execute the command passed as arguments
exec "$@"
