package testutils

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"telegram_queue_bot/internal/config"
	"telegram_queue_bot/internal/scheduler/memory"
	"telegram_queue_bot/internal/server"
	"telegram_queue_bot/internal/storage/models"
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
	return logger.New(logger.LevelWarn) // Используем WARN для тестов чтобы уменьшить шум
}

// TestContext создает контекст для тестов
func TestContext() context.Context {
	return context.Background()
}

// SetupTestConfig создает тестовую конфигурацию
func SetupTestConfig() *config.Config {
	return &config.Config{
		Telegram: config.TelegramConfig{
			Token:       "test_token_123456:TEST-TOKEN-FOR-TESTING",
			WebhookURL:  "https://test.example.com/webhook",
			SecretToken: "test_secret_token_for_testing",
		},
		Server: config.ServerConfig{
			Port: "8080",
		},
		Database: config.DatabaseConfig{
			Path: ":memory:",
		},
		Schedule: config.ScheduleConfig{
			WorkStart:        "09:00",
			WorkEnd:          "18:00",
			SlotDurationMins: 30,
			ScheduleDays:     7,
		},
	}
}

// SetupTestServer создает тестовый HTTP сервер
func SetupTestServer(t *testing.T) (*server.Server, *config.Config) {
	cfg := SetupTestConfig()
	log := SetupTestLogger()

	// Для тестов создаем пустые заглушки
	srv := server.New(cfg, *log, nil, nil)

	return srv, cfg
}

// CreateTestUser создает тестового пользователя
func CreateTestUser(t *testing.T, storage *sqlite.SQLiteStorage, chatID int64) *models.User {
	ctx := TestContext()

	phone := fmt.Sprintf("+123456789%d", chatID%100)
	firstName := fmt.Sprintf("TestUser%d", chatID)
	lastName := "TestLastName"

	err := storage.SaveUser(ctx, chatID, phone, firstName, lastName)
	if err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}

	user, err := storage.GetUserByID(ctx, chatID)
	if err != nil {
		t.Fatalf("failed to get test user: %v", err)
	}

	return user
}

// CreateTestSlot создает тестовый слот
func CreateTestSlot(t *testing.T, storage *sqlite.SQLiteStorage, date, startTime, endTime string) *models.Slot {
	ctx := TestContext()

	slot := &models.Slot{
		Date:      date,
		StartTime: startTime,
		EndTime:   endTime,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	err := storage.CreateSlot(ctx, slot)
	if err != nil {
		t.Fatalf("failed to create test slot: %v", err)
	}

	return slot
}

// CreateReservedTestSlot создает зарезервированный тестовый слот
func CreateReservedTestSlot(t *testing.T, storage *sqlite.SQLiteStorage, chatID int64, date, startTime, endTime string) *models.Slot {
	slot := CreateTestSlot(t, storage, date, startTime, endTime)

	ctx := TestContext()
	err := storage.ReserveSlot(ctx, slot.ID, chatID)
	if err != nil {
		t.Fatalf("failed to reserve test slot: %v", err)
	}

	// Получаем обновленный слот
	updatedSlot, err := storage.GetSlotByID(ctx, slot.ID)
	if err != nil {
		t.Fatalf("failed to get updated slot: %v", err)
	}

	return updatedSlot
}

// SetupTestScheduler создает тестовый планировщик
func SetupTestScheduler(t *testing.T) *memory.MemoryScheduler {
	mockSender := &MockNotificationSender{}
	scheduler := memory.NewMemoryScheduler(mockSender)

	t.Cleanup(func() {
		scheduler.Stop()
	})

	return scheduler
}

// MockNotificationSender является mock реализацией NotificationSender для тестов
type MockNotificationSender struct {
	SentNotifications []MockNotification
}

type MockNotification struct {
	ChatID  int64
	Message string
	Slot    *models.Slot
}

func (m *MockNotificationSender) SendNotification(ctx context.Context, chatID int64, message string) error {
	m.SentNotifications = append(m.SentNotifications, MockNotification{
		ChatID:  chatID,
		Message: message,
	})
	return nil
}

func (m *MockNotificationSender) SendSlotReminder(ctx context.Context, slot *models.Slot) error {
	if slot.UserChatID != nil {
		m.SentNotifications = append(m.SentNotifications, MockNotification{
			ChatID: *slot.UserChatID,
			Slot:   slot,
		})
	}
	return nil
}

// CreateHTTPTestRequest создает HTTP запрос для тестирования
func CreateHTTPTestRequest(method, path string, body string, headers map[string]string) *http.Request {
	req := httptest.NewRequest(method, path, nil)

	if body != "" {
		req = httptest.NewRequest(method, path, nil)
		req.Header.Set("Content-Length", fmt.Sprintf("%d", len(body)))
	}

	// Устанавливаем заголовки
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	// Устанавливаем базовые заголовки для POST запросов
	if method == "POST" && req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}

	return req
}

// AssertNoError проверяет отсутствие ошибки
func AssertNoError(t *testing.T, err error, msg string) {
	if err != nil {
		t.Fatalf("%s: %v", msg, err)
	}
}

// AssertError проверяет наличие ошибки
func AssertError(t *testing.T, err error, msg string) {
	if err == nil {
		t.Fatalf("%s: expected error but got none", msg)
	}
}

// AssertEqual проверяет равенство значений
func AssertEqual(t *testing.T, expected, actual interface{}, msg string) {
	if expected != actual {
		t.Errorf("%s: expected %v, got %v", msg, expected, actual)
	}
}

// AssertNotEqual проверяет неравенство значений
func AssertNotEqual(t *testing.T, notExpected, actual interface{}, msg string) {
	if notExpected == actual {
		t.Errorf("%s: expected not to be %v, but got %v", msg, notExpected, actual)
	}
}

// AssertTrue проверяет истинность
func AssertTrue(t *testing.T, condition bool, msg string) {
	if !condition {
		t.Errorf("%s: expected true but got false", msg)
	}
}

// AssertFalse проверяет ложность
func AssertFalse(t *testing.T, condition bool, msg string) {
	if condition {
		t.Errorf("%s: expected false but got true", msg)
	}
}
