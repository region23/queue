package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"telegram_queue_bot/internal/bot/dispatcher"
	"telegram_queue_bot/internal/bot/service"
	"telegram_queue_bot/internal/config"
	"telegram_queue_bot/internal/scheduler/memory"
	"telegram_queue_bot/internal/server"
	"telegram_queue_bot/internal/storage/models"
	"telegram_queue_bot/internal/storage/sqlite"
	"telegram_queue_bot/pkg/logger"

	tgbot "github.com/go-telegram/bot"
	tgmodels "github.com/go-telegram/bot/models"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.Printf("Starting Telegram Queue Bot...")

	// Загружаем конфигурацию
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Инициализируем логгер
	logger := logger.New(logger.LevelInfo)
	logger.Info("Configuration loaded successfully")

	// Инициализируем хранилище
	storage, err := sqlite.New(cfg.Database.Path)
	if err != nil {
		log.Fatalf("Failed to initialize storage: %v", err)
	}
	defer func() {
		if err := storage.Close(); err != nil {
			log.Printf("Error closing storage: %v", err)
		}
	}()

	logger.Info("Storage initialized successfully")

	// Создаем бота
	telegramBot, err := tgbot.New(cfg.Telegram.Token)
	if err != nil {
		log.Fatalf("Failed to create Telegram bot: %v", err)
	}

	logger.Info("Telegram bot created successfully")

	// Создаем планировщик уведомлений
	notificationSender := &TelegramNotificationSender{bot: telegramBot}
	scheduler := memory.NewMemoryScheduler(notificationSender)

	// Создаем сервис бота
	botService := service.NewService(telegramBot, storage, scheduler, cfg)

	// Создаем диспетчер обновлений
	updateDispatcher := dispatcher.NewDispatcher(botService)

	// Настраиваем webhook
	err = setupWebhook(telegramBot, cfg.Telegram.WebhookURL)
	if err != nil {
		log.Fatalf("Failed to setup webhook: %v", err)
	}

	logger.Info("Webhook configured successfully")

	// Перепланируем ожидающие уведомления
	ctx := context.Background()
	if err := botService.ReschedulePendingNotifications(ctx); err != nil {
		log.Printf("Failed to reschedule pending notifications: %v", err)
	} else {
		logger.Info("Pending notifications rescheduled successfully")
	}

	// Создаем HTTP сервер
	srv := server.New(cfg, *logger)

	// Настраиваем обработчик webhook
	http.HandleFunc("/webhook", func(w http.ResponseWriter, r *http.Request) {
		webhookHandler(w, r, updateDispatcher, telegramBot)
	})

	// Настраиваем graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Обрабатываем системные сигналы
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		logger.Info("Shutdown signal received, starting graceful shutdown...")
		cancel()
	}()

	// Стартуем сервер
	logger.Info("Starting HTTP server on port " + cfg.Server.Port)
	if err := srv.Start(ctx); err != nil && err != http.ErrServerClosed {
		log.Fatalf("Server error: %v", err)
	}

	logger.Info("Server stopped gracefully")
}

// setupWebhook настраивает webhook для Telegram бота
func setupWebhook(bot *tgbot.Bot, webhookURL string) error {
	ctx := context.Background()

	// Удаляем существующий webhook
	if _, err := bot.DeleteWebhook(ctx, &tgbot.DeleteWebhookParams{}); err != nil {
		log.Printf("Warning: failed to delete existing webhook: %v", err)
	}

	// Устанавливаем новый webhook
	params := &tgbot.SetWebhookParams{
		URL: webhookURL,
	}

	if _, err := bot.SetWebhook(ctx, params); err != nil {
		return err
	}

	log.Printf("Webhook set to %s", webhookURL)
	return nil
}

// webhookHandler обрабатывает входящие webhook запросы от Telegram
func webhookHandler(w http.ResponseWriter, r *http.Request, dispatcher *dispatcher.Dispatcher, bot *tgbot.Bot) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var update tgmodels.Update
	if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
		log.Printf("Failed to decode update: %v", err)
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	// Обрабатываем обновление
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	dispatcher.HandleUpdate(ctx, bot, &update)

	// Отвечаем Telegram что запрос обработан
	w.WriteHeader(http.StatusOK)
}

// TelegramNotificationSender реализует интерфейс NotificationSender для отправки уведомлений через Telegram
type TelegramNotificationSender struct {
	bot *tgbot.Bot
}

// SendNotification отправляет уведомление пользователю
func (s *TelegramNotificationSender) SendNotification(ctx context.Context, chatID int64, message string) error {
	params := &tgbot.SendMessageParams{
		ChatID: chatID,
		Text:   message,
	}

	_, err := s.bot.SendMessage(ctx, params)
	return err
}

// SendSlotReminder отправляет напоминание о слоте
func (s *TelegramNotificationSender) SendSlotReminder(ctx context.Context, slot *models.Slot) error {
	if slot.UserChatID == nil {
		return fmt.Errorf("slot has no assigned user")
	}

	message := fmt.Sprintf("Ваша очередь подошла! Слот %s %s‑%s.", slot.Date, slot.StartTime, slot.EndTime)
	return s.SendNotification(ctx, *slot.UserChatID, message)
}
