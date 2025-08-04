package errors

import "fmt"

// BotError представляет ошибку бота с кодом и контекстом
type BotError struct {
	Code    string      `json:"code"`
	Message string      `json:"message"`
	Err     error       `json:"-"`
	Context interface{} `json:"context,omitempty"`
}

// Error реализует интерфейс error
func (e *BotError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %s: %v", e.Code, e.Message, e.Err)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// Unwrap позволяет использовать errors.Is и errors.As
func (e *BotError) Unwrap() error {
	return e.Err
}

// WithContext добавляет контекст к ошибке
func (e *BotError) WithContext(ctx interface{}) *BotError {
	return &BotError{
		Code:    e.Code,
		Message: e.Message,
		Err:     e.Err,
		Context: ctx,
	}
}

// WithError добавляет underlying ошибку
func (e *BotError) WithError(err error) *BotError {
	return &BotError{
		Code:    e.Code,
		Message: e.Message,
		Err:     err,
		Context: e.Context,
	}
}

// Предопределенные ошибки
var (
	// Ошибки пользователя
	ErrUserNotRegistered = &BotError{
		Code:    "USER_NOT_REGISTERED",
		Message: "пользователь не зарегистрирован",
	}

	ErrUserAlreadyRegistered = &BotError{
		Code:    "USER_ALREADY_REGISTERED",
		Message: "пользователь уже зарегистрирован",
	}

	// Ошибки слотов
	ErrSlotNotFound = &BotError{
		Code:    "SLOT_NOT_FOUND",
		Message: "слот не найден",
	}

	ErrSlotAlreadyReserved = &BotError{
		Code:    "SLOT_ALREADY_RESERVED",
		Message: "слот уже зарезервирован",
	}

	ErrSlotNotReserved = &BotError{
		Code:    "SLOT_NOT_RESERVED",
		Message: "слот не зарезервирован",
	}

	ErrSlotNotBelongsToUser = &BotError{
		Code:    "SLOT_NOT_BELONGS_TO_USER",
		Message: "слот не принадлежит пользователю",
	}

	ErrNoAvailableSlots = &BotError{
		Code:    "NO_AVAILABLE_SLOTS",
		Message: "нет доступных слотов на выбранную дату",
	}

	// Ошибки валидации
	ErrInvalidSlotID = &BotError{
		Code:    "INVALID_SLOT_ID",
		Message: "некорректный ID слота",
	}

	ErrInvalidDate = &BotError{
		Code:    "INVALID_DATE",
		Message: "некорректная дата",
	}

	ErrInvalidTime = &BotError{
		Code:    "INVALID_TIME",
		Message: "некорректное время",
	}

	ErrInvalidPhoneNumber = &BotError{
		Code:    "INVALID_PHONE_NUMBER",
		Message: "некорректный номер телефона",
	}

	// Системные ошибки
	ErrDatabaseConnection = &BotError{
		Code:    "DATABASE_CONNECTION",
		Message: "ошибка подключения к базе данных",
	}

	ErrConfigurationInvalid = &BotError{
		Code:    "CONFIGURATION_INVALID",
		Message: "некорректная конфигурация",
	}

	ErrTelegramAPI = &BotError{
		Code:    "TELEGRAM_API",
		Message: "ошибка Telegram API",
	}

	ErrSchedulerUnavailable = &BotError{
		Code:    "SCHEDULER_UNAVAILABLE",
		Message: "планировщик недоступен",
	}

	// Ошибки бизнес-логики
	ErrWorkingHoursViolation = &BotError{
		Code:    "WORKING_HOURS_VIOLATION",
		Message: "время не входит в рабочие часы",
	}

	ErrTooManyActiveSlots = &BotError{
		Code:    "TOO_MANY_ACTIVE_SLOTS",
		Message: "слишком много активных записей",
	}

	ErrSlotTooSoon = &BotError{
		Code:    "SLOT_TOO_SOON",
		Message: "нельзя записаться на ближайшее время",
	}
)

// NewBotError создает новую ошибку бота
func NewBotError(code, message string) *BotError {
	return &BotError{
		Code:    code,
		Message: message,
	}
}

// Wrap оборачивает обычную ошибку в BotError
func Wrap(err error, code, message string) *BotError {
	return &BotError{
		Code:    code,
		Message: message,
		Err:     err,
	}
}

// IsBotError проверяет, является ли ошибка BotError
func IsBotError(err error) bool {
	_, ok := err.(*BotError)
	return ok
}

// GetBotError извлекает BotError из ошибки
func GetBotError(err error) (*BotError, bool) {
	botErr, ok := err.(*BotError)
	return botErr, ok
}
