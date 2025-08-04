package unit

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"telegram_queue_bot/internal/config"
	"telegram_queue_bot/pkg/logger"
)

func TestSecurityMiddleware(t *testing.T) {
	// Настройка
	cfg := &config.Config{
		Telegram: config.TelegramConfig{
			Token:       "test_token",
			WebhookURL:  "https://example.com/webhook",
			SecretToken: "test_secret",
		},
		Server: config.ServerConfig{
			Port: "8080",
		},
	}

	log := logger.New(logger.LevelInfo)
	server := New(cfg, log)

	tests := []struct {
		name           string
		method         string
		path           string
		body           string
		headers        map[string]string
		expectedStatus int
		expectedBlock  bool
	}{
		{
			name:           "Valid health check",
			method:         "GET",
			path:           "/health",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "Invalid method for webhook",
			method:         "GET",
			path:           "/webhook",
			expectedStatus: http.StatusMethodNotAllowed,
			expectedBlock:  true,
		},
		{
			name:           "Valid POST to webhook",
			method:         "POST",
			path:           "/webhook",
			body:           `{"update_id": 1, "message": {"message_id": 1, "from": {"id": 123, "is_bot": false, "first_name": "Test"}, "chat": {"id": 123, "type": "private"}, "date": ` + string(rune(time.Now().Unix())) + `, "text": "test"}}`,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "Invalid JSON",
			method:         "POST",
			path:           "/webhook",
			body:           `{"invalid": json}`,
			expectedStatus: http.StatusBadRequest,
			expectedBlock:  true,
		},
		{
			name:   "Suspicious User-Agent",
			method: "POST",
			path:   "/webhook",
			body:   `{"update_id": 1}`,
			headers: map[string]string{
				"User-Agent": "python-requests/2.0 bot scanner",
			},
			expectedStatus: http.StatusBadRequest,
			expectedBlock:  true,
		},
		{
			name:           "Large request body",
			method:         "POST",
			path:           "/webhook",
			body:           strings.Repeat("a", 5*1024*1024), // 5MB
			expectedStatus: http.StatusRequestEntityTooLarge,
			expectedBlock:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, bytes.NewBufferString(tt.body))

			// Устанавливаем заголовки
			if tt.headers != nil {
				for key, value := range tt.headers {
					req.Header.Set(key, value)
				}
			}

			// Устанавливаем Content-Type для POST запросов
			if tt.method == "POST" {
				req.Header.Set("Content-Type", "application/json")
			}

			recorder := httptest.NewRecorder()
			server.httpServer.Handler.ServeHTTP(recorder, req)

			if recorder.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, recorder.Code)
			}
		})
	}
}

func TestRateLimiting(t *testing.T) {
	cfg := &config.Config{
		Telegram: config.TelegramConfig{
			Token:      "test_token",
			WebhookURL: "https://example.com/webhook",
		},
		Server: config.ServerConfig{
			Port: "8080",
		},
	}

	log := logger.New(logger.LevelWarn) // Снижаем уровень логирования для тестов
	server := New(cfg, log)

	// Тестируем rate limiting
	req := httptest.NewRequest("GET", "/health", nil)
	req.RemoteAddr = "127.0.0.1:12345"

	// Отправляем множество запросов
	successCount := 0
	rateLimitedCount := 0

	for i := 0; i < 120; i++ { // Больше лимита (100 в минуту)
		recorder := httptest.NewRecorder()
		server.httpServer.Handler.ServeHTTP(recorder, req)

		switch recorder.Code {
		case http.StatusOK:
			successCount++
		case http.StatusTooManyRequests:
			rateLimitedCount++
		}
	}

	if rateLimitedCount == 0 {
		t.Error("Expected some requests to be rate limited")
	}

	if successCount == 0 {
		t.Error("Expected some requests to succeed")
	}

	t.Logf("Successful requests: %d, Rate limited: %d", successCount, rateLimitedCount)
}

func TestSecurityHeaders(t *testing.T) {
	cfg := &config.Config{
		Telegram: config.TelegramConfig{
			Token:      "test_token",
			WebhookURL: "https://example.com/webhook",
		},
		Server: config.ServerConfig{
			Port: "8080",
		},
	}

	log := logger.New(logger.LevelWarn)
	server := New(cfg, log)

	req := httptest.NewRequest("GET", "/health", nil)
	recorder := httptest.NewRecorder()

	server.httpServer.Handler.ServeHTTP(recorder, req)

	expectedHeaders := map[string]string{
		"X-Content-Type-Options": "nosniff",
		"X-Frame-Options":        "DENY",
		"X-XSS-Protection":       "1; mode=block",
	}

	for header, expectedValue := range expectedHeaders {
		actualValue := recorder.Header().Get(header)
		if actualValue != expectedValue {
			t.Errorf("Expected header %s: %s, got: %s", header, expectedValue, actualValue)
		}
	}
}

func TestTelegramAuthValidation(t *testing.T) {
	cfg := &config.Config{
		Telegram: config.TelegramConfig{
			Token:       "test_token",
			WebhookURL:  "https://example.com/webhook",
			SecretToken: "test_secret",
		},
		Server: config.ServerConfig{
			Port: "8080",
		},
	}

	log := logger.New(logger.LevelWarn)
	server := New(cfg, log)

	tests := []struct {
		name           string
		secretToken    string
		body           string
		expectedStatus int
	}{
		{
			name:           "Valid secret token",
			secretToken:    "test_secret",
			body:           `{"update_id": 1, "message": {"message_id": 1, "from": {"id": 123, "is_bot": false, "first_name": "Test"}, "chat": {"id": 123, "type": "private"}, "date": ` + string(rune(time.Now().Unix())) + `, "text": "test"}}`,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "Invalid secret token",
			secretToken:    "wrong_secret",
			body:           `{"update_id": 1}`,
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:           "Missing secret token",
			secretToken:    "",
			body:           `{"update_id": 1}`,
			expectedStatus: http.StatusUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/webhook", bytes.NewBufferString(tt.body))
			req.Header.Set("Content-Type", "application/json")

			if tt.secretToken != "" {
				req.Header.Set("X-Telegram-Bot-Api-Secret-Token", tt.secretToken)
			}

			recorder := httptest.NewRecorder()
			server.httpServer.Handler.ServeHTTP(recorder, req)

			if recorder.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, recorder.Code)
			}
		})
	}
}

func TestRequestValidation(t *testing.T) {
	cfg := &config.Config{
		Telegram: config.TelegramConfig{
			Token:      "test_token",
			WebhookURL: "https://example.com/webhook",
		},
		Server: config.ServerConfig{
			Port: "8080",
		},
	}

	log := logger.New(logger.LevelWarn)
	validator := NewRequestValidator(log)

	tests := []struct {
		name          string
		body          string
		valid         bool
		errorContains string
	}{
		{
			name: "Valid message update",
			body: `{
				"update_id": 1,
				"message": {
					"message_id": 1,
					"from": {"id": 123, "is_bot": false, "first_name": "Test"},
					"chat": {"id": 123, "type": "private"},
					"date": ` + string(rune(time.Now().Unix())) + `,
					"text": "Hello"
				}
			}`,
			valid: true,
		},
		{
			name: "Invalid update_id",
			body: `{
				"update_id": 0,
				"message": {
					"message_id": 1,
					"from": {"id": 123, "is_bot": false, "first_name": "Test"},
					"chat": {"id": 123, "type": "private"},
					"date": ` + string(rune(time.Now().Unix())) + `,
					"text": "Hello"
				}
			}`,
			valid:         false,
			errorContains: "update_id",
		},
		{
			name: "Bot message (should be rejected)",
			body: `{
				"update_id": 1,
				"message": {
					"message_id": 1,
					"from": {"id": 123, "is_bot": true, "first_name": "Bot"},
					"chat": {"id": 123, "type": "private"},
					"date": ` + string(rune(time.Now().Unix())) + `,
					"text": "Hello"
				}
			}`,
			valid:         false,
			errorContains: "bot",
		},
		{
			name: "Group chat (should be rejected)",
			body: `{
				"update_id": 1,
				"message": {
					"message_id": 1,
					"from": {"id": 123, "is_bot": false, "first_name": "Test"},
					"chat": {"id": -456, "type": "group"},
					"date": ` + string(rune(time.Now().Unix())) + `,
					"text": "Hello"
				}
			}`,
			valid:         false,
			errorContains: "chat_type",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/webhook", bytes.NewBufferString(tt.body))
			req.Header.Set("Content-Type", "application/json")

			update, err := validator.ValidateWebhookRequest(req)

			if tt.valid {
				if err != nil {
					t.Errorf("Expected valid request, got error: %v", err)
				}
				if update == nil {
					t.Error("Expected update to be parsed")
				}
			} else {
				if err == nil {
					t.Error("Expected validation error")
				}
				if tt.errorContains != "" && !strings.Contains(err.Error(), tt.errorContains) {
					t.Errorf("Expected error to contain '%s', got: %v", tt.errorContains, err)
				}
			}
		})
	}
}

func TestSecurityConfig(t *testing.T) {
	cfg := &config.Config{
		Telegram: config.TelegramConfig{
			Token:       "test_token",
			SecretToken: "test_secret",
		},
	}

	secConfig := LoadSecurityConfig(cfg)

	// Тест валидации конфигурации
	if err := secConfig.ValidateSecurityConfig(); err != nil {
		t.Errorf("Security config validation failed: %v", err)
	}

	// Тест проверки IP
	if !secConfig.IsIPAllowed("127.0.0.1") {
		t.Error("Should allow localhost by default")
	}

	// Тест получения заголовков безопасности
	headers := secConfig.GetSecurityHeaders()
	if len(headers) == 0 {
		t.Error("Should return security headers")
	}

	// Тест валидации User-Agent
	if secConfig.IsValidUserAgent("normal browser") != true {
		t.Error("Should allow normal user agents")
	}

	if secConfig.IsValidUserAgent("python-requests bot") != false {
		t.Error("Should block suspicious user agents")
	}
}

func TestGracefulShutdown(t *testing.T) {
	cfg := &config.Config{
		Telegram: config.TelegramConfig{
			Token:      "test_token",
			WebhookURL: "https://example.com/webhook",
		},
		Server: config.ServerConfig{
			Port: "8080",
		},
	}

	log := logger.New(logger.LevelWarn)
	server := New(cfg, log)

	// Тестируем graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		t.Errorf("Graceful shutdown failed: %v", err)
	}
}
