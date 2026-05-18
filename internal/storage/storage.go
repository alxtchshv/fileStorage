package storage

import (
	"context"
	"io"
	"time"
)

// FileStorage — интерфейс объектного хранилища.
// Реализации: MinIO (self-hosted), AWS S3, GCS.
// Сервисный слой зависит от интерфейса — провайдер можно менять не трогая бизнес-логику.
type FileStorage interface {
	Upload(ctx context.Context, key string, src io.Reader, size int64, contentType string) error
	Download(ctx context.Context, key string) (io.ReadCloser, error)
	Delete(ctx context.Context, key string) error
	Exists(ctx context.Context, key string) (bool, error)
	GeneratePresignedURL(ctx context.Context, key string, ttl time.Duration) (string, error)
}

// StorageKey формирует путь объекта в хранилище.
// Формат: "users/{userID}/files/{fileID}"
func StorageKey(userID, fileID string) string {
	return "users/" + userID + "/files/" + fileID
}
