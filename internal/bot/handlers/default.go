package handlers

import (
	"context"
	"log"

	botservice "telegram_queue_bot/internal/bot/service"

	tgbot "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

// DefaultHandler обрабатывает неопознанные сообщения
type DefaultHandler struct {
	service *botservice.Service
}

// NewDefaultHandler создает новый обработчик по умолчанию
func NewDefaultHandler(service *botservice.Service) *DefaultHandler {
	return &DefaultHandler{service: service}
}

// Handle обрабатывает все остальные типы сообщений
func (h *DefaultHandler) Handle(ctx context.Context, b *tgbot.Bot, update *models.Update) {
	if update.Message == nil {
		return
	}

	chatID := update.Message.Chat.ID

	// Отправляем напоминание о том, как пользоваться ботом
	message := "Пожалуйста, нажмите /start, чтобы начать."
	if err := h.service.SendSimpleMessage(ctx, chatID, message); err != nil {
		log.Printf("Failed to send default message to %d: %v", chatID, err)
	}
}
