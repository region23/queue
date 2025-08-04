package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"telegram_queue_bot/internal/storage/models"
	"time"

	_ "modernc.org/sqlite"
)

// SQLiteStorage реализует интерфейс Storage для SQLite
type SQLiteStorage struct {
	db *sql.DB
}

// New создает новое подключение к SQLite базе данных
func New(dbPath string) (*SQLiteStorage, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Настройка подключения
	db.SetMaxOpenConns(1) // SQLite поддерживает только одно write-подключение
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(time.Hour)

	storage := &SQLiteStorage{db: db}

	if err := storage.migrate(); err != nil {
		db.Close()
		return nil, fmt.Errorf("migration failed: %w", err)
	}

	return storage, nil
}

// migrate выполняет миграции базы данных
func (s *SQLiteStorage) migrate() error {
	// Включаем WAL mode для лучшей конкурентности
	if _, err := s.db.Exec(`PRAGMA journal_mode=WAL`); err != nil {
		return fmt.Errorf("failed to set WAL mode: %w", err)
	}

	// Включаем foreign keys
	if _, err := s.db.Exec(`PRAGMA foreign_keys=ON`); err != nil {
		return fmt.Errorf("failed to enable foreign keys: %w", err)
	}

	queries := []string{
		`CREATE TABLE IF NOT EXISTS users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			chat_id INTEGER UNIQUE NOT NULL,
			phone TEXT NOT NULL,
			first_name TEXT NOT NULL,
			last_name TEXT NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS slots (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			date TEXT NOT NULL,
			start_time TEXT NOT NULL,
			end_time TEXT NOT NULL,
			user_chat_id INTEGER,
			notified INTEGER DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(date, start_time),
			FOREIGN KEY(user_chat_id) REFERENCES users(chat_id) ON DELETE SET NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_slots_date ON slots(date)`,
		`CREATE INDEX IF NOT EXISTS idx_slots_user_chat_id ON slots(user_chat_id)`,
		`CREATE INDEX IF NOT EXISTS idx_slots_notified ON slots(notified)`,
		`CREATE INDEX IF NOT EXISTS idx_users_chat_id ON users(chat_id)`,
	}

	for _, query := range queries {
		if _, err := s.db.Exec(query); err != nil {
			return fmt.Errorf("failed to execute migration query: %w", err)
		}
	}

	return nil
}

// Close закрывает подключение к базе данных
func (s *SQLiteStorage) Close() error {
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}

// Ping проверяет подключение к базе данных
func (s *SQLiteStorage) Ping(ctx context.Context) error {
	return s.db.PingContext(ctx)
}

// SaveUser сохраняет пользователя в базе данных
func (s *SQLiteStorage) SaveUser(ctx context.Context, chatID int64, phone, firstName, lastName string) error {
	query := `INSERT OR REPLACE INTO users (chat_id, phone, first_name, last_name, updated_at) 
			  VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP)`

	_, err := s.db.ExecContext(ctx, query, chatID, phone, firstName, lastName)
	if err != nil {
		return fmt.Errorf("failed to save user: %w", err)
	}

	return nil
}

// IsUserRegistered проверяет, зарегистрирован ли пользователь
func (s *SQLiteStorage) IsUserRegistered(ctx context.Context, chatID int64) (bool, error) {
	var count int
	query := `SELECT COUNT(*) FROM users WHERE chat_id = ?`

	err := s.db.QueryRowContext(ctx, query, chatID).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("failed to check user registration: %w", err)
	}

	return count > 0, nil
}

// GetUserByID получает пользователя по chat_id
func (s *SQLiteStorage) GetUserByID(ctx context.Context, chatID int64) (*models.User, error) {
	user := &models.User{}
	query := `SELECT id, chat_id, phone, first_name, last_name, created_at, updated_at 
			  FROM users WHERE chat_id = ?`

	err := s.db.QueryRowContext(ctx, query, chatID).Scan(
		&user.ID, &user.ChatID, &user.Phone, &user.FirstName, &user.LastName,
		&user.CreatedAt, &user.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("user not found")
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	return user, nil
}

// CreateSlot создает новый слот
func (s *SQLiteStorage) CreateSlot(ctx context.Context, slot *models.Slot) error {
	query := `INSERT INTO slots (date, start_time, end_time, user_chat_id, notified) 
			  VALUES (?, ?, ?, ?, ?)`

	result, err := s.db.ExecContext(ctx, query, slot.Date, slot.StartTime, slot.EndTime, slot.UserChatID, slot.Notified)
	if err != nil {
		return fmt.Errorf("failed to create slot: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("failed to get slot ID: %w", err)
	}

	slot.ID = int(id)
	return nil
}

// GetAvailableSlots получает доступные слоты на дату
func (s *SQLiteStorage) GetAvailableSlots(ctx context.Context, date string) ([]*models.Slot, error) {
	query := `SELECT id, date, start_time, end_time, user_chat_id, notified, created_at, updated_at 
			  FROM slots WHERE date = ? AND user_chat_id IS NULL 
			  ORDER BY start_time`

	rows, err := s.db.QueryContext(ctx, query, date)
	if err != nil {
		return nil, fmt.Errorf("failed to get available slots: %w", err)
	}
	defer rows.Close()

	var slots []*models.Slot
	for rows.Next() {
		slot := &models.Slot{}
		err := rows.Scan(
			&slot.ID, &slot.Date, &slot.StartTime, &slot.EndTime, &slot.UserChatID,
			&slot.Notified, &slot.CreatedAt, &slot.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan slot: %w", err)
		}
		slots = append(slots, slot)
	}

	return slots, nil
}

// ReserveSlot бронирует слот для пользователя
func (s *SQLiteStorage) ReserveSlot(ctx context.Context, slotID int, chatID int64) error {
	query := `UPDATE slots SET user_chat_id = ?, updated_at = CURRENT_TIMESTAMP 
			  WHERE id = ? AND user_chat_id IS NULL`

	result, err := s.db.ExecContext(ctx, query, chatID, slotID)
	if err != nil {
		return fmt.Errorf("failed to reserve slot: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get affected rows: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("slot is not available or does not exist")
	}

	return nil
}

// GetSlotByID получает слот по ID
func (s *SQLiteStorage) GetSlotByID(ctx context.Context, id int) (*models.Slot, error) {
	slot := &models.Slot{}
	query := `SELECT id, date, start_time, end_time, user_chat_id, notified, created_at, updated_at 
			  FROM slots WHERE id = ?`

	err := s.db.QueryRowContext(ctx, query, id).Scan(
		&slot.ID, &slot.Date, &slot.StartTime, &slot.EndTime, &slot.UserChatID,
		&slot.Notified, &slot.CreatedAt, &slot.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("slot not found")
		}
		return nil, fmt.Errorf("failed to get slot: %w", err)
	}

	return slot, nil
}

// GetUserTodaySlot получает сегодняшний слот пользователя
func (s *SQLiteStorage) GetUserTodaySlot(ctx context.Context, chatID int64) (*models.Slot, bool, error) {
	today := time.Now().Format("2006-01-02")

	slot := &models.Slot{}
	query := `SELECT id, date, start_time, end_time, user_chat_id, notified, created_at, updated_at 
			  FROM slots WHERE user_chat_id = ? AND date = ?`

	err := s.db.QueryRowContext(ctx, query, chatID, today).Scan(
		&slot.ID, &slot.Date, &slot.StartTime, &slot.EndTime, &slot.UserChatID,
		&slot.Notified, &slot.CreatedAt, &slot.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("failed to get user today slot: %w", err)
	}

	return slot, true, nil
}

// MarkSlotNotified помечает слот как уведомленный
func (s *SQLiteStorage) MarkSlotNotified(ctx context.Context, slotID int) error {
	query := `UPDATE slots SET notified = 1, updated_at = CURRENT_TIMESTAMP WHERE id = ?`

	_, err := s.db.ExecContext(ctx, query, slotID)
	if err != nil {
		return fmt.Errorf("failed to mark slot as notified: %w", err)
	}

	return nil
}

// GetPendingNotifications получает слоты, требующие уведомления
func (s *SQLiteStorage) GetPendingNotifications(ctx context.Context) ([]*models.Slot, error) {
	query := `SELECT id, date, start_time, end_time, user_chat_id, notified, created_at, updated_at 
			  FROM slots WHERE user_chat_id IS NOT NULL AND notified = 0`

	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to get pending notifications: %w", err)
	}
	defer rows.Close()

	var slots []*models.Slot
	for rows.Next() {
		slot := &models.Slot{}
		err := rows.Scan(
			&slot.ID, &slot.Date, &slot.StartTime, &slot.EndTime, &slot.UserChatID,
			&slot.Notified, &slot.CreatedAt, &slot.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan pending notification slot: %w", err)
		}
		slots = append(slots, slot)
	}

	return slots, nil
}

// GetUserActiveSlots получает активные слоты пользователя
func (s *SQLiteStorage) GetUserActiveSlots(ctx context.Context, chatID int64) ([]*models.Slot, error) {
	query := `SELECT id, date, start_time, end_time, user_chat_id, notified, created_at, updated_at 
			  FROM slots WHERE user_chat_id = ? AND date >= date('now') 
			  ORDER BY date, start_time`

	rows, err := s.db.QueryContext(ctx, query, chatID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user active slots: %w", err)
	}
	defer rows.Close()

	var slots []*models.Slot
	for rows.Next() {
		slot := &models.Slot{}
		err := rows.Scan(
			&slot.ID, &slot.Date, &slot.StartTime, &slot.EndTime, &slot.UserChatID,
			&slot.Notified, &slot.CreatedAt, &slot.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan user active slot: %w", err)
		}
		slots = append(slots, slot)
	}

	return slots, nil
}

// CancelSlot отменяет бронирование слота
func (s *SQLiteStorage) CancelSlot(ctx context.Context, slotID int, chatID int64) error {
	query := `UPDATE slots SET user_chat_id = NULL, notified = 0, updated_at = CURRENT_TIMESTAMP 
			  WHERE id = ? AND user_chat_id = ?`

	result, err := s.db.ExecContext(ctx, query, slotID, chatID)
	if err != nil {
		return fmt.Errorf("failed to cancel slot: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get affected rows: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("slot not found or not belongs to user")
	}

	return nil
}
