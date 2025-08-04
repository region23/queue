package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Config holds application configuration
type Config struct {
	TelegramToken string
	WebhookURL    string
	ServerAddress string
	DBFile        string
	WorkStart     string
	WorkEnd       string
	SlotDuration  int
	AdminIDs      []int64
}

// LoadConfig loads configuration from environment variables
func LoadConfig() (*Config, error) {
	config := &Config{
		TelegramToken: getEnv("TELEGRAM_TOKEN", ""),
		WebhookURL:    getEnv("WEBHOOK_URL", ""),
		ServerAddress: getEnv("SERVER_ADDRESS", ":8080"),
		DBFile:        getEnv("DB_FILE", "queue.db"),
		WorkStart:     getEnv("WORK_START", "09:00"),
		WorkEnd:       getEnv("WORK_END", "18:00"),
		SlotDuration:  getEnvInt("SLOT_DURATION", 30),
	}

	// Parse admin IDs
	adminIDsStr := getEnv("ADMIN_IDS", "")
	if adminIDsStr != "" {
		for _, idStr := range strings.Split(adminIDsStr, ",") {
			if id, err := strconv.ParseInt(strings.TrimSpace(idStr), 10, 64); err == nil {
				config.AdminIDs = append(config.AdminIDs, id)
			}
		}
	}

	// Validate required fields
	if config.TelegramToken == "" {
		return nil, fmt.Errorf("TELEGRAM_TOKEN is required")
	}
	if config.WebhookURL == "" {
		return nil, fmt.Errorf("WEBHOOK_URL is required")
	}

	return config, nil
}

// getEnv gets environment variable with default value
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getEnvInt gets environment variable as int with default value
func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intVal, err := strconv.Atoi(value); err == nil {
			return intVal
		}
	}
	return defaultValue
}

// IsAdmin checks if user is admin
func IsAdmin(config *Config, userID int64) bool {
	for _, adminID := range config.AdminIDs {
		if adminID == userID {
			return true
		}
	}
	return false
}