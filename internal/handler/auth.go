package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"managerFiles/internal/model"
	"managerFiles/internal/service"
)

type AuthHandler struct {
	svc service.AuthService
}

// Register — POST /api/auth/register
// Декодируй JSON в model.RegisterInput, вызови svc.Register, верни 201 + UserResponse.
func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
}

// Login — POST /api/auth/login
// Декодируй JSON в model.LoginInput, вызови svc.Login, верни 200 + TokenPair.
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
}

// Refresh — POST /api/auth/refresh
// Декодируй JSON в model.RefreshInput, вызови svc.Refresh, верни 200 + новый TokenPair.
func (h *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
}

// Logout — POST /api/auth/logout
// Достань токен из заголовка "Authorization: Bearer <token>" через strings.CutPrefix.
// Вызови svc.Logout, верни 204 No Content.
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
}

// respondJSON сериализует data в JSON и пишет в w с нужным статусом.
func respondJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

// respondError пишет JSON ошибку: {"error": "msg"}.
func respondError(w http.ResponseWriter, status int, msg string) {
	respondJSON(w, status, map[string]string{"error": msg})
}

// mapServiceError переводит доменные ошибки в HTTP статус коды.
// errors.Is проверяет всю цепочку ошибок (errors.Unwrap).
func mapServiceError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, model.ErrUnauthorized), errors.Is(err, model.ErrInvalidCredentials),
		errors.Is(err, model.ErrTokenExpired), errors.Is(err, model.ErrTokenInvalid),
		errors.Is(err, model.ErrTokenRevoked):
		respondError(w, http.StatusUnauthorized, err.Error())
	case errors.Is(err, model.ErrForbidden):
		respondError(w, http.StatusForbidden, err.Error())
	case errors.Is(err, model.ErrUserNotFound), errors.Is(err, model.ErrFileNotFound),
		errors.Is(err, model.ErrDirNotFound):
		respondError(w, http.StatusNotFound, err.Error())
	case errors.Is(err, model.ErrEmailAlreadyExists):
		respondError(w, http.StatusConflict, err.Error())
	case errors.Is(err, model.ErrFileTooLarge):
		respondError(w, http.StatusRequestEntityTooLarge, err.Error())
	default:
		respondError(w, http.StatusUnprocessableEntity, err.Error())
	}
}
