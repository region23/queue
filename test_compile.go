package main

import (
	"context"
	"log"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

func testCompile() {
	b, err := bot.New("test-token")
	if err != nil {
		log.Fatal(err)
	}

	ctx := context.Background()

	// Тест отправки сообщения
	params := &bot.SendMessageParams{
		ChatID: int64(123),
		Text:   "test",
	}
	b.SendMessage(ctx, params)

	// Тест работы с callback query
	callbackParams := &bot.AnswerCallbackQueryParams{
		CallbackQueryID: "test",
		Text:            "test",
	}
	b.AnswerCallbackQuery(ctx, callbackParams)

	// Тест создания клавиатуры
	keyboard := &models.InlineKeyboardMarkup{
		InlineKeyboard: [][]models.InlineKeyboardButton{
			{
				{
					Text:         "Button",
					CallbackData: "test",
				},
			},
		},
	}

	// Тест работы с ReplyKeyboard
	replyKeyboard := &models.ReplyKeyboardMarkup{
		Keyboard: [][]models.KeyboardButton{
			{
				{
					Text:           "Contact",
					RequestContact: true,
				},
			},
		},
		OneTimeKeyboard: true,
		ResizeKeyboard:  true,
	}

	_ = keyboard
	_ = replyKeyboard
}
