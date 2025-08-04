package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/region23/queue/internal/bot/dispatcher"
	"github.com/region23/queue/internal/config"
	"github.com/region23/queue/internal/middleware"
	"github.com/region23/queue/pkg/logger"

	tgbot "github.com/go-telegram/bot"
	tgmodels "github.com/go-telegram/bot/models"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Server представляет HTTP сервер с middleware
type Server struct {
	httpServer      *http.Server
	config          *config.Config
	logger          logger.Logger
	rateLimiter     *middleware.RateLimiter
	telegramLimiter *middleware.TelegramRateLimiter
	securityLogger  *SecurityLogger
	healthChecker   *HealthChecker
	dispatcher      *dispatcher.Dispatcher
	telegramBot     *tgbot.Bot
}

// New создает новый HTTP сервер
func New(cfg *config.Config, logger logger.Logger, dispatcher *dispatcher.Dispatcher, telegramBot *tgbot.Bot) *Server {
	// Создаем rate limiter для HTTP запросов
	rateLimiter := middleware.NewRateLimiter(100, time.Minute, logger)

	// Создаем rate limiter для Telegram
	telegramLimiter := middleware.NewTelegramRateLimiter(30, 10, logger)

	// Создаем security logger
	securityLogger := NewSecurityLogger(logger)

	// Создаем health checker
	healthChecker := NewHealthChecker(nil, "1.0.0") // TODO: передать storage и версию

	server := &Server{
		config:          cfg,
		logger:          logger,
		rateLimiter:     rateLimiter,
		telegramLimiter: telegramLimiter,
		securityLogger:  securityLogger,
		healthChecker:   healthChecker,
		dispatcher:      dispatcher,
		telegramBot:     telegramBot,
	}

	// Создаем HTTP сервер с таймаутами
	server.httpServer = &http.Server{
		Addr:           ":" + cfg.Server.Port,
		Handler:        server.setupRoutes(),
		ReadTimeout:    30 * time.Second,
		WriteTimeout:   30 * time.Second,
		IdleTimeout:    120 * time.Second,
		MaxHeaderBytes: 1 << 20, // 1MB
	}

	return server
}

// setupRoutes настраивает маршруты с middleware
func (s *Server) setupRoutes() http.Handler {
	mux := http.NewServeMux()

	// Основные маршруты
	mux.HandleFunc("/health", s.healthChecker.HealthHandler)
	mux.HandleFunc("/webhook", s.handleWebhook)

	// Метрики Prometheus
	mux.Handle("/metrics", promhttp.Handler())

	// Применяем middleware в правильном порядке
	handler := s.applyMiddleware(mux)

	return handler
}

// applyMiddleware применяет middleware в правильном порядке
func (s *Server) applyMiddleware(handler http.Handler) http.Handler {
	// Применяем middleware в обратном порядке (последний применяется первым)

	// 7. Основной обработчик
	h := handler

	// 6. Prometheus метрики
	h = middleware.PrometheusMiddleware(h)

	// 5. User rate limiting для Telegram
	h = middleware.TelegramRateLimitMiddleware(s.telegramLimiter)(h)

	// 4. Rate limiting
	h = middleware.HTTPRateLimitMiddleware(s.rateLimiter)(h)

	// 3. Anomaly detection
	h = s.anomalyDetectionMiddleware(s.securityLogger)(h)

	// 2. Security audit
	h = s.securityAuditMiddleware(s.securityLogger)(h)

	// 1. Security headers (применяется первым, выполняется последним)
	h = s.securityHeadersMiddleware(h)

	return h
}

// handleWebhook обрабатывает Telegram webhook
func (s *Server) handleWebhook(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	if r.Method != http.MethodPost {
		s.securityLogger.LogSuspiciousActivity(r, "invalid_webhook_method", map[string]interface{}{
			"method": r.Method,
		})
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Парсим обновление от Telegram используя библиотеку
	var update tgmodels.Update
	if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
		s.logger.Error("Failed to decode Telegram update",
			logger.Field{Key: "error", Value: err.Error()},
		)
		s.securityLogger.LogValidationError(r, "webhook_validation", r.URL.Path, "Failed to decode JSON: "+err.Error())
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	// Обрабатываем обновление через dispatcher
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	s.dispatcher.HandleUpdate(ctx, s.telegramBot, &update)

	// Логируем успешную обработку
	s.securityLogger.LogTelegramUpdate(&update, time.Since(start))
	s.logger.Info("Webhook processed successfully",
		logger.Field{Key: "update_id", Value: update.ID},
		logger.Field{Key: "processing_time_ms", Value: time.Since(start).Milliseconds()},
	)

	w.WriteHeader(http.StatusOK)
}

// Start запускает сервер
func (s *Server) Start(ctx context.Context) error {
	s.logger.Info("Starting HTTP server",
		logger.Field{Key: "addr", Value: s.httpServer.Addr},
	)

	// Запускаем сервер в отдельной горутине
	errCh := make(chan error, 1)
	go func() {
		if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- fmt.Errorf("server failed to start: %w", err)
		}
	}()

	// Ждем завершения контекста или ошибки
	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		return s.Shutdown(context.Background())
	}
}

// Shutdown корректно завершает работу сервера
func (s *Server) Shutdown(ctx context.Context) error {
	s.logger.Info("Shutting down HTTP server")

	// Логируем системное событие
	s.securityLogger.LogSystemEvent("server_shutdown", "info", map[string]interface{}{
		"initiated_at": time.Now().UTC().Unix(),
	})

	// Устанавливаем таймаут для graceful shutdown
	shutdownCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// Закрываем компоненты в правильном порядке
	if s.telegramLimiter != nil {
		s.telegramLimiter.Close()
	}

	if s.rateLimiter != nil {
		s.rateLimiter.Close()
	}

	// Завершаем HTTP сервер
	if err := s.httpServer.Shutdown(shutdownCtx); err != nil {
		s.logger.Error("Error during server shutdown",
			logger.Field{Key: "error", Value: err.Error()},
		)
		s.securityLogger.LogSystemEvent("server_shutdown_error", "error", map[string]interface{}{
			"error": err.Error(),
		})
		return err
	}

	s.logger.Info("HTTP server shut down successfully")
	s.securityLogger.LogSystemEvent("server_shutdown_complete", "info", map[string]interface{}{
		"completed_at": time.Now().UTC().Unix(),
	})
	return nil
}
