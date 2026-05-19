package middleware

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"managerFiles/internal/repository"
	"managerFiles/pkg/jwt"
)

type ContextKey string

const (
	ContextKeyUserID ContextKey = "user_id"
	ContextKeyEmail  ContextKey = "email"
	ContextKeyJTI    ContextKey = "jti"
)

// JWTAuth — middleware проверки JWT токена на каждый защищённый запрос.
// Достаёт токен из Authorization: Bearer <token>, валидирует, проверяет blacklist,
// кладёт userID/email/jti в контекст для хендлеров.
func JWTAuth(jwtManager *jwt.Manager, tokenStore repository.TokenStore) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				respondMWError(w, http.StatusUnauthorized, "требуется авторизация")
				return
			}

			tokenStr, found := strings.CutPrefix(authHeader, "Bearer ")
			if !found {
				respondMWError(w, http.StatusUnauthorized, "неверный формат: Authorization: Bearer <token>")
				return
			}

			userID, email, jti, _, err := jwtManager.ValidateAccess(tokenStr)
			if err != nil {
				if errors.Is(err, jwt.ErrExpiredToken) {
					respondMWError(w, http.StatusUnauthorized, "токен истёк, используйте refresh")
				} else {
					respondMWError(w, http.StatusUnauthorized, "невалидный токен")
				}
				return
			}

			// O(1) запрос в Redis. Если Redis недоступен — пропускаем (fail open).
			revoked, err := tokenStore.IsBlacklisted(r.Context(), jti)
			if err == nil && revoked {
				respondMWError(w, http.StatusUnauthorized, "токен отозван")
				return
			}

			// context.WithValue передаёт данные вниз по call stack без глобальных переменных.
			// Ключи типа ContextKey (не string) — защита от коллизий между пакетами.
			ctx := context.WithValue(r.Context(), ContextKeyUserID, userID)
			ctx = context.WithValue(ctx, ContextKeyEmail, email)
			ctx = context.WithValue(ctx, ContextKeyJTI, jti)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// UserIDFromCtx извлекает UserID из контекста запроса.
func UserIDFromCtx(ctx context.Context) string {
	id, _ := ctx.Value(ContextKeyUserID).(string)
	return id
}

// respondMWError пишет JSON ошибку из middleware.
// Отдельная функция чтобы не было циклической зависимости с пакетом handler.
func respondMWError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	w.Write([]byte(`{"error":"` + msg + `"}`))
}
