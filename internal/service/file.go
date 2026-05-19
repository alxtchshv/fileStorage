package service

import (
	"context"
	"io"
	"mime"
	"path/filepath"
	"time"

	"managerFiles/internal/kafka"
	"managerFiles/internal/model"
	"managerFiles/internal/repository"
	"managerFiles/internal/storage"
	"managerFiles/internal/worker"

	"github.com/google/uuid"
)

type fileService struct {
	files    repository.FileRepo
	storage  storage.FileStorage
	encrypt  EncryptionService
	pool     *worker.Pool
	producer *kafka.Producer
}

func NewFileService(
	files repository.FileRepo,
	stor storage.FileStorage,
	encrypt EncryptionService,
	pool *worker.Pool,
	producer *kafka.Producer,
) FileService {
	return &fileService{files: files, storage: stor, encrypt: encrypt, pool: pool, producer: producer}
}

// Upload шифрует и загружает файл через воркер пул.
// Ждёт результата синхронно через канал — клиент получает ответ когда файл сохранён.
func (s *fileService) Upload(ctx context.Context, userID string, input *model.UploadInput, reader io.Reader) (*model.FileResponse, error) {
	if err := input.Validate(); err != nil {
		return nil, err
	}

	fileID := uuid.New().String()
	file := &model.File{
		ID:           fileID,
		UserID:       userID,
		DirectoryID:  input.DirectoryID,
		OriginalName: input.FileName,
		StorageKey:   storage.StorageKey(userID, fileID),
		SizeBytes:    input.SizeBytes,
		MimeType:     mimeFromFilename(input.FileName),
		IsEncrypted:  input.Encrypt,
	}

	errCh := make(chan error, 1)

	s.pool.Submit(worker.EncryptAndUploadJob(
		file, reader, s.encrypt, s.storage,
		func(ctx context.Context) error {
			_, err := s.files.Create(ctx, file)
			errCh <- err
			return err
		},
		func(ctx context.Context, err error) {
			errCh <- err
		},
	))

	select {
	case err := <-errCh:
		if err != nil {
			return nil, err
		}
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	// Kafka публикация асинхронна — не блокируем ответ клиенту.
	go func() {
		_ = s.producer.PublishFileUploaded(context.Background(), &model.EventFileUploaded{
			EventID:   uuid.New().String(),
			FileID:    file.ID,
			UserID:    userID,
			FileName:  file.OriginalName,
			SizeBytes: file.SizeBytes,
			MimeType:  file.MimeType,
			OccuredAt: time.Now(),
		})
	}()

	return file.ToResponse(), nil
}

// Download скачивает файл из MinIO и расшифровывает, стримя прямо в writer (HTTP response).
func (s *fileService) Download(ctx context.Context, fileID, userID string, writer io.Writer) error {
	file, err := s.files.GetByID(ctx, fileID)
	if err != nil {
		return err
	}
	if file.UserID != userID {
		return model.ErrForbidden
	}

	storageReader, err := s.storage.Download(ctx, file.StorageKey)
	if err != nil {
		return err
	}
	defer storageReader.Close()

	if file.IsEncrypted {
		return s.encrypt.Decrypt(writer, storageReader)
	}
	_, err = io.Copy(writer, storageReader)
	return err
}

// GetMeta возвращает метаданные без скачивания (для HEAD запроса).
func (s *fileService) GetMeta(ctx context.Context, fileID, userID string) (*model.FileMeta, error) {
	file, err := s.files.GetByID(ctx, fileID)
	if err != nil {
		return nil, err
	}
	if file.UserID != userID {
		return nil, model.ErrForbidden
	}
	return file.ToMeta(), nil
}

// Delete помечает файл удалённым в PostgreSQL и публикует событие для очистки MinIO.
func (s *fileService) Delete(ctx context.Context, fileID, userID string) error {
	file, err := s.files.GetByID(ctx, fileID)
	if err != nil {
		return err
	}
	if file.UserID != userID {
		return model.ErrForbidden
	}
	if err := s.files.SoftDelete(ctx, fileID, userID); err != nil {
		return err
	}
	go func() {
		_ = s.producer.PublishFileDeleted(context.Background(), &model.EventFileDeleted{
			EventID:    uuid.New().String(),
			FileID:     file.ID,
			UserID:     userID,
			StorageKey: file.StorageKey,
			OccuredAt:  time.Now(),
		})
	}()
	return nil
}

func (s *fileService) List(ctx context.Context, dirID, userID string) ([]*model.FileResponse, error) {
	files, err := s.files.ListByDirectory(ctx, dirID, userID)
	if err != nil {
		return nil, err
	}
	result := make([]*model.FileResponse, 0, len(files))
	for _, f := range files {
		result = append(result, f.ToResponse())
	}
	return result, nil
}

func mimeFromFilename(filename string) string {
	ext := filepath.Ext(filename)
	if t := mime.TypeByExtension(ext); t != "" {
		return t
	}
	return "application/octet-stream"
}
