package sqlite

import (
	"context"
	"testing"
	"time"

	"telegram_queue_bot/internal/storage/models"
)

func TestGetAvailableSlots_TimeFiltering(t *testing.T) {
	// Создаем временную базу данных
	storage, err := New(":memory:")
	if err != nil {
		t.Fatalf("Failed to create test storage: %v", err)
	}
	defer storage.Close()

	ctx := context.Background()
	today := time.Now().Format("2006-01-02")
	currentTime := time.Now()

	// Создаем слоты - один в прошлом, один в будущем
	pastSlot := &models.Slot{
		Date:      today,
		StartTime: currentTime.Add(-1 * time.Hour).Format("15:04"),
		EndTime:   currentTime.Add(-1 * time.Hour).Add(15 * time.Minute).Format("15:04"),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	futureSlot := &models.Slot{
		Date:      today,
		StartTime: currentTime.Add(1 * time.Hour).Format("15:04"),
		EndTime:   currentTime.Add(1 * time.Hour).Add(15 * time.Minute).Format("15:04"),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Сохраняем слоты
	if err := storage.CreateSlot(ctx, pastSlot); err != nil {
		t.Fatalf("Failed to create past slot: %v", err)
	}

	if err := storage.CreateSlot(ctx, futureSlot); err != nil {
		t.Fatalf("Failed to create future slot: %v", err)
	}

	// Получаем доступные слоты
	availableSlots, err := storage.GetAvailableSlots(ctx, today)
	if err != nil {
		t.Fatalf("Failed to get available slots: %v", err)
	}

	// Проверяем что возвращен только будущий слот
	if len(availableSlots) != 1 {
		t.Errorf("Expected 1 available slot, got %d", len(availableSlots))
	}

	if len(availableSlots) > 0 {
		slot := availableSlots[0]
		if slot.StartTime != futureSlot.StartTime {
			t.Errorf("Expected slot start time %s, got %s", futureSlot.StartTime, slot.StartTime)
		}
	}
}

func TestGetAvailableSlots_TomorrowSlots(t *testing.T) {
	// Создаем временную базу данных
	storage, err := New(":memory:")
	if err != nil {
		t.Fatalf("Failed to create test storage: %v", err)
	}
	defer storage.Close()

	ctx := context.Background()
	tomorrow := time.Now().Add(24 * time.Hour).Format("2006-01-02")
	currentTime := time.Now()

	// Создаем слот на завтра, который был бы "в прошлом" если бы был сегодня
	tomorrowSlot := &models.Slot{
		Date:      tomorrow,
		StartTime: currentTime.Add(-2 * time.Hour).Format("15:04"), // Время в прошлом относительно сегодня
		EndTime:   currentTime.Add(-2 * time.Hour).Add(15 * time.Minute).Format("15:04"),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Сохраняем слот
	if err := storage.CreateSlot(ctx, tomorrowSlot); err != nil {
		t.Fatalf("Failed to create tomorrow slot: %v", err)
	}

	// Получаем доступные слоты на завтра
	availableSlots, err := storage.GetAvailableSlots(ctx, tomorrow)
	if err != nil {
		t.Fatalf("Failed to get available slots: %v", err)
	}

	// Проверяем что слот на завтра доступен (фильтрация по времени только для сегодня)
	if len(availableSlots) != 1 {
		t.Errorf("Expected 1 available slot for tomorrow, got %d", len(availableSlots))
	}

	if len(availableSlots) > 0 {
		slot := availableSlots[0]
		if slot.StartTime != tomorrowSlot.StartTime {
			t.Errorf("Expected slot start time %s, got %s", tomorrowSlot.StartTime, slot.StartTime)
		}
	}
}
