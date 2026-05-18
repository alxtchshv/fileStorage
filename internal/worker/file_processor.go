package worker

import (
	"context"
	"io"
	"log/slog"

	"managerFiles/internal/model"
)

// EncryptAndUploadJob создаёт задачу для воркер пула:
// читает src, шифрует поток данных, загружает результат в хранилище.
//
// Почему в воркер пуле, а не в HTTP хендлере?
// HTTP хендлер не должен делать тяжёлую работу — иначе перегрузим пул HTTP горутин.
// Воркер пул контролирует параллелизм: не более N файлов шифруются одновременно.
func EncryptAndUploadJob(
	file *model.File,
	src io.Reader,
	// encryptor encrypt.Encryptor — интерфейс шифратора
	// storage storage.FileStorage — интерфейс хранилища
	onSuccess func(ctx context.Context) error,
	onError func(ctx context.Context, err error),
) Job {
	return Job{
		ID: "encrypt-upload:" + file.ID,
		Fn: func(ctx context.Context) error {
			slog.Info("начало шифрования файла", "file_id", file.ID, "size", file.SizeBytes)

			// 1. Создать pipe: pr, pw := io.Pipe()
			//    Pipe позволяет стримить данные между двумя горутинами без буферизации в памяти.
			//    Писатель (шифратор) пишет в pw, читатель (MinIO) читает из pr.

			// 2. В горутине: encryptor.EncryptStream(pw, src)
			//    Горутина позволяет шифровать и загружать одновременно (конвейер).
			//    go func() {
			//        defer pw.Close()
			//        err := encryptor.EncryptStream(pw, src)
			//        if err != nil { pw.CloseWithError(err) }
			//    }()

			// 3. storage.Upload(ctx, file.StorageKey, pr, encryptedSize, file.MimeType)
			//    MinIO читает из pr по мере того как шифратор пишет в pw.
			//    Весь файл никогда не находится в памяти целиком — только текущий чанк.

			// 4. Если Upload успешен — вызвать onSuccess (сохранить метаданные в PostgreSQL)
			// 5. Если ошибка — вызвать onError (логировать, удалить частично загруженный объект)

			_ = src
			_ = onSuccess
			return nil
		},
	}
}

// DecryptAndDownloadJob создаёт задачу для скачивания и расшифровки файла.
// Используется когда нужно ограничить одновременное количество скачиваний.
// dst — http.ResponseWriter в который пишем расшифрованные данные.
func DecryptAndDownloadJob(
	file *model.File,
	dst io.Writer,
	// encryptor encrypt.Encryptor
	// storage storage.FileStorage
) Job {
	return Job{
		ID: "decrypt-download:" + file.ID,
		Fn: func(ctx context.Context) error {
			slog.Info("начало расшифровки файла", "file_id", file.ID)

			// 1. reader, err := storage.Download(ctx, file.StorageKey)
			//    defer reader.Close()

			// 2. Создать pipe для стриминга расшифровки в HTTP response

			// 3. encryptor.DecryptStream(dst, reader)
			//    Расшифрованные данные стримятся прямо в HTTP response без промежуточного буфера.

			_ = dst
			return nil
		},
	}
}
