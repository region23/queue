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
	// Load configuration (includes .env loading)
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

	// Register bot commands
	if err := app.registerBotCommands(); err != nil {
		log.Printf("Warning: failed to register bot commands: %v", err)
	}

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
	app.handlers["start"] = handleStart
	app.handlers["help"] = handleHelp
	app.handlers["book"] = handleBook
	app.handlers["myslots"] = handleMySlots
	app.handlers["cancel"] = handleCancel
	app.handlers["admin"] = handleAdmin
}

// registerBotCommands registers commands in Telegram Bot Menu
func (app *App) registerBotCommands() error {
	commands := []tgbotapi.BotCommand{
		{
			Command:     "start",
			Description: "🚀 Начать работу с ботом",
		},
		{
			Command:     "book",
			Description: "📅 Записаться на приём",
		},
		{
			Command:     "myslots",
			Description: "📋 Мои записи",
		},
		{
			Command:     "cancel",
			Description: "❌ Отменить запись",
		},
		{
			Command:     "help",
			Description: "❓ Справка",
		},
		{
			Command:     "admin",
			Description: "⚙️ Админ панель",
		},
	}

	setCommands := tgbotapi.NewSetMyCommands(commands...)
	_, err := app.bot.Request(setCommands)
	if err != nil {
		return fmt.Errorf("failed to set bot commands: %w", err)
	}

	log.Println("Bot commands registered successfully")
	return nil
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
	if update.Message != nil {
		if update.Message.IsCommand() {
			command := update.Message.Command()
			log.Printf("Received command: '%s' from user %d", command, update.Message.From.ID)

			handler, exists := app.handlers[command]
			if !exists {
				log.Printf("Unknown command: '%s'", command)
				return app.sendMessage(update.Message.Chat.ID, "Неизвестная команда. Используйте /help")
			}
			return handler(app, update)
		} else if update.Message.Contact != nil {
			// Handle shared contact
			return app.handleContact(update)
		} else {
			// Handle regular text messages
			log.Printf("Received text message: '%s' from user %d", update.Message.Text, update.Message.From.ID)
			return app.sendMessage(update.Message.Chat.ID, "Привет! Используйте команды из меню или /help для справки.")
		}
	}

	return nil
}

// handleContact processes shared contact
func (app *App) handleContact(update *tgbotapi.Update) error {
	contact := update.Message.Contact
	userID := update.Message.From.ID

	// Verify this is the user's own contact
	if contact.UserID != userID {
		return app.sendMessage(update.Message.Chat.ID, "Пожалуйста, поделитесь своим собственным контактом.")
	}

	// Update user's phone in database
	err := UpdateUserPhone(app.db, userID, contact.PhoneNumber)
	if err != nil {
		log.Printf("Error updating user phone: %v", err)
		return app.sendMessage(update.Message.Chat.ID, "Произошла ошибка при сохранении номера. Попробуйте позже.")
	}

	log.Printf("Updated phone for user %d: %s", userID, contact.PhoneNumber)

	// Remove keyboard and send success message
	msg := tgbotapi.NewMessage(update.Message.Chat.ID, fmt.Sprintf(`✅ Отлично! Ваш номер телефона сохранен: %s

Теперь вы можете записываться на приём:
📅 /book - Записаться на приём
📋 /myslots - Мои записи`, contact.PhoneNumber))

	msg.ReplyMarkup = tgbotapi.NewRemoveKeyboard(true)

	_, err = app.bot.Send(msg)
	return err
}

// handleCallbackQuery processes callback queries
func (app *App) handleCallbackQuery(callback *tgbotapi.CallbackQuery) error {
	// Answer callback to remove loading state
	callbackConfig := tgbotapi.NewCallback(callback.ID, "")
	app.bot.Send(callbackConfig)

	// Parse callback data
	parts := strings.Split(callback.Data, "_")
	if len(parts) < 2 {
		return nil
	}

	action := parts[0]

	switch action {
	case "date":
		if len(parts) != 2 {
			return nil
		}
		return app.handleDateCallback(callback, parts[1])
	case "slot":
		if len(parts) != 3 {
			return nil
		}
		dateTimeStr := parts[1] + "_" + parts[2]
		return app.handleSlotCallback(callback, dateTimeStr)
	case "book":
		slotID, err := strconv.Atoi(parts[1])
		if err != nil {
			return nil
		}
		return app.handleBookCallback(callback, slotID)
	case "cancel":
		slotID, err := strconv.Atoi(parts[1])
		if err != nil {
			return nil
		}
		return app.handleCancelCallback(callback, slotID)
	}

	return nil
}

// handleDateCallback handles date selection
func (app *App) handleDateCallback(callback *tgbotapi.CallbackQuery, dateStr string) error {
	date, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		return app.sendMessage(callback.Message.Chat.ID, "Неверный формат даты")
	}

	// Delete the date selection message
	deleteMsg := tgbotapi.NewDeleteMessage(callback.Message.Chat.ID, callback.Message.MessageID)
	app.bot.Send(deleteMsg)

	// Show slots for selected date
	return app.showSlotsForDate(callback.Message.Chat.ID, date)
}

// handleSlotCallback handles time slot selection and booking
func (app *App) handleSlotCallback(callback *tgbotapi.CallbackQuery, dateTimeStr string) error {
	// Parse date and time
	slotTime, err := time.Parse("2006-01-02_15:04", dateTimeStr)
	if err != nil {
		return app.sendMessage(callback.Message.Chat.ID, "Неверный формат времени")
	}

	userID := callback.From.ID
	username := callback.From.UserName
	if username == "" {
		username = callback.From.FirstName
	}

	// Book the slot
	err = BookTimeSlot(app.db, slotTime, userID, username, app.config)
	if err != nil {
		log.Printf("Error booking slot: %v", err)
		return app.sendMessage(callback.Message.Chat.ID, fmt.Sprintf("❌ Не удалось забронировать слот: %v", err))
	}

	// Delete the slot selection message
	deleteMsg := tgbotapi.NewDeleteMessage(callback.Message.Chat.ID, callback.Message.MessageID)
	app.bot.Send(deleteMsg)

	message := fmt.Sprintf("✅ Вы успешно записались на приём:\n📅 %s", slotTime.Format("02.01.2006 15:04"))
	return app.sendMessage(callback.Message.Chat.ID, message)
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

	return app.sendMessage(callback.Message.Chat.ID, "✅ Вы успешно записались на приём!")
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

	// Send cancellation confirmation
	app.sendMessage(callback.Message.Chat.ID, "❌ Запись отменена.")

	// Automatically show booking options
	if app.config.ScheduleDays > 1 {
		return app.showBookingDates(callback.Message.Chat.ID)
	} else {
		return app.showSlotsForDate(callback.Message.Chat.ID, time.Now())
	}
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
	userID := update.Message.From.ID

	// Check if user exists
	user, err := GetUserByTelegramID(app.db, userID)
	if err != nil {
		log.Printf("Error checking user: %v", err)
		return app.sendMessage(update.Message.Chat.ID, "Произошла ошибка. Попробуйте позже.")
	}

	// If user doesn't exist, create them
	if user == nil {
		firstName := update.Message.From.FirstName
		lastName := update.Message.From.LastName
		username := update.Message.From.UserName

		user, err = CreateUser(app.db, userID, firstName, lastName, username)
		if err != nil {
			log.Printf("Error creating user: %v", err)
			return app.sendMessage(update.Message.Chat.ID, "Произошла ошибка при регистрации. Попробуйте позже.")
		}
		log.Printf("Created new user: %d (%s)", userID, firstName)
	}

	// Check if user has phone number
	if user.PhoneNumber == "" {
		message := fmt.Sprintf(`Привет, %s! 👋

Для записи на приём нам нужен ваш номер телефона.

Пожалуйста, поделитесь своим контактом, нажав кнопку ниже:`, user.FirstName)

		// Create contact request keyboard
		contactButton := tgbotapi.NewKeyboardButtonContact("📱 Поделиться номером телефона")
		keyboard := tgbotapi.NewReplyKeyboard([]tgbotapi.KeyboardButton{contactButton})
		keyboard.OneTimeKeyboard = true
		keyboard.ResizeKeyboard = true

		msg := tgbotapi.NewMessage(update.Message.Chat.ID, message)
		msg.ReplyMarkup = keyboard
		msg.ParseMode = "HTML"

		_, err = app.bot.Send(msg)
		return err
	}

	// User is fully registered
	message := fmt.Sprintf(`Добро пожаловать, %s! 👋

Вы зарегистрированы в системе.
Телефон: %s

Доступные команды:
📅 /book - Записаться на приём
📋 /myslots - Мои записи  
❌ /cancel - Отменить запись
❓ /help - Справка`, user.FirstName, user.PhoneNumber)

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
	userID := update.Message.From.ID

	// Check if user is registered and has phone
	registered, err := IsUserRegistered(app.db, userID)
	if err != nil {
		log.Printf("Error checking user registration: %v", err)
		return app.sendMessage(update.Message.Chat.ID, "Произошла ошибка. Попробуйте позже.")
	}

	if !registered {
		return app.sendMessage(update.Message.Chat.ID, `❌ Для записи на приём необходимо зарегистрироваться и указать номер телефона.

Пожалуйста, используйте команду /start для регистрации.`)
	}

	// Check if user already has an active booking
	activeSlot, err := GetUserActiveSlot(app.db, userID)
	if err != nil {
		log.Printf("Error checking user active slot: %v", err)
		return app.sendMessage(update.Message.Chat.ID, "Произошла ошибка. Попробуйте позже.")
	}

	if activeSlot != nil {
		message := fmt.Sprintf(`У вас уже есть активная запись:
📅 %s

Хотите отменить её и записаться на другое время?`,
			activeSlot.StartTime.Format("02.01.2006 15:04"))

		cancelBtn := tgbotapi.NewInlineKeyboardButtonData("❌ Отменить запись", fmt.Sprintf("cancel_%d", activeSlot.ID))
		keyboard := tgbotapi.NewInlineKeyboardMarkup([]tgbotapi.InlineKeyboardButton{cancelBtn})

		msg := tgbotapi.NewMessage(update.Message.Chat.ID, message)
		msg.ReplyMarkup = keyboard

		_, err = app.bot.Send(msg)
		return err
	}

	// Show dates or slots based on SCHEDULE_DAYS
	if app.config.ScheduleDays > 1 {
		return app.showBookingDates(update.Message.Chat.ID)
	} else {
		return app.showSlotsForDate(update.Message.Chat.ID, time.Now())
	}
}

// showBookingDates shows available dates for booking
func (app *App) showBookingDates(chatID int64) error {
	dates := GetBookingDates(app.config.ScheduleDays, app.config)

	if len(dates) == 0 {
		return app.sendMessage(chatID, "Нет доступных дат для записи")
	}

	var rows [][]tgbotapi.InlineKeyboardButton
	for _, date := range dates {
		dateStr := date.Format("2006-01-02")
		displayStr := date.Format("02.01 (Mon)")

		// Translate day names to Russian
		dayName := ""
		switch date.Weekday() {
		case time.Monday:
			dayName = "Пн"
		case time.Tuesday:
			dayName = "Вт"
		case time.Wednesday:
			dayName = "Ср"
		case time.Thursday:
			dayName = "Чт"
		case time.Friday:
			dayName = "Пт"
		}

		displayStr = date.Format("02.01") + " (" + dayName + ")"

		btn := tgbotapi.NewInlineKeyboardButtonData(displayStr, "date_"+dateStr)
		rows = append(rows, []tgbotapi.InlineKeyboardButton{btn})
	}

	keyboard := tgbotapi.NewInlineKeyboardMarkup(rows...)
	msg := tgbotapi.NewMessage(chatID, "Выберите дату для записи:")
	msg.ReplyMarkup = keyboard

	_, err := app.bot.Send(msg)
	return err
}

// showSlotsForDate shows available time slots for a specific date
func (app *App) showSlotsForDate(chatID int64, date time.Time) error {
	slots, err := GetAvailableSlotsForDate(app.db, date, app.config)
	if err != nil {
		log.Printf("Error getting available slots: %v", err)
		return app.sendMessage(chatID, "Ошибка при получении доступных слотов")
	}

	if len(slots) == 0 {
		// If no slots available for today, suggest next working day
		today := time.Now()
		if date.Format("2006-01-02") == today.Format("2006-01-02") {
			nextWorkday := GetNextAvailableWorkday(today, app.config)
			nextSlots, err := GetAvailableSlotsForDate(app.db, nextWorkday, app.config)
			if err != nil {
				log.Printf("Error getting next day slots: %v", err)
				return app.sendMessage(chatID, "К сожалению, нет доступных слотов на сегодня")
			}

			if len(nextSlots) > 0 {
				nextDateStr := nextWorkday.Format("02.01.2006")
				message := fmt.Sprintf(`К сожалению, нет доступных слотов на сегодня.

Хотите посмотреть доступные слоты на %s?`, nextDateStr)

				nextDayBtn := tgbotapi.NewInlineKeyboardButtonData(
					fmt.Sprintf("📅 Показать слоты на %s", nextDateStr),
					fmt.Sprintf("date_%s", nextWorkday.Format("2006-01-02")),
				)
				keyboard := tgbotapi.NewInlineKeyboardMarkup([]tgbotapi.InlineKeyboardButton{nextDayBtn})

				msg := tgbotapi.NewMessage(chatID, message)
				msg.ReplyMarkup = keyboard

				_, err = app.bot.Send(msg)
				return err
			}
		}

		dateStr := date.Format("02.01.2006")
		return app.sendMessage(chatID, fmt.Sprintf("К сожалению, нет доступных слотов на %s", dateStr))
	}

	var rows [][]tgbotapi.InlineKeyboardButton
	var currentRow []tgbotapi.InlineKeyboardButton

	for i, slot := range slots {
		timeStr := slot.Format("15:04")
		slotData := fmt.Sprintf("slot_%s", slot.Format("2006-01-02_15:04"))

		btn := tgbotapi.NewInlineKeyboardButtonData(timeStr, slotData)
		currentRow = append(currentRow, btn)

		// Add row when we have 3 buttons or it's the last slot
		if len(currentRow) == 3 || i == len(slots)-1 {
			rows = append(rows, currentRow)
			currentRow = []tgbotapi.InlineKeyboardButton{}
		}
	}

	keyboard := tgbotapi.NewInlineKeyboardMarkup(rows...)
	dateStr := date.Format("02.01.2006")
	msg := tgbotapi.NewMessage(chatID, fmt.Sprintf("Выберите время на %s:", dateStr))
	msg.ReplyMarkup = keyboard

	_, err = app.bot.Send(msg)
	return err
}

func handleMySlots(app *App, update *tgbotapi.Update) error {
	userID := update.Message.From.ID

	// Check if user is registered
	registered, err := IsUserRegistered(app.db, userID)
	if err != nil {
		log.Printf("Error checking user registration: %v", err)
		return app.sendMessage(update.Message.Chat.ID, "Произошла ошибка. Попробуйте позже.")
	}

	if !registered {
		return app.sendMessage(update.Message.Chat.ID, `❌ Для просмотра записей необходимо зарегистрироваться.

Пожалуйста, используйте команду /start для регистрации.`)
	}

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

	// Check if user is registered
	registered, err := IsUserRegistered(app.db, userID)
	if err != nil {
		log.Printf("Error checking user registration: %v", err)
		return app.sendMessage(update.Message.Chat.ID, "Произошла ошибка. Попробуйте позже.")
	}

	if !registered {
		return app.sendMessage(update.Message.Chat.ID, `❌ Для отмены записей необходимо зарегистрироваться.

Пожалуйста, используйте команду /start для регистрации.`)
	}

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
