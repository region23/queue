package testutils

import (
	"context"
	"testing"

	"telegram_queue_bot/internal/storage/sqlite"
	"telegram_queue_bot/pkg/logger"
)

// SetupTestDB создает in-memory SQLite базу данных для тестов
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

// SetupTestLogger создает тестовый логгер
func SetupTestLogger() *logger.Logger {
	return logger.New(logger.LevelDebug)
}

// TestContext создает контекст для тестов
func TestContext() context.Context {
	return context.Background()
}
