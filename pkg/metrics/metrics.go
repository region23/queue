package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Метрики для Telegram бота
var (
	// Общие метрики
	RequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "telegram_bot_requests_total",
			Help: "Общее количество обработанных запросов",
		},
		[]string{"handler", "status"},
	)

	RequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "telegram_bot_request_duration_seconds",
			Help:    "Время обработки запросов в секундах",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"handler"},
	)

	// Метрики пользователей
	ActiveUsers = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "telegram_bot_active_users",
			Help: "Количество активных пользователей",
		},
	)

	UserRegistrations = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "telegram_bot_user_registrations_total",
			Help: "Общее количество регистраций пользователей",
		},
	)

	// Метрики слотов
	SlotsCreated = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "telegram_bot_slots_created_total",
			Help: "Общее количество созданных слотов",
		},
	)

	SlotsReserved = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "telegram_bot_slots_reserved_total",
			Help: "Общее количество зарезервированных слотов",
		},
	)

	SlotsCancelled = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "telegram_bot_slots_cancelled_total",
			Help: "Общее количество отмененных слотов",
		},
	)

	AvailableSlots = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "telegram_bot_available_slots",
			Help: "Количество доступных слотов по дням",
		},
		[]string{"date"},
	)

	// Метрики уведомлений
	NotificationsSent = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "telegram_bot_notifications_sent_total",
			Help: "Общее количество отправленных уведомлений",
		},
		[]string{"type", "status"},
	)

	PendingNotifications = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "telegram_bot_pending_notifications",
			Help: "Количество ожидающих уведомлений",
		},
	)

	// Метрики базы данных
	DatabaseOperations = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "telegram_bot_database_operations_total",
			Help: "Общее количество операций с базой данных",
		},
		[]string{"operation", "table", "status"},
	)

	DatabaseConnectionPool = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "telegram_bot_database_connections",
			Help: "Состояние пула соединений с базой данных",
		},
		[]string{"state"}, // active, idle, open
	)

	// Метрики производительности
	MemoryUsage = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "telegram_bot_memory_usage_bytes",
			Help: "Использование памяти в байтах",
		},
	)

	GoroutinesCount = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "telegram_bot_goroutines_count",
			Help: "Количество активных горутин",
		},
	)

	// Метрики ошибок
	ErrorsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "telegram_bot_errors_total",
			Help: "Общее количество ошибок",
		},
		[]string{"component", "error_type"},
	)

	// Метрики HTTP сервера
	HTTPRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "telegram_bot_http_requests_total",
			Help: "Общее количество HTTP запросов",
		},
		[]string{"method", "endpoint", "status"},
	)

	HTTPRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "telegram_bot_http_request_duration_seconds",
			Help:    "Время обработки HTTP запросов в секундах",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "endpoint"},
	)
)

// RecordRequest записывает метрику обработки запроса
func RecordRequest(handler, status string) {
	RequestsTotal.WithLabelValues(handler, status).Inc()
}

// RecordUserRegistration записывает метрику регистрации пользователя
func RecordUserRegistration() {
	UserRegistrations.Inc()
}

// RecordSlotCreation записывает метрику создания слота
func RecordSlotCreation() {
	SlotsCreated.Inc()
}

// RecordSlotReservation записывает метрику резервирования слота
func RecordSlotReservation() {
	SlotsReserved.Inc()
}

// RecordSlotCancellation записывает метрику отмены слота
func RecordSlotCancellation() {
	SlotsCancelled.Inc()
}

// RecordNotification записывает метрику отправки уведомления
func RecordNotification(notificationType, status string) {
	NotificationsSent.WithLabelValues(notificationType, status).Inc()
}

// RecordDatabaseOperation записывает метрику операции с БД
func RecordDatabaseOperation(operation, table, status string) {
	DatabaseOperations.WithLabelValues(operation, table, status).Inc()
}

// RecordError записывает метрику ошибки
func RecordError(component, errorType string) {
	ErrorsTotal.WithLabelValues(component, errorType).Inc()
}

// RecordHTTPRequest записывает метрику HTTP запроса
func RecordHTTPRequest(method, endpoint, status string) {
	HTTPRequestsTotal.WithLabelValues(method, endpoint, status).Inc()
}

// SetActiveUsers устанавливает количество активных пользователей
func SetActiveUsers(count float64) {
	ActiveUsers.Set(count)
}

// SetPendingNotifications устанавливает количество ожидающих уведомлений
func SetPendingNotifications(count float64) {
	PendingNotifications.Set(count)
}

// SetAvailableSlots устанавливает количество доступных слотов для даты
func SetAvailableSlots(date string, count float64) {
	AvailableSlots.WithLabelValues(date).Set(count)
}
