package middleware

import (
	"net/http"
	"time"

	"managerFiles/internal/metrics"
)

// Metrics — middleware сбора Prometheus метрик.
// Нормализуй путь перед записью: /api/files/<uuid> -> /api/files/{id}
// иначе каждый UUID создаст отдельный label — cardinality explosion в Prometheus.
func Metrics(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		wrapped := &responseWriter{
			ResponseWriter: w,
			status:         http.StatusOK,
		}

		next.ServeHTTP(wrapped, r)

		metrics.RecordRequest(r.Method, r.URL.Path, wrapped.status, time.Since(start))
	})
}

// normalizePath заменяет UUID-сегменты на {id}.
// Используй regexp или chi RouteContext(r.Context()).RoutePattern().
func normalizePath(path string) string {
	return path
}
