package middleware

import (
	"log/slog"
	"net/http"
	"time"
)

// responseWriter — обёртка над http.ResponseWriter для перехвата статус кода и байт.
// Стандартный ResponseWriter не позволяет прочитать статус после его записи.
type responseWriter struct {
	http.ResponseWriter
	status      int
	written     int64
	wroteHeader bool
}

func (rw *responseWriter) WriteHeader(status int) {
	if !rw.wroteHeader {
		rw.status = status
		rw.wroteHeader = true
		rw.ResponseWriter.WriteHeader(status)
	}
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	rw.wroteHeader = true
	n, err := rw.ResponseWriter.Write(b)
	rw.written += int64(n)
	return n, err
}

// Logging — middleware структурированного логирования каждого HTTP запроса.
func Logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		wrapped := &responseWriter{
			ResponseWriter: w,
			status:         http.StatusOK,
		}

		next.ServeHTTP(wrapped, r)

		userID, _ := r.Context().Value(ContextKeyUserID).(string)

		slog.Info("http",
			"method", r.Method,
			"path", r.URL.Path,
			"status", wrapped.status,
			"ms", time.Since(start).Milliseconds(),
			"bytes", wrapped.written,
			"user", userID,
		)
	})
}
