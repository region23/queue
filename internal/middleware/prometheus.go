package middleware

import (
	"net/http"
	"strconv"
	"time"

	"github.com/region23/queue/pkg/metrics"
)

// PrometheusMiddleware добавляет метрики Prometheus для HTTP запросов
func PrometheusMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Создаем ResponseWriter для захвата статус-кода
		wrappedWriter := &responseWriter{
			ResponseWriter: w,
			statusCode:     http.StatusOK,
		}

		// Выполняем следующий обработчик
		next.ServeHTTP(wrappedWriter, r)

		// Записываем метрики
		duration := time.Since(start).Seconds()
		status := strconv.Itoa(wrappedWriter.statusCode)

		metrics.RecordHTTPRequest(r.Method, r.URL.Path, status)
		metrics.HTTPRequestDuration.WithLabelValues(r.Method, r.URL.Path).Observe(duration)
	})
}

// responseWriter оборачивает http.ResponseWriter для захвата статус-кода
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

// WriteHeader захватывает статус-код ответа
func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}
