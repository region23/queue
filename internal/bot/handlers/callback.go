package handlers

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	botservice "telegram_queue_bot/internal/bot/service"

	tgbot "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

// CallbackHandler обрабатывает callback query от inline кнопок
type CallbackHandler struct {
	service *botservice.Service
}

// NewCallbackHandler создает новый обработчик callback query
func NewCallbackHandler(service *botservice.Service) *CallbackHandler {
	return &CallbackHandler{service: service}
}

// Handle обрабатывает callback query
func (h *CallbackHandler) Handle(ctx context.Context, b *tgbot.Bot, update *models.Update) {
	if update.CallbackQuery == nil {
		return
	}

	cb := update.CallbackQuery
	chatID := cb.Message.Message.Chat.ID
	data := cb.Data

	if strings.HasPrefix(data, "DATE:") {
		h.handleDateSelection(ctx, cb, chatID, data)
		return
	}

	if strings.HasPrefix(data, "SLOT:") {
		h.handleSlotSelection(ctx, cb, chatID, data)
		return
	}

	// Неизвестный callback
	h.service.AnswerCallbackQuery(ctx, cb.ID, "Неверный выбор")
}

func (h *CallbackHandler) handleDateSelection(ctx context.Context, cb *models.CallbackQuery, chatID int64, data string) {
	date := strings.TrimPrefix(data, "DATE:")

	// Показываем слоты для выбранной даты
	h.showSlotSelection(ctx, chatID, date)

	// Отвечаем на callback query чтобы убрать индикатор загрузки
	if err := h.service.AnswerCallbackQuery(ctx, cb.ID, ""); err != nil {
		log.Printf("Failed to answer callback query: %v", err)
	}
}

func (h *CallbackHandler) handleSlotSelection(ctx context.Context, cb *models.CallbackQuery, chatID int64, data string) {
	idStr := strings.TrimPrefix(data, "SLOT:")

	// Валидируем ID слота
	id, err := h.service.ValidateSlotID(idStr)
	if err != nil {
		h.service.AnswerCallbackQuery(ctx, cb.ID, "Неверный ID слота")
		h.service.SendError(ctx, chatID, "Ошибка: неверный ID слота")
		return
	}

	// Пытаемся зарезервировать слот
	err = h.service.ReserveSlot(ctx, id, chatID)
	if err != nil {
		// Слот уже занят
		h.service.AnswerCallbackQuery(ctx, cb.ID, "Этот слот уже занят")
		h.service.SendSimpleMessage(ctx, chatID, "К сожалению, кто‑то забронировал этот слот раньше. Пожалуйста, выберите другой.")
		return
	}

	// Слот успешно зарезервирован
	h.service.AnswerCallbackQuery(ctx, cb.ID, "Слот забронирован")

	// Удаляем сообщение с кнопками слотов
	if err := h.service.DeleteMessage(ctx, chatID, cb.Message.Message.ID); err != nil {
		log.Printf("Failed to delete message: %v", err)
	}

	// Получаем детали слота для отображения
	slot, err := h.service.GetSlotByID(ctx, id)
	var successText string
	if err == nil && slot != nil {
		// Форматируем дату и время слота для отображения
		slotDateTime := fmt.Sprintf("%s %s", slot.Date, slot.StartTime)
		successText = fmt.Sprintf("Вы успешно забронировали слот %s. Мы уведомим вас, когда ваша очередь подойдёт.", slotDateTime)
	} else {
		// Fallback сообщение если данные слота недоступны
		successText = "Вы успешно забронировали слот. Мы уведомим вас, когда ваша очередь подойдёт."
	}

	h.service.SendSimpleMessage(ctx, chatID, successText)
}

func (h *CallbackHandler) showSlotSelection(ctx context.Context, chatID int64, date string) {
	slots, err := h.service.GetAvailableSlots(ctx, date)
	if err != nil {
		log.Printf("Failed to get available slots for %s: %v", date, err)
		h.service.SendError(ctx, chatID, "Ошибка при получении слотов")
		return
	}

	today := time.Now().Format("2006-01-02")
	var messageText string
	var noSlotsText string

	if date == today {
		messageText = "Выберите свободный временной слот на сегодня:"
		noSlotsText = "На сегодня нет свободных слотов, доступных для записи."
	} else {
		messageText = "Выберите свободный временной слот:"
		noSlotsText = "На выбранную дату нет свободных слотов. Попробуйте другую дату."
	}

	if len(slots) == 0 {
		h.service.SendSimpleMessage(ctx, chatID, noSlotsText)
		return
	}

	var rows [][]models.InlineKeyboardButton
	for _, s := range slots {
		text := fmt.Sprintf("%s-%s", s.StartTime, s.EndTime)
		cbData := fmt.Sprintf("SLOT:%d", s.ID)
		btn := models.InlineKeyboardButton{
			Text:         text,
			CallbackData: cbData,
		}
		rows = append(rows, []models.InlineKeyboardButton{btn})
	}

	keyboard := &models.InlineKeyboardMarkup{
		InlineKeyboard: rows,
	}

	if err := h.service.SendMessage(ctx, chatID, messageText, keyboard); err != nil {
		log.Printf("Failed to send slot selection to %d: %v", chatID, err)
	}
}
