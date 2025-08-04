package unit

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/region23/queue/internal/middleware"
	"github.com/region23/queue/tests/testutils"
)

func TestTelegramRateLimiter(t *testing.T) {
	logger := testutils.SetupTestLogger()
	// Создаем rate limiter с достаточно большими лимитами для тестирования
	limiter := middleware.NewTelegramRateLimiter(60, 10, *logger) // 60 запросов в минуту на пользователя, 10 в секунду глобально

	// Тестируем что базовые запросы проходят
	chatID := int64(12345)

	// Первые несколько запросов должны пройти
	for i := 0; i < 3; i++ {
		allowed := limiter.AllowUser(chatID)
		testutils.AssertTrue(t, allowed, "Request should be allowed within limits")
	}
}

func TestTelegramRateLimiter_UserLimit(t *testing.T) {
	logger := testutils.SetupTestLogger()
	// Создаем rate limiter с очень маленьким пользовательским лимитом
	limiter := middleware.NewTelegramRateLimiter(1, 10, *logger) // 1 запрос в минуту на пользователя

	chatID := int64(12345)

	// Первый запрос должен пройти
	allowed := limiter.AllowUser(chatID)
	testutils.AssertTrue(t, allowed, "First request should be allowed")

	// Второй запрос должен быть заблокирован пользовательским лимитом
	allowed = limiter.AllowUser(chatID)
	testutils.AssertFalse(t, allowed, "Second request should be blocked by user limit")
}

func TestTelegramRateLimiter_GlobalLimit(t *testing.T) {
	logger := testutils.SetupTestLogger()
	// Глобальный лимит 2 запроса в секунду
	limiter := middleware.NewTelegramRateLimiter(10, 2, *logger)

	// Отправляем запросы от разных пользователей до глобального лимита
	users := []int64{12345, 67890, 11111}

	// Первые 2 запроса должны пройти
	for i := 0; i < 2; i++ {
		allowed := limiter.AllowUser(users[i%len(users)])
		testutils.AssertTrue(t, allowed, "Request within global limit should be allowed")
	}

	// Третий запрос должен быть заблокирован глобальным лимитом
	allowed := limiter.AllowUser(users[2])
	testutils.AssertFalse(t, allowed, "Request over global limit should be denied")
}

func TestRateLimiter(t *testing.T) {
	logger := testutils.SetupTestLogger()
	limiter := middleware.NewRateLimiter(5, 1*time.Minute, *logger) // 5 запросов в минуту

	key := "test_key"

	// Первые 5 запросов должны пройти
	for i := 0; i < 5; i++ {
		allowed := limiter.Allow(key)
		testutils.AssertTrue(t, allowed, "Request within limit should be allowed")
	}

	// Следующий запрос должен быть заблокирован
	allowed := limiter.Allow(key)
	testutils.AssertFalse(t, allowed, "Request over limit should be denied")

	// Другой ключ должен иметь свой лимит
	allowed = limiter.Allow("different_key")
	testutils.AssertTrue(t, allowed, "Different key should have its own limit")
}

func TestTokenBucket(t *testing.T) {
	// Создаем bucket с емкостью 3 и скоростью пополнения 1 токен в секунду
	bucket := middleware.NewTokenBucket(3, 1)

	// Должно быть 3 доступных токена изначально
	for i := 0; i < 3; i++ {
		allowed := bucket.Allow()
		testutils.AssertTrue(t, allowed, "Initial tokens should be available")
	}

	// Четвертый запрос должен быть отклонен
	allowed := bucket.Allow()
	testutils.AssertFalse(t, allowed, "Request when bucket is empty should be denied")

	// Ждем 2 секунды для пополнения (должно добавиться 2 токена)
	time.Sleep(2100 * time.Millisecond)

	// Теперь должно быть доступно 2 токена
	for i := 0; i < 2; i++ {
		allowed := bucket.Allow()
		testutils.AssertTrue(t, allowed, "Refilled tokens should be available")
	}

	// Третий запрос должен быть отклонен
	allowed = bucket.Allow()
	testutils.AssertFalse(t, allowed, "Request beyond refilled capacity should be denied")
}

func TestHTTPRateLimitMiddleware(t *testing.T) {
	logger := testutils.SetupTestLogger()

	// Создаем тестовый handler
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// Создаем rate limiter с достаточным лимитом для базового теста
	limiter := middleware.NewRateLimiter(10, 1*time.Minute, *logger)
	rateLimitedHandler := middleware.HTTPRateLimitMiddleware(limiter)(handler)

	// Тестируем что запросы проходят
	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "127.0.0.1:12345"

	recorder := httptest.NewRecorder()
	rateLimitedHandler.ServeHTTP(recorder, req)

	testutils.AssertEqual(t, 200, recorder.Code, "Request should succeed")
	testutils.AssertEqual(t, "OK", recorder.Body.String(), "Response body should match")
}

func TestHTTPRateLimitMiddleware_Blocking(t *testing.T) {
	logger := testutils.SetupTestLogger()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Создаем rate limiter с очень маленьким лимитом
	limiter := middleware.NewRateLimiter(1, 1*time.Minute, *logger)
	rateLimitedHandler := middleware.HTTPRateLimitMiddleware(limiter)(handler)

	ip := "192.168.1.100"

	// Первый запрос должен пройти
	req1 := httptest.NewRequest("GET", "/test", nil)
	req1.RemoteAddr = ip + ":12345"
	recorder1 := httptest.NewRecorder()
	rateLimitedHandler.ServeHTTP(recorder1, req1)
	testutils.AssertEqual(t, 200, recorder1.Code, "First request should succeed")

	// Второй запрос должен быть заблокирован
	req2 := httptest.NewRequest("GET", "/test", nil)
	req2.RemoteAddr = ip + ":12345"
	recorder2 := httptest.NewRecorder()
	rateLimitedHandler.ServeHTTP(recorder2, req2)
	testutils.AssertEqual(t, 429, recorder2.Code, "Second request should be rate limited")
}

func TestTelegramRateLimitMiddleware(t *testing.T) {
	logger := testutils.SetupTestLogger()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	limiter := middleware.NewTelegramRateLimiter(10, 1, *logger) // 1 запрос в секунду глобально
	rateLimitedHandler := middleware.TelegramRateLimitMiddleware(limiter)(handler)

	// Первый запрос должен пройти
	req1 := httptest.NewRequest("POST", "/webhook", nil)
	recorder1 := httptest.NewRecorder()
	rateLimitedHandler.ServeHTTP(recorder1, req1)
	testutils.AssertEqual(t, 200, recorder1.Code, "First request should succeed")

	// Второй запрос должен быть заблокирован глобальным лимитом
	req2 := httptest.NewRequest("POST", "/webhook", nil)
	recorder2 := httptest.NewRecorder()
	rateLimitedHandler.ServeHTTP(recorder2, req2)
	testutils.AssertEqual(t, 429, recorder2.Code, "Second request should be rate limited")
}

func TestRateLimitMiddleware_RealIP(t *testing.T) {
	logger := testutils.SetupTestLogger()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	limiter := middleware.NewRateLimiter(1, 1*time.Minute, *logger) // 1 запрос в минуту
	rateLimitedHandler := middleware.HTTPRateLimitMiddleware(limiter)(handler)

	tests := []struct {
		name          string
		headers       map[string]string
		remoteAddr    string
		secondReqCode int
	}{
		{
			name: "X-Real-IP header",
			headers: map[string]string{
				"X-Real-IP": "192.168.1.100",
			},
			remoteAddr:    "127.0.0.1:12345",
			secondReqCode: 429,
		},
		{
			name: "X-Forwarded-For header",
			headers: map[string]string{
				"X-Forwarded-For": "10.0.0.1, 192.168.1.1",
			},
			remoteAddr:    "127.0.0.1:12345",
			secondReqCode: 429,
		},
		{
			name:          "RemoteAddr fallback",
			headers:       map[string]string{},
			remoteAddr:    "203.0.113.1:54321",
			secondReqCode: 429,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Первый запрос
			req1 := httptest.NewRequest("GET", "/test", nil)
			req1.RemoteAddr = tt.remoteAddr
			for key, value := range tt.headers {
				req1.Header.Set(key, value)
			}

			recorder1 := httptest.NewRecorder()
			rateLimitedHandler.ServeHTTP(recorder1, req1)
			testutils.AssertEqual(t, 200, recorder1.Code, "First request should succeed")

			// Второй запрос с тем же IP (должен быть заблокирован)
			req2 := httptest.NewRequest("GET", "/test", nil)
			req2.RemoteAddr = tt.remoteAddr
			for key, value := range tt.headers {
				req2.Header.Set(key, value)
			}

			recorder2 := httptest.NewRecorder()
			rateLimitedHandler.ServeHTTP(recorder2, req2)
			testutils.AssertEqual(t, tt.secondReqCode, recorder2.Code, "Second request should be rate limited")
		})
	}
}

func TestRateLimiter_Cleanup(t *testing.T) {
	logger := testutils.SetupTestLogger()

	// Создаем rate limiter с коротким интервалом очистки для тестирования
	limiter := middleware.NewRateLimiter(5, 1*time.Minute, *logger)

	// Используем несколько ключей
	keys := []string{"key1", "key2", "key3"}
	for _, key := range keys {
		allowed := limiter.Allow(key)
		testutils.AssertTrue(t, allowed, "Request should be allowed")
	}

	// Проверяем что лимитеры созданы
	for _, key := range keys {
		bucket := limiter.GetLimiter(key)
		testutils.AssertNotEqual(t, (*middleware.TokenBucket)(nil), bucket, "Limiter should exist")
	}

	// Закрываем limiter для остановки cleanup routine
	limiter.Close()
}

func TestRateLimiter_ConcurrentAccess(t *testing.T) {
	logger := testutils.SetupTestLogger()
	limiter := middleware.NewRateLimiter(10, 1*time.Minute, *logger)
	defer limiter.Close()

	key := "test_key"

	// Запускаем конкурентные запросы
	done := make(chan bool, 20)
	allowedCount := make(chan bool, 20)

	for i := 0; i < 20; i++ {
		go func() {
			defer func() { done <- true }()
			allowed := limiter.Allow(key)
			allowedCount <- allowed
		}()
	}

	// Ждем завершения всех горутин
	for i := 0; i < 20; i++ {
		select {
		case <-done:
			// OK
		case <-time.After(5 * time.Second):
			t.Fatal("Timeout waiting for concurrent operations")
		}
	}

	// Подсчитываем разрешенные запросы
	allowed := 0
	for i := 0; i < 20; i++ {
		if <-allowedCount {
			allowed++
		}
	}

	// Должно быть разрешено не больше 10 запросов
	testutils.AssertTrue(t, allowed <= 10, "Should not allow more than limit")
	testutils.AssertTrue(t, allowed > 0, "Should allow some requests")
}
