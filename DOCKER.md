# Docker Deployment Guide

## Quick Start

### Development

```bash
# Build and run locally
make build
make run

# Development with ngrok
make dev
```

### Production

```bash
# Build Docker image
make docker-build

# Start production stack
make docker-run

# Start with monitoring (Prometheus + Grafana)
make monitoring
```

## Monitoring

After starting with `make monitoring`, access:

- **Bot Health**: <http://localhost:8080/health>
- **Metrics**: <http://localhost:8080/metrics>  
- **Prometheus**: <http://localhost:9090>
- **Grafana**: <http://localhost:3000> (admin/admin)

## Management Commands

```bash
# View logs
make docker-logs

# Stop containers
make docker-down

# Stop monitoring
make stop-monitoring

# Health check
make health

# Database backup
make backup-dir
make backup
```

## Configuration

Copy `.env.example` to `.env` and configure:

```bash
TELEGRAM_TOKEN=your_bot_token_here
WEBHOOK_URL=https://your-domain.com/webhook
PORT=8080
DB_FILE=/app/data/queue.db
```

## Docker Compose Profiles

- **Default**: Only the bot
- **monitoring**: Bot + Prometheus + Grafana

```bash
# Only bot
docker-compose up -d

# With monitoring
docker-compose --profile monitoring up -d
```

## Troubleshooting

### Health Check

```bash
curl http://localhost:8080/health
```

### Container Logs  

```bash
docker-compose logs -f queue-bot
```

### Metrics

```bash
curl http://localhost:8080/metrics | grep telegram_bot
```
