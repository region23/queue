package handlers

import (
	"context"
	"fmt"
	"log"
	"time"

	botservice "telegram_queue_bot/internal/bot/service"

	tgbot "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

// ContactHandler обрабатывает получение контактной информации от пользователя
type ContactHandler struct {
	service *botservice.Service
}

// NewContactHandler создает новый обработчик контактов
func NewContactHandler(service *botservice.Service) *ContactHandler {
	return &ContactHandler{service: service}
}

// Handle обрабатывает сообщения с контактной информацией
func (h *ContactHandler) Handle(ctx context.Context, b *tgbot.Bot, update *models.Update) {
	if update.Message == nil || update.Message.Contact == nil {
		return
	}

	chatID := update.Message.Chat.ID
	contact := update.Message.Contact

	if contact.PhoneNumber == "" {
		log.Printf("Received empty phone number from user %d", chatID)
		h.service.SendError(ctx, chatID, "Не получен номер телефона. Попробуйте еще раз.")
		return
	}

	// Сохраняем пользователя
	err := h.service.SaveUser(ctx, chatID, contact.PhoneNumber, contact.FirstName, contact.LastName)
	if err != nil {
		log.Printf("Failed to save user %d: %v", chatID, err)
		h.service.SendError(ctx, chatID, "Ошибка при сохранении данных пользователя")
		return
	}

	// Убираем клавиатуру и отправляем подтверждение
	replyMarkup := &models.ReplyKeyboardRemove{
		RemoveKeyboard: true,
	}

	message := "Телефон получен. Давайте запишемся."
	if err := h.service.SendMessage(ctx, chatID, message, replyMarkup); err != nil {
		log.Printf("Failed to send confirmation message to %d: %v", chatID, err)
	}

	// Показываем выбор даты
	h.showDateSelection(ctx, chatID)
}

func (h *ContactHandler) showDateSelection(ctx context.Context, chatID int64) {
	dates := h.service.ListAvailableDates()

	// Если настроен только один день, показываем слоты сразу
	if len(dates) == 1 {
		today := dates[0]
		// Генерируем слоты для сегодня
		date, err := time.Parse("2006-01-02", today)
		if err != nil {
			log.Printf("Failed to parse date %s: %v", today, err)
			h.service.SendError(ctx, chatID, "Ошибка при обработке даты")
			return
		}

		if err := h.service.GenerateSlotsForDate(ctx, date); err != nil {
			log.Printf("Failed to generate slots for %s: %v", today, err)
		}

		h.showSlotSelection(ctx, chatID, today)
		return
	}

	// Предварительно генерируем слоты для всех дат
	for _, dateStr := range dates {
		date, err := time.Parse("2006-01-02", dateStr)
		if err != nil {
			log.Printf("Failed to parse date %s: %v", dateStr, err)
			continue
		}
		if err := h.service.GenerateSlotsForDate(ctx, date); err != nil {
			log.Printf("Failed to generate slots for %s: %v", dateStr, err)
		}
	}

	var rows [][]models.InlineKeyboardButton
	for _, d := range dates {
		btn := models.InlineKeyboardButton{
			Text:         d,
			CallbackData: "DATE:" + d,
		}
		rows = append(rows, []models.InlineKeyboardButton{btn})
	}

	keyboard := &models.InlineKeyboardMarkup{
		InlineKeyboard: rows,
	}

	message := "Выберите дату для записи:"
	if err := h.service.SendMessage(ctx, chatID, message, keyboard); err != nil {
		log.Printf("Failed to send date selection to %d: %v", chatID, err)
	}
}

func (h *ContactHandler) showSlotSelection(ctx context.Context, chatID int64, date string) {
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
