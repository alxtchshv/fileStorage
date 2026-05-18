package handler

import (
	"net/http"

	"managerFiles/internal/repository"
	"managerFiles/internal/service"
	"managerFiles/pkg/jwt"
)

// NewRouter собирает HTTP роутер.
//
// Маршруты:
//   GET  /health                  — без авторизации
//   GET  /ready                   — без авторизации
//   POST /api/auth/register
//   POST /api/auth/login
//   POST /api/auth/refresh
//   POST /api/auth/logout
//   --- защищённые (JWT middleware) ---
//   GET    /api/dirs
//   POST   /api/dirs
//   GET    /api/dirs/{id}
//   DELETE /api/dirs/{id}
//   POST   /api/files
//   GET    /api/files/{id}
//   HEAD   /api/files/{id}
//   DELETE /api/files/{id}
func NewRouter(
	authSvc service.AuthService,
	fileSvc service.FileService,
	dirSvc service.DirectoryService,
	jwtManager *jwt.Manager,
	tokenStore repository.TokenStore,
) http.Handler {
	mux := http.NewServeMux()

	auth := &AuthHandler{svc: authSvc}
	health := &HealthHandler{}

	mux.HandleFunc("GET /health", health.Live)
	mux.HandleFunc("GET /ready", health.Ready)

	mux.HandleFunc("POST /api/auth/register", auth.Register)
	mux.HandleFunc("POST /api/auth/login", auth.Login)
	mux.HandleFunc("POST /api/auth/refresh", auth.Refresh)
	mux.HandleFunc("POST /api/auth/logout", auth.Logout)

	// Защищённые маршруты — добавятся после реализации JWTAuth middleware:
	// protected := http.NewServeMux()
	// protected.HandleFunc("GET /api/dirs", ...)
	// mux.Handle("/api/dirs", middleware.JWTAuth(jwtManager, tokenStore)(protected))

	_ = fileSvc
	_ = dirSvc
	_ = jwtManager
	_ = tokenStore

	return mux
}
