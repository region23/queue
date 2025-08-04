# Dockerfile для Telegram Queue Bot
# Многоэтапная сборка для уменьшения размера образа

# Этап 1: Сборка
FROM golang:1.23-alpine AS builder

LABEL maintainer="GitHub Copilot"
LABEL description="Telegram Queue Bot - система электронной очереди"

# Устанавливаем рабочую директорию
WORKDIR /app

# Устанавливаем git и ca-certificates для работы с модулями
RUN apk add --no-cache git ca-certificates tzdata

# Копируем файлы модулей Go
COPY go.mod go.sum ./

# Загружаем зависимости
RUN go mod download && go mod verify

# Копируем исходный код
COPY . .

# Собираем приложение
RUN CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build \
    -ldflags='-w -s -extldflags "-static"' \
    -a \
    -installsuffix cgo \
    -o queue_bot \
    cmd/server/main.go

# Этап 2: Финальный образ
FROM alpine:latest

# Устанавливаем минимальные зависимости
RUN apk --no-cache add ca-certificates tzdata sqlite

# Создаем пользователя для безопасности
RUN addgroup -g 1001 -S appgroup && \
    adduser -u 1001 -S appuser -G appgroup

WORKDIR /app

# Копируем исполняемый файл из builder
COPY --from=builder /app/queue_bot .

# Копируем пример конфигурации
COPY --from=builder /app/.env.example .env.example

# Создаем директории для данных
RUN mkdir -p /app/data && \
    chown -R appuser:appgroup /app

# Переключаемся на непривилегированного пользователя
USER appuser

# Открываем порт
EXPOSE 8080

# Переменные окружения по умолчанию
ENV PORT=8080
ENV DB_FILE=/app/data/queue.db

# Проверка здоровья контейнера
HEALTHCHECK --interval=30s --timeout=10s --start-period=30s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:${PORT}/health || exit 1

# Запуск приложения
CMD ["./queue_bot"]
