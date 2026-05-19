package handler

import (
	"log/slog"
	"net/http"
	"strconv"

	"managerFiles/internal/middleware"
	"managerFiles/internal/model"
	"managerFiles/internal/service"

	"github.com/go-chi/chi/v5"
)

type FileHandler struct {
	svc service.FileService
}

// Upload — POST /api/files (multipart/form-data: file, directory_id, encrypt?)
func (h *FileHandler) Upload(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, model.MaxFileSizeBytes)
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		respondError(w, http.StatusBadRequest, "invalid multipart form")
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		respondError(w, http.StatusBadRequest, "file is required")
		return
	}
	defer file.Close()

	userID := r.Context().Value(middleware.ContextKeyUserID).(string)
	input := &model.UploadInput{
		FileName:    header.Filename,
		SizeBytes:   header.Size,
		DirectoryID: r.FormValue("directory_id"),
		Encrypt:     r.FormValue("encrypt") != "false",
	}
	if err := input.Validate(); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	response, err := h.svc.Upload(r.Context(), userID, input, file)
	if err != nil {
		mapServiceError(w, err)
		return
	}
	respondJSON(w, http.StatusCreated, response)
}

// Download — GET /api/files/{id}: стримит файл с расшифровкой прямо в HTTP response.
func (h *FileHandler) Download(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	userID := r.Context().Value(middleware.ContextKeyUserID).(string)

	meta, err := h.svc.GetMeta(r.Context(), id, userID)
	if err != nil {
		mapServiceError(w, err)
		return
	}
	w.Header().Set("Content-Type", meta.MimeType)
	w.Header().Set("Content-Length", strconv.FormatInt(meta.SizeBytes, 10))
	w.Header().Set("Content-Disposition", `attachment; filename="`+meta.OriginalName+`"`)

	if err := h.svc.Download(r.Context(), id, userID, w); err != nil {
		// Заголовки уже отправлены — не можем изменить статус. Логируем.
		slog.Error("ошибка стриминга файла", "file_id", id, "err", err)
	}
}

// Meta — HEAD /api/files/{id}: только заголовки без тела.
func (h *FileHandler) Meta(w http.ResponseWriter, r *http.Request) {
	fileID := chi.URLParam(r, "id")
	userID := r.Context().Value(middleware.ContextKeyUserID).(string)

	meta, err := h.svc.GetMeta(r.Context(), fileID, userID)
	if err != nil {
		mapServiceError(w, err)
		return
	}
	w.Header().Set("Content-Type", meta.MimeType)
	w.Header().Set("Content-Length", strconv.FormatInt(meta.SizeBytes, 10))
	w.Header().Set("Last-Modified", meta.UpdatedAt.Format(http.TimeFormat))
}

// Delete — DELETE /api/files/{id}: soft delete + Kafka событие.
func (h *FileHandler) Delete(w http.ResponseWriter, r *http.Request) {
	fileID := chi.URLParam(r, "id")
	userID := r.Context().Value(middleware.ContextKeyUserID).(string)

	if err := h.svc.Delete(r.Context(), fileID, userID); err != nil {
		mapServiceError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// List — GET /api/dirs/{id}/files
func (h *FileHandler) List(w http.ResponseWriter, r *http.Request) {
	dirID := chi.URLParam(r, "id")
	userID := r.Context().Value(middleware.ContextKeyUserID).(string)

	files, err := h.svc.List(r.Context(), dirID, userID)
	if err != nil {
		mapServiceError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, files)
}
