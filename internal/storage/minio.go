package storage

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"time"

	"managerFiles/internal/config"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// MinioStorage реализует FileStorage интерфейс через MinIO S3-совместимый API.
// MinIO можно развернуть локально (Docker) или использовать AWS S3 с тем же кодом.
// Зависимость: github.com/minio/minio-go/v7
type MinioStorage struct {
	client *minio.Client
	bucket string
}

// NewMinio создаёт MinIO клиент и проверяет/создаёт бакет.
func NewMinio(cfg *config.Config) *MinioStorage {
	minioClient, err := minio.New(cfg.MinioEndpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.MinioAccessKey, cfg.MinioSecretKey, ""),
		Secure: cfg.MinioUseSSL,
	})
	if err != nil {
		slog.Error("не удалось создать MinIO клиент", "error", err)
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	exists, err := minioClient.BucketExists(ctx, cfg.MinioBucket)
	if err != nil {
		slog.Error("не удалось проверить существование бакета в MinIO", "bucket", cfg.MinioBucket, "error", err)
		return nil
	}

	if !exists {
		if err := minioClient.MakeBucket(ctx, cfg.MinioBucket, minio.MakeBucketOptions{}); err != nil {
			slog.Error("не удалось создать бакет в MinIO", "bucket", cfg.MinioBucket, "error", err)
			return nil
		}
	}

	return &MinioStorage{client: minioClient, bucket: cfg.MinioBucket}
}

// Upload загружает файл в MinIO.
// key — путь объекта (например "users/uuid/files/uuid")
// size — размер в байтах (нужен для Content-Length заголовка MinIO)
func (s *MinioStorage) Upload(ctx context.Context, key string, src io.Reader, size int64, contentType string) error {

	_, err := s.client.PutObject(ctx, s.bucket, key, src, size, minio.PutObjectOptions{
		ContentType: contentType,
	})

	return err
}

// Download скачивает объект из MinIO. Возвращает io.ReadCloser.
// Важно: io.ReadCloser нужно закрыть вызывающим кодом после чтения!
// Данные стримятся — не буферизуются в памяти целиком.
func (s *MinioStorage) Download(ctx context.Context, key string) (io.ReadCloser, error) {

	obj, err := s.client.GetObject(ctx, s.bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return nil, err
	}

	return obj, nil
}

// Delete удаляет объект из MinIO.
func (s *MinioStorage) Delete(ctx context.Context, key string) error {

	err := s.client.RemoveObject(ctx, s.bucket, key, minio.RemoveObjectOptions{})
	return err

}

// Exists проверяет существование объекта через StatObject (не скачивает файл).
func (s *MinioStorage) Exists(ctx context.Context, key string) (bool, error) {

	_, err := s.client.StatObject(ctx, s.bucket, key, minio.StatObjectOptions{})

	if err != nil {

		var mErr minio.ErrorResponse
		if errors.As(err, &mErr) && mErr.StatusCode == 404 {
			return false, nil
		}

		return false, err
	}

	return true, nil
}

// GeneratePresignedURL создаёт временную подписанную ссылку для скачивания.
// Пользователь может скачать файл напрямую из MinIO без проксирования через наш сервер.
// Полезно для больших файлов — снижает нагрузку на API сервер.
func (s *MinioStorage) GeneratePresignedURL(ctx context.Context, key string, ttl time.Duration) (string, error) {

	url, err := s.client.PresignedGetObject(ctx, s.bucket, key, ttl, nil)
	if err != nil {
		return "", err
	}

	return url.String(), nil
}
