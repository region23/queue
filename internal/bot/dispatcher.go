package bot

import (
	"context"
	"log"

	"github.com/region23/queue/internal/bot/handlers"
	"github.com/region23/queue/internal/bot/service"

	tgbot "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

// Dispatcher управляет обработкой входящих обновлений от Telegram
type Dispatcher struct {
	startHandler    *handlers.StartHandler
	contactHandler  *handlers.ContactHandler
	callbackHandler *handlers.CallbackHandler
	defaultHandler  *handlers.DefaultHandler
}

// NewDispatcher создает новый диспетчер обновлений
func NewDispatcher(service *service.Service) *Dispatcher {
	return &Dispatcher{
		startHandler:    handlers.NewStartHandler(service),
		contactHandler:  handlers.NewContactHandler(service),
		callbackHandler: handlers.NewCallbackHandler(service),
		defaultHandler:  handlers.NewDefaultHandler(service),
	}
}

// HandleUpdate обрабатывает входящее обновление от Telegram
func (d *Dispatcher) HandleUpdate(ctx context.Context, bot *tgbot.Bot, update *models.Update) {
	// Логируем входящее обновление (в продакшене можно убрать или использовать debug уровень)
	if update.Message != nil {
		log.Printf("Received message from %d: %s", update.Message.Chat.ID, update.Message.Text)
	} else if update.CallbackQuery != nil {
		log.Printf("Received callback query from %d: %s", update.CallbackQuery.Message.Message.Chat.ID, update.CallbackQuery.Data)
	}

	// Обрабатываем callback query от inline кнопок
	if update.CallbackQuery != nil {
		d.callbackHandler.Handle(ctx, bot, update)
		return
	}

	// Обрабатываем сообщения
	if update.Message != nil {
		// Если получен контакт, обрабатываем его
		if update.Message.Contact != nil {
			d.contactHandler.Handle(ctx, bot, update)
			return
		}

		// Обрабатываем команды
		if update.Message.Text != "" {
			// Команда /start
			if update.Message.Text == "/start" || update.Message.Text == "/start@your_bot_name" {
				d.startHandler.Handle(ctx, bot, update)
				return
			}
		}

		// Все остальные сообщения
		d.defaultHandler.Handle(ctx, bot, update)
		return
	}

	// Логируем неизвестные типы обновлений
	log.Printf("Received unknown update type: %+v", update)
}
