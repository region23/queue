package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// Config содержит всю конфигурацию приложения
type Config struct {
	Telegram TelegramConfig `json:"telegram"`
	Server   ServerConfig   `json:"server"`
	Database DatabaseConfig `json:"database"`
	Schedule ScheduleConfig `json:"schedule"`
}

// TelegramConfig содержит настройки Telegram бота
type TelegramConfig struct {
	Token      string `json:"token"`
	WebhookURL string `json:"webhook_url"`
}

// ServerConfig содержит настройки HTTP сервера
type ServerConfig struct {
	Port         string        `json:"port"`
	ReadTimeout  time.Duration `json:"read_timeout"`
	WriteTimeout time.Duration `json:"write_timeout"`
	IdleTimeout  time.Duration `json:"idle_timeout"`
}

// DatabaseConfig содержит настройки базы данных
type DatabaseConfig struct {
	Path           string        `json:"path"`
	MaxConnections int           `json:"max_connections"`
	ConnTimeout    time.Duration `json:"conn_timeout"`
}

// ScheduleConfig содержит настройки расписания
type ScheduleConfig struct {
	WorkStart         string `json:"work_start"`
	WorkEnd           string `json:"work_end"`
	SlotDurationMins  int    `json:"slot_duration_mins"`
	ScheduleDays      int    `json:"schedule_days"`
	NotificationMins  int    `json:"notification_mins"`
	CleanupAfterHours int    `json:"cleanup_after_hours"`
}

// Load загружает конфигурацию из переменных окружения
func Load() (*Config, error) {
	cfg := &Config{
		Telegram: TelegramConfig{
			Token:      os.Getenv("TELEGRAM_TOKEN"),
			WebhookURL: os.Getenv("WEBHOOK_URL"),
		},
		Server: ServerConfig{
			Port:         getEnv("PORT", "8080"),
			ReadTimeout:  getEnvAsDuration("SERVER_READ_TIMEOUT", 30*time.Second),
			WriteTimeout: getEnvAsDuration("SERVER_WRITE_TIMEOUT", 30*time.Second),
			IdleTimeout:  getEnvAsDuration("SERVER_IDLE_TIMEOUT", 120*time.Second),
		},
		Database: DatabaseConfig{
			Path:           getEnv("DB_FILE", "queue.db"),
			MaxConnections: getEnvAsInt("DB_MAX_CONNECTIONS", 10),
			ConnTimeout:    getEnvAsDuration("DB_CONN_TIMEOUT", 5*time.Second),
		},
		Schedule: ScheduleConfig{
			WorkStart:         getEnv("WORK_START", "09:00"),
			WorkEnd:           getEnv("WORK_END", "18:00"),
			SlotDurationMins:  getEnvAsInt("SLOT_DURATION", 30),
			ScheduleDays:      getEnvAsInt("SCHEDULE_DAYS", 7),
			NotificationMins:  getEnvAsInt("NOTIFICATION_MINS", 15),
			CleanupAfterHours: getEnvAsInt("CLEANUP_AFTER_HOURS", 24),
		},
	}

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	return cfg, nil
}

// Validate проверяет корректность конфигурации
func (c *Config) Validate() error {
	if c.Telegram.Token == "" {
		return fmt.Errorf("TELEGRAM_TOKEN is required")
	}
	if c.Telegram.WebhookURL == "" {
		return fmt.Errorf("WEBHOOK_URL is required")
	}

	// Валидация времени работы
	if _, err := time.Parse("15:04", c.Schedule.WorkStart); err != nil {
		return fmt.Errorf("invalid WORK_START format (expected HH:MM): %w", err)
	}
	if _, err := time.Parse("15:04", c.Schedule.WorkEnd); err != nil {
		return fmt.Errorf("invalid WORK_END format (expected HH:MM): %w", err)
	}

	// Проверка логичности временных настроек
	if c.Schedule.SlotDurationMins <= 0 {
		return fmt.Errorf("SLOT_DURATION must be positive")
	}
	if c.Schedule.ScheduleDays <= 0 {
		return fmt.Errorf("SCHEDULE_DAYS must be positive")
	}
	if c.Schedule.NotificationMins < 0 {
		return fmt.Errorf("NOTIFICATION_MINS must be non-negative")
	}

	return nil
}

// getEnv получает переменную окружения или возвращает значение по умолчанию
func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// getEnvAsInt получает переменную окружения как число
func getEnvAsInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return fallback
}

// getEnvAsDuration получает переменную окружения как duration
func getEnvAsDuration(key string, fallback time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return fallback
}
