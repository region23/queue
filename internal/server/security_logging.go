package server

import (
	"net/http"
	"strings"
	"time"

	"github.com/region23/queue/pkg/logger"

	tgmodels "github.com/go-telegram/bot/models"
)

// SecurityLogger логирует события безопасности
type SecurityLogger struct {
	logger logger.Logger
}

// NewSecurityLogger создает новый логгер безопасности
func NewSecurityLogger(logger logger.Logger) *SecurityLogger {
	return &SecurityLogger{
		logger: logger,
	}
}

// LogFailedAuth логирует неудачную попытку аутентификации
func (sl *SecurityLogger) LogFailedAuth(r *http.Request, reason string) {
	sl.logger.Warn("Authentication failed",
		logger.Field{Key: "reason", Value: reason},
		logger.Field{Key: "ip", Value: sl.getRealIP(r)},
		logger.Field{Key: "user_agent", Value: r.UserAgent()},
		logger.Field{Key: "path", Value: r.URL.Path},
		logger.Field{Key: "method", Value: r.Method},
		logger.Field{Key: "timestamp", Value: time.Now().UTC().Unix()},
	)
}

// LogRateLimitExceeded логирует превышение rate limit
func (sl *SecurityLogger) LogRateLimitExceeded(r *http.Request, limitType string, identifier string) {
	sl.logger.Warn("Rate limit exceeded",
		logger.Field{Key: "limit_type", Value: limitType},
		logger.Field{Key: "identifier", Value: identifier},
		logger.Field{Key: "ip", Value: sl.getRealIP(r)},
		logger.Field{Key: "user_agent", Value: r.UserAgent()},
		logger.Field{Key: "path", Value: r.URL.Path},
		logger.Field{Key: "timestamp", Value: time.Now().UTC().Unix()},
	)
}

// LogSuspiciousActivity логирует подозрительную активность
func (sl *SecurityLogger) LogSuspiciousActivity(r *http.Request, activity string, details map[string]interface{}) {
	fields := []logger.Field{
		{Key: "activity", Value: activity},
		{Key: "ip", Value: sl.getRealIP(r)},
		{Key: "user_agent", Value: r.UserAgent()},
		{Key: "path", Value: r.URL.Path},
		{Key: "method", Value: r.Method},
		{Key: "timestamp", Value: time.Now().UTC().Unix()},
	}

	// Добавляем дополнительные детали
	for key, value := range details {
		fields = append(fields, logger.Field{Key: key, Value: value})
	}

	sl.logger.Warn("Suspicious activity detected", fields...)
}

// LogValidationError логирует ошибки валидации
func (sl *SecurityLogger) LogValidationError(r *http.Request, fieldName string, value interface{}, reason string) {
	sl.logger.Warn("Validation error",
		logger.Field{Key: "field", Value: fieldName},
		logger.Field{Key: "value", Value: value},
		logger.Field{Key: "reason", Value: reason},
		logger.Field{Key: "ip", Value: sl.getRealIP(r)},
		logger.Field{Key: "user_agent", Value: r.UserAgent()},
		logger.Field{Key: "path", Value: r.URL.Path},
		logger.Field{Key: "timestamp", Value: time.Now().UTC().Unix()},
	)
}

// LogBlockedRequest логирует заблокированные запросы
func (sl *SecurityLogger) LogBlockedRequest(r *http.Request, reason string, action string) {
	sl.logger.Error("Request blocked",
		logger.Field{Key: "reason", Value: reason},
		logger.Field{Key: "action", Value: action},
		logger.Field{Key: "ip", Value: sl.getRealIP(r)},
		logger.Field{Key: "user_agent", Value: r.UserAgent()},
		logger.Field{Key: "path", Value: r.URL.Path},
		logger.Field{Key: "method", Value: r.Method},
		logger.Field{Key: "content_length", Value: r.ContentLength},
		logger.Field{Key: "timestamp", Value: time.Now().UTC().Unix()},
	)
}

// LogTelegramUpdate логирует обработку Telegram update
func (sl *SecurityLogger) LogTelegramUpdate(update *tgmodels.Update, processingTime time.Duration) {
	var chatID int64
	var userID int64
	var updateType string

	if update.Message != nil {
		updateType = "message"
		chatID = update.Message.Chat.ID
		if update.Message.From != nil {
			userID = update.Message.From.ID
		}
	} else if update.CallbackQuery != nil {
		updateType = "callback_query"
		userID = update.CallbackQuery.From.ID
		chatID = userID // Для callback_query chat_id равен user_id
	}

	sl.logger.Info("Telegram update processed",
		logger.Field{Key: "update_id", Value: update.ID},
		logger.Field{Key: "type", Value: updateType},
		logger.Field{Key: "chat_id", Value: chatID},
		logger.Field{Key: "user_id", Value: userID},
		logger.Field{Key: "processing_time_ms", Value: processingTime.Milliseconds()},
		logger.Field{Key: "timestamp", Value: time.Now().UTC().Unix()},
	)
}

// LogUserAction логирует действия пользователей
func (sl *SecurityLogger) LogUserAction(chatID int64, action string, details map[string]interface{}) {
	fields := []logger.Field{
		{Key: "chat_id", Value: chatID},
		{Key: "action", Value: action},
		{Key: "timestamp", Value: time.Now().UTC().Unix()},
	}

	// Добавляем дополнительные детали
	for key, value := range details {
		fields = append(fields, logger.Field{Key: key, Value: value})
	}

	sl.logger.Info("User action", fields...)
}

// LogSystemEvent логирует системные события
func (sl *SecurityLogger) LogSystemEvent(event string, level string, details map[string]interface{}) {
	fields := []logger.Field{
		{Key: "event", Value: event},
		{Key: "timestamp", Value: time.Now().UTC().Unix()},
	}

	// Добавляем дополнительные детали
	for key, value := range details {
		fields = append(fields, logger.Field{Key: key, Value: value})
	}

	switch strings.ToLower(level) {
	case "error":
		sl.logger.Error("System event", fields...)
	case "warn", "warning":
		sl.logger.Warn("System event", fields...)
	case "debug":
		sl.logger.Debug("System event", fields...)
	default:
		sl.logger.Info("System event", fields...)
	}
}

// LogDatabaseOperation логирует операции с базой данных
func (sl *SecurityLogger) LogDatabaseOperation(operation string, table string, affectedRows int, duration time.Duration, err error) {
	fields := []logger.Field{
		{Key: "operation", Value: operation},
		{Key: "table", Value: table},
		{Key: "affected_rows", Value: affectedRows},
		{Key: "duration_ms", Value: duration.Milliseconds()},
		{Key: "timestamp", Value: time.Now().UTC().Unix()},
	}

	if err != nil {
		fields = append(fields, logger.Field{Key: "error", Value: err.Error()})
		sl.logger.Error("Database operation failed", fields...)
	} else {
		sl.logger.Debug("Database operation completed", fields...)
	}
}

// getRealIP получает реальный IP адрес клиента
func (sl *SecurityLogger) getRealIP(r *http.Request) string {
	// Проверяем заголовки в порядке приоритета
	headers := []string{
		"CF-Connecting-IP",    // Cloudflare
		"True-Client-IP",      // Cloudflare Enterprise
		"X-Real-IP",           // nginx
		"X-Forwarded-For",     // Стандартный заголовок
		"X-Client-IP",         // Apache
		"X-Forwarded",         // Некоторые прокси
		"X-Cluster-Client-IP", // GCE Load Balancer
		"Forwarded-For",       // RFC 7239
		"Forwarded",           // RFC 7239
	}

	for _, header := range headers {
		if ip := r.Header.Get(header); ip != "" {
			// Для X-Forwarded-For может быть список IP
			if header == "X-Forwarded-For" {
				parts := strings.Split(ip, ",")
				if len(parts) > 0 {
					return strings.TrimSpace(parts[0])
				}
			}
			return strings.TrimSpace(ip)
		}
	}

	// Fallback на RemoteAddr
	if ip := r.RemoteAddr; ip != "" {
		// Убираем порт если есть
		if colonIndex := strings.LastIndex(ip, ":"); colonIndex != -1 {
			return ip[:colonIndex]
		}
		return ip
	}

	return "unknown"
}

// securityAuditMiddleware создает middleware для аудита безопасности
func (s *Server) securityAuditMiddleware(securityLogger *SecurityLogger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			// Создаем wrapper для ResponseWriter
			wrapped := &auditResponseWriter{
				ResponseWriter: w,
				statusCode:     http.StatusOK,
				bytesWritten:   0,
			}

			// Выполняем запрос
			next.ServeHTTP(wrapped, r)

			// Логируем результат если это важный эндпоинт или ошибка
			duration := time.Since(start)

			if r.URL.Path == "/webhook" || wrapped.statusCode >= 400 {
				details := map[string]interface{}{
					"status_code":    wrapped.statusCode,
					"duration_ms":    duration.Milliseconds(),
					"bytes_written":  wrapped.bytesWritten,
					"content_length": r.ContentLength,
				}

				if wrapped.statusCode >= 400 {
					securityLogger.LogSuspiciousActivity(r, "http_error", details)
				} else {
					securityLogger.LogSystemEvent("http_request", "info", details)
				}
			}
		})
	}
}

// auditResponseWriter оборачивает ResponseWriter для сбора метрик
type auditResponseWriter struct {
	http.ResponseWriter
	statusCode   int
	bytesWritten int64
}

// WriteHeader перехватывает status code
func (arw *auditResponseWriter) WriteHeader(code int) {
	arw.statusCode = code
	arw.ResponseWriter.WriteHeader(code)
}

// Write перехватывает количество записанных байт
func (arw *auditResponseWriter) Write(data []byte) (int, error) {
	n, err := arw.ResponseWriter.Write(data)
	arw.bytesWritten += int64(n)
	return n, err
}

// anomalyDetectionMiddleware обнаруживает аномальное поведение
func (s *Server) anomalyDetectionMiddleware(securityLogger *SecurityLogger) func(http.Handler) http.Handler {
	// Простая система обнаружения аномалий
	requestCounts := make(map[string]int)
	lastReset := time.Now()

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := securityLogger.getRealIP(r)

			// Сбрасываем счетчики каждые 5 минут
			if time.Since(lastReset) > 5*time.Minute {
				requestCounts = make(map[string]int)
				lastReset = time.Now()
			}

			// Увеличиваем счетчик для IP
			requestCounts[ip]++

			// Проверяем аномалии
			if requestCounts[ip] > 100 { // Более 100 запросов за 5 минут
				securityLogger.LogSuspiciousActivity(r, "high_request_rate", map[string]interface{}{
					"request_count": requestCounts[ip],
					"time_window":   "5_minutes",
				})
			}

			// Проверяем подозрительные User-Agent
			userAgent := strings.ToLower(r.UserAgent())
			suspiciousAgents := []string{"bot", "crawler", "spider", "scraper", "scanner"}
			for _, agent := range suspiciousAgents {
				if strings.Contains(userAgent, agent) && r.URL.Path == "/webhook" {
					securityLogger.LogSuspiciousActivity(r, "suspicious_user_agent", map[string]interface{}{
						"user_agent": r.UserAgent(),
					})
					break
				}
			}

			next.ServeHTTP(w, r)
		})
	}
}
