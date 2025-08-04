package integration

import (
	"testing"
	"time"

	"github.com/region23/queue/internal/config"
	"github.com/region23/queue/internal/storage/models"
	"github.com/region23/queue/internal/storage/sqlite"
	"github.com/region23/queue/tests/testutils"
)

// TestStorageSchedulerIntegration тестирует интеграцию хранилища и планировщика
func TestStorageSchedulerIntegration(t *testing.T) {
	// Настраиваем компоненты
	storage := testutils.SetupTestDB(t)
	scheduler := testutils.SetupTestScheduler(t)
	ctx := testutils.TestContext()

	// Запускаем планировщик
	err := scheduler.Start(ctx)
	testutils.AssertNoError(t, err, "Scheduler should start")

	// Создаем пользователя
	chatID := int64(12345)
	user := testutils.CreateTestUser(t, storage, chatID)
	testutils.AssertEqual(t, chatID, user.ChatID, "User chat ID should match")

	// Создаем слот
	tomorrow := time.Now().Add(24 * time.Hour).Format("2006-01-02")
	slot := testutils.CreateTestSlot(t, storage, tomorrow, "10:00", "10:30")

	// Резервируем слот
	err = storage.ReserveSlot(ctx, slot.ID, chatID)
	testutils.AssertNoError(t, err, "Should reserve slot")

	// Получаем обновленный слот
	reservedSlot, err := storage.GetSlotByID(ctx, slot.ID)
	testutils.AssertNoError(t, err, "Should get reserved slot")
	testutils.AssertNotEqual(t, (*int64)(nil), reservedSlot.UserChatID, "Slot should have user")
	testutils.AssertEqual(t, chatID, *reservedSlot.UserChatID, "Slot should be reserved by correct user")

	// Планируем уведомление
	notifyAt := time.Now().Add(1 * time.Hour)
	err = scheduler.Schedule(ctx, reservedSlot, notifyAt)
	testutils.AssertNoError(t, err, "Should schedule notification")

	// Отменяем резервирование
	err = storage.CancelSlot(ctx, slot.ID, chatID)
	testutils.AssertNoError(t, err, "Should cancel slot reservation")

	// Отменяем уведомление
	err = scheduler.Cancel(ctx, slot.ID)
	testutils.AssertNoError(t, err, "Should cancel notification")
}

// TestFullSlotLifecycle тестирует полный жизненный цикл слота
func TestFullSlotLifecycle(t *testing.T) {
	storage := testutils.SetupTestDB(t)
	scheduler := testutils.SetupTestScheduler(t)
	ctx := testutils.TestContext()

	err := scheduler.Start(ctx)
	testutils.AssertNoError(t, err, "Scheduler should start")

	// 1. Создаем пользователя
	chatID := int64(12345)
	testutils.CreateTestUser(t, storage, chatID)

	registered, err := storage.IsUserRegistered(ctx, chatID)
	testutils.AssertNoError(t, err, "Should check user registration")
	testutils.AssertTrue(t, registered, "User should be registered")

	// 2. Создаем слоты на завтра
	tomorrow := time.Now().Add(24 * time.Hour).Format("2006-01-02")
	morningSlot := testutils.CreateTestSlot(t, storage, tomorrow, "09:00", "09:30")
	testutils.CreateTestSlot(t, storage, tomorrow, "14:00", "14:30") // afternoonSlot не используется далее

	// 3. Получаем доступные слоты
	availableSlots, err := storage.GetAvailableSlots(ctx, tomorrow)
	testutils.AssertNoError(t, err, "Should get available slots")
	testutils.AssertEqual(t, 2, len(availableSlots), "Should have 2 available slots")

	// 4. Резервируем утренний слот
	err = storage.ReserveSlot(ctx, morningSlot.ID, chatID)
	testutils.AssertNoError(t, err, "Should reserve morning slot")

	// 5. Проверяем что слотов стало меньше
	availableSlots, err = storage.GetAvailableSlots(ctx, tomorrow)
	testutils.AssertNoError(t, err, "Should get available slots after reservation")
	testutils.AssertEqual(t, 1, len(availableSlots), "Should have 1 available slot after reservation")

	// 6. Получаем активные слоты пользователя
	userSlots, err := storage.GetUserActiveSlots(ctx, chatID)
	testutils.AssertNoError(t, err, "Should get user active slots")
	testutils.AssertEqual(t, 1, len(userSlots), "User should have one active slot")
	userSlot := userSlots[0]
	testutils.AssertEqual(t, morningSlot.ID, userSlot.ID, "Should be the morning slot")

	// 7. Планируем уведомление
	notifyAt := time.Now().Add(30 * time.Minute)
	err = scheduler.Schedule(ctx, userSlot, notifyAt)
	testutils.AssertNoError(t, err, "Should schedule notification")

	// 8. Пытаемся зарезервировать уже зарезервированный слот (должна быть ошибка)
	err = storage.ReserveSlot(ctx, morningSlot.ID, int64(67890))
	testutils.AssertError(t, err, "Should not reserve already reserved slot")

	// 9. Отменяем резервирование
	err = storage.CancelSlot(ctx, morningSlot.ID, chatID)
	testutils.AssertNoError(t, err, "Should cancel slot reservation")

	// 10. Отменяем уведомление
	err = scheduler.Cancel(ctx, morningSlot.ID)
	testutils.AssertNoError(t, err, "Should cancel notification")

	// 11. Проверяем что слот снова доступен
	availableSlots, err = storage.GetAvailableSlots(ctx, tomorrow)
	testutils.AssertNoError(t, err, "Should get available slots after cancellation")
	testutils.AssertEqual(t, 2, len(availableSlots), "Should have 2 available slots after cancellation")

	// 12. Проверяем что у пользователя нет активных слотов
	userSlots, err = storage.GetUserActiveSlots(ctx, chatID)
	testutils.AssertNoError(t, err, "Should get user active slots after cancellation")
	testutils.AssertEqual(t, 0, len(userSlots), "User should not have active slots after cancellation")
}

// TestMultipleUsersIntegration тестирует работу с несколькими пользователями
func TestMultipleUsersIntegration(t *testing.T) {
	storage := testutils.SetupTestDB(t)
	scheduler := testutils.SetupTestScheduler(t)
	ctx := testutils.TestContext()

	err := scheduler.Start(ctx)
	testutils.AssertNoError(t, err, "Scheduler should start")

	// Создаем трех пользователей
	users := []int64{12345, 67890, 11111}
	for _, chatID := range users {
		testutils.CreateTestUser(t, storage, chatID)
		registered, err := storage.IsUserRegistered(ctx, chatID)
		testutils.AssertNoError(t, err, "Should check user registration")
		testutils.AssertTrue(t, registered, "User should be registered")
	}

	// Создаем слоты на завтра
	tomorrow := time.Now().Add(24 * time.Hour).Format("2006-01-02")
	slots := []*models.Slot{
		testutils.CreateTestSlot(t, storage, tomorrow, "09:00", "09:30"),
		testutils.CreateTestSlot(t, storage, tomorrow, "10:00", "10:30"),
		testutils.CreateTestSlot(t, storage, tomorrow, "11:00", "11:30"),
		testutils.CreateTestSlot(t, storage, tomorrow, "14:00", "14:30"),
	}

	// Каждый пользователь резервирует по слоту
	for i, chatID := range users {
		err = storage.ReserveSlot(ctx, slots[i].ID, chatID)
		testutils.AssertNoError(t, err, "Should reserve slot for user")

		// Планируем уведомления
		notifyAt := time.Now().Add(time.Duration(i+1) * time.Hour)
		reservedSlot, err := storage.GetSlotByID(ctx, slots[i].ID)
		testutils.AssertNoError(t, err, "Should get reserved slot")

		err = scheduler.Schedule(ctx, reservedSlot, notifyAt)
		testutils.AssertNoError(t, err, "Should schedule notification")
	}

	// Должен остаться один доступный слот
	availableSlots, err := storage.GetAvailableSlots(ctx, tomorrow)
	testutils.AssertNoError(t, err, "Should get available slots")
	testutils.AssertEqual(t, 1, len(availableSlots), "Should have 1 available slot")

	// Каждый пользователь должен иметь по одному активному слоту
	for _, chatID := range users {
		userSlots, err := storage.GetUserActiveSlots(ctx, chatID)
		testutils.AssertNoError(t, err, "Should get user active slots")
		testutils.AssertEqual(t, 1, len(userSlots), "User should have one active slot")
		testutils.AssertEqual(t, chatID, *userSlots[0].UserChatID, "Slot should belong to correct user")
	}

	// Первый пользователь отменяет свое резервирование
	firstUser := users[0]
	firstSlot := slots[0]

	err = storage.CancelSlot(ctx, firstSlot.ID, firstUser)
	testutils.AssertNoError(t, err, "Should cancel slot for first user")

	err = scheduler.Cancel(ctx, firstSlot.ID)
	testutils.AssertNoError(t, err, "Should cancel notification for first user")

	// Должно стать два доступных слота
	availableSlots, err = storage.GetAvailableSlots(ctx, tomorrow)
	testutils.AssertNoError(t, err, "Should get available slots after cancellation")
	testutils.AssertEqual(t, 2, len(availableSlots), "Should have 2 available slots after cancellation")

	// У первого пользователя не должно быть активного слота
	userSlots, err := storage.GetUserActiveSlots(ctx, firstUser)
	testutils.AssertNoError(t, err, "Should get first user active slots")
	testutils.AssertEqual(t, 0, len(userSlots), "First user should not have active slots")

	// У остальных пользователей слоты должны остаться
	for _, chatID := range users[1:] {
		userSlots, err := storage.GetUserActiveSlots(ctx, chatID)
		testutils.AssertNoError(t, err, "Should get user active slots")
		testutils.AssertEqual(t, 1, len(userSlots), "User should still have active slot")
	}
}

// TestConfigStorageIntegration тестирует интеграцию конфигурации с хранилищем
func TestConfigStorageIntegration(t *testing.T) {
	// Создаем конфигурацию с кастомными настройками
	cfg := &config.Config{
		Telegram: config.TelegramConfig{
			Token:       "123456:ABC-DEF1234ghIkl-zyx57W2v1u123ew11",
			WebhookURL:  "https://test.example.com/webhook",
			SecretToken: "test_secret",
		},
		Server: config.ServerConfig{
			Port: "9000",
		},
		Database: config.DatabaseConfig{
			Path:           ":memory:",
			MaxConnections: 10,
			ConnTimeout:    30 * time.Second,
		},
		Schedule: config.ScheduleConfig{
			WorkStart:         "08:00",
			WorkEnd:           "20:00",
			SlotDurationMins:  60,
			ScheduleDays:      14,
			NotificationMins:  15,
			CleanupAfterHours: 48,
		},
	}

	// Валидируем конфигурацию
	err := cfg.Validate()
	testutils.AssertNoError(t, err, "Configuration should be valid")

	// Создаем хранилище с настройками из конфигурации
	storage, err := sqlite.New(cfg.Database.Path)
	testutils.AssertNoError(t, err, "Should create storage from config")
	defer storage.Close()

	ctx := testutils.TestContext()

	// Тестируем что настройки расписания работают
	testUser := testutils.CreateTestUser(t, storage, 12345)
	testutils.AssertEqual(t, int64(12345), testUser.ChatID, "User should be created")

	// Проверяем работу с настройками расписания
	today := time.Now().Format("2006-01-02")

	// Создаем слот с продолжительностью из конфигурации
	workStart, err := time.Parse("15:04", cfg.Schedule.WorkStart)
	testutils.AssertNoError(t, err, "Should parse work start time")

	workEnd := workStart.Add(time.Duration(cfg.Schedule.SlotDurationMins) * time.Minute)

	slot := &models.Slot{
		Date:      today,
		StartTime: workStart.Format("15:04"),
		EndTime:   workEnd.Format("15:04"),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	err = storage.CreateSlot(ctx, slot)
	testutils.AssertNoError(t, err, "Should create slot with config duration")

	// Проверяем что слот создался с правильными параметрами
	testutils.AssertEqual(t, cfg.Schedule.WorkStart, slot.StartTime, "Start time should match config")
}

// TestNotificationIntegration тестирует интеграцию уведомлений
func TestNotificationIntegration(t *testing.T) {
	storage := testutils.SetupTestDB(t)
	scheduler := testutils.SetupTestScheduler(t)
	ctx := testutils.TestContext()

	err := scheduler.Start(ctx)
	testutils.AssertNoError(t, err, "Scheduler should start")

	// Создаем пользователя и слот
	chatID := int64(12345)
	testutils.CreateTestUser(t, storage, chatID)

	tomorrow := time.Now().Add(24 * time.Hour).Format("2006-01-02")
	slot := testutils.CreateReservedTestSlot(t, storage, chatID, tomorrow, "10:00", "10:30")

	// Планируем уведомление на близкое время
	notifyAt := time.Now().Add(100 * time.Millisecond)
	err = scheduler.Schedule(ctx, slot, notifyAt)
	testutils.AssertNoError(t, err, "Should schedule notification")

	// Ждем немного чтобы уведомление могло сработать
	time.Sleep(200 * time.Millisecond)

	// Отменяем уведомление
	err = scheduler.Cancel(ctx, slot.ID)
	testutils.AssertNoError(t, err, "Should cancel notification")
}

// TestDatabaseMigrationIntegration тестирует миграции базы данных
func TestDatabaseMigrationIntegration(t *testing.T) {
	// Создаем временный путь к БД
	dbPath := ":memory:"

	// Создаем хранилище (должно выполнить миграции)
	storage, err := sqlite.New(dbPath)
	testutils.AssertNoError(t, err, "Should create storage and run migrations")
	defer storage.Close()

	ctx := testutils.TestContext()

	// Тестируем что все таблицы созданы корректно
	testUser := testutils.CreateTestUser(t, storage, 12345)
	testutils.AssertNotEqual(t, (*models.User)(nil), testUser, "Should create user after migration")

	testSlot := testutils.CreateTestSlot(t, storage, "2025-08-05", "10:00", "10:30")
	testutils.AssertNotEqual(t, (*models.Slot)(nil), testSlot, "Should create slot after migration")

	// Тестируем foreign key constraints
	err = storage.ReserveSlot(ctx, testSlot.ID, testUser.ChatID)
	testutils.AssertNoError(t, err, "Should reserve slot with valid user")

	// Попытка резервирования с несуществующим пользователем
	err = storage.ReserveSlot(ctx, testSlot.ID, 99999)
	testutils.AssertError(t, err, "Should not reserve slot with invalid user")
}
