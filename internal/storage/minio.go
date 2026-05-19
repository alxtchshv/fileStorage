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
// Если MinIO недоступен — возвращает unavailableStorage, который возвращает ошибку на каждый вызов.
// Это позволяет серверу стартовать без MinIO (auth и dirs работают, файлы — нет).
func NewMinio(cfg *config.Config) FileStorage {
	minioClient, err := minio.New(cfg.MinioEndpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.MinioAccessKey, cfg.MinioSecretKey, ""),
		Secure: cfg.MinioUseSSL,
	})
	if err != nil {
		slog.Warn("MinIO недоступен, загрузка файлов будет недоступна", "err", err)
		return &unavailableStorage{reason: err.Error()}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	exists, err := minioClient.BucketExists(ctx, cfg.MinioBucket)
	if err != nil {
		slog.Warn("MinIO недоступен, загрузка файлов будет недоступна", "err", err)
		return &unavailableStorage{reason: err.Error()}
	}

	if !exists {
		if err := minioClient.MakeBucket(ctx, cfg.MinioBucket, minio.MakeBucketOptions{}); err != nil {
			slog.Warn("не удалось создать бакет MinIO", "bucket", cfg.MinioBucket, "err", err)
			return &unavailableStorage{reason: err.Error()}
		}
	}

	slog.Info("MinIO подключён", "endpoint", cfg.MinioEndpoint, "bucket", cfg.MinioBucket)
	return &MinioStorage{client: minioClient, bucket: cfg.MinioBucket}
}

// unavailableStorage возвращает ошибку на все операции когда MinIO недоступен.
// Позволяет серверу работать без MinIO (auth, dirs доступны, файлы — нет).
type unavailableStorage struct{ reason string }

func (u *unavailableStorage) Upload(_ context.Context, _ string, _ io.Reader, _ int64, _ string) error {
	return errors.New("хранилище файлов недоступно: " + u.reason)
}
func (u *unavailableStorage) Download(_ context.Context, _ string) (io.ReadCloser, error) {
	return nil, errors.New("хранилище файлов недоступно: " + u.reason)
}
func (u *unavailableStorage) Delete(_ context.Context, _ string) error {
	return errors.New("хранилище файлов недоступно: " + u.reason)
}
func (u *unavailableStorage) Exists(_ context.Context, _ string) (bool, error) {
	return false, errors.New("хранилище файлов недоступно: " + u.reason)
}
func (u *unavailableStorage) GeneratePresignedURL(_ context.Context, _ string, _ time.Duration) (string, error) {
	return "", errors.New("хранилище файлов недоступно: " + u.reason)
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
