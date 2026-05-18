package middleware

import (
	"context"
	"net/http"

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
//
// Алгоритм:
// 1. Достать заголовок Authorization, отрезать "Bearer " через strings.CutPrefix.
// 2. jwtManager.ValidateAccess(tokenStr) — проверить подпись, exp, тип токена.
//    ErrExpiredToken -> 401, остальное -> 401.
// 3. tokenStore.IsBlacklisted(ctx, jti) — O(1) запрос в Redis.
//    Если в blacklist -> 401 "токен отозван".
// 4. Положить userID, email, jti в контекст через context.WithValue.
//    Ключи типа ContextKey (не string) — защита от коллизий с другими пакетами.
// 5. next.ServeHTTP(w, r.WithContext(ctx))
func JWTAuth(jwtManager *jwt.Manager, tokenStore repository.TokenStore) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, r)
		})
	}
}

// UserIDFromCtx извлекает UserID из контекста запроса.
// Вызывается в хендлерах вместо прямого обращения к context.Value.
func UserIDFromCtx(ctx context.Context) string {
	return ""
}

// respondError пишет JSON ошибку в middleware.
// Отдельная функция чтобы не было циклической зависимости с пакетом handler.
func respondError(w http.ResponseWriter, status int, msg string) {
}

// подавить "unused import"
var _ = jwt.ErrExpiredToken
