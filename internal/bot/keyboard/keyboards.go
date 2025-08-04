package keyboard

import (
	"fmt"

	"github.com/go-telegram/bot/models"
)

// CreateContactKeyboard создает клавиатуру для запроса контакта
func CreateContactKeyboard() *models.ReplyKeyboardMarkup {
	return &models.ReplyKeyboardMarkup{
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
}

// CreateRemoveKeyboard создает объект для удаления клавиатуры
func CreateRemoveKeyboard() *models.ReplyKeyboardRemove {
	return &models.ReplyKeyboardRemove{
		RemoveKeyboard: true,
	}
}

// CreateDateSelectionKeyboard создает inline клавиатуру для выбора даты
func CreateDateSelectionKeyboard(dates []string) *models.InlineKeyboardMarkup {
	var rows [][]models.InlineKeyboardButton

	for _, d := range dates {
		btn := models.InlineKeyboardButton{
			Text:         d,
			CallbackData: "DATE:" + d,
		}
		rows = append(rows, []models.InlineKeyboardButton{btn})
	}

	return &models.InlineKeyboardMarkup{
		InlineKeyboard: rows,
	}
}

// CreateSlotSelectionKeyboard создает inline клавиатуру для выбора слота
func CreateSlotSelectionKeyboard(slots []SlotInfo) *models.InlineKeyboardMarkup {
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

	return &models.InlineKeyboardMarkup{
		InlineKeyboard: rows,
	}
}

// SlotInfo представляет информацию о слоте для создания клавиатуры
type SlotInfo struct {
	ID        int
	StartTime string
	EndTime   string
}
