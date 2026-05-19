package worker

import (
	"context"
	"io"
	"log/slog"

	"managerFiles/internal/model"
)

// Encryptor — локальный интерфейс шифратора (соответствует service.EncryptionService).
// Go использует структурную типизацию: любой тип с этими методами реализует интерфейс.
type Encryptor interface {
	Encrypt(dst io.Writer, src io.Reader) error
	Decrypt(dst io.Writer, src io.Reader) error
}

// FileStorage — локальный интерфейс хранилища (соответствует storage.FileStorage).
type FileStorage interface {
	Upload(ctx context.Context, key string, src io.Reader, size int64, contentType string) error
	Download(ctx context.Context, key string) (io.ReadCloser, error)
}

// EncryptAndUploadJob создаёт задачу для воркер пула:
// шифрует src через io.Pipe (без буферизации всего файла в памяти) и загружает в хранилище.
// onSuccess вызывается после успешной загрузки (сохранить метаданные в БД).
// onError вызывается при ошибке.
func EncryptAndUploadJob(
	file *model.File,
	src io.Reader,
	encryptor Encryptor,
	storage FileStorage,
	onSuccess func(ctx context.Context) error,
	onError func(ctx context.Context, err error),
) Job {
	return Job{
		ID: "encrypt-upload:" + file.ID,
		Fn: func(ctx context.Context) error {
			slog.Info("начало шифрования файла", "file_id", file.ID, "size", file.SizeBytes)

			pipeR, pipeW := io.Pipe()

			// Горутина шифрует src и пишет в pipeW.
			// Хранилище читает из pipeR одновременно — данные стримятся без буфера.
			go func() {
				defer pipeW.Close()
				if err := encryptor.Encrypt(pipeW, src); err != nil {
					pipeW.CloseWithError(err)
				}
			}()

			// -1 = неизвестный размер (MinIO использует multipart upload).
			// Нельзя передать file.SizeBytes — после шифрования размер больше (nonce + tags).
			if err := storage.Upload(ctx, file.StorageKey, pipeR, -1, file.MimeType); err != nil {
				slog.Error("ошибка загрузки в хранилище", "file_id", file.ID, "err", err)
				onError(ctx, err)
				return err
			}

			if err := onSuccess(ctx); err != nil {
				slog.Error("ошибка сохранения метаданных", "file_id", file.ID, "err", err)
				onError(ctx, err)
				return err
			}

			slog.Info("файл загружен", "file_id", file.ID)
			return nil
		},
	}
}

// DecryptAndDownloadJob скачивает файл из хранилища и расшифровывает в dst.
func DecryptAndDownloadJob(
	file *model.File,
	dst io.Writer,
	encryptor Encryptor,
	storage FileStorage,
) Job {
	return Job{
		ID: "decrypt-download:" + file.ID,
		Fn: func(ctx context.Context) error {
			reader, err := storage.Download(ctx, file.StorageKey)
			if err != nil {
				return err
			}
			defer reader.Close()

			pipeR, pipeW := io.Pipe()
			go func() {
				defer pipeW.Close()
				if err := encryptor.Decrypt(pipeW, reader); err != nil {
					pipeW.CloseWithError(err)
				}
			}()

			_, err = io.Copy(dst, pipeR)
			return err
		},
	}
}
