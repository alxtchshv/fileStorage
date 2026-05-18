package middleware

import (
	"net/http"
	"sync"
	"time"
)

// userLimiter — состояние Token Bucket для одного пользователя.
// Token Bucket: бакет вмещает maxBurst токенов, пополняется со скоростью refillRate/сек.
// Каждый запрос тратит 1 токен. Если токенов нет — 429.
type userLimiter struct {
	tokens     float64
	maxBurst   float64
	refillRate float64
	lastRefill time.Time
	mu         sync.Mutex
}

// RateLimitStore хранит лимитеры для всех пользователей.
// sync.Map — конкурентно-безопасная map без явного мьютекса.
// Для распределённого rate limiting (несколько инстансов) используй Redis + IncrCounter.
type RateLimitStore struct {
	limiters sync.Map
}

// RateLimit — middleware ограничения запросов.
// Применяй ПОСЛЕ JWTAuth — нужен userID из контекста.
//
// Алгоритм:
// 1. Достать userID из контекста.
// 2. store.limiters.LoadOrStore(userID, &userLimiter{...}) — получить или создать лимитер.
// 3. lim.mu.Lock() — захватить мьютекс (конкурентный доступ из разных горутин).
// 4. Пополнить токены: elapsed := time.Since(lim.lastRefill); tokens += elapsed * refillRate.
// 5. Если tokens < 1 — вернуть 429 с заголовком Retry-After.
// 6. tokens-- и next.ServeHTTP.
func RateLimit(limit int, period time.Duration) func(http.Handler) http.Handler {
	store := &RateLimitStore{}
	refillRate := float64(limit) / period.Seconds()

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_ = store
			_ = refillRate
			next.ServeHTTP(w, r)
		})
	}
}
