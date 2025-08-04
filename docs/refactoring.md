# Пошаговый план рефакторинга Telegram Queue Bot

> **Дата аудита:** 4 августа 2025  
> **Статус:** В разработке  
> **Оценка текущего состояния:** Требует серьезного рефакторинга

## 📊 Текущее состояние

### Проблемы

- ❌ Монолитная архитектура (689 строк в одном файле)
- ❌ Отсутствие тестов
- ❌ Глобальные переменные
- ❌ Potential race conditions
- ❌ Смешение бизнес-логики с инфраструктурой
- ❌ Отсутствие proper error handling

### Положительные стороны

- ✅ Защита от SQL injection (параметризованные запросы)
- ✅ Минимальные зависимости
- ✅ Читаемый код
- ✅ Работающий функционал

---

## 🎯 ЭТАП 1: ПОДГОТОВКА И ПЛАНИРОВАНИЕ

### Шаг 1.1: Создание backup и git branch

```bash
git checkout -b refactoring/architecture-cleanup
git add .
git commit -m "backup: current working state before refactoring"
```

### Шаг 1.2: Анализ и документирование зависимостей

- [ ] Создать диаграмму текущих зависимостей
- [ ] Выделить основные компоненты
- [ ] Определить интерфейсы между компонентами

### Шаг 1.3: Создание новой структуры проекта

```
telegram_queue_bot/
├── cmd/
│   └── server/
│       └── main.go           # Точка входа
├── internal/
│   ├── config/
│   │   ├── config.go         # Конфигурация
│   │   └── validation.go     # Валидация конфигурации
│   ├── storage/
│   │   ├── interfaces.go     # Интерфейсы хранилища
│   │   ├── sqlite/
│   │   │   ├── sqlite.go     # SQLite реализация
│   │   │   └── migrations.go # Миграции БД
│   │   └── models/
│   │       └── models.go     # Модели данных
│   ├── bot/
│   │   ├── handlers/
│   │   │   ├── start.go      # Обработчик /start
│   │   │   ├── contact.go    # Обработчик контактов
│   │   │   ├── callback.go   # Обработчик callback'ов
│   │   │   └── common.go     # Общие утилиты
│   │   ├── keyboard/
│   │   │   └── keyboards.go  # Клавиатуры
│   │   └── service.go        # Сервис бота
│   ├── scheduler/
│   │   ├── interfaces.go     # Интерфейсы планировщика
│   │   ├── memory/
│   │   │   └── scheduler.go  # In-memory планировщик
│   │   └── persistent/
│   │       └── scheduler.go  # Персистентный планировщик
│   └── server/
│       ├── server.go         # HTTP сервер
│       └── middleware.go     # Middleware
├── pkg/
│   ├── logger/
│   │   └── logger.go         # Структурированное логирование
│   └── errors/
│       └── errors.go         # Кастомные ошибки
├── tests/
│   ├── integration/
│   └── unit/
├── docs/
│   ├── api.md
│   ├── deployment.md
│   └── refactoring.md
├── docker-compose.yml
├── Dockerfile
└── Makefile
```

---

## 🚨 ЭТАП 2: КРИТИЧЕСКИЕ ИСПРАВЛЕНИЯ (Приоритет: ВЫСОКИЙ)

### Шаг 2.1: Создание базовых интерфейсов

**Время:** 2-3 часа

#### 2.1.1 Создать `internal/storage/interfaces.go`

```go
package storage

import "context"

type UserRepository interface {
    SaveUser(ctx context.Context, chatID int64, phone, firstName, lastName string) error
    IsUserRegistered(ctx context.Context, chatID int64) bool
    GetUserByID(ctx context.Context, chatID int64) (*User, error)
}

type SlotRepository interface {
    CreateSlot(ctx context.Context, slot *Slot) error
    GetAvailableSlots(ctx context.Context, date string) ([]*Slot, error)
    ReserveSlot(ctx context.Context, slotID int, chatID int64) error
    GetSlotByID(ctx context.Context, id int) (*Slot, error)
    GetUserTodaySlot(ctx context.Context, chatID int64) (*Slot, bool, error)
    MarkSlotNotified(ctx context.Context, slotID int) error
    GetPendingNotifications(ctx context.Context) ([]*Slot, error)
}

type Storage interface {
    UserRepository
    SlotRepository
    Close() error
}
```

#### 2.1.2 Создать `internal/storage/models/models.go`

```go
package models

import "time"

type User struct {
    ID        int64  `json:"id"`
    ChatID    int64  `json:"chat_id"`
    Phone     string `json:"phone"`
    FirstName string `json:"first_name"`
    LastName  string `json:"last_name"`
}

type Slot struct {
    ID         int       `json:"id"`
    Date       string    `json:"date"`
    StartTime  string    `json:"start_time"`
    EndTime    string    `json:"end_time"`
    UserChatID *int64    `json:"user_chat_id,omitempty"`
    Notified   bool      `json:"notified"`
    CreatedAt  time.Time `json:"created_at"`
    UpdatedAt  time.Time `json:"updated_at"`
}
```

### Шаг 2.2: Создание конфигурационного модуля

**Время:** 1-2 часа

#### 2.2.1 Создать `internal/config/config.go`

```go
package config

import (
    "fmt"
    "os"
    "strconv"
    "time"
)

type Config struct {
    Telegram TelegramConfig `json:"telegram"`
    Server   ServerConfig   `json:"server"`
    Database DatabaseConfig `json:"database"`
    Schedule ScheduleConfig `json:"schedule"`
}

type TelegramConfig struct {
    Token      string `json:"token"`
    WebhookURL string `json:"webhook_url"`
}

type ServerConfig struct {
    Port    string `json:"port"`
    Timeout int    `json:"timeout"`
}

type DatabaseConfig struct {
    Path string `json:"path"`
}

type ScheduleConfig struct {
    WorkStart        string `json:"work_start"`
    WorkEnd          string `json:"work_end"`
    SlotDurationMins int    `json:"slot_duration_mins"`
    ScheduleDays     int    `json:"schedule_days"`
}

func Load() (*Config, error) {
    cfg := &Config{
        Telegram: TelegramConfig{
            Token:      os.Getenv("TELEGRAM_TOKEN"),
            WebhookURL: os.Getenv("WEBHOOK_URL"),
        },
        Server: ServerConfig{
            Port:    getEnv("PORT", "8080"),
            Timeout: getEnvAsInt("SERVER_TIMEOUT", 30),
        },
        Database: DatabaseConfig{
            Path: getEnv("DB_FILE", "queue.db"),
        },
        Schedule: ScheduleConfig{
            WorkStart:        getEnv("WORK_START", "09:00"),
            WorkEnd:          getEnv("WORK_END", "18:00"),
            SlotDurationMins: getEnvAsInt("SLOT_DURATION", 30),
            ScheduleDays:     getEnvAsInt("SCHEDULE_DAYS", 7),
        },
    }

    if err := cfg.Validate(); err != nil {
        return nil, fmt.Errorf("config validation failed: %w", err)
    }

    return cfg, nil
}

func (c *Config) Validate() error {
    if c.Telegram.Token == "" {
        return fmt.Errorf("TELEGRAM_TOKEN is required")
    }
    if c.Telegram.WebhookURL == "" {
        return fmt.Errorf("WEBHOOK_URL is required")
    }
    
    // Валидация времени
    if _, err := time.Parse("15:04", c.Schedule.WorkStart); err != nil {
        return fmt.Errorf("invalid WORK_START format: %w", err)
    }
    if _, err := time.Parse("15:04", c.Schedule.WorkEnd); err != nil {
        return fmt.Errorf("invalid WORK_END format: %w", err)
    }
    
    return nil
}

func getEnv(key, fallback string) string {
    if v := os.Getenv(key); v != "" {
        return v
    }
    return fallback
}

func getEnvAsInt(key string, fallback int) int {
    if v := os.Getenv(key); v != "" {
        if i, err := strconv.Atoi(v); err == nil {
            return i
        }
    }
    return fallback
}
```

### Шаг 2.3: Реализация SQLite хранилища

**Время:** 4-5 часов

#### 2.3.1 Создать `internal/storage/sqlite/sqlite.go`

```go
package sqlite

import (
    "context"
    "database/sql"
    "fmt"
    "time"
    
    "telegram_queue_bot/internal/storage/models"
    _ "modernc.org/sqlite"
)

type SQLiteStorage struct {
    db *sql.DB
}

func New(dbPath string) (*SQLiteStorage, error) {
    db, err := sql.Open("sqlite", dbPath)
    if err != nil {
        return nil, fmt.Errorf("failed to open database: %w", err)
    }

    storage := &SQLiteStorage{db: db}
    
    if err := storage.migrate(); err != nil {
        return nil, fmt.Errorf("migration failed: %w", err)
    }

    return storage, nil
}

func (s *SQLiteStorage) migrate() error {
    // WAL mode для лучшей конкурентности
    if _, err := s.db.Exec(`PRAGMA journal_mode=WAL`); err != nil {
        return err
    }

    queries := []string{
        `CREATE TABLE IF NOT EXISTS users (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            chat_id INTEGER UNIQUE,
            phone TEXT,
            first_name TEXT,
            last_name TEXT,
            created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
            updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
        )`,
        `CREATE TABLE IF NOT EXISTS slots (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            date TEXT,
            start_time TEXT,
            end_time TEXT,
            user_chat_id INTEGER,
            notified INTEGER DEFAULT 0,
            created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
            updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
            UNIQUE(date, start_time),
            FOREIGN KEY(user_chat_id) REFERENCES users(chat_id)
        )`,
    }

    for _, query := range queries {
        if _, err := s.db.Exec(query); err != nil {
            return fmt.Errorf("failed to execute migration: %w", err)
        }
    }

    return nil
}
```

### Шаг 2.4: Создание планировщика уведомлений

**Время:** 3-4 часа

#### 2.4.1 Создать `internal/scheduler/interfaces.go`

```go
package scheduler

import (
    "context"
    "telegram_queue_bot/internal/storage/models"
)

type NotificationScheduler interface {
    Schedule(ctx context.Context, slot *models.Slot) error
    Cancel(ctx context.Context, slotID int) error
    ReschedulePending(ctx context.Context) error
    Stop() error
}

type NotificationSender interface {
    SendNotification(ctx context.Context, chatID int64, message string) error
}
```

---

## 🔧 ЭТАП 3: ИСПРАВЛЕНИЕ БАГОВ (Приоритет: ВЫСОКИЙ)

### Шаг 3.1: Исправление race conditions

**Время:** 2-3 часа

#### 3.1.1 Создать thread-safe планировщик

```go
// internal/scheduler/memory/scheduler.go
type MemoryScheduler struct {
    timers map[int]*time.Timer
    mu     sync.RWMutex
    sender NotificationSender
    ctx    context.Context
    cancel context.CancelFunc
}

func (s *MemoryScheduler) Schedule(ctx context.Context, slot *models.Slot) error {
    s.mu.Lock()
    defer s.mu.Unlock()
    
    // Отменить существующий таймер если есть
    if timer, exists := s.timers[slot.ID]; exists {
        timer.Stop()
    }
    
    // Создать новый таймер
    timer := time.AfterFunc(delay, func() {
        s.handleNotification(slot)
    })
    
    s.timers[slot.ID] = timer
    return nil
}
```

### Шаг 3.2: Улучшение обработки ошибок

**Время:** 2-3 часа

#### 3.2.1 Создать `pkg/errors/errors.go`

```go
package errors

import "fmt"

type BotError struct {
    Code    string
    Message string
    Err     error
}

func (e *BotError) Error() string {
    if e.Err != nil {
        return fmt.Sprintf("%s: %s: %v", e.Code, e.Message, e.Err)
    }
    return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

var (
    ErrSlotAlreadyReserved = &BotError{
        Code:    "SLOT_RESERVED",
        Message: "slot is already reserved",
    }
    
    ErrUserNotRegistered = &BotError{
        Code:    "USER_NOT_REGISTERED", 
        Message: "user is not registered",
    }
)
```

---

## 🛡️ ЭТАП 4: БЕЗОПАСНОСТЬ И MIDDLEWARE (Приоритет: СРЕДНИЙ)

### Шаг 4.1: Rate Limiting

**Время:** 2-3 часа

#### 4.1.1 Создать `internal/server/middleware.go`

```go
package server

import (
    "net/http"
    "time"
    
    "golang.org/x/time/rate"
)

func RateLimitMiddleware(requests int, duration time.Duration) func(http.Handler) http.Handler {
    limiter := rate.NewLimiter(rate.Every(duration), requests)
    
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            if !limiter.Allow() {
                http.Error(w, "too many requests", http.StatusTooManyRequests)
                return
            }
            next.ServeHTTP(w, r)
        })
    }
}
```

### Шаг 4.2: Валидация входных данных

**Время:** 1-2 часа

#### 4.2.1 Создать валидаторы

```go
// internal/bot/validation.go
func ValidateSlotID(idStr string) (int, error) {
    id, err := strconv.Atoi(idStr)
    if err != nil {
        return 0, fmt.Errorf("invalid slot ID: %w", err)
    }
    if id <= 0 {
        return 0, fmt.Errorf("slot ID must be positive")
    }
    return id, nil
}
```

---

## 🧪 ЭТАП 5: ТЕСТИРОВАНИЕ (Приоритет: СРЕДНИЙ)

### Шаг 5.1: Создание тестовой инфраструктуры

**Время:** 3-4 часа

#### 5.1.1 Создать `tests/testutils/testutils.go`

```go
package testutils

import (
    "context"
    "database/sql"
    "testing"
    "telegram_queue_bot/internal/storage/sqlite"
)

func SetupTestDB(t *testing.T) *sqlite.SQLiteStorage {
    storage, err := sqlite.New(":memory:")
    if err != nil {
        t.Fatalf("failed to create test database: %v", err)
    }
    
    t.Cleanup(func() {
        storage.Close()
    })
    
    return storage
}
```

### Шаг 5.2: Юнит-тесты для storage

**Время:** 4-5 часов

#### 5.2.1 Создать `tests/unit/storage_test.go`

```go
package unit

import (
    "context"
    "testing"
    "telegram_queue_bot/tests/testutils"
)

func TestUserRepository_SaveUser(t *testing.T) {
    storage := testutils.SetupTestDB(t)
    ctx := context.Background()
    
    err := storage.SaveUser(ctx, 12345, "+1234567890", "John", "Doe")
    if err != nil {
        t.Fatalf("expected no error, got %v", err)
    }
    
    registered := storage.IsUserRegistered(ctx, 12345)
    if !registered {
        t.Fatal("expected user to be registered")
    }
}
```

---

## 🔧 ЭТАП 6: РЕФАКТОРИНГ ОСНОВНОГО ФАЙЛА (Приоритет: ВЫСОКИЙ)

### Шаг 6.1: Создание сервиса бота

**Время:** 4-5 часов

#### 6.1.1 Создать `internal/bot/service.go`

```go
package bot

import (
    "context"
    "telegram_queue_bot/internal/config"
    "telegram_queue_bot/internal/storage"
    "telegram_queue_bot/internal/scheduler"
    
    "github.com/go-telegram/bot"
)

type Service struct {
    bot       *bot.Bot
    storage   storage.Storage
    scheduler scheduler.NotificationScheduler
    config    *config.Config
}

func NewService(
    bot *bot.Bot,
    storage storage.Storage,
    scheduler scheduler.NotificationScheduler,
    config *config.Config,
) *Service {
    return &Service{
        bot:       bot,
        storage:   storage,
        scheduler: scheduler,
        config:    config,
    }
}
```

### Шаг 6.2: Миграция обработчиков

**Время:** 5-6 часов

#### 6.2.1 Создать `internal/bot/handlers/start.go`

```go
package handlers

import (
    "context"
    "strings"
    
    "github.com/go-telegram/bot"
    "github.com/go-telegram/bot/models"
)

type StartHandler struct {
    service *bot.Service
}

func NewStartHandler(service *bot.Service) *StartHandler {
    return &StartHandler{service: service}
}

func (h *StartHandler) Handle(ctx context.Context, bot *bot.Bot, update *models.Update) {
    if update.Message == nil || !strings.HasPrefix(update.Message.Text, "/start") {
        return
    }
    
    chatID := update.Message.Chat.ID
    
    registered, err := h.service.IsUserRegistered(ctx, chatID)
    if err != nil {
        h.service.SendError(ctx, chatID, "Произошла ошибка")
        return
    }
    
    if registered {
        h.handleRegisteredUser(ctx, chatID)
    } else {
        h.handleNewUser(ctx, chatID)
    }
}
```

### Шаг 6.3: Создание новой точки входа

**Время:** 2-3 часа

#### 6.3.1 Создать `cmd/server/main.go`

```go
package main

import (
    "context"
    "log"
    "os"
    "os/signal"
    "syscall"
    
    "telegram_queue_bot/internal/config"
    "telegram_queue_bot/internal/storage/sqlite"
    "telegram_queue_bot/internal/bot"
    "telegram_queue_bot/internal/server"
)

func main() {
    // Загрузка конфигурации
    cfg, err := config.Load()
    if err != nil {
        log.Fatal("Failed to load config:", err)
    }
    
    // Инициализация хранилища
    storage, err := sqlite.New(cfg.Database.Path)
    if err != nil {
        log.Fatal("Failed to initialize storage:", err)
    }
    defer storage.Close()
    
    // Инициализация бота
    botService, err := bot.NewService(cfg, storage)
    if err != nil {
        log.Fatal("Failed to initialize bot:", err)
    }
    
    // Инициализация сервера
    srv := server.New(cfg, botService)
    
    // Graceful shutdown
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()
    
    go func() {
        sigChan := make(chan os.Signal, 1)
        signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
        <-sigChan
        cancel()
    }()
    
    if err := srv.Run(ctx); err != nil {
        log.Fatal("Server error:", err)
    }
}
```

---

## 🔧 ЭТАП 7: ФИНАЛЬНЫЕ УЛУЧШЕНИЯ (Приоритет: НИЗКИЙ)

### Шаг 7.1: Docker контейнеризация

**Время:** 2-3 часа

#### 7.1.1 Создать `Dockerfile`

```dockerfile
FROM golang:1.23-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o queue_bot cmd/server/main.go

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/

COPY --from=builder /app/queue_bot .
COPY --from=builder /app/.env.example .env

CMD ["./queue_bot"]
```

### Шаг 7.2: Мониторинг и метрики

**Время:** 3-4 часа

#### 7.2.1 Добавить Prometheus метрики

```go
// pkg/metrics/metrics.go
var (
    RequestsTotal = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "telegram_bot_requests_total",
            Help: "Total number of requests",
        },
        []string{"handler", "status"},
    )
)
```

---

## 📋 ЧЕКЛИСТ ВЫПОЛНЕНИЯ

### Этап 1: Подготовка

- [x] Создан backup branch
- [x] Создана новая структура проекта
- [x] Проанализированы зависимости

### Этап 2: Критические исправления

- [x] Созданы базовые интерфейсы
- [x] Реализован конфигурационный модуль
- [x] Реализовано SQLite хранилище
- [x] Создан планировщик уведомлений

### Этап 3: Исправление багов

- [x] Исправлены race conditions
- [x] Улучшена обработка ошибок
- [x] Добавлена валидация входных данных

### Этап 4: Безопасность

- [ ] Добавлен rate limiting
- [ ] Реализована валидация данных
- [ ] Добавлено структурированное логирование

### Этап 5: Тестирование

- [ ] Создана тестовая инфраструктура
- [ ] Написаны юнит-тесты
- [ ] Написаны интеграционные тесты

### Этап 6: Рефакторинг основного файла

- [ ] Создан сервис бота
- [ ] Мигрированы обработчики
- [ ] Создана новая точка входа

### Этап 7: Финальные улучшения

- [ ] Добавлена контейнеризация
- [ ] Реализован мониторинг
- [ ] Добавлен graceful shutdown

---

## ⏱️ ВРЕМЕННЫЕ ОЦЕНКИ

| Этап | Время | Сложность |
|------|-------|-----------|
| Этап 1: Подготовка | 4-6 часов | Низкая |
| Этап 2: Критические исправления | 12-15 часов | Высокая |
| Этап 3: Исправление багов | 6-8 часов | Средняя |
| Этап 4: Безопасность | 4-6 часов | Средняя |
| Этап 5: Тестирование | 8-10 часов | Высокая |
| Этап 6: Рефакторинг основного файла | 12-15 часов | Высокая |
| Этап 7: Финальные улучшения | 6-8 часов | Низкая |

**Общее время:** 52-68 часов (7-9 рабочих дней)

---

## 🚀 КРИТЕРИИ ГОТОВНОСТИ

### MVP (Минимально жизнеспособный продукт)

- ✅ Разделение на модули
- ✅ Базовые интерфейсы
- ✅ Исправлены race conditions
- ✅ Добавлены базовые тесты

### Production-ready

- ✅ Все MVP критерии
- ✅ Rate limiting
- ✅ Полное покрытие тестами
- ✅ Мониторинг и метрики
- ✅ Docker контейнеризация

### Enterprise-ready

- ✅ Все Production-ready критерии
- ✅ Distributed scheduler
- ✅ Database migrations
- ✅ High availability setup
- ✅ Performance optimization

---

## ⚠️ РИСКИ И ПРЕДУПРЕЖДЕНИЯ

1. **Совместимость:** Новая архитектура может сломать существующие интеграции
2. **Миграция данных:** Потребуется миграция существующих данных
3. **Время простоя:** Возможен простой сервиса при развертывании
4. **Сложность:** Увеличение сложности кодовой базы

## 💡 РЕКОМЕНДАЦИИ

1. **Поэтапное развертывание:** Реализовывать и тестировать каждый этап отдельно
2. **Feature flags:** Использовать для плавного перехода между версиями
3. **Monitoring:** Внимательно следить за метриками после каждого изменения
4. **Rollback plan:** Иметь план отката на случай проблем

---

**Автор:** GitHub Copilot  
**Дата создания:** 4 августа 2025  
**Версия:** 1.0
