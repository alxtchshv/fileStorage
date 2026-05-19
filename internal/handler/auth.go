package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"managerFiles/internal/model"
	"managerFiles/internal/service"
)

type AuthHandler struct {
	svc service.AuthService
}

// Register — POST /api/auth/register
func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var input model.RegisterInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		respondError(w, http.StatusBadRequest, "неверный формат JSON")
		return
	}
	user, err := h.svc.Register(r.Context(), &input)
	if err != nil {
		mapServiceError(w, err)
		return
	}
	respondJSON(w, http.StatusCreated, user)
}

// Login — POST /api/auth/login
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var input model.LoginInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		respondError(w, http.StatusBadRequest, "неверный формат JSON")
		return
	}
	pair, err := h.svc.Login(r.Context(), &input)
	if err != nil {
		mapServiceError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, pair)
}

// Refresh — POST /api/auth/refresh
func (h *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	var input model.RefreshInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		respondError(w, http.StatusBadRequest, "неверный формат JSON")
		return
	}
	pair, err := h.svc.Refresh(r.Context(), input.RefreshToken)
	if err != nil {
		mapServiceError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, pair)
}

// Logout — POST /api/auth/logout (Authorization: Bearer <access_token>)
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	tokenStr, found := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
	if !found || tokenStr == "" {
		respondError(w, http.StatusUnauthorized, "требуется токен")
		return
	}
	if err := h.svc.Logout(r.Context(), tokenStr); err != nil {
		mapServiceError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func respondJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func respondError(w http.ResponseWriter, status int, msg string) {
	respondJSON(w, status, map[string]string{"error": msg})
}

// mapServiceError переводит доменные ошибки в HTTP статус коды.
// errors.Is проверяет всю цепочку ошибок через errors.Unwrap.
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
