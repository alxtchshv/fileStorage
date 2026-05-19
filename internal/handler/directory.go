package handler

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"managerFiles/internal/middleware"
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

	userID := r.Context().Value(middleware.ContextKeyUserID).(string)

	dirs, err := h.svc.GetRoot(r.Context(), userID)
	if err != nil {
		mapServiceError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, dirs)
}

// Create обрабатывает POST /api/dirs
// Body: {"name": "Документы", "parent_id": null}
// Response 201: DirectoryResponse
func (h *DirHandler) Create(w http.ResponseWriter, r *http.Request) {
	var input model.CreateDirInput

	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		respondError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	if err := input.Validate(); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	userID := r.Context().Value(middleware.ContextKeyUserID).(string)

	dirResponse, err := h.svc.Create(r.Context(), userID, &input)
	if err != nil {
		mapServiceError(w, err)
		return
	}

	respondJSON(w, http.StatusCreated, dirResponse)
}

// Get обрабатывает GET /api/dirs/{id}
// Возвращает содержимое директории: сама директория + поддиректории + файлы.
// Response 200: DirectoryContents
func (h *DirHandler) Get(w http.ResponseWriter, r *http.Request) {

	dirID := chi.URLParam(r, "id")
	userID := r.Context().Value(middleware.ContextKeyUserID).(string)

	contents, err := h.svc.Get(r.Context(), dirID, userID)
	if err != nil {
		mapServiceError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, contents)
}

// Delete обрабатывает DELETE /api/dirs/{id}
// Рекурсивно удаляет директорию и все её содержимое.
// Response 204: No Content
func (h *DirHandler) Delete(w http.ResponseWriter, r *http.Request) {

	dirID := chi.URLParam(r, "id")
	userID := r.Context().Value(middleware.ContextKeyUserID).(string)

	err := h.svc.Delete(r.Context(), dirID, userID)
	if err != nil {
		mapServiceError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
