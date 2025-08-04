package main

// A simple Telegram queue bot implemented in Go.  This program sets up a webhook
// endpoint that Telegram can call whenever users interact with the bot.  It
// allows users to share their phone number, pick a date, choose a free time
// slot and receive a notification when their slot begins.  All data is stored
// in a local SQLite database, but a small in‑memory map of timers is also
// maintained to fire notifications at the appropriate time.  To keep the
// example self‑contained, this code relies only on the standard library
// (except for the Telegram API client and SQLite driver) and uses
// `time.AfterFunc` for scheduling instead of external cron daemons.  If you
// restart the process it will reschedule notifications for any future slots
// that have not yet fired.

import (
    "context"
    "database/sql"
    "encoding/json"
    "errors"
    "fmt"
    "log"
    "net/http"
    "os"
    "strconv"
    "strings"
    "sync"
    "time"

    _ "modernc.org/sqlite" // register the sqlite driver

    tgbotapi "github.com/go-telegram/bot"
)

// dateFormat and dateTimeFormat define the layouts used for parsing and
// formatting dates and times.  Go uses a special reference date (Mon Jan 2
// 15:04:05 MST 2006) to define layouts.  See the time package for details.
const (
    dateFormat     = "2006-01-02"
    timeFormat     = "15:04"
    dateTimeFormat = "2006-01-02 15:04"
)

// global variables for the bot, database and timers.  A mutex protects the
// timers map because timers may be inserted from concurrent goroutines.
var (
    bot       *tgbotapi.BotAPI
    db        *sql.DB
    timers    = make(map[int]*time.Timer)
    timersMtx sync.Mutex
)

// configuration values.  These can be overridden through environment
// variables.  The admin may specify the daily start and end hours for slots
// (formatted as "HH:MM") and the slot duration in minutes.  By default the
// working day runs from 09:00 to 18:00 with 30 minute slots.
var (
    workStart        = getEnv("WORK_START", "09:00")
    workEnd          = getEnv("WORK_END", "18:00")
    slotDurationMins = getEnvAsInt("SLOT_DURATION", 30)
    scheduleDays     = getEnvAsInt("SCHEDULE_DAYS", 7)
)

// getEnv reads an environment variable and returns its value or a fallback.
func getEnv(key, fallback string) string {
    if v := os.Getenv(key); v != "" {
        return v
    }
    return fallback
}

// getEnvAsInt reads an environment variable and converts it to an integer.
// Returns fallback if conversion fails.
func getEnvAsInt(key string, fallback int) int {
    if v := os.Getenv(key); v != "" {
        if i, err := strconv.Atoi(v); err == nil {
            return i
        }
    }
    return fallback
}

// Slot represents a single reservable time interval on a specific date.  If
// UserChatID is non‑zero, the slot has been reserved by a user.  Notified
// indicates whether the user has already been notified that their slot has
// started.
type Slot struct {
    ID         int
    Date       string
    StartTime  string
    EndTime    string
    UserChatID int64
    Notified   bool
}

// initDB opens the SQLite database at path and creates the required tables if
// they do not exist.  The users table stores basic contact details and the
// slots table stores the schedule along with reservation information.
func initDB(path string) error {
    var err error
    db, err = sql.Open("sqlite", path)
    if err != nil {
        return err
    }
    // enable WAL mode for better concurrency
    if _, err = db.Exec(`PRAGMA journal_mode=WAL`); err != nil {
        return err
    }
    _, err = db.Exec(`CREATE TABLE IF NOT EXISTS users (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        chat_id INTEGER UNIQUE,
        phone TEXT,
        first_name TEXT,
        last_name TEXT
    );`)
    if err != nil {
        return err
    }
    _, err = db.Exec(`CREATE TABLE IF NOT EXISTS slots (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        date TEXT,
        start_time TEXT,
        end_time TEXT,
        user_chat_id INTEGER,
        notified INTEGER DEFAULT 0,
        UNIQUE(date, start_time)
    );`)
    return err
}

// saveUser inserts a new user into the users table.  It uses INSERT OR IGNORE
// so that repeated calls with the same chatID will not create duplicate
// entries.
func saveUser(chatID int64, phone, firstName, lastName string) error {
    _, err := db.Exec(`INSERT OR IGNORE INTO users(chat_id, phone, first_name, last_name) VALUES (?, ?, ?, ?)`, chatID, phone, firstName, lastName)
    return err
}

// generateSlotsForDate populates the slots table with all possible time slots for
// a given date.  It computes intervals starting at workStart and ending at
// workEnd, each of length slotDurationMins.  Slots already in the table are
// ignored thanks to the UNIQUE constraint.
func generateSlotsForDate(date time.Time) error {
    start, err := time.Parse(timeFormat, workStart)
    if err != nil {
        return err
    }
    end, err := time.Parse(timeFormat, workEnd)
    if err != nil {
        return err
    }
    duration := time.Duration(slotDurationMins) * time.Minute
    // Build full times for the specific date
    current := time.Date(date.Year(), date.Month(), date.Day(), start.Hour(), start.Minute(), 0, 0, date.Location())
    endDateTime := time.Date(date.Year(), date.Month(), date.Day(), end.Hour(), end.Minute(), 0, 0, date.Location())
    for !current.After(endDateTime.Add(-duration)) {
        startStr := current.Format(timeFormat)
        endStr := current.Add(duration).Format(timeFormat)
        if _, err := db.Exec(`INSERT OR IGNORE INTO slots(date, start_time, end_time) VALUES (?, ?, ?)`, date.Format(dateFormat), startStr, endStr); err != nil {
            return err
        }
        current = current.Add(duration)
    }
    return nil
}

// listAvailableDates returns a slice of ISO date strings for the next n days.
func listAvailableDates(days int) []string {
    var dates []string
    now := time.Now()
    for i := 0; i < days; i++ {
        d := now.AddDate(0, 0, i)
        dates = append(dates, d.Format(dateFormat))
    }
    return dates
}

// getAvailableSlots returns all unreserved slots for the given date sorted by
// start_time.
func getAvailableSlots(date string) ([]Slot, error) {
    rows, err := db.Query(`SELECT id, start_time, end_time FROM slots WHERE date = ? AND user_chat_id IS NULL ORDER BY start_time`, date)
    if err != nil {
        return nil, err
    }
    defer rows.Close()
    var slots []Slot
    for rows.Next() {
        var s Slot
        s.Date = date
        if err := rows.Scan(&s.ID, &s.StartTime, &s.EndTime); err != nil {
            return nil, err
        }
        slots = append(slots, s)
    }
    return slots, nil
}

// reserveSlot attempts to assign the slot with the given ID to the user with
// chatID.  It only succeeds if the slot is currently unreserved.
func reserveSlot(slotID int, chatID int64) error {
    res, err := db.Exec(`UPDATE slots SET user_chat_id = ? WHERE id = ? AND user_chat_id IS NULL`, chatID, slotID)
    if err != nil {
        return err
    }
    rowsAffected, _ := res.RowsAffected()
    if rowsAffected == 0 {
        return errors.New("slot already reserved")
    }
    return nil
}

// getSlotByID retrieves a slot by its primary key.
func getSlotByID(id int) (Slot, error) {
    var s Slot
    var notifiedInt int
    err := db.QueryRow(`SELECT id, date, start_time, end_time, user_chat_id, notified FROM slots WHERE id = ?`, id).
        Scan(&s.ID, &s.Date, &s.StartTime, &s.EndTime, &s.UserChatID, &notifiedInt)
    if err != nil {
        return s, err
    }
    s.Notified = notifiedInt != 0
    return s, nil
}

// askForContact sends a message prompting the user to share their phone number.
// It uses a reply keyboard with the `request_contact` property so Telegram
// clients will show a button that, when pressed, sends the user's phone
// number.  According to the Bot API documentation, optional fields
// `request_contact` and `request_location` in a keyboard button are mutually
// exclusive and instruct the client to send the phone number or location on
// click【761115012080273†L2370-L2385】.
func askForContact(chatID int64) {
    msg := tgbotapi.NewMessage(chatID, "Пожалуйста, поделитесь своим номером телефона, нажав кнопку ниже.")
    button := tgbotapi.NewKeyboardButtonContact("Поделиться телефоном")
    row := tgbotapi.NewKeyboardButtonRow(button)
    keyboard := tgbotapi.NewReplyKeyboard(row)
    keyboard.OneTimeKeyboard = true
    keyboard.ResizeKeyboard = true
    msg.ReplyMarkup = keyboard
    if _, err := bot.Send(msg); err != nil {
        log.Println("failed to send contact request:", err)
    }
}

// showDateSelection displays an inline keyboard with the next scheduleDays dates.
// Each button uses callback data prefixed with "DATE:" so the handler can
// distinguish it from time slot selections.  Before presenting dates the
// function ensures that the slots table contains intervals for each date.
func showDateSelection(chatID int64) {
    dates := listAvailableDates(scheduleDays)
    // Pre‑populate the database with slots for each date
    for _, d := range dates {
        dt, _ := time.ParseInLocation(dateFormat, d, time.Local)
        if err := generateSlotsForDate(dt); err != nil {
            log.Println("failed to generate slots:", err)
        }
    }
    var rows [][]tgbotapi.InlineKeyboardButton
    for _, d := range dates {
        btn := tgbotapi.NewInlineKeyboardButtonData(d, "DATE:"+d)
        rows = append(rows, tgbotapi.NewInlineKeyboardRow(btn))
    }
    keyboard := tgbotapi.NewInlineKeyboardMarkup(rows...)
    msg := tgbotapi.NewMessage(chatID, "Выберите дату для записи:")
    msg.ReplyMarkup = keyboard
    if _, err := bot.Send(msg); err != nil {
        log.Println("failed to send date selection:", err)
    }
}

// showSlotSelection sends an inline keyboard listing all unreserved slots for
// the specified date.  Each button's callback data is prefixed with "SLOT:".
func showSlotSelection(chatID int64, date string) {
    slots, err := getAvailableSlots(date)
    if err != nil {
        log.Println("failed to list slots:", err)
        bot.Send(tgbotapi.NewMessage(chatID, "Ошибка при получении слотов"))
        return
    }
    if len(slots) == 0 {
        bot.Send(tgbotapi.NewMessage(chatID, "На выбранную дату нет свободных слотов. Попробуйте другую дату."))
        return
    }
    var rows [][]tgbotapi.InlineKeyboardButton
    for _, s := range slots {
        text := fmt.Sprintf("%s-%s", s.StartTime, s.EndTime)
        cbData := fmt.Sprintf("SLOT:%d", s.ID)
        btn := tgbotapi.NewInlineKeyboardButtonData(text, cbData)
        rows = append(rows, tgbotapi.NewInlineKeyboardRow(btn))
    }
    keyboard := tgbotapi.NewInlineKeyboardMarkup(rows...)
    msg := tgbotapi.NewMessage(chatID, "Выберите свободный временной слот:")
    msg.ReplyMarkup = keyboard
    if _, err := bot.Send(msg); err != nil {
        log.Println("failed to send slot selection:", err)
    }
}

// scheduleNotification schedules a one‑time notification for the given slot.  It
// uses time.AfterFunc to execute sendNotification when the slot's start time
// arrives.  The returned timer is stored so it can be cancelled if the slot is
// later cancelled.  Using time.AfterFunc is a lightweight way to schedule
// delayed execution in Go【835547532025432†L167-L185】.
func scheduleNotification(slot Slot) {
    // parse combined date and time
    t, err := time.ParseInLocation(dateTimeFormat, fmt.Sprintf("%s %s", slot.Date, slot.StartTime), time.Local)
    if err != nil {
        log.Println("failed to parse slot time:", err)
        return
    }
    delay := time.Until(t)
    if delay <= 0 {
        // time already passed; notify immediately
        go sendNotification(slot)
        return
    }
    timer := time.AfterFunc(delay, func() {
        sendNotification(slot)
    })
    timersMtx.Lock()
    timers[slot.ID] = timer
    timersMtx.Unlock()
}

// sendNotification sends a message to the user whose chat ID is stored in the
// slot and marks the slot as notified in the database.  After sending the
// message, the associated timer is removed from the map.
func sendNotification(slot Slot) {
    // Compose the notification text
    text := fmt.Sprintf("Ваша очередь подошла! Слот %s %s‑%s.", slot.Date, slot.StartTime, slot.EndTime)
    msg := tgbotapi.NewMessage(slot.UserChatID, text)
    if _, err := bot.Send(msg); err != nil {
        log.Println("failed to send notification:", err)
    }
    // mark the slot as notified
    if _, err := db.Exec(`UPDATE slots SET notified = 1 WHERE id = ?`, slot.ID); err != nil {
        log.Println("failed to mark slot notified:", err)
    }
    // remove timer
    timersMtx.Lock()
    if timer, ok := timers[slot.ID]; ok {
        timer.Stop()
        delete(timers, slot.ID)
    }
    timersMtx.Unlock()
}

// reschedulePending walks through all slots that have been reserved but not yet
// notified and schedules a notification for each.  Call this when the bot
// starts to ensure no appointments are missed after a restart.
func reschedulePending() {
    rows, err := db.Query(`SELECT id, date, start_time, end_time, user_chat_id FROM slots WHERE user_chat_id IS NOT NULL AND notified = 0`)
    if err != nil {
        log.Println("failed to query pending slots:", err)
        return
    }
    defer rows.Close()
    for rows.Next() {
        var s Slot
        if err := rows.Scan(&s.ID, &s.Date, &s.StartTime, &s.EndTime, &s.UserChatID); err != nil {
            log.Println("scan error:", err)
            continue
        }
        scheduleNotification(s)
    }
}

// handleUpdate dispatches incoming updates to either message or callback
// handlers.  It handles /start commands, contact sharing, and callback data for
// date and slot selection.
func handleUpdate(update tgbotapi.Update) {
    if update.Message != nil {
        chatID := update.Message.Chat.ID
        // If contact is shared, save user and proceed to date selection
        if update.Message.Contact != nil && update.Message.Contact.PhoneNumber != "" {
            c := update.Message.Contact
            if err := saveUser(chatID, c.PhoneNumber, c.FirstName, c.LastName); err != nil {
                log.Println("failed to save user:", err)
            }
            bot.Send(tgbotapi.NewMessage(chatID, "Телефон получен. Давайте запишемся."))
            showDateSelection(chatID)
            return
        }
        // On /start or unknown message, ask for contact
        if update.Message.IsCommand() && update.Message.Command() == "start" {
            askForContact(chatID)
            return
        }
        // Fallback: remind user to share contact or use /start
        bot.Send(tgbotapi.NewMessage(chatID, "Пожалуйста, нажмите /start, чтобы начать."))
        return
    }
    if update.CallbackQuery != nil {
        handleCallbackQuery(update.CallbackQuery)
    }
}

// handleCallbackQuery processes callback data from inline keyboard buttons.
// Two types of callback data are supported: DATE:YYYY‑MM‑DD to list slots and
// SLOT:<id> to reserve a slot.
func handleCallbackQuery(cb *tgbotapi.CallbackQuery) {
    chatID := cb.Message.Chat.ID
    data := cb.Data
    if strings.HasPrefix(data, "DATE:") {
        date := strings.TrimPrefix(data, "DATE:")
        showSlotSelection(chatID, date)
        // answer the callback to remove loading indicator
        bot.AnswerCallbackQuery(tgbotapi.NewCallback(cb.ID, ""))
        return
    }
    if strings.HasPrefix(data, "SLOT:") {
        idStr := strings.TrimPrefix(data, "SLOT:")
        id, _ := strconv.Atoi(idStr)
        // try to reserve
        if err := reserveSlot(id, chatID); err != nil {
            bot.AnswerCallbackQuery(tgbotapi.NewCallback(cb.ID, "Этот слот уже занят"))
            bot.Send(tgbotapi.NewMessage(chatID, "К сожалению, кто‑то забронировал этот слот раньше. Пожалуйста, выберите другой."))
            return
        }
        // fetch the slot to schedule notification
        slot, err := getSlotByID(id)
        if err != nil {
            log.Println("failed to fetch slot:", err)
        } else {
            scheduleNotification(slot)
        }
        bot.AnswerCallbackQuery(tgbotapi.NewCallback(cb.ID, "Слот забронирован"))
        bot.Send(tgbotapi.NewMessage(chatID, "Вы успешно забронировали слот. Мы уведомим вас, когда ваша очередь подойдёт."))
        return
    }
    // unknown callback
    bot.AnswerCallbackQuery(tgbotapi.NewCallback(cb.ID, "Неверный выбор"))
}

// webhookHandler reads the update from the request body and passes it to
// handleUpdate.  Telegram will POST JSON updates to this endpoint once the
// webhook is registered.
func webhookHandler(w http.ResponseWriter, r *http.Request) {
    var update tgbotapi.Update
    if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
        log.Println("cannot decode update:", err)
        return
    }
    handleUpdate(update)
}

func main() {
    log.SetFlags(log.LstdFlags | log.Lshortfile)
    token := os.Getenv("TELEGRAM_TOKEN")
    webhookURL := os.Getenv("WEBHOOK_URL")
    if token == "" || webhookURL == "" {
        log.Fatal("TELEGRAM_TOKEN and WEBHOOK_URL environment variables must be set")
    }
    // initialize database (path can be overridden via DB_FILE)
    dbPath := getEnv("DB_FILE", "queue.db")
    if err := initDB(dbPath); err != nil {
        log.Fatalf("cannot open database %s: %v", dbPath, err)
    }
    // create bot
    var err error
    bot, err = tgbotapi.NewBotAPI(token)
    if err != nil {
        log.Fatalf("cannot create bot: %v", err)
    }
    // remove any existing webhook
    if _, err := bot.Request(tgbotapi.DeleteWebhookConfig{}); err != nil {
        log.Println("failed to delete webhook:", err)
    }
    // set webhook
    if _, err := bot.Request(tgbotapi.NewWebhook(webhookURL)); err != nil {
        log.Fatalf("failed to set webhook: %v", err)
    }
    log.Printf("Webhook set to %s", webhookURL)
    // reschedule any pending notifications from previous runs
    reschedulePending()
    // configure HTTP handler
    http.HandleFunc("/webhook", webhookHandler)
    // listen and serve.  In production you should run behind TLS (443/8443) as
    // required by Telegram’s webhook guidelines【929523376617048†L49-L67】.  For
    // local development you can use tools like ngrok to expose your server.
    port := getEnv("PORT", "8080")
    log.Printf("Server started at :%s", port)
    if err := http.ListenAndServe(":"+port, nil); err != nil {
        log.Fatalf("listen: %v", err)
    }
}
