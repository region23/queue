package main

import (
	"database/sql"
	"fmt"
	"time"
)

// User represents a registered user
type User struct {
	ID          int64
	TelegramID  int64
	FirstName   string
	LastName    string
	Username    string
	PhoneNumber string
	CreatedAt   time.Time
	IsActive    bool
}

// Slot represents a time slot
type Slot struct {
	ID        int
	StartTime time.Time
	EndTime   time.Time
	UserID    sql.NullInt64
	Username  sql.NullString
	CreatedAt time.Time
}

// Stats holds statistics
type Stats struct {
	TotalSlots     int
	BookedSlots    int
	AvailableSlots int
	TotalUsers     int
}

// InitDB initializes the database
func InitDB(dbFile string) (*sql.DB, error) {
	db, err := sql.Open("sqlite3", dbFile)
	if err != nil {
		return nil, err
	}

	// Create tables if not exist
	query := `
	CREATE TABLE IF NOT EXISTS users (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		telegram_id INTEGER UNIQUE NOT NULL,
		first_name TEXT NOT NULL,
		last_name TEXT,
		username TEXT,
		phone_number TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		is_active BOOLEAN DEFAULT 1
	);

	CREATE TABLE IF NOT EXISTS slots (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		start_time DATETIME NOT NULL,
		end_time DATETIME NOT NULL,
		user_id INTEGER,
		username TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (user_id) REFERENCES users (telegram_id)
	);

	CREATE INDEX IF NOT EXISTS idx_users_telegram_id ON users(telegram_id);
	CREATE INDEX IF NOT EXISTS idx_start_time ON slots(start_time);
	CREATE INDEX IF NOT EXISTS idx_user_id ON slots(user_id);
	`

	if _, err := db.Exec(query); err != nil {
		return nil, err
	}

	return db, nil
}

// GetUserSlots returns slots for a specific user
func GetUserSlots(db *sql.DB, userID int64) ([]Slot, error) {
	query := `
		SELECT id, start_time, end_time, created_at
		FROM slots
		WHERE user_id = ? AND start_time > ?
		ORDER BY start_time
	`

	rows, err := db.Query(query, userID, time.Now())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var slots []Slot
	for rows.Next() {
		var slot Slot
		if err := rows.Scan(&slot.ID, &slot.StartTime, &slot.EndTime, &slot.CreatedAt); err != nil {
			return nil, err
		}
		slot.UserID.Int64 = userID
		slot.UserID.Valid = true
		slots = append(slots, slot)
	}

	return slots, nil
}

// CancelSlot cancels a user's slot
func CancelSlot(db *sql.DB, slotID int, userID int64) error {
	query := `
		UPDATE slots 
		SET user_id = NULL, username = NULL
		WHERE id = ? AND user_id = ?
	`

	result, err := db.Exec(query, slotID, userID)
	if err != nil {
		return err
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if affected == 0 {
		return fmt.Errorf("slot not found or not owned by user")
	}

	return nil
}

// GetStatistics returns booking statistics
func GetStatistics(db *sql.DB) (*Stats, error) {
	stats := &Stats{}

	// Total slots
	err := db.QueryRow("SELECT COUNT(*) FROM slots").Scan(&stats.TotalSlots)
	if err != nil {
		return nil, err
	}

	// Booked slots
	err = db.QueryRow("SELECT COUNT(*) FROM slots WHERE user_id IS NOT NULL").Scan(&stats.BookedSlots)
	if err != nil {
		return nil, err
	}

	// Available slots
	stats.AvailableSlots = stats.TotalSlots - stats.BookedSlots

	// Total users (registered users, not just those who booked)
	err = db.QueryRow("SELECT COUNT(*) FROM users WHERE is_active = 1").Scan(&stats.TotalUsers)
	if err != nil {
		return nil, err
	}

	return stats, nil
}

// GenerateSlots generates slots for a date range
func GenerateSlots(db *sql.DB, config *Config, from, to time.Time) error {
	// Parse work hours
	workStart, err := time.Parse("15:04", config.WorkStart)
	if err != nil {
		return err
	}
	workEnd, err := time.Parse("15:04", config.WorkEnd)
	if err != nil {
		return err
	}

	// Generate slots for each day
	for d := from; d.Before(to); d = d.AddDate(0, 0, 1) {
		// Skip weekends only if SKIP_WEEKEND is enabled
		if config.SkipWeekend && IsWeekend(d) {
			continue
		}

		// Generate slots for the day
		start := time.Date(d.Year(), d.Month(), d.Day(), workStart.Hour(), workStart.Minute(), 0, 0, d.Location())
		end := time.Date(d.Year(), d.Month(), d.Day(), workEnd.Hour(), workEnd.Minute(), 0, 0, d.Location())

		for slot := start; slot.Before(end); slot = slot.Add(time.Duration(config.SlotDuration) * time.Minute) {
			slotEnd := slot.Add(time.Duration(config.SlotDuration) * time.Minute)

			// Check if slot already exists
			var exists bool
			err := db.QueryRow("SELECT EXISTS(SELECT 1 FROM slots WHERE start_time = ?)", slot).Scan(&exists)
			if err != nil {
				return err
			}

			if !exists {
				_, err = db.Exec("INSERT INTO slots (start_time, end_time) VALUES (?, ?)", slot, slotEnd)
				if err != nil {
					return err
				}
			}
		}
	}

	return nil
}

// User management functions

// GetUserByTelegramID gets user by Telegram ID
func GetUserByTelegramID(db *sql.DB, telegramID int64) (*User, error) {
	query := `
		SELECT id, telegram_id, first_name, last_name, username, phone_number, created_at, is_active
		FROM users
		WHERE telegram_id = ? AND is_active = 1
	`

	var user User
	var lastName, username, phoneNumber sql.NullString

	err := db.QueryRow(query, telegramID).Scan(
		&user.ID, &user.TelegramID, &user.FirstName, &lastName,
		&username, &phoneNumber, &user.CreatedAt, &user.IsActive,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // User not found
		}
		return nil, err
	}

	// Handle nullable fields
	if lastName.Valid {
		user.LastName = lastName.String
	}
	if username.Valid {
		user.Username = username.String
	}
	if phoneNumber.Valid {
		user.PhoneNumber = phoneNumber.String
	}

	return &user, nil
}

// CreateUser creates a new user
func CreateUser(db *sql.DB, telegramID int64, firstName, lastName, username string) (*User, error) {
	query := `
		INSERT INTO users (telegram_id, first_name, last_name, username)
		VALUES (?, ?, ?, ?)
	`

	var lastNamePtr, usernamePtr *string
	if lastName != "" {
		lastNamePtr = &lastName
	}
	if username != "" {
		usernamePtr = &username
	}

	result, err := db.Exec(query, telegramID, firstName, lastNamePtr, usernamePtr)
	if err != nil {
		return nil, err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, err
	}

	// Return created user
	user := &User{
		ID:         id,
		TelegramID: telegramID,
		FirstName:  firstName,
		LastName:   lastName,
		Username:   username,
		CreatedAt:  time.Now(),
		IsActive:   true,
	}

	return user, nil
}

// UpdateUserPhone updates user's phone number
func UpdateUserPhone(db *sql.DB, telegramID int64, phoneNumber string) error {
	query := `
		UPDATE users 
		SET phone_number = ?
		WHERE telegram_id = ?
	`

	result, err := db.Exec(query, phoneNumber, telegramID)
	if err != nil {
		return err
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if affected == 0 {
		return fmt.Errorf("user not found")
	}

	return nil
}

// IsUserRegistered checks if user is registered and has phone
func IsUserRegistered(db *sql.DB, telegramID int64) (bool, error) {
	query := `
		SELECT COUNT(*) 
		FROM users 
		WHERE telegram_id = ? AND phone_number IS NOT NULL AND phone_number != '' AND is_active = 1
	`

	var count int
	err := db.QueryRow(query, telegramID).Scan(&count)
	if err != nil {
		return false, err
	}

	return count > 0, nil
}

// Booking logic functions

// GetBookingDates returns available booking dates from today to today + (scheduleDays-1)
func GetBookingDates(scheduleDays int, config *Config) []time.Time {
	var dates []time.Time
	today := time.Now()

	for i := 0; i < scheduleDays; i++ {
		date := today.AddDate(0, 0, i)
		// Skip weekends only if SKIP_WEEKEND is enabled
		if !config.SkipWeekend || !IsWeekend(date) {
			dates = append(dates, date)
		}
	}

	return dates
}

// GenerateSlotsForDate creates time slots for a specific date based on config
func GenerateSlotsForDate(date time.Time, config *Config) []time.Time {
	var slots []time.Time

	// Parse work hours
	workStart, err := time.Parse("15:04", config.WorkStart)
	if err != nil {
		return slots
	}
	workEnd, err := time.Parse("15:04", config.WorkEnd)
	if err != nil {
		return slots
	}

	// Create slots for the day
	start := time.Date(date.Year(), date.Month(), date.Day(), workStart.Hour(), workStart.Minute(), 0, 0, date.Location())
	end := time.Date(date.Year(), date.Month(), date.Day(), workEnd.Hour(), workEnd.Minute(), 0, 0, date.Location())

	for slot := start; slot.Before(end); slot = slot.Add(time.Duration(config.SlotDuration) * time.Minute) {
		slots = append(slots, slot)
	}

	return slots
}

// FilterFutureSlots removes past slots from the list
func FilterFutureSlots(slots []time.Time, now time.Time) []time.Time {
	var futureSlots []time.Time

	for _, slot := range slots {
		if slot.After(now) {
			futureSlots = append(futureSlots, slot)
		}
	}

	return futureSlots
}

// GetAvailableSlotsForDate returns available (unbooked) slots for a specific date
func GetAvailableSlotsForDate(db *sql.DB, date time.Time, config *Config) ([]time.Time, error) {
	// Generate all possible slots for the date
	allSlots := GenerateSlotsForDate(date, config)

	// Filter future slots if it's today
	now := time.Now()
	if date.Format("2006-01-02") == now.Format("2006-01-02") {
		allSlots = FilterFutureSlots(allSlots, now)
	}

	// Get booked slots from database
	query := `
		SELECT start_time 
		FROM slots 
		WHERE DATE(start_time) = DATE(?) AND user_id IS NOT NULL
	`

	rows, err := db.Query(query, date)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	bookedSlots := make(map[string]bool)
	for rows.Next() {
		var bookedTime time.Time
		if err := rows.Scan(&bookedTime); err != nil {
			continue
		}
		bookedSlots[bookedTime.Format("15:04")] = true
	}

	// Filter out booked slots
	var availableSlots []time.Time
	for _, slot := range allSlots {
		if !bookedSlots[slot.Format("15:04")] {
			availableSlots = append(availableSlots, slot)
		}
	}

	return availableSlots, nil
}

// GetUserActiveSlot returns user's active slot (future booking)
func GetUserActiveSlot(db *sql.DB, userID int64) (*Slot, error) {
	query := `
		SELECT id, start_time, end_time, created_at
		FROM slots
		WHERE user_id = ? AND start_time > ?
		ORDER BY start_time
		LIMIT 1
	`

	var slot Slot
	err := db.QueryRow(query, userID, time.Now()).Scan(
		&slot.ID, &slot.StartTime, &slot.EndTime, &slot.CreatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // No active slot
		}
		return nil, err
	}

	slot.UserID.Int64 = userID
	slot.UserID.Valid = true

	return &slot, nil
}

// BookTimeSlot books a specific time slot for a user
func BookTimeSlot(db *sql.DB, slotTime time.Time, userID int64, username string, config *Config) error {
	// Check if user already has an active booking
	activeSlot, err := GetUserActiveSlot(db, userID)
	if err != nil {
		return err
	}
	if activeSlot != nil {
		return fmt.Errorf("пользователь уже имеет активную запись на %s", activeSlot.StartTime.Format("02.01.2006 15:04"))
	}

	// Calculate end time
	endTime := slotTime.Add(time.Duration(config.SlotDuration) * time.Minute)

	// First try to update existing empty slot
	updateQuery := `
		UPDATE slots 
		SET user_id = ?, username = ?
		WHERE start_time = ? AND user_id IS NULL
	`

	result, err := db.Exec(updateQuery, userID, username, slotTime)
	if err != nil {
		return err
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if affected > 0 {
		return nil // Successfully booked existing slot
	}

	// If no existing slot was updated, try to create new one
	insertQuery := `
		INSERT INTO slots (start_time, end_time, user_id, username)
		VALUES (?, ?, ?, ?)
	`

	_, err = db.Exec(insertQuery, slotTime, endTime, userID, username)
	if err != nil {
		return fmt.Errorf("слот уже забронирован или произошла ошибка")
	}

	return nil
}

// IsWeekend checks if the given date is a weekend day
func IsWeekend(date time.Time) bool {
	return date.Weekday() == time.Saturday || date.Weekday() == time.Sunday
}

// GetNextAvailableWorkday finds the next available day based on SKIP_WEEKEND config
func GetNextAvailableWorkday(startDate time.Time, config *Config) time.Time {
	nextDay := startDate.AddDate(0, 0, 1)

	// If skip weekend is enabled, find next workday
	if config.SkipWeekend {
		for IsWeekend(nextDay) {
			nextDay = nextDay.AddDate(0, 0, 1)
		}
	}

	return nextDay
}
