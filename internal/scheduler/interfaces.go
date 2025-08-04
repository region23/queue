package scheduler

import (
	"context"
	"telegram_queue_bot/internal/storage/models"
	"time"
)

// NotificationScheduler определяет интерфейс для планирования уведомлений
type NotificationScheduler interface {
	// Schedule планирует уведомление для слота
	Schedule(ctx context.Context, slot *models.Slot, notifyAt time.Time) error

	// Cancel отменяет запланированное уведомление
	Cancel(ctx context.Context, slotID int) error

	// ReschedulePending перепланирует все ожидающие уведомления
	ReschedulePending(ctx context.Context) error

	// Start запускает планировщик
	Start(ctx context.Context) error

	// Stop останавливает планировщик
	Stop() error
}

// NotificationSender определяет интерфейс для отправки уведомлений
type NotificationSender interface {
	// SendNotification отправляет уведомление пользователю
	SendNotification(ctx context.Context, chatID int64, message string) error

	// SendSlotReminder отправляет напоминание о записи
	SendSlotReminder(ctx context.Context, slot *models.Slot) error
}

// SlotGenerator определяет интерфейс для генерации слотов
type SlotGenerator interface {
	// GenerateDailySlots генерирует слоты на день
	GenerateDailySlots(ctx context.Context, date time.Time) ([]*models.Slot, error)

	// GenerateWeeklySlots генерирует слоты на неделю
	GenerateWeeklySlots(ctx context.Context, startDate time.Time) error

	// CleanupExpiredSlots удаляет истекшие слоты
	CleanupExpiredSlots(ctx context.Context, olderThan time.Time) error
}
