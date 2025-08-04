package server

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"telegram_queue_bot/internal/middleware"
	"telegram_queue_bot/pkg/logger"
)

// TelegramUpdate представляет структуру webhook update от Telegram
type TelegramUpdate struct {
	UpdateID int `json:"update_id"`
	Message  *struct {
		MessageID int `json:"message_id"`
		From      *struct {
			ID           int64  `json:"id"`
			IsBot        bool   `json:"is_bot"`
			FirstName    string `json:"first_name"`
			LastName     string `json:"last_name,omitempty"`
			Username     string `json:"username,omitempty"`
			LanguageCode string `json:"language_code,omitempty"`
		} `json:"from,omitempty"`
		Chat *struct {
			ID        int64  `json:"id"`
			Type      string `json:"type"`
			FirstName string `json:"first_name,omitempty"`
			LastName  string `json:"last_name,omitempty"`
			Username  string `json:"username,omitempty"`
		} `json:"chat,omitempty"`
		Date int64  `json:"date"`
		Text string `json:"text,omitempty"`
	} `json:"message,omitempty"`
	CallbackQuery *struct {
		ID   string `json:"id"`
		From *struct {
			ID           int64  `json:"id"`
			IsBot        bool   `json:"is_bot"`
			FirstName    string `json:"first_name"`
			LastName     string `json:"last_name,omitempty"`
			Username     string `json:"username,omitempty"`
			LanguageCode string `json:"language_code,omitempty"`
		} `json:"from,omitempty"`
		Data string `json:"data,omitempty"`
	} `json:"callback_query,omitempty"`
}

// telegramAuthMiddleware проверяет подпись Telegram webhook
func (s *Server) telegramAuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Проверяем только webhook эндпоинт
		if r.URL.Path != "/webhook" {
			next.ServeHTTP(w, r)
			return
		}

		// Читаем тело запроса
		body, err := io.ReadAll(r.Body)
		if err != nil {
			s.logger.Error("Failed to read request body",
				logger.Field{Key: "error", Value: err.Error()},
			)
			http.Error(w, "Bad request", http.StatusBadRequest)
			return
		}

		// Проверяем подпись Telegram
		if !s.verifyTelegramSignature(r, body) {
			s.logger.Warn("Invalid Telegram signature",
				logger.Field{Key: "remote_addr", Value: r.RemoteAddr},
				logger.Field{Key: "user_agent", Value: r.UserAgent()},
			)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		// Парсим и валидируем update
		var update TelegramUpdate
		if err := json.Unmarshal(body, &update); err != nil {
			s.logger.Error("Failed to parse Telegram update",
				logger.Field{Key: "error", Value: err.Error()},
			)
			http.Error(w, "Bad request", http.StatusBadRequest)
			return
		}

		// Проверяем базовую валидность update
		if !s.validateTelegramUpdate(&update) {
			s.logger.Warn("Invalid Telegram update",
				logger.Field{Key: "update_id", Value: update.UpdateID},
			)
			http.Error(w, "Bad request", http.StatusBadRequest)
			return
		}

		// Восстанавливаем тело запроса для следующих middleware
		r.Body = io.NopCloser(strings.NewReader(string(body)))

		next.ServeHTTP(w, r)
	})
}

// verifyTelegramSignature проверяет подпись webhook от Telegram
func (s *Server) verifyTelegramSignature(r *http.Request, body []byte) bool {
	// Получаем подпись из заголовка
	signature := r.Header.Get("X-Telegram-Bot-Api-Secret-Token")

	// Если задан секретный токен в конфигурации, проверяем его
	if s.config.Telegram.SecretToken != "" {
		if signature != s.config.Telegram.SecretToken {
			return false
		}
	}

	// Дополнительно можем проверить HMAC подпись (если используется)
	expectedMAC := s.computeHMAC(body, s.config.Telegram.Token)
	providedMAC := r.Header.Get("X-Hub-Signature-256")

	if providedMAC != "" {
		providedMAC = strings.TrimPrefix(providedMAC, "sha256=")
		expectedBytes, _ := hex.DecodeString(expectedMAC)
		providedBytes, _ := hex.DecodeString(providedMAC)

		return hmac.Equal(expectedBytes, providedBytes)
	}

	return true // Если HMAC не используется, считаем валидным
}

// computeHMAC вычисляет HMAC-SHA256
func (s *Server) computeHMAC(message []byte, secret string) string {
	h := hmac.New(sha256.New, []byte(secret))
	h.Write(message)
	return hex.EncodeToString(h.Sum(nil))
}

// validateTelegramUpdate проверяет базовую валидность Telegram update
func (s *Server) validateTelegramUpdate(update *TelegramUpdate) bool {
	// Проверяем ID update
	if update.UpdateID <= 0 {
		return false
	}

	// Должно быть хотя бы одно из: message или callback_query
	if update.Message == nil && update.CallbackQuery == nil {
		return false
	}

	// Проверяем message если есть
	if update.Message != nil {
		if update.Message.From == nil || update.Message.Chat == nil {
			return false
		}

		// Проверяем временную метку (не старше 5 минут)
		msgTime := time.Unix(update.Message.Date, 0)
		if time.Since(msgTime) > 5*time.Minute {
			s.logger.Warn("Message too old",
				logger.Field{Key: "message_date", Value: msgTime.Unix()},
				logger.Field{Key: "age_minutes", Value: int(time.Since(msgTime).Minutes())},
			)
			return false
		}

		// Проверяем, что пользователь не бот (защита от ботов)
		if update.Message.From.IsBot {
			return false
		}
	}

	// Проверяем callback_query если есть
	if update.CallbackQuery != nil {
		if update.CallbackQuery.From == nil {
			return false
		}

		// Проверяем, что пользователь не бот
		if update.CallbackQuery.From.IsBot {
			return false
		}
	}

	return true
}

// ipWhitelistMiddleware проверяет IP адрес по белому списку
func (s *Server) ipWhitelistMiddleware(allowedIPs []string) func(http.Handler) http.Handler {
	// Преобразуем в map для быстрого поиска
	allowedMap := make(map[string]bool)
	for _, ip := range allowedIPs {
		allowedMap[ip] = true
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Получаем реальный IP
			clientIP := s.getRealIP(r)

			// Проверяем whitelist
			if len(allowedMap) > 0 && !allowedMap[clientIP] {
				s.logger.Warn("IP not in whitelist",
					logger.Field{Key: "ip", Value: clientIP},
					logger.Field{Key: "path", Value: r.URL.Path},
				)
				http.Error(w, "Forbidden", http.StatusForbidden)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// getRealIP получает реальный IP адрес клиента
func (s *Server) getRealIP(r *http.Request) string {
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

// userRateLimitMiddleware применяет rate limiting по пользователям
func (s *Server) userRateLimitMiddleware(telegramLimiter *middleware.TelegramRateLimiter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Применяем только к webhook
			if r.URL.Path != "/webhook" {
				next.ServeHTTP(w, r)
				return
			}

			// Пытаемся извлечь chat_id из запроса
			chatID := s.extractChatIDFromRequest(r)
			if chatID != 0 {
				if !telegramLimiter.AllowUser(chatID) {
					s.logger.Warn("User rate limit exceeded",
						logger.Field{Key: "chat_id", Value: chatID},
					)
					http.Error(w, "Too Many Requests", http.StatusTooManyRequests)
					return
				}
			}

			next.ServeHTTP(w, r)
		})
	}
}

// extractChatIDFromRequest извлекает chat_id из Telegram update
func (s *Server) extractChatIDFromRequest(r *http.Request) int64 {
	// Читаем тело запроса
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return 0
	}

	// Восстанавливаем тело для следующих обработчиков
	r.Body = io.NopCloser(strings.NewReader(string(body)))

	// Парсим JSON
	var update TelegramUpdate
	if err := json.Unmarshal(body, &update); err != nil {
		return 0
	}

	// Извлекаем chat_id
	if update.Message != nil && update.Message.Chat != nil {
		return update.Message.Chat.ID
	}

	if update.CallbackQuery != nil && update.CallbackQuery.From != nil {
		return update.CallbackQuery.From.ID
	}

	return 0
}
