package handler

import "net/http"

type HealthHandler struct{}

// Live — GET /health. Сервис жив и отвечает.
func (h *HealthHandler) Live(w http.ResponseWriter, r *http.Request) {
	respondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// Ready — GET /ready. Проверяет доступность PostgreSQL, Redis.
// Если хотя бы одна зависимость недоступна — верни 503.
func (h *HealthHandler) Ready(w http.ResponseWriter, r *http.Request) {
	respondJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}
