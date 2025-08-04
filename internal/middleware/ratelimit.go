package middleware

import (
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"telegram_queue_bot/pkg/logger"
)

// TokenBucket реализует алгоритм Token Bucket для rate limiting
type TokenBucket struct {
	capacity   int
	tokens     int
	refillRate int // токенов в секунду
	lastRefill time.Time
	mu         sync.Mutex
}

// NewTokenBucket создает новый TokenBucket
func NewTokenBucket(capacity, refillRate int) *TokenBucket {
	return &TokenBucket{
		capacity:   capacity,
		tokens:     capacity,
		refillRate: refillRate,
		lastRefill: time.Now(),
	}
}

// Allow проверяет, доступен ли токен
func (tb *TokenBucket) Allow() bool {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(tb.lastRefill)

	// Добавляем токены в зависимости от прошедшего времени
	tokensToAdd := int(elapsed.Seconds()) * tb.refillRate
	tb.tokens = min(tb.capacity, tb.tokens+tokensToAdd)
	tb.lastRefill = now

	if tb.tokens > 0 {
		tb.tokens--
		return true
	}

	return false
}

// RateLimiter предоставляет функциональность ограничения скорости запросов
type RateLimiter struct {
	limiters   map[string]*TokenBucket
	mu         sync.RWMutex
	capacity   int
	refillRate int
	logger     logger.Logger

	// Cleanup
	cleanupInterval time.Duration
	lastAccess      map[string]time.Time
	done            chan struct{}
}

// NewRateLimiter создает новый rate limiter
func NewRateLimiter(requests int, duration time.Duration, logger logger.Logger) *RateLimiter {
	refillRate := int(float64(requests) / duration.Seconds())
	if refillRate == 0 {
		refillRate = 1
	}

	rl := &RateLimiter{
		limiters:        make(map[string]*TokenBucket),
		lastAccess:      make(map[string]time.Time),
		capacity:        requests,
		refillRate:      refillRate,
		logger:          logger,
		cleanupInterval: 5 * time.Minute,
		done:            make(chan struct{}),
	}

	// Запускаем goroutine для очистки неиспользуемых limiters
	go rl.cleanupRoutine()

	return rl
}

// GetLimiter возвращает rate limiter для конкретного ключа (например, IP или user ID)
func (rl *RateLimiter) GetLimiter(key string) *TokenBucket {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	limiter, exists := rl.limiters[key]
	if !exists {
		limiter = NewTokenBucket(rl.capacity, rl.refillRate)
		rl.limiters[key] = limiter
	}

	rl.lastAccess[key] = time.Now()
	return limiter
}

// Allow проверяет, разрешен ли запрос для данного ключа
func (rl *RateLimiter) Allow(key string) bool {
	return rl.GetLimiter(key).Allow()
}

// cleanupRoutine периодически удаляет неиспользуемые limiters
func (rl *RateLimiter) cleanupRoutine() {
	ticker := time.NewTicker(rl.cleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			rl.cleanup()
		case <-rl.done:
			return
		}
	}
}

// cleanup удаляет limiters, которые не использовались более 10 минут
func (rl *RateLimiter) cleanup() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	cutoff := time.Now().Add(-10 * time.Minute)
	var cleaned int

	for key, lastAccessed := range rl.lastAccess {
		if lastAccessed.Before(cutoff) {
			delete(rl.limiters, key)
			delete(rl.lastAccess, key)
			cleaned++
		}
	}

	if cleaned > 0 {
		rl.logger.Debug("Cleaned up rate limiters",
			logger.Field{Key: "cleaned_count", Value: cleaned},
			logger.Field{Key: "remaining_count", Value: len(rl.limiters)},
		)
	}
}

// Close останавливает cleanup routine
func (rl *RateLimiter) Close() {
	close(rl.done)
}

// HTTPRateLimitMiddleware создает HTTP middleware для rate limiting
func HTTPRateLimitMiddleware(limiter *RateLimiter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Используем IP адрес как ключ
			key := getRealIP(r)

			if !limiter.Allow(key) {
				limiter.logger.Warn("Rate limit exceeded",
					logger.Field{Key: "ip", Value: key},
					logger.Field{Key: "user_agent", Value: r.UserAgent()},
				)

				http.Error(w, "Too Many Requests", http.StatusTooManyRequests)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// TelegramRateLimiter специализированный rate limiter для Telegram
type TelegramRateLimiter struct {
	userLimiter   *RateLimiter // Ограничение по пользователям
	globalLimiter *TokenBucket // Глобальное ограничение
	logger        logger.Logger
}

// NewTelegramRateLimiter создает rate limiter для Telegram бота
func NewTelegramRateLimiter(
	userRequestsPerMinute int,
	globalRequestsPerSecond int,
	logger logger.Logger,
) *TelegramRateLimiter {
	return &TelegramRateLimiter{
		userLimiter:   NewRateLimiter(userRequestsPerMinute, time.Minute, logger),
		globalLimiter: NewTokenBucket(globalRequestsPerSecond, globalRequestsPerSecond),
		logger:        logger,
	}
}

// AllowUser проверяет, может ли пользователь отправить запрос
func (trl *TelegramRateLimiter) AllowUser(chatID int64) bool {
	userKey := fmt.Sprintf("user_%d", chatID)

	// Проверяем глобальный лимит
	if !trl.globalLimiter.Allow() {
		trl.logger.Warn("Global rate limit exceeded",
			logger.Field{Key: "chat_id", Value: chatID},
		)
		return false
	}

	// Проверяем лимит пользователя
	if !trl.userLimiter.Allow(userKey) {
		trl.logger.Warn("User rate limit exceeded",
			logger.Field{Key: "chat_id", Value: chatID},
		)
		return false
	}

	return true
}

// Close закрывает все ресурсы
func (trl *TelegramRateLimiter) Close() {
	trl.userLimiter.Close()
}

// TelegramRateLimitMiddleware создает middleware для обработчиков Telegram
func TelegramRateLimitMiddleware(limiter *TelegramRateLimiter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Здесь можно добавить логику извлечения chat_id из webhook payload
			// Для простоты пока используем IP
			if !limiter.globalLimiter.Allow() {
				limiter.logger.Warn("Global Telegram rate limit exceeded",
					logger.Field{Key: "ip", Value: getRealIP(r)},
				)
				http.Error(w, "Too Many Requests", http.StatusTooManyRequests)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// getRealIP извлекает реальный IP адрес из запроса
func getRealIP(r *http.Request) string {
	// Проверяем заголовки в порядке приоритета
	headers := []string{
		"CF-Connecting-IP", // Cloudflare
		"X-Forwarded-For",  // Стандартный заголовок
		"X-Real-IP",        // Nginx
		"X-Forwarded",
		"X-Cluster-Client-IP",
	}

	for _, header := range headers {
		ip := r.Header.Get(header)
		if ip != "" {
			// X-Forwarded-For может содержать несколько IP через запятую
			if header == "X-Forwarded-For" {
				parts := strings.Split(ip, ",")
				if len(parts) > 0 {
					return strings.TrimSpace(parts[0])
				}
			}
			return ip
		}
	}

	// Fallback на RemoteAddr
	return r.RemoteAddr
}

// min возвращает минимум из двух чисел
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
