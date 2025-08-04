package server

import (
	"net"
	"strings"
	"time"

	"github.com/region23/queue/internal/config"
)

// SecurityConfig содержит настройки безопасности
type SecurityConfig struct {
	// Rate Limiting
	HTTPRequestsPerMinute    int           `json:"http_requests_per_minute"`
	TelegramRequestsPerMin   int           `json:"telegram_requests_per_minute"`
	GlobalRequestsPerSecond  int           `json:"global_requests_per_second"`
	RateLimitCleanupInterval time.Duration `json:"rate_limit_cleanup_interval"`

	// IP Filtering
	AllowedIPs       []string `json:"allowed_ips"`
	BlockedIPs       []string `json:"blocked_ips"`
	EnableGeoIP      bool     `json:"enable_geoip"`
	AllowedCountries []string `json:"allowed_countries"`

	// Request Validation
	MaxRequestSize    int64         `json:"max_request_size"`
	MaxHeaderSize     int           `json:"max_header_size"`
	RequestTimeout    time.Duration `json:"request_timeout"`
	MaxConcurrentReqs int           `json:"max_concurrent_requests"`

	// Authentication
	RequireSecretToken    bool          `json:"require_secret_token"`
	EnableHMACValidation  bool          `json:"enable_hmac_validation"`
	TokenRotationInterval time.Duration `json:"token_rotation_interval"`

	// Monitoring
	EnableSecurityLogging bool          `json:"enable_security_logging"`
	AnomalyDetection      bool          `json:"anomaly_detection"`
	SecurityAuditLevel    string        `json:"security_audit_level"` // "basic", "detailed", "full"
	LogRotationInterval   time.Duration `json:"log_rotation_interval"`

	// Advanced Protection
	EnableDDOSProtection        bool          `json:"enable_ddos_protection"`
	SuspiciousActivityThreshold int           `json:"suspicious_activity_threshold"`
	AutoBlockDuration           time.Duration `json:"auto_block_duration"`
	FailedAuthThreshold         int           `json:"failed_auth_threshold"`
}

// LoadSecurityConfig загружает конфигурацию безопасности
func LoadSecurityConfig(cfg *config.Config) *SecurityConfig {
	return &SecurityConfig{
		// Rate Limiting - конвертируем из основной конфигурации
		HTTPRequestsPerMinute:    100,
		TelegramRequestsPerMin:   30,
		GlobalRequestsPerSecond:  10,
		RateLimitCleanupInterval: 5 * time.Minute,

		// IP Filtering
		AllowedIPs:       []string{}, // Пустой список = разрешены все
		BlockedIPs:       []string{},
		EnableGeoIP:      false,
		AllowedCountries: []string{},

		// Request Validation
		MaxRequestSize:    4 * 1024 * 1024, // 4MB для Telegram
		MaxHeaderSize:     1024,            // 1KB заголовки
		RequestTimeout:    30 * time.Second,
		MaxConcurrentReqs: 100,

		// Authentication
		RequireSecretToken:    cfg.Telegram.SecretToken != "",
		EnableHMACValidation:  false, // Включается только если есть настройки
		TokenRotationInterval: 24 * time.Hour,

		// Monitoring
		EnableSecurityLogging: true,
		AnomalyDetection:      true,
		SecurityAuditLevel:    "detailed",
		LogRotationInterval:   24 * time.Hour,

		// Advanced Protection
		EnableDDOSProtection:        true,
		SuspiciousActivityThreshold: 50,
		AutoBlockDuration:           1 * time.Hour,
		FailedAuthThreshold:         5,
	}
}

// ValidateSecurityConfig проверяет корректность настроек безопасности
func (sc *SecurityConfig) ValidateSecurityConfig() error {
	// Проверяем rate limits
	if sc.HTTPRequestsPerMinute <= 0 {
		sc.HTTPRequestsPerMinute = 100
	}
	if sc.TelegramRequestsPerMin <= 0 {
		sc.TelegramRequestsPerMin = 30
	}
	if sc.GlobalRequestsPerSecond <= 0 {
		sc.GlobalRequestsPerSecond = 10
	}

	// Проверяем IP адреса
	sc.AllowedIPs = sc.validateIPList(sc.AllowedIPs)
	sc.BlockedIPs = sc.validateIPList(sc.BlockedIPs)

	// Проверяем размеры запросов
	if sc.MaxRequestSize <= 0 {
		sc.MaxRequestSize = 4 * 1024 * 1024
	}
	if sc.MaxHeaderSize <= 0 {
		sc.MaxHeaderSize = 1024
	}

	// Проверяем уровень аудита
	validLevels := map[string]bool{"basic": true, "detailed": true, "full": true}
	if !validLevels[sc.SecurityAuditLevel] {
		sc.SecurityAuditLevel = "detailed"
	}

	return nil
}

// validateIPList проверяет и фильтрует список IP адресов
func (sc *SecurityConfig) validateIPList(ips []string) []string {
	var validIPs []string
	for _, ip := range ips {
		ip = strings.TrimSpace(ip)
		if ip == "" {
			continue
		}

		// Проверяем, является ли это валидным IP или CIDR
		if net.ParseIP(ip) != nil {
			validIPs = append(validIPs, ip)
		} else if _, _, err := net.ParseCIDR(ip); err == nil {
			validIPs = append(validIPs, ip)
		}
		// Игнорируем невалидные IP
	}
	return validIPs
}

// IsIPAllowed проверяет, разрешен ли IP адрес
func (sc *SecurityConfig) IsIPAllowed(ip string) bool {
	clientIP := net.ParseIP(ip)
	if clientIP == nil {
		return false
	}

	// Проверяем заблокированные IP
	for _, blockedIP := range sc.BlockedIPs {
		if sc.ipMatches(clientIP, blockedIP) {
			return false
		}
	}

	// Если есть белый список, проверяем его
	if len(sc.AllowedIPs) > 0 {
		for _, allowedIP := range sc.AllowedIPs {
			if sc.ipMatches(clientIP, allowedIP) {
				return true
			}
		}
		return false // Не в белом списке
	}

	return true // Нет ограничений
}

// ipMatches проверяет, соответствует ли IP адрес или CIDR
func (sc *SecurityConfig) ipMatches(clientIP net.IP, pattern string) bool {
	// Проверяем точное совпадение IP
	if pattern == clientIP.String() {
		return true
	}

	// Проверяем CIDR
	if _, network, err := net.ParseCIDR(pattern); err == nil {
		return network.Contains(clientIP)
	}

	return false
}

// GetRateLimitConfig возвращает настройки rate limiting
func (sc *SecurityConfig) GetRateLimitConfig() (httpRPM, telegramRPM, globalRPS int) {
	return sc.HTTPRequestsPerMinute, sc.TelegramRequestsPerMin, sc.GlobalRequestsPerSecond
}

// ShouldBlockSuspiciousActivity определяет, блокировать ли подозрительную активность
func (sc *SecurityConfig) ShouldBlockSuspiciousActivity(activityCount int) bool {
	return sc.EnableDDOSProtection && activityCount >= sc.SuspiciousActivityThreshold
}

// GetSecurityHeaders возвращает заголовки безопасности
func (sc *SecurityConfig) GetSecurityHeaders() map[string]string {
	headers := map[string]string{
		"X-Content-Type-Options": "nosniff",
		"X-Frame-Options":        "DENY",
		"X-XSS-Protection":       "1; mode=block",
		"Referrer-Policy":        "strict-origin-when-cross-origin",
		"Server":                 "", // Скрываем информацию о сервере
		"X-Powered-By":           "", // Скрываем технологию
	}

	// Добавляем HSTS только для HTTPS
	headers["Strict-Transport-Security"] = "max-age=31536000; includeSubDomains"

	// Базовая CSP политика
	headers["Content-Security-Policy"] = "default-src 'self'; script-src 'none'; style-src 'none'; img-src 'none'"

	return headers
}

// IsValidUserAgent проверяет User-Agent на подозрительность
func (sc *SecurityConfig) IsValidUserAgent(userAgent string) bool {
	if !sc.AnomalyDetection {
		return true
	}

	ua := strings.ToLower(userAgent)

	// Подозрительные паттерны
	suspicious := []string{
		"bot", "crawler", "spider", "scraper", "scanner",
		"hack", "exploit", "vulnerability", "penetration",
		"sqlmap", "nmap", "nikto", "dirb", "gobuster",
		"python-requests", "curl", "wget", "httpclient",
	}

	for _, pattern := range suspicious {
		if strings.Contains(ua, pattern) {
			return false
		}
	}

	return true
}

// GetAnomalyThresholds возвращает пороги для обнаружения аномалий
func (sc *SecurityConfig) GetAnomalyThresholds() (requestsPerWindow int, timeWindow time.Duration) {
	return sc.SuspiciousActivityThreshold, 5 * time.Minute
}
