package main

import (
	"log"
	"net/http"
	"sync"
	"time"
)

// Simple rate limiter
type RateLimiter struct {
	mu       sync.Mutex
	requests map[string][]time.Time
	limit    int
	window   time.Duration
}

// NewRateLimiter creates a simple rate limiter
func NewRateLimiter(limit int, window time.Duration) *RateLimiter {
	return &RateLimiter{
		requests: make(map[string][]time.Time),
		limit:    limit,
		window:   window,
	}
}

// Allow checks if request is allowed
func (rl *RateLimiter) Allow(key string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	windowStart := now.Add(-rl.window)

	// Clean old requests
	var validRequests []time.Time
	for _, t := range rl.requests[key] {
		if t.After(windowStart) {
			validRequests = append(validRequests, t)
		}
	}

	// Check limit
	if len(validRequests) >= rl.limit {
		return false
	}

	// Add new request
	rl.requests[key] = append(validRequests, now)
	return true
}

// Simple middleware functions

// LoggingMiddleware logs HTTP requests
func LoggingMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		log.Printf("%s %s from %s", r.Method, r.URL.Path, r.RemoteAddr)
		next(w, r)
		log.Printf("Request completed in %v", time.Since(start))
	}
}

// RateLimitMiddleware applies simple rate limiting
func RateLimitMiddleware(rl *RateLimiter) func(http.HandlerFunc) http.HandlerFunc {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			if !rl.Allow(r.RemoteAddr) {
				http.Error(w, "Too many requests", http.StatusTooManyRequests)
				return
			}
			next(w, r)
		}
	}
}