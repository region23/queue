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

// StartHandler обрабатывает команду /start
type StartHandler struct {
	service *botservice.Service
}

// NewStartHandler создает новый обработчик команды /start
func NewStartHandler(service *botservice.Service) *StartHandler {
	return &StartHandler{service: service}
}

// Handle обрабатывает команду /start
func (h *StartHandler) Handle(ctx context.Context, b *tgbot.Bot, update *models.Update) {
	if update.Message == nil || !strings.HasPrefix(update.Message.Text, "/start") {
		return
	}

	chatID := update.Message.Chat.ID

	registered, err := h.service.IsUserRegistered(ctx, chatID)
	if err != nil {
		log.Printf("Failed to check user registration for %d: %v", chatID, err)
		h.service.SendError(ctx, chatID, "Произошла ошибка при проверке регистрации")
		return
	}

	if registered {
		h.handleRegisteredUser(ctx, chatID)
	} else {
		h.handleNewUser(ctx, chatID)
	}
}

func (h *StartHandler) handleRegisteredUser(ctx context.Context, chatID int64) {
	// Пользователь зарегистрирован, убираем клавиатуру
	h.askForContact(ctx, chatID, true)

	// Проверяем, есть ли у пользователя слот на сегодня
	slot, hasSlot, err := h.service.GetUserTodaySlot(ctx, chatID)
	if err != nil {
		log.Printf("Failed to get user today slot for %d: %v", chatID, err)
		h.service.SendError(ctx, chatID, "Произошла ошибка при получении информации о слотах")
		return
	}

	if hasSlot && slot != nil {
		// У пользователя есть слот на сегодня, показываем его
		message := fmt.Sprintf("У вас уже забронирован слот на сегодня: %s %s-%s",
			slot.Date, slot.StartTime, slot.EndTime)
		h.service.SendSimpleMessage(ctx, chatID, message)
	} else {
		// У пользователя нет слота на сегодня, показываем доступные слоты
		h.showDateSelection(ctx, chatID)
	}
}

func (h *StartHandler) handleNewUser(ctx context.Context, chatID int64) {
	// Пользователь не зарегистрирован, запрашиваем контакт
	h.askForContact(ctx, chatID, false)
}

func (h *StartHandler) askForContact(ctx context.Context, chatID int64, hideKeyboard bool) {
	if hideKeyboard {
		replyMarkup := &models.ReplyKeyboardRemove{
			RemoveKeyboard: true,
		}
		if err := h.service.SendMessage(ctx, chatID, "Добро пожаловать обратно!", replyMarkup); err != nil {
			log.Printf("Failed to send welcome message to %d: %v", chatID, err)
		}
		return
	}

	keyboard := &models.ReplyKeyboardMarkup{
		Keyboard: [][]models.KeyboardButton{
			{
				{
					Text:           "Поделиться телефоном",
					RequestContact: true,
				},
			},
		},
		OneTimeKeyboard: true,
		ResizeKeyboard:  true,
	}

	message := "Пожалуйста, поделитесь своим номером телефона, нажав кнопку ниже."
	if err := h.service.SendMessage(ctx, chatID, message, keyboard); err != nil {
		log.Printf("Failed to send contact request to %d: %v", chatID, err)
	}
}

func (h *StartHandler) showDateSelection(ctx context.Context, chatID int64) {
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

func (h *StartHandler) showSlotSelection(ctx context.Context, chatID int64, date string) {
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
