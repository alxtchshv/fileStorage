package handler

import (
	"net/http"

	"managerFiles/internal/middleware"
	"managerFiles/internal/repository"
	"managerFiles/internal/service"
	"managerFiles/pkg/jwt"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
)

// NewRouter собирает HTTP роутер с chi.
// Публичные маршруты: /health, /ready, /, /api/auth/*
// Защищённые (JWT middleware): /api/dirs/*, /api/files/*
func NewRouter(
	authSvc service.AuthService,
	fileSvc service.FileService,
	dirSvc service.DirectoryService,
	jwtManager *jwt.Manager,
	tokenStore repository.TokenStore,
) http.Handler {
	r := chi.NewRouter()

	r.Use(chimw.Recoverer)
	r.Use(middleware.Logging)

	health := &HealthHandler{}
	auth := &AuthHandler{svc: authSvc}
	dirs := &DirHandler{svc: dirSvc}
	files := &FileHandler{svc: fileSvc}

	r.Get("/health", health.Live)
	r.Get("/ready", health.Ready)
	r.Get("/", serveUI)

	r.Route("/api/auth", func(r chi.Router) {
		r.Post("/register", auth.Register)
		r.Post("/login", auth.Login)
		r.Post("/refresh", auth.Refresh)
		r.Post("/logout", auth.Logout)
	})

	r.Group(func(r chi.Router) {
		r.Use(middleware.JWTAuth(jwtManager, tokenStore))

		r.Route("/api/dirs", func(r chi.Router) {
			r.Get("/", dirs.GetRoot)
			r.Post("/", dirs.Create)
			r.Get("/{id}", dirs.Get)
			r.Delete("/{id}", dirs.Delete)
		})

		r.Route("/api/files", func(r chi.Router) {
			r.Post("/", files.Upload)
			r.Get("/{id}", files.Download)
			r.Head("/{id}", files.Meta)
			r.Delete("/{id}", files.Delete)
		})
	})

	return r
}
