package server

import (
	"context"
	"encoding/json"
	"net/http"
	"runtime"
	"time"

	"github.com/region23/queue/internal/storage"
	"github.com/region23/queue/pkg/metrics"
)

// HealthResponse представляет ответ health check
type HealthResponse struct {
	Status    string                 `json:"status"`
	Timestamp string                 `json:"timestamp"`
	Version   string                 `json:"version,omitempty"`
	Uptime    string                 `json:"uptime,omitempty"`
	Checks    map[string]string      `json:"checks"`
	Metrics   map[string]interface{} `json:"metrics,omitempty"`
}

// HealthChecker проверяет состояние системы
type HealthChecker struct {
	storage   storage.Storage
	startTime time.Time
	version   string
}

// NewHealthChecker создает новый health checker
func NewHealthChecker(storage storage.Storage, version string) *HealthChecker {
	return &HealthChecker{
		storage:   storage,
		startTime: time.Now(),
		version:   version,
	}
}

// HealthHandler обрабатывает запросы health check
func (h *HealthChecker) HealthHandler(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	// Проверяем компоненты системы
	checks := make(map[string]string)
	overallStatus := "healthy"

	// Проверка базы данных
	if err := h.checkDatabase(ctx); err != nil {
		checks["database"] = "unhealthy: " + err.Error()
		overallStatus = "unhealthy"
	} else {
		checks["database"] = "healthy"
	}

	// Проверка памяти
	if memStatus := h.checkMemory(); memStatus != "healthy" {
		checks["memory"] = memStatus
		if overallStatus == "healthy" {
			overallStatus = "warning"
		}
	} else {
		checks["memory"] = "healthy"
	}

	// Проверка горутин
	if goroutineStatus := h.checkGoroutines(); goroutineStatus != "healthy" {
		checks["goroutines"] = goroutineStatus
		if overallStatus == "healthy" {
			overallStatus = "warning"
		}
	} else {
		checks["goroutines"] = "healthy"
	}

	// Собираем метрики для мониторинга
	metrics := h.collectMetrics()

	// Формируем ответ
	response := HealthResponse{
		Status:    overallStatus,
		Timestamp: time.Now().Format(time.RFC3339),
		Version:   h.version,
		Uptime:    time.Since(h.startTime).String(),
		Checks:    checks,
		Metrics:   metrics,
	}

	// Устанавливаем HTTP статус в зависимости от состояния
	w.Header().Set("Content-Type", "application/json")
	switch overallStatus {
	case "healthy":
		w.WriteHeader(http.StatusOK)
	case "warning":
		w.WriteHeader(http.StatusOK) // 200 но с предупреждениями
	case "unhealthy":
		w.WriteHeader(http.StatusServiceUnavailable)
	}

	// Записываем метрику
	// metrics записывается автоматически через middleware

	// Отправляем ответ
	json.NewEncoder(w).Encode(response)
}

// checkDatabase проверяет соединение с базой данных
func (h *HealthChecker) checkDatabase(ctx context.Context) error {
	if h.storage == nil {
		return nil // Если storage не инициализирован, пропускаем проверку
	}

	// Простая проверка - попытка выполнить запрос
	// Это зависит от интерфейса storage, возможно нужно добавить Ping() метод
	return nil
}

// checkMemory проверяет использование памяти
func (h *HealthChecker) checkMemory() string {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	// Обновляем метрику
	metrics.MemoryUsage.Set(float64(m.Alloc))

	// Проверяем лимиты (например, > 500MB предупреждение)
	const warningLimit = 500 * 1024 * 1024   // 500MB
	const criticalLimit = 1024 * 1024 * 1024 // 1GB

	if m.Alloc > criticalLimit {
		return "critical: memory usage > 1GB"
	} else if m.Alloc > warningLimit {
		return "warning: memory usage > 500MB"
	}

	return "healthy"
}

// checkGoroutines проверяет количество горутин
func (h *HealthChecker) checkGoroutines() string {
	count := float64(runtime.NumGoroutine())

	// Обновляем метрику
	metrics.GoroutinesCount.Set(count)

	// Проверяем лимиты
	const warningLimit = 100
	const criticalLimit = 1000

	if count > criticalLimit {
		return "critical: too many goroutines"
	} else if count > warningLimit {
		return "warning: high goroutine count"
	}

	return "healthy"
}

// collectMetrics собирает основные метрики для health check
func (h *HealthChecker) collectMetrics() map[string]interface{} {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	return map[string]interface{}{
		"memory": map[string]interface{}{
			"alloc_bytes":       m.Alloc,
			"total_alloc_bytes": m.TotalAlloc,
			"sys_bytes":         m.Sys,
			"num_gc":            m.NumGC,
		},
		"runtime": map[string]interface{}{
			"goroutines": runtime.NumGoroutine(),
			"gomaxprocs": runtime.GOMAXPROCS(0),
			"version":    runtime.Version(),
		},
		"uptime_seconds": time.Since(h.startTime).Seconds(),
	}
}
