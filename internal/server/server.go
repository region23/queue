package server

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"telegram_queue_bot/internal/config"
	"telegram_queue_bot/internal/middleware"
	"telegram_queue_bot/pkg/logger"

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
	validator       *RequestValidator
	healthChecker   *HealthChecker
}

// New создает новый HTTP сервер
func New(cfg *config.Config, logger logger.Logger) *Server {
	// Создаем rate limiter для HTTP запросов
	rateLimiter := middleware.NewRateLimiter(100, time.Minute, logger)

	// Создаем rate limiter для Telegram
	telegramLimiter := middleware.NewTelegramRateLimiter(30, 10, logger)

	// Создаем security logger
	securityLogger := NewSecurityLogger(logger)

	// Создаем валидатор запросов
	validator := NewRequestValidator(logger)

	// Создаем health checker
	healthChecker := NewHealthChecker(nil, "1.0.0") // TODO: передать storage и версию

	server := &Server{
		config:          cfg,
		logger:          logger,
		rateLimiter:     rateLimiter,
		telegramLimiter: telegramLimiter,
		securityLogger:  securityLogger,
		validator:       validator,
		healthChecker:   healthChecker,
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

	// 9. Основной обработчик
	h := handler

	// 8. Prometheus метрики
	h = middleware.PrometheusMiddleware(h)

	// 7. Валидация запросов
	h = s.requestValidationMiddleware(h)

	// 6. User rate limiting для Telegram
	h = s.userRateLimitMiddleware(s.telegramLimiter)(h)

	// 5. Telegram authentication
	h = s.telegramAuthMiddleware(h)

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

	// Валидируем webhook запрос
	update, err := s.validator.ValidateWebhookRequest(r)
	if err != nil {
		s.securityLogger.LogValidationError(r, "webhook_validation", r.URL.Path, err.Error())
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	// Логируем успешную обработку
	s.securityLogger.LogTelegramUpdate(update, time.Since(start))

	// TODO: Интеграция с обработчиком Telegram бота
	s.logger.Info("Webhook processed successfully",
		logger.Field{Key: "update_id", Value: update.UpdateID},
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
