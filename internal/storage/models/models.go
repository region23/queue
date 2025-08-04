package models

import "time"

// User представляет пользователя системы
type User struct {
	ID        int64     `json:"id" db:"id"`
	ChatID    int64     `json:"chat_id" db:"chat_id"`
	Phone     string    `json:"phone" db:"phone"`
	FirstName string    `json:"first_name" db:"first_name"`
	LastName  string    `json:"last_name" db:"last_name"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

// Slot представляет слот для записи
type Slot struct {
	ID         int       `json:"id" db:"id"`
	Date       string    `json:"date" db:"date"`
	StartTime  string    `json:"start_time" db:"start_time"`
	EndTime    string    `json:"end_time" db:"end_time"`
	UserChatID *int64    `json:"user_chat_id,omitempty" db:"user_chat_id"`
	Notified   bool      `json:"notified" db:"notified"`
	CreatedAt  time.Time `json:"created_at" db:"created_at"`
	UpdatedAt  time.Time `json:"updated_at" db:"updated_at"`
}

// IsReserved проверяет, зарезервирован ли слот
func (s *Slot) IsReserved() bool {
	return s.UserChatID != nil
}

// IsAvailable проверяет, доступен ли слот для бронирования
func (s *Slot) IsAvailable() bool {
	return s.UserChatID == nil
}

// GetFormattedTime возвращает отформатированное время слота
func (s *Slot) GetFormattedTime() string {
	return s.StartTime + " - " + s.EndTime
}

// GetFormattedDateTime возвращает отформатированные дату и время
func (s *Slot) GetFormattedDateTime() string {
	return s.Date + " " + s.GetFormattedTime()
}
