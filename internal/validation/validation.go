package validation

import (
	"fmt"
	"regexp"
	"strconv"
	"time"

	"github.com/region23/queue/pkg/errors"
)

// Регулярные выражения для валидации
var (
	phoneRegex = regexp.MustCompile(`^\+[1-9]\d{1,14}$`)
	dateRegex  = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)
	timeRegex  = regexp.MustCompile(`^\d{2}:\d{2}$`)
)

// ValidateSlotID валидирует ID слота
func ValidateSlotID(idStr string) (int, error) {
	if idStr == "" {
		return 0, errors.ErrInvalidSlotID.WithContext("ID слота не может быть пустым")
	}

	id, err := strconv.Atoi(idStr)
	if err != nil {
		return 0, errors.ErrInvalidSlotID.WithError(err).WithContext(map[string]interface{}{
			"input": idStr,
		})
	}

	if id <= 0 {
		return 0, errors.ErrInvalidSlotID.WithContext(map[string]interface{}{
			"input":  idStr,
			"reason": "ID должен быть положительным числом",
		})
	}

	return id, nil
}

// ValidatePhoneNumber валидирует номер телефона
func ValidatePhoneNumber(phone string) error {
	if phone == "" {
		return errors.ErrInvalidPhoneNumber.WithContext("номер телефона не может быть пустым")
	}

	if !phoneRegex.MatchString(phone) {
		return errors.ErrInvalidPhoneNumber.WithContext(map[string]interface{}{
			"phone":  phone,
			"reason": "номер должен быть в международном формате (+1234567890)",
		})
	}

	return nil
}

// ValidateDate валидирует дату в формате YYYY-MM-DD
func ValidateDate(dateStr string) (*time.Time, error) {
	if dateStr == "" {
		return nil, errors.ErrInvalidDate.WithContext("дата не может быть пустой")
	}

	if !dateRegex.MatchString(dateStr) {
		return nil, errors.ErrInvalidDate.WithContext(map[string]interface{}{
			"date":   dateStr,
			"reason": "дата должна быть в формате YYYY-MM-DD",
		})
	}

	date, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		return nil, errors.ErrInvalidDate.WithError(err).WithContext(map[string]interface{}{
			"date": dateStr,
		})
	}

	// Проверяем, что дата не в прошлом
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	if date.Before(today) {
		return nil, errors.ErrInvalidDate.WithContext(map[string]interface{}{
			"date":   dateStr,
			"reason": "нельзя выбрать дату в прошлом",
		})
	}

	return &date, nil
}

// ValidateTime валидирует время в формате HH:MM
func ValidateTime(timeStr string) (*time.Time, error) {
	if timeStr == "" {
		return nil, errors.ErrInvalidTime.WithContext("время не может быть пустым")
	}

	if !timeRegex.MatchString(timeStr) {
		return nil, errors.ErrInvalidTime.WithContext(map[string]interface{}{
			"time":   timeStr,
			"reason": "время должно быть в формате HH:MM",
		})
	}

	parsedTime, err := time.Parse("15:04", timeStr)
	if err != nil {
		return nil, errors.ErrInvalidTime.WithError(err).WithContext(map[string]interface{}{
			"time": timeStr,
		})
	}

	return &parsedTime, nil
}

// ValidateWorkingHours проверяет, входит ли время в рабочие часы
func ValidateWorkingHours(timeStr, workStart, workEnd string) error {
	targetTime, err := ValidateTime(timeStr)
	if err != nil {
		return err
	}

	startTime, err := ValidateTime(workStart)
	if err != nil {
		return errors.Wrap(err, "INVALID_CONFIG", "некорректное время начала работы")
	}

	endTime, err := ValidateTime(workEnd)
	if err != nil {
		return errors.Wrap(err, "INVALID_CONFIG", "некорректное время окончания работы")
	}

	// Сравниваем только часы и минуты
	target := targetTime.Hour()*60 + targetTime.Minute()
	start := startTime.Hour()*60 + startTime.Minute()
	end := endTime.Hour()*60 + endTime.Minute()

	if target < start || target >= end {
		return errors.ErrWorkingHoursViolation.WithContext(map[string]interface{}{
			"time":       timeStr,
			"work_start": workStart,
			"work_end":   workEnd,
		})
	}

	return nil
}

// ValidateSlotDuration проверяет корректность продолжительности слота
func ValidateSlotDuration(startTime, endTime string) error {
	start, err := ValidateTime(startTime)
	if err != nil {
		return fmt.Errorf("некорректное время начала: %w", err)
	}

	end, err := ValidateTime(endTime)
	if err != nil {
		return fmt.Errorf("некорректное время окончания: %w", err)
	}

	if end.Before(*start) || end.Equal(*start) {
		return errors.ErrInvalidTime.WithContext(map[string]interface{}{
			"start_time": startTime,
			"end_time":   endTime,
			"reason":     "время окончания должно быть позже времени начала",
		})
	}

	return nil
}

// ValidateChatID валидирует Telegram Chat ID
func ValidateChatID(chatID int64) error {
	if chatID == 0 {
		return errors.NewBotError("INVALID_CHAT_ID", "Chat ID не может быть равен нулю")
	}

	// В Telegram Chat ID могут быть отрицательными для групп
	// Для пользователей они обычно положительные
	// Но мы принимаем любые ненулевые значения
	return nil
}

// ValidateUserName валидирует имя пользователя
func ValidateUserName(name string) error {
	if name == "" {
		return errors.NewBotError("INVALID_USER_NAME", "имя пользователя не может быть пустым")
	}

	if len(name) > 100 {
		return errors.NewBotError("INVALID_USER_NAME", "имя пользователя слишком длинное (максимум 100 символов)")
	}

	return nil
}

// ValidateScheduleDays валидирует количество дней для планирования
func ValidateScheduleDays(days int) error {
	if days <= 0 {
		return errors.NewBotError("INVALID_SCHEDULE_DAYS", "количество дней должно быть положительным")
	}

	if days > 365 {
		return errors.NewBotError("INVALID_SCHEDULE_DAYS", "слишком много дней для планирования (максимум 365)")
	}

	return nil
}

// ValidateSlotDurationMinutes валидирует продолжительность слота в минутах
func ValidateSlotDurationMinutes(duration int) error {
	if duration <= 0 {
		return errors.NewBotError("INVALID_SLOT_DURATION", "продолжительность слота должна быть положительной")
	}

	if duration > 480 { // 8 часов
		return errors.NewBotError("INVALID_SLOT_DURATION", "слишком длинный слот (максимум 8 часов)")
	}

	if duration < 5 {
		return errors.NewBotError("INVALID_SLOT_DURATION", "слишком короткий слот (минимум 5 минут)")
	}

	return nil
}
