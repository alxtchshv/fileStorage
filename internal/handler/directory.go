package handler

import (
	"encoding/json"
	"net/http"

	"managerFiles/internal/model"
	"managerFiles/internal/service"
)

// DirHandler обрабатывает HTTP запросы управления директориями.
type DirHandler struct {
	svc service.DirectoryService
}

// GetRoot обрабатывает GET /api/dirs
// Возвращает корневые директории аутентифицированного пользователя.
// Response 200: массив DirectoryResponse
func (h *DirHandler) GetRoot(w http.ResponseWriter, r *http.Request) {
	// 1. userID := r.Context().Value(middleware.ContextKeyUserID).(string)
	// 2. dirs, err := h.svc.GetRoot(r.Context(), userID)
	// 3. Маппинг ошибок через mapServiceError
	// 4. respondJSON(w, 200, dirs)
}

// Create обрабатывает POST /api/dirs
// Body: {"name": "Документы", "parent_id": null}
// Response 201: DirectoryResponse
func (h *DirHandler) Create(w http.ResponseWriter, r *http.Request) {
	var input model.CreateDirInput
	// 1. json.NewDecoder(r.Body).Decode(&input)
	// 2. userID из контекста
	// 3. h.svc.Create(r.Context(), userID, &input)
	// 4. Успех -> respondJSON(w, 201, dirResponse)

	_ = json.NewDecoder(r.Body)
	_ = input
}

// Get обрабатывает GET /api/dirs/{id}
// Возвращает содержимое директории: сама директория + поддиректории + файлы.
// Response 200: DirectoryContents
func (h *DirHandler) Get(w http.ResponseWriter, r *http.Request) {
	// 1. dirID := chi.URLParam(r, "id")
	// 2. userID из контекста
	// 3. h.svc.Get(r.Context(), dirID, userID)
	// 4. ErrForbidden -> 403, ErrDirNotFound -> 404
	// 5. respondJSON(w, 200, contents)
}

// Delete обрабатывает DELETE /api/dirs/{id}
// Рекурсивно удаляет директорию и все её содержимое.
// Response 204: No Content
func (h *DirHandler) Delete(w http.ResponseWriter, r *http.Request) {
	// 1. dirID := chi.URLParam(r, "id")
	// 2. userID из контекста
	// 3. h.svc.Delete(r.Context(), dirID, userID)
	// 4. respondError или w.WriteHeader(204)
}
