package storage

import (
	"context"
	"telegram_queue_bot/internal/storage/models"
)

// UserRepository определяет интерфейс для работы с пользователями
type UserRepository interface {
	SaveUser(ctx context.Context, chatID int64, phone, firstName, lastName string) error
	IsUserRegistered(ctx context.Context, chatID int64) (bool, error)
	GetUserByID(ctx context.Context, chatID int64) (*models.User, error)
}

// SlotRepository определяет интерфейс для работы со слотами записи
type SlotRepository interface {
	CreateSlot(ctx context.Context, slot *models.Slot) error
	GetAvailableSlots(ctx context.Context, date string) ([]*models.Slot, error)
	ReserveSlot(ctx context.Context, slotID int, chatID int64) error
	GetSlotByID(ctx context.Context, id int) (*models.Slot, error)
	GetUserTodaySlot(ctx context.Context, chatID int64) (*models.Slot, bool, error)
	MarkSlotNotified(ctx context.Context, slotID int) error
	GetPendingNotifications(ctx context.Context) ([]*models.Slot, error)
	GetUserActiveSlots(ctx context.Context, chatID int64) ([]*models.Slot, error)
	CancelSlot(ctx context.Context, slotID int, chatID int64) error
}

// Storage объединяет все репозитории в единый интерфейс
type Storage interface {
	UserRepository
	SlotRepository
	Close() error
	Ping(ctx context.Context) error
}
