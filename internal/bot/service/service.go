package service

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/region23/queue/internal/config"
	"github.com/region23/queue/internal/scheduler"
	"github.com/region23/queue/internal/storage"
	"github.com/region23/queue/internal/storage/models"
	"github.com/region23/queue/pkg/errors"

	"github.com/go-telegram/bot"
	tgmodels "github.com/go-telegram/bot/models"
)

// Service представляет основной сервис Telegram бота
type Service struct {
	bot       *bot.Bot
	storage   storage.Storage
	scheduler scheduler.NotificationScheduler
	config    *config.Config
}

// NewService создает новый экземпляр сервиса бота
func NewService(
	bot *bot.Bot,
	storage storage.Storage,
	scheduler scheduler.NotificationScheduler,
	config *config.Config,
) *Service {
	return &Service{
		bot:       bot,
		storage:   storage,
		scheduler: scheduler,
		config:    config,
	}
}

// IsUserRegistered проверяет, зарегистрирован ли пользователь
func (s *Service) IsUserRegistered(ctx context.Context, chatID int64) (bool, error) {
	return s.storage.IsUserRegistered(ctx, chatID)
}

// SaveUser сохраняет нового пользователя
func (s *Service) SaveUser(ctx context.Context, chatID int64, phone, firstName, lastName string) error {
	return s.storage.SaveUser(ctx, chatID, phone, firstName, lastName)
}

// GetUserTodaySlot получает слот пользователя на сегодня
func (s *Service) GetUserTodaySlot(ctx context.Context, chatID int64) (*models.Slot, bool, error) {
	return s.storage.GetUserTodaySlot(ctx, chatID)
}

// GetAvailableSlots получает доступные слоты на дату
func (s *Service) GetAvailableSlots(ctx context.Context, date string) ([]*models.Slot, error) {
	return s.storage.GetAvailableSlots(ctx, date)
}

// ReserveSlot резервирует слот для пользователя
func (s *Service) ReserveSlot(ctx context.Context, slotID int, chatID int64) error {
	err := s.storage.ReserveSlot(ctx, slotID, chatID)
	if err != nil {
		return err
	}

	// Получаем информацию о слоте для планирования уведомления
	slot, err := s.storage.GetSlotByID(ctx, slotID)
	if err != nil {
		log.Printf("Failed to get slot for scheduling: %v", err)
		return err
	}

	// Вычисляем время уведомления (начало слота)
	slotDateTime := fmt.Sprintf("%s %s", slot.Date, slot.StartTime)
	notifyAt, err := time.ParseInLocation("2006-01-02 15:04", slotDateTime, time.Local)
	if err != nil {
		log.Printf("Failed to parse slot time for scheduling: %v", err)
		return err
	}

	// Планируем уведомление
	if err := s.scheduler.Schedule(ctx, slot, notifyAt); err != nil {
		log.Printf("Failed to schedule notification: %v", err)
		// Не возвращаем ошибку, так как слот уже зарезервирован
	}

	return nil
}

// GetSlotByID получает слот по ID
func (s *Service) GetSlotByID(ctx context.Context, id int) (*models.Slot, error) {
	return s.storage.GetSlotByID(ctx, id)
}

// SendMessage отправляет сообщение пользователю
func (s *Service) SendMessage(ctx context.Context, chatID int64, text string, replyMarkup tgmodels.ReplyMarkup) error {
	params := &bot.SendMessageParams{
		ChatID:      chatID,
		Text:        text,
		ReplyMarkup: replyMarkup,
	}

	_, err := s.bot.SendMessage(ctx, params)
	return err
}

// SendSimpleMessage отправляет простое текстовое сообщение
func (s *Service) SendSimpleMessage(ctx context.Context, chatID int64, text string) error {
	return s.SendMessage(ctx, chatID, text, nil)
}

// SendError отправляет сообщение об ошибке пользователю
func (s *Service) SendError(ctx context.Context, chatID int64, message string) {
	if err := s.SendSimpleMessage(ctx, chatID, message); err != nil {
		log.Printf("Failed to send error message to %d: %v", chatID, err)
	}
}

// AnswerCallbackQuery отвечает на callback query
func (s *Service) AnswerCallbackQuery(ctx context.Context, callbackQueryID, text string) error {
	params := &bot.AnswerCallbackQueryParams{
		CallbackQueryID: callbackQueryID,
		Text:            text,
	}

	_, err := s.bot.AnswerCallbackQuery(ctx, params)
	return err
}

// DeleteMessage удаляет сообщение
func (s *Service) DeleteMessage(ctx context.Context, chatID int64, messageID int) error {
	params := &bot.DeleteMessageParams{
		ChatID:    chatID,
		MessageID: messageID,
	}

	_, err := s.bot.DeleteMessage(ctx, params)
	return err
}

// GenerateSlotsForDate генерирует слоты для указанной даты
func (s *Service) GenerateSlotsForDate(ctx context.Context, date time.Time) error {
	start, err := time.Parse("15:04", s.config.Schedule.WorkStart)
	if err != nil {
		return fmt.Errorf("invalid work start time: %w", err)
	}

	end, err := time.Parse("15:04", s.config.Schedule.WorkEnd)
	if err != nil {
		return fmt.Errorf("invalid work end time: %w", err)
	}

	duration := time.Duration(s.config.Schedule.SlotDurationMins) * time.Minute

	// Построение полного времени для конкретной даты
	current := time.Date(date.Year(), date.Month(), date.Day(), start.Hour(), start.Minute(), 0, 0, date.Location())
	endDateTime := time.Date(date.Year(), date.Month(), date.Day(), end.Hour(), end.Minute(), 0, 0, date.Location())

	for !current.After(endDateTime.Add(-duration)) {
		startStr := current.Format("15:04")
		endStr := current.Add(duration).Format("15:04")

		slot := &models.Slot{
			Date:      date.Format("2006-01-02"),
			StartTime: startStr,
			EndTime:   endStr,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}

		// Создаем слот (игнорируем ошибки дублирования)
		if err := s.storage.CreateSlot(ctx, slot); err != nil {
			// Логируем, но не прерываем процесс для других слотов
			log.Printf("Failed to create slot %s %s-%s: %v", slot.Date, slot.StartTime, slot.EndTime, err)
		}

		current = current.Add(duration)
	}

	return nil
}

// ListAvailableDates возвращает список доступных дат
func (s *Service) ListAvailableDates() []string {
	var dates []string
	now := time.Now()
	for i := 0; i < s.config.Schedule.ScheduleDays; i++ {
		d := now.AddDate(0, 0, i)
		dates = append(dates, d.Format("2006-01-02"))
	}
	return dates
}

// ReschedulePendingNotifications перепланирует ожидающие уведомления
func (s *Service) ReschedulePendingNotifications(ctx context.Context) error {
	return s.scheduler.ReschedulePending(ctx)
}

// ValidateSlotID валидирует ID слота
func (s *Service) ValidateSlotID(idStr string) (int, error) {
	id, err := strconv.Atoi(idStr)
	if err != nil {
		return 0, &errors.BotError{
			Code:    "INVALID_SLOT_ID",
			Message: "invalid slot ID format",
			Err:     err,
		}
	}
	if id <= 0 {
		return 0, &errors.BotError{
			Code:    "INVALID_SLOT_ID",
			Message: "slot ID must be positive",
		}
	}
	return id, nil
}

// Close закрывает соединения сервиса
func (s *Service) Close() error {
	var errs []error

	if err := s.scheduler.Stop(); err != nil {
		errs = append(errs, fmt.Errorf("failed to stop scheduler: %w", err))
	}

	if err := s.storage.Close(); err != nil {
		errs = append(errs, fmt.Errorf("failed to close storage: %w", err))
	}

	if len(errs) > 0 {
		return fmt.Errorf("multiple errors during close: %v", errs)
	}

	return nil
}
