package unit

import (
	"testing"
	"time"

	"github.com/region23/queue/internal/storage/models"
	"github.com/region23/queue/tests/testutils"
)

func TestMemoryScheduler_Schedule(t *testing.T) {
	scheduler := testutils.SetupTestScheduler(t)
	ctx := testutils.TestContext()

	// Запускаем планировщик
	err := scheduler.Start(ctx)
	testutils.AssertNoError(t, err, "Scheduler should start without error")

	// Создаем тестовый слот
	slot := &models.Slot{
		ID:         1,
		Date:       time.Now().Add(24 * time.Hour).Format("2006-01-02"),
		StartTime:  "10:00",
		EndTime:    "10:30",
		UserChatID: func() *int64 { id := int64(12345); return &id }(),
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	// Планируем уведомление на 100ms в будущем для быстрого теста
	notifyAt := time.Now().Add(100 * time.Millisecond)

	err = scheduler.Schedule(ctx, slot, notifyAt)
	testutils.AssertNoError(t, err, "Should schedule notification without error")
}

func TestMemoryScheduler_Cancel(t *testing.T) {
	scheduler := testutils.SetupTestScheduler(t)
	ctx := testutils.TestContext()

	err := scheduler.Start(ctx)
	testutils.AssertNoError(t, err, "Scheduler should start without error")

	// Создаем тестовый слот
	slot := &models.Slot{
		ID:         1,
		Date:       time.Now().Add(24 * time.Hour).Format("2006-01-02"),
		StartTime:  "10:00",
		EndTime:    "10:30",
		UserChatID: func() *int64 { id := int64(12345); return &id }(),
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	// Планируем уведомление
	notifyAt := time.Now().Add(1 * time.Hour)
	err = scheduler.Schedule(ctx, slot, notifyAt)
	testutils.AssertNoError(t, err, "Should schedule notification without error")

	// Отменяем уведомление
	err = scheduler.Cancel(ctx, slot.ID)
	testutils.AssertNoError(t, err, "Should cancel notification without error")
}

func TestMemoryScheduler_MultipleSlots(t *testing.T) {
	scheduler := testutils.SetupTestScheduler(t)
	ctx := testutils.TestContext()

	err := scheduler.Start(ctx)
	testutils.AssertNoError(t, err, "Scheduler should start without error")

	// Создаем несколько слотов
	slots := []*models.Slot{
		{
			ID:         1,
			Date:       time.Now().Add(24 * time.Hour).Format("2006-01-02"),
			StartTime:  "10:00",
			EndTime:    "10:30",
			UserChatID: func() *int64 { id := int64(12345); return &id }(),
			CreatedAt:  time.Now(),
			UpdatedAt:  time.Now(),
		},
		{
			ID:         2,
			Date:       time.Now().Add(24 * time.Hour).Format("2006-01-02"),
			StartTime:  "11:00",
			EndTime:    "11:30",
			UserChatID: func() *int64 { id := int64(67890); return &id }(),
			CreatedAt:  time.Now(),
			UpdatedAt:  time.Now(),
		},
		{
			ID:         3,
			Date:       time.Now().Add(24 * time.Hour).Format("2006-01-02"),
			StartTime:  "12:00",
			EndTime:    "12:30",
			UserChatID: func() *int64 { id := int64(11111); return &id }(),
			CreatedAt:  time.Now(),
			UpdatedAt:  time.Now(),
		},
	}

	// Планируем уведомления для всех слотов
	baseTime := time.Now().Add(1 * time.Hour)
	for i, slot := range slots {
		notifyAt := baseTime.Add(time.Duration(i) * time.Minute)
		err = scheduler.Schedule(ctx, slot, notifyAt)
		testutils.AssertNoError(t, err, "Should schedule notification for slot")
	}

	// Отменяем одно уведомление
	err = scheduler.Cancel(ctx, 2)
	testutils.AssertNoError(t, err, "Should cancel notification for slot 2")
}

func TestMemoryScheduler_RescheduleSlot(t *testing.T) {
	scheduler := testutils.SetupTestScheduler(t)
	ctx := testutils.TestContext()

	err := scheduler.Start(ctx)
	testutils.AssertNoError(t, err, "Scheduler should start without error")

	slot := &models.Slot{
		ID:         1,
		Date:       time.Now().Add(24 * time.Hour).Format("2006-01-02"),
		StartTime:  "10:00",
		EndTime:    "10:30",
		UserChatID: func() *int64 { id := int64(12345); return &id }(),
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	// Планируем уведомление
	originalTime := time.Now().Add(2 * time.Hour)
	err = scheduler.Schedule(ctx, slot, originalTime)
	testutils.AssertNoError(t, err, "Should schedule original notification")

	// Перепланируем то же уведомление (должно заменить предыдущее)
	newTime := time.Now().Add(3 * time.Hour)
	err = scheduler.Schedule(ctx, slot, newTime)
	testutils.AssertNoError(t, err, "Should reschedule notification")
}

func TestMemoryScheduler_Stop(t *testing.T) {
	scheduler := testutils.SetupTestScheduler(t)
	ctx := testutils.TestContext()

	err := scheduler.Start(ctx)
	testutils.AssertNoError(t, err, "Scheduler should start without error")

	// Планируем уведомление
	slot := &models.Slot{
		ID:         1,
		Date:       time.Now().Add(24 * time.Hour).Format("2006-01-02"),
		StartTime:  "10:00",
		EndTime:    "10:30",
		UserChatID: func() *int64 { id := int64(12345); return &id }(),
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	notifyAt := time.Now().Add(1 * time.Hour)
	err = scheduler.Schedule(ctx, slot, notifyAt)
	testutils.AssertNoError(t, err, "Should schedule notification")

	// Останавливаем планировщик
	err = scheduler.Stop()
	testutils.AssertNoError(t, err, "Should stop scheduler without error")

	// Попробуем запланировать уведомление после остановки
	err = scheduler.Schedule(ctx, slot, notifyAt)
	testutils.AssertError(t, err, "Should not schedule after stop")
}

func TestMemoryScheduler_ConcurrentOperations(t *testing.T) {
	scheduler := testutils.SetupTestScheduler(t)
	ctx := testutils.TestContext()

	err := scheduler.Start(ctx)
	testutils.AssertNoError(t, err, "Scheduler should start without error")

	// Тестируем конкурентные операции
	done := make(chan bool, 10)

	// Запускаем несколько горутин для планирования
	for i := 0; i < 5; i++ {
		go func(id int) {
			defer func() { done <- true }()

			slot := &models.Slot{
				ID:         id,
				Date:       time.Now().Add(24 * time.Hour).Format("2006-01-02"),
				StartTime:  "10:00",
				EndTime:    "10:30",
				UserChatID: func() *int64 { slotID := int64(12345 + id); return &slotID }(),
				CreatedAt:  time.Now(),
				UpdatedAt:  time.Now(),
			}

			notifyAt := time.Now().Add(time.Duration(id+1) * time.Hour)
			err := scheduler.Schedule(ctx, slot, notifyAt)
			if err != nil {
				t.Errorf("Failed to schedule slot %d: %v", id, err)
			}
		}(i)
	}

	// Запускаем несколько горутин для отмены
	for i := 0; i < 5; i++ {
		go func(id int) {
			defer func() { done <- true }()

			// Даем время для планирования
			time.Sleep(10 * time.Millisecond)

			err := scheduler.Cancel(ctx, id)
			if err != nil {
				t.Errorf("Failed to cancel slot %d: %v", id, err)
			}
		}(i)
	}

	// Ждем завершения всех горутин
	for i := 0; i < 10; i++ {
		select {
		case <-done:
			// OK
		case <-time.After(5 * time.Second):
			t.Fatal("Timeout waiting for concurrent operations")
		}
	}
}

func TestMemoryScheduler_InvalidSlot(t *testing.T) {
	scheduler := testutils.SetupTestScheduler(t)
	ctx := testutils.TestContext()

	err := scheduler.Start(ctx)
	testutils.AssertNoError(t, err, "Scheduler should start without error")

	tests := []struct {
		name      string
		slot      *models.Slot
		expectErr bool
	}{
		{
			name:      "Nil slot",
			slot:      nil,
			expectErr: true, // Ожидаем ошибку или panic
		},
		{
			name: "Slot without UserChatID",
			slot: &models.Slot{
				ID:        1,
				Date:      time.Now().Add(24 * time.Hour).Format("2006-01-02"),
				StartTime: "10:00",
				EndTime:   "10:30",
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			},
			expectErr: false, // Может быть валидным случаем
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Используем defer для обработки panic
			defer func() {
				if r := recover(); r != nil {
					if !tt.expectErr {
						t.Errorf("Unexpected panic: %v", r)
					}
					// Если ожидали ошибку и получили panic - это нормально
				}
			}()

			notifyAt := time.Now().Add(1 * time.Hour)

			err := scheduler.Schedule(ctx, tt.slot, notifyAt)
			if tt.expectErr && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.expectErr && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		})
	}
}

func TestMemoryScheduler_PastNotificationTime(t *testing.T) {
	scheduler := testutils.SetupTestScheduler(t)
	ctx := testutils.TestContext()

	err := scheduler.Start(ctx)
	testutils.AssertNoError(t, err, "Scheduler should start without error")

	slot := &models.Slot{
		ID:         1,
		Date:       time.Now().Add(24 * time.Hour).Format("2006-01-02"),
		StartTime:  "10:00",
		EndTime:    "10:30",
		UserChatID: func() *int64 { id := int64(12345); return &id }(),
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	// Пытаемся запланировать уведомление в прошлом
	pastTime := time.Now().Add(-1 * time.Hour)

	err = scheduler.Schedule(ctx, slot, pastTime)
	// В зависимости от реализации может вернуть ошибку или выполнить немедленно
	if err != nil {
		t.Logf("Expected behavior for past time: %v", err)
	}
}

func TestMemoryScheduler_ReschedulePending(t *testing.T) {
	scheduler := testutils.SetupTestScheduler(t)
	ctx := testutils.TestContext()

	err := scheduler.Start(ctx)
	testutils.AssertNoError(t, err, "Scheduler should start without error")

	// Создаем несколько слотов
	for i := 1; i <= 3; i++ {
		slot := &models.Slot{
			ID:         i,
			Date:       time.Now().Add(24 * time.Hour).Format("2006-01-02"),
			StartTime:  "10:00",
			EndTime:    "10:30",
			UserChatID: func() *int64 { id := int64(12345 + i); return &id }(),
			CreatedAt:  time.Now(),
			UpdatedAt:  time.Now(),
		}

		notifyAt := time.Now().Add(time.Duration(i) * time.Hour)
		err = scheduler.Schedule(ctx, slot, notifyAt)
		testutils.AssertNoError(t, err, "Should schedule slot")
	}

	// Перепланируем все ожидающие уведомления
	err = scheduler.ReschedulePending(ctx)
	testutils.AssertNoError(t, err, "Should reschedule pending notifications")
}

func TestMemoryScheduler_CancelNonExistentSlot(t *testing.T) {
	scheduler := testutils.SetupTestScheduler(t)
	ctx := testutils.TestContext()

	err := scheduler.Start(ctx)
	testutils.AssertNoError(t, err, "Scheduler should start without error")

	// Пытаемся отменить несуществующий слот
	err = scheduler.Cancel(ctx, 999)
	// Не должно быть ошибки при отмене несуществующего слота
	testutils.AssertNoError(t, err, "Cancel of non-existent slot should not error")
}
