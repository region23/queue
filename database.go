package main

import (
	"database/sql"
	"fmt"
	"time"
)

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
	CREATE TABLE IF NOT EXISTS slots (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		start_time DATETIME NOT NULL,
		end_time DATETIME NOT NULL,
		user_id INTEGER,
		username TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE INDEX IF NOT EXISTS idx_start_time ON slots(start_time);
	CREATE INDEX IF NOT EXISTS idx_user_id ON slots(user_id);
	`

	if _, err := db.Exec(query); err != nil {
		return nil, err
	}

	return db, nil
}

// GetAvailableSlots returns available slots between dates
func GetAvailableSlots(db *sql.DB, from, to time.Time) ([]Slot, error) {
	query := `
		SELECT id, start_time, end_time 
		FROM slots 
		WHERE start_time >= ? AND start_time <= ? AND user_id IS NULL
		ORDER BY start_time
		LIMIT 10
	`

	rows, err := db.Query(query, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var slots []Slot
	for rows.Next() {
		var slot Slot
		if err := rows.Scan(&slot.ID, &slot.StartTime, &slot.EndTime); err != nil {
			return nil, err
		}
		slots = append(slots, slot)
	}

	return slots, nil
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

// BookSlot books a slot for a user
func BookSlot(db *sql.DB, slotID int, userID int64, username string) error {
	query := `
		UPDATE slots 
		SET user_id = ?, username = ?
		WHERE id = ? AND user_id IS NULL
	`

	result, err := db.Exec(query, userID, username, slotID)
	if err != nil {
		return err
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if affected == 0 {
		return fmt.Errorf("slot already booked or not found")
	}

	return nil
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

	// Total users
	err = db.QueryRow("SELECT COUNT(DISTINCT user_id) FROM slots WHERE user_id IS NOT NULL").Scan(&stats.TotalUsers)
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
		// Skip weekends
		if d.Weekday() == time.Saturday || d.Weekday() == time.Sunday {
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