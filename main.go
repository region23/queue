package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	_ "github.com/mattn/go-sqlite3"
)

// App contains all dependencies
type App struct {
	bot      *tgbotapi.BotAPI
	db       *sql.DB
	config   *Config
	handlers map[string]HandlerFunc
}

// HandlerFunc is a simple handler function type
type HandlerFunc func(*App, *tgbotapi.Update) error

func main() {
	// Load configuration
	config, err := LoadConfig()
	if err != nil {
		log.Fatal("Failed to load config:", err)
	}

	// Initialize database
	db, err := InitDB(config.DBFile)
	if err != nil {
		log.Fatal("Failed to init database:", err)
	}
	defer db.Close()

	// Initialize bot
	bot, err := tgbotapi.NewBotAPI(config.TelegramToken)
	if err != nil {
		log.Fatal("Failed to create bot:", err)
	}

	// Create app instance
	app := &App{
		bot:      bot,
		db:       db,
		config:   config,
		handlers: make(map[string]HandlerFunc),
	}

	// Register handlers
	app.registerHandlers()

	// Generate initial slots for next 30 days
	if err := GenerateSlots(db, config, time.Now(), time.Now().AddDate(0, 0, 30)); err != nil {
		log.Printf("Warning: failed to generate slots: %v", err)
	}

	// Set webhook
	webhookURL := fmt.Sprintf("%s/webhook/%s", config.WebhookURL, bot.Token)
	webhook, _ := tgbotapi.NewWebhook(webhookURL)
	if _, err := bot.Request(webhook); err != nil {
		log.Fatal("Failed to set webhook:", err)
	}

	// Create rate limiter
	rateLimiter := NewRateLimiter(60, time.Minute) // 60 requests per minute

	// Start HTTP server with middleware
	http.HandleFunc("/webhook/"+bot.Token, LoggingMiddleware(RateLimitMiddleware(rateLimiter)(app.handleWebhook)))
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	log.Printf("Starting server on %s", config.ServerAddress)
	if err := http.ListenAndServe(config.ServerAddress, nil); err != nil {
		log.Fatal("Server failed:", err)
	}
}

// registerHandlers registers all command handlers
func (app *App) registerHandlers() {
	app.handlers["/start"] = handleStart
	app.handlers["/help"] = handleHelp
	app.handlers["/book"] = handleBook
	app.handlers["/myslots"] = handleMySlots
	app.handlers["/cancel"] = handleCancel
	app.handlers["/admin"] = handleAdmin
}

// handleWebhook processes incoming webhook requests
func (app *App) handleWebhook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	var update tgbotapi.Update
	if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
		log.Printf("Failed to decode update: %v", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// Process update
	if err := app.processUpdate(&update); err != nil {
		log.Printf("Error processing update: %v", err)
	}

	w.WriteHeader(http.StatusOK)
}

// processUpdate routes updates to appropriate handlers
func (app *App) processUpdate(update *tgbotapi.Update) error {
	// Handle callback queries
	if update.CallbackQuery != nil {
		return app.handleCallbackQuery(update.CallbackQuery)
	}

	// Handle messages
	if update.Message != nil && update.Message.IsCommand() {
		handler, exists := app.handlers[update.Message.Command()]
		if !exists {
			return app.sendMessage(update.Message.Chat.ID, "Неизвестная команда. Используйте /help")
		}
		return handler(app, update)
	}

	return nil
}

// handleCallbackQuery processes callback queries
func (app *App) handleCallbackQuery(callback *tgbotapi.CallbackQuery) error {
	// Answer callback to remove loading state
	callbackConfig := tgbotapi.NewCallback(callback.ID, "")
	app.bot.Send(callbackConfig)

	// Parse callback data
	parts := strings.Split(callback.Data, "_")
	if len(parts) != 2 {
		return nil
	}

	action := parts[0]
	slotID, err := strconv.Atoi(parts[1])
	if err != nil {
		return nil
	}

	switch action {
	case "book":
		return app.handleBookCallback(callback, slotID)
	case "cancel":
		return app.handleCancelCallback(callback, slotID)
	}

	return nil
}

// handleBookCallback handles slot booking
func (app *App) handleBookCallback(callback *tgbotapi.CallbackQuery, slotID int) error {
	userID := callback.From.ID
	username := callback.From.UserName
	if username == "" {
		username = callback.From.FirstName
	}

	err := BookSlot(app.db, slotID, userID, username)
	if err != nil {
		return app.sendMessage(callback.Message.Chat.ID, "Не удалось забронировать слот. Возможно, он уже занят.")
	}

	// Delete the keyboard message
	deleteMsg := tgbotapi.NewDeleteMessage(callback.Message.Chat.ID, callback.Message.MessageID)
	app.bot.Send(deleteMsg)

	return app.sendMessage(callback.Message.Chat.ID, "✅ Вы успешно записались на прием!")
}

// handleCancelCallback handles slot cancellation
func (app *App) handleCancelCallback(callback *tgbotapi.CallbackQuery, slotID int) error {
	userID := callback.From.ID

	err := CancelSlot(app.db, slotID, userID)
	if err != nil {
		return app.sendMessage(callback.Message.Chat.ID, "Не удалось отменить запись.")
	}

	// Delete the keyboard message
	deleteMsg := tgbotapi.NewDeleteMessage(callback.Message.Chat.ID, callback.Message.MessageID)
	app.bot.Send(deleteMsg)

	return app.sendMessage(callback.Message.Chat.ID, "❌ Запись отменена.")
}

// sendMessage sends a message to a user
func (app *App) sendMessage(chatID int64, text string) error {
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = "HTML"
	_, err := app.bot.Send(msg)
	return err
}

// Handler implementations

func handleStart(app *App, update *tgbotapi.Update) error {
	message := `Добро пожаловать в бот записи на прием!

Доступные команды:
/book - Записаться на прием
/myslots - Мои записи
/cancel - Отменить запись
/help - Справка`

	return app.sendMessage(update.Message.Chat.ID, message)
}

func handleHelp(app *App, update *tgbotapi.Update) error {
	message := `Справка по командам:

/start - Начать работу с ботом
/book - Выбрать время для записи
/myslots - Посмотреть свои записи
/cancel - Отменить существующую запись
/help - Показать это сообщение`

	return app.sendMessage(update.Message.Chat.ID, message)
}

func handleBook(app *App, update *tgbotapi.Update) error {
	// Get available slots
	slots, err := GetAvailableSlots(app.db, time.Now(), time.Now().AddDate(0, 0, 7))
	if err != nil {
		return app.sendMessage(update.Message.Chat.ID, "Ошибка при получении доступных слотов")
	}

	if len(slots) == 0 {
		return app.sendMessage(update.Message.Chat.ID, "К сожалению, нет доступных слотов для записи")
	}

	// Create inline keyboard
	var rows [][]tgbotapi.InlineKeyboardButton
	for _, slot := range slots {
		btn := tgbotapi.NewInlineKeyboardButtonData(
			slot.StartTime.Format("02.01 15:04"),
			fmt.Sprintf("book_%d", slot.ID),
		)
		rows = append(rows, []tgbotapi.InlineKeyboardButton{btn})
	}

	keyboard := tgbotapi.NewInlineKeyboardMarkup(rows...)
	msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Выберите удобное время:")
	msg.ReplyMarkup = keyboard

	_, err = app.bot.Send(msg)
	return err
}

func handleMySlots(app *App, update *tgbotapi.Update) error {
	userID := update.Message.From.ID

	slots, err := GetUserSlots(app.db, int64(userID))
	if err != nil {
		return app.sendMessage(update.Message.Chat.ID, "Ошибка при получении ваших записей")
	}

	if len(slots) == 0 {
		return app.sendMessage(update.Message.Chat.ID, "У вас нет активных записей")
	}

	message := "Ваши записи:\n\n"
	for _, slot := range slots {
		message += fmt.Sprintf("📅 %s\n", slot.StartTime.Format("02.01.2006 15:04"))
	}

	return app.sendMessage(update.Message.Chat.ID, message)
}

func handleCancel(app *App, update *tgbotapi.Update) error {
	userID := update.Message.From.ID

	slots, err := GetUserSlots(app.db, int64(userID))
	if err != nil {
		return app.sendMessage(update.Message.Chat.ID, "Ошибка при получении ваших записей")
	}

	if len(slots) == 0 {
		return app.sendMessage(update.Message.Chat.ID, "У вас нет записей для отмены")
	}

	// Create inline keyboard for cancellation
	var rows [][]tgbotapi.InlineKeyboardButton
	for _, slot := range slots {
		btn := tgbotapi.NewInlineKeyboardButtonData(
			slot.StartTime.Format("02.01 15:04"),
			fmt.Sprintf("cancel_%d", slot.ID),
		)
		rows = append(rows, []tgbotapi.InlineKeyboardButton{btn})
	}

	keyboard := tgbotapi.NewInlineKeyboardMarkup(rows...)
	msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Выберите запись для отмены:")
	msg.ReplyMarkup = keyboard

	_, err = app.bot.Send(msg)
	return err
}

func handleAdmin(app *App, update *tgbotapi.Update) error {
	userID := update.Message.From.ID

	// Check if user is admin
	if !IsAdmin(app.config, int64(userID)) {
		return app.sendMessage(update.Message.Chat.ID, "У вас нет прав администратора")
	}

	// Get statistics
	stats, err := GetStatistics(app.db)
	if err != nil {
		return app.sendMessage(update.Message.Chat.ID, "Ошибка при получении статистики")
	}

	message := fmt.Sprintf(`📊 Статистика:

Всего слотов: %d
Забронировано: %d
Доступно: %d
Пользователей: %d`,
		stats.TotalSlots,
		stats.BookedSlots,
		stats.AvailableSlots,
		stats.TotalUsers,
	)

	return app.sendMessage(update.Message.Chat.ID, message)
}