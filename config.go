package main

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
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
	ScheduleDays  int
	SkipWeekend   bool
	AdminIDs      []int64
	RateLimit     int // Requests per minute
	SlotsPerRow   int // Number of time slot buttons per row
}

// LoadConfig loads configuration from environment variables and .env file
func LoadConfig() (*Config, error) {
	// Load .env file if it exists
	if err := godotenv.Load(); err != nil {
		log.Printf("Warning: .env file not found or could not be loaded: %v", err)
	} else {
		log.Println("Loaded environment variables from .env file")
	}

	config := &Config{
		TelegramToken: os.Getenv("TELEGRAM_TOKEN"),
		WebhookURL:    os.Getenv("WEBHOOK_URL"),
		ServerAddress: getEnvOrDefault("SERVER_ADDRESS", ":8080"),
		DBFile:        getEnvOrDefault("DB_FILE", "queue.db"),
		WorkStart:     getEnvOrDefault("WORK_START", "09:00"),
		WorkEnd:       getEnvOrDefault("WORK_END", "18:00"),
		SlotDuration:  getEnvIntOrDefault("SLOT_DURATION", 30),
		ScheduleDays:  getEnvIntOrDefault("SCHEDULE_DAYS", 1),
		SkipWeekend:   getEnvBoolOrDefault("SKIP_WEEKEND", true),
		RateLimit:     getEnvIntOrDefault("RATE_LIMIT", 60),
		SlotsPerRow:   getEnvIntOrDefault("SLOTS_PER_ROW", 3),
	}

	// Parse admin IDs
	adminIDsStr := os.Getenv("ADMIN_IDS")
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

// getEnvOrDefault gets environment variable with default value
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getEnvIntOrDefault gets environment variable as int with default value
func getEnvIntOrDefault(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intVal, err := strconv.Atoi(value); err == nil {
			return intVal
		}
	}
	return defaultValue
}

// getEnvBoolOrDefault gets environment variable as bool with default value
func getEnvBoolOrDefault(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		return value == "1" || strings.ToLower(value) == "true"
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
