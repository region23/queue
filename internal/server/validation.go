package server

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"telegram_queue_bot/internal/validation"
	"telegram_queue_bot/pkg/errors"
	"telegram_queue_bot/pkg/logger"
)

// RequestValidator предоставляет методы валидации HTTP запросов
type RequestValidator struct {
	logger logger.Logger
}

// NewRequestValidator создает новый валидатор запросов
func NewRequestValidator(logger logger.Logger) *RequestValidator {
	return &RequestValidator{
		logger: logger,
	}
}

// ValidateWebhookRequest валидирует webhook запрос от Telegram
func (v *RequestValidator) ValidateWebhookRequest(r *http.Request) (*TelegramUpdate, error) {
	// Проверяем метод
	if r.Method != http.MethodPost {
		return nil, errors.NewBotError("INVALID_METHOD", "webhook должен использовать POST метод")
	}

	// Проверяем Content-Type
	contentType := r.Header.Get("Content-Type")
	if !strings.Contains(contentType, "application/json") {
		return nil, errors.NewBotError("INVALID_CONTENT_TYPE", "Content-Type должен быть application/json")
	}

	// Проверяем размер запроса
	if r.ContentLength > 4*1024*1024 { // 4MB лимит для Telegram
		v.logger.Warn("Request too large",
			logger.Field{Key: "content_length", Value: r.ContentLength},
		)
		return nil, errors.NewBotError("REQUEST_TOO_LARGE", "размер запроса превышает лимит")
	}

	// Парсим JSON
	var update TelegramUpdate
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields() // Строгая валидация JSON

	if err := decoder.Decode(&update); err != nil {
		v.logger.Error("Failed to parse webhook JSON",
			logger.Field{Key: "error", Value: err.Error()},
		)
		return nil, errors.NewBotError("INVALID_JSON", "некорректный JSON в запросе").WithError(err)
	}

	// Валидируем структуру update
	if err := v.validateUpdateStructure(&update); err != nil {
		return nil, err
	}

	return &update, nil
}

// validateUpdateStructure проверяет структуру Telegram update
func (v *RequestValidator) validateUpdateStructure(update *TelegramUpdate) error {
	// Проверяем обязательные поля
	if update.UpdateID <= 0 {
		return errors.NewBotError("INVALID_UPDATE_ID", "некорректный update_id")
	}

	// Проверяем наличие хотя бы одного типа update
	if update.Message == nil && update.CallbackQuery == nil {
		return errors.NewBotError("EMPTY_UPDATE", "update не содержит ни message, ни callback_query")
	}

	// Валидируем message если есть
	if update.Message != nil {
		if err := v.validateMessage(update.Message); err != nil {
			return err
		}
	}

	// Валидируем callback_query если есть
	if update.CallbackQuery != nil {
		if err := v.validateCallbackQuery(update.CallbackQuery); err != nil {
			return err
		}
	}

	return nil
}

// validateMessage валидирует message из update
func (v *RequestValidator) validateMessage(message *struct {
	MessageID int `json:"message_id"`
	From      *struct {
		ID           int64  `json:"id"`
		IsBot        bool   `json:"is_bot"`
		FirstName    string `json:"first_name"`
		LastName     string `json:"last_name,omitempty"`
		Username     string `json:"username,omitempty"`
		LanguageCode string `json:"language_code,omitempty"`
	} `json:"from,omitempty"`
	Chat *struct {
		ID        int64  `json:"id"`
		Type      string `json:"type"`
		FirstName string `json:"first_name,omitempty"`
		LastName  string `json:"last_name,omitempty"`
		Username  string `json:"username,omitempty"`
	} `json:"chat,omitempty"`
	Date int64  `json:"date"`
	Text string `json:"text,omitempty"`
}) error {
	// Проверяем ID сообщения
	if message.MessageID <= 0 {
		return errors.NewBotError("INVALID_MESSAGE_ID", "некорректный message_id")
	}

	// Проверяем отправителя
	if message.From == nil {
		return errors.NewBotError("MISSING_FROM", "отсутствует информация об отправителе")
	}

	if message.From.ID <= 0 {
		return errors.NewBotError("INVALID_USER_ID", "некорректный ID пользователя")
	}

	if message.From.IsBot {
		return errors.NewBotError("BOT_MESSAGE", "сообщения от ботов не принимаются")
	}

	if strings.TrimSpace(message.From.FirstName) == "" {
		return errors.NewBotError("MISSING_FIRST_NAME", "отсутствует имя пользователя")
	}

	// Проверяем чат
	if message.Chat == nil {
		return errors.NewBotError("MISSING_CHAT", "отсутствует информация о чате")
	}

	if message.Chat.ID == 0 {
		return errors.NewBotError("INVALID_CHAT_ID", "некорректный ID чата")
	}

	// Проверяем тип чата (принимаем только private)
	if message.Chat.Type != "private" {
		return errors.NewBotError("INVALID_CHAT_TYPE", "поддерживаются только приватные чаты")
	}

	// Проверяем временную метку
	if message.Date <= 0 {
		return errors.NewBotError("INVALID_DATE", "некорректная временная метка")
	}

	msgTime := time.Unix(message.Date, 0)
	if time.Since(msgTime) > 5*time.Minute {
		return errors.NewBotError("MESSAGE_TOO_OLD", "сообщение слишком старое")
	}

	// Проверяем длину текста
	if len(message.Text) > 4096 { // Лимит Telegram
		return errors.NewBotError("TEXT_TOO_LONG", "текст сообщения слишком длинный")
	}

	return nil
}

// validateCallbackQuery валидирует callback_query из update
func (v *RequestValidator) validateCallbackQuery(callback *struct {
	ID   string `json:"id"`
	From *struct {
		ID           int64  `json:"id"`
		IsBot        bool   `json:"is_bot"`
		FirstName    string `json:"first_name"`
		LastName     string `json:"last_name,omitempty"`
		Username     string `json:"username,omitempty"`
		LanguageCode string `json:"language_code,omitempty"`
	} `json:"from,omitempty"`
	Data string `json:"data,omitempty"`
}) error {
	// Проверяем ID callback
	if strings.TrimSpace(callback.ID) == "" {
		return errors.NewBotError("MISSING_CALLBACK_ID", "отсутствует ID callback")
	}

	// Проверяем отправителя
	if callback.From == nil {
		return errors.NewBotError("MISSING_FROM", "отсутствует информация об отправителе callback")
	}

	if callback.From.ID <= 0 {
		return errors.NewBotError("INVALID_USER_ID", "некорректный ID пользователя в callback")
	}

	if callback.From.IsBot {
		return errors.NewBotError("BOT_CALLBACK", "callback от ботов не принимаются")
	}

	// Проверяем данные callback
	if len(callback.Data) > 64 { // Лимит Telegram для callback_data
		return errors.NewBotError("CALLBACK_DATA_TOO_LONG", "данные callback слишком длинные")
	}

	return nil
}

// ValidateSlotRequest валидирует запрос на операции со слотами
func (v *RequestValidator) ValidateSlotRequest(slotID string, chatID int64) error {
	// Валидируем ID слота
	if _, err := validation.ValidateSlotID(slotID); err != nil {
		return err
	}

	// Валидируем chat ID
	if err := validation.ValidateChatID(chatID); err != nil {
		return err
	}

	return nil
}

// ValidateUserRegistration валидирует данные для регистрации пользователя
func (v *RequestValidator) ValidateUserRegistration(chatID int64, phone, firstName, lastName string) error {
	// Валидируем chat ID
	if err := validation.ValidateChatID(chatID); err != nil {
		return err
	}

	// Валидируем телефон
	if err := validation.ValidatePhoneNumber(phone); err != nil {
		return err
	}

	// Валидируем имя
	if err := validation.ValidateUserName(firstName); err != nil {
		return errors.NewBotError("INVALID_FIRST_NAME", "некорректное имя").WithError(err)
	}

	// Валидируем фамилию (опционально)
	if lastName != "" {
		if err := validation.ValidateUserName(lastName); err != nil {
			return errors.NewBotError("INVALID_LAST_NAME", "некорректная фамилия").WithError(err)
		}
	}

	return nil
}

// ValidateDateRequest валидирует запрос на получение слотов по дате
func (v *RequestValidator) ValidateDateRequest(dateStr string) error {
	_, err := validation.ValidateDate(dateStr)
	return err
}

// ValidateSlotCreation валидирует данные для создания слота
func (v *RequestValidator) ValidateSlotCreation(date, startTime, endTime string) error {
	// Валидируем дату
	if _, err := validation.ValidateDate(date); err != nil {
		return err
	}

	// Валидируем время начала
	startParsed, err := validation.ValidateTime(startTime)
	if err != nil {
		return errors.NewBotError("INVALID_START_TIME", "некорректное время начала").WithError(err)
	}

	// Валидируем время окончания
	endParsed, err := validation.ValidateTime(endTime)
	if err != nil {
		return errors.NewBotError("INVALID_END_TIME", "некорректное время окончания").WithError(err)
	}

	// Проверяем логику времени
	if !endParsed.After(*startParsed) {
		return errors.NewBotError("INVALID_TIME_RANGE", "время окончания должно быть после времени начала")
	}

	// Проверяем продолжительность
	duration := endParsed.Sub(*startParsed)
	if duration < 5*time.Minute {
		return errors.NewBotError("SLOT_TOO_SHORT", "слот должен быть минимум 5 минут")
	}

	if duration > 8*time.Hour {
		return errors.NewBotError("SLOT_TOO_LONG", "слот не может быть больше 8 часов")
	}

	return nil
}

// sanitizeInput очищает входную строку от потенциально опасных символов
func (v *RequestValidator) sanitizeInput(input string) string {
	// Убираем управляющие символы
	cleaned := strings.Map(func(r rune) rune {
		if r < 32 && r != 9 && r != 10 && r != 13 { // Оставляем только tab, LF, CR
			return -1
		}
		return r
	}, input)

	// Ограничиваем длину
	if len(cleaned) > 1000 {
		cleaned = cleaned[:1000]
	}

	return strings.TrimSpace(cleaned)
}

// validateHTMLContent проверяет контент на наличие HTML/скриптов
func (v *RequestValidator) validateHTMLContent(content string) error {
	dangerous := []string{
		"<script", "</script>", "javascript:", "data:",
		"<iframe", "</iframe>", "<object", "</object>",
		"<embed", "</embed>", "onload=", "onerror=",
	}

	lowerContent := strings.ToLower(content)
	for _, pattern := range dangerous {
		if strings.Contains(lowerContent, pattern) {
			return errors.NewBotError("DANGEROUS_CONTENT", "контент содержит потенциально опасные элементы")
		}
	}

	return nil
}
