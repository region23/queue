package server

import (
	"net/http"
	"time"

	"github.com/region23/queue/pkg/logger"
)

// loggingMiddleware логирует HTTP запросы
func (s *Server) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Создаем wrapper для ResponseWriter чтобы захватить status code
		wrapped := &responseWriterWrapper{
			ResponseWriter: w,
			statusCode:     http.StatusOK,
		}

		// Логируем входящий запрос
		s.logger.Info("HTTP request started",
			logger.Field{Key: "method", Value: r.Method},
			logger.Field{Key: "path", Value: r.URL.Path},
			logger.Field{Key: "remote_addr", Value: r.RemoteAddr},
			logger.Field{Key: "user_agent", Value: r.UserAgent()},
		)

		// Выполняем запрос
		next.ServeHTTP(wrapped, r)

		// Логируем результат
		duration := time.Since(start)
		s.logger.Info("HTTP request completed",
			logger.Field{Key: "method", Value: r.Method},
			logger.Field{Key: "path", Value: r.URL.Path},
			logger.Field{Key: "status_code", Value: wrapped.statusCode},
			logger.Field{Key: "duration_ms", Value: duration.Milliseconds()},
		)
	})
}

// securityHeadersMiddleware добавляет заголовки безопасности
func (s *Server) securityHeadersMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Security headers
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("X-XSS-Protection", "1; mode=block")
		w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		w.Header().Set("Content-Security-Policy", "default-src 'self'")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")

		// Убираем информационные заголовки
		w.Header().Set("Server", "")
		w.Header().Set("X-Powered-By", "")

		next.ServeHTTP(w, r)
	})
}

// responseWriterWrapper оборачивает ResponseWriter для захвата status code
type responseWriterWrapper struct {
	http.ResponseWriter
	statusCode int
}

// WriteHeader перехватывает status code
func (rw *responseWriterWrapper) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// corsMiddleware добавляет CORS заголовки (если необходимо)
func (s *Server) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Добавляем CORS заголовки только если это необходимо
		// Для Telegram webhook'ов CORS обычно не нужен

		// Для preflight запросов
		if r.Method == http.MethodOptions {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// requestValidationMiddleware валидирует базовые параметры запроса
func (s *Server) requestValidationMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Проверяем размер запроса
		if r.ContentLength > 10*1024*1024 { // 10MB
			s.logger.Warn("Request too large",
				logger.Field{Key: "content_length", Value: r.ContentLength},
				logger.Field{Key: "remote_addr", Value: r.RemoteAddr},
			)
			http.Error(w, "Request too large", http.StatusRequestEntityTooLarge)
			return
		}

		// Проверяем Content-Type для POST запросов
		if r.Method == http.MethodPost {
			contentType := r.Header.Get("Content-Type")
			if contentType == "" {
				s.logger.Warn("Missing Content-Type header",
					logger.Field{Key: "path", Value: r.URL.Path},
					logger.Field{Key: "remote_addr", Value: r.RemoteAddr},
				)
				http.Error(w, "Content-Type header is required", http.StatusBadRequest)
				return
			}
		}

		next.ServeHTTP(w, r)
	})
}
