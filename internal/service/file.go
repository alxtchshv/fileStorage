package service

import (
	"context"
	"io"
	"log/slog"

	"managerFiles/internal/kafka"
	"managerFiles/internal/model"
	"managerFiles/internal/repository"
	"managerFiles/internal/storage"
	"managerFiles/internal/worker"
)

// fileService реализует FileService.
type fileService struct {
	files    repository.FileRepo
	storage  storage.FileStorage
	encrypt  EncryptionService
	pool     *worker.Pool
	producer *kafka.Producer
}

// NewFileService создаёт сервис управления файлами.
func NewFileService(
	files repository.FileRepo,
	stor storage.FileStorage,
	encrypt EncryptionService,
	pool *worker.Pool,
	producer *kafka.Producer,
) FileService {
	return &fileService{
		files:    files,
		storage:  stor,
		encrypt:  encrypt,
		pool:     pool,
		producer: producer,
	}
}

// Upload загружает и шифрует файл.
// Это самый сложный метод в сервисе — оркестрирует несколько систем.
func (s *fileService) Upload(ctx context.Context, userID string, input *model.UploadInput, reader io.Reader) (*model.FileResponse, error) {

	// 1. Валидировать input.Validate()

	// 2. Сгенерировать fileID: uuid.New().String()

	// 3. Вычислить storageKey: storage.StorageKey(userID, fileID)

	// 4. Отправить задачу в воркер пул для шифрования и загрузки:
	//    job := worker.EncryptAndUploadJob(file, reader, encrypt, storage, ...)
	//    s.pool.Submit(job)
	//
	//    ВАЖНО: воркер пул асинхронный — Upload возвращает управление до завершения загрузки.
	//    Для синхронного ожидания результата используй channel или errgroup.
	//    Для production: синхронно — проще для клиента, знает сразу успех/ошибку.

	// 5. После успешной загрузки создать запись в PostgreSQL:
	//    files.Create(ctx, &model.File{...})

	// 6. Опубликовать событие в Kafka:
	//    producer.PublishFileUploaded(ctx, &model.EventFileUploaded{...})
	//    Это асинхронно — другие сервисы/воркеры отреагируют позже.

	// 7. Вернуть FileResponse с метаданными
	return nil, nil
}

// Download расшифровывает и отдаёт файл.
func (s *fileService) Download(ctx context.Context, fileID, userID string, writer io.Writer) error {
	// 1. Получить метаданные: files.GetByID(ctx, fileID)
	//    Если не найден или удалён -> ErrFileNotFound

	// 2. Проверить права доступа: file.UserID == userID, иначе -> ErrForbidden

	// 3. Скачать из MinIO: storage.Download(ctx, file.StorageKey)
	//    Возвращает io.ReadCloser — стрим данных

	// 4. Если файл зашифрован (file.IsEncrypted):
	//    encrypt.Decrypt(writer, storageReader)
	//    Расшифрованные данные стримятся прямо в writer (HTTP response)

	// 5. Иначе: io.Copy(writer, storageReader)

	// 6. Опубликовать audit log в Kafka

	slog.Info("скачивание файла", "file_id", fileID, "user_id", userID)
	return nil
}

// GetMeta возвращает метаданные без скачивания файла.
func (s *fileService) GetMeta(ctx context.Context, fileID, userID string) (*model.FileMeta, error) {
	// 1. files.GetByID(ctx, fileID)
	// 2. Проверить права: file.UserID == userID
	// 3. Вернуть file.ToMeta()
	return nil, nil
}

// Delete помечает файл удалённым и публикует событие для очистки MinIO.
func (s *fileService) Delete(ctx context.Context, fileID, userID string) error {
	// 1. files.GetByID(ctx, fileID) — проверить существование и получить storageKey
	// 2. Проверить права: file.UserID == userID
	// 3. files.SoftDelete(ctx, fileID, userID) — пометить в PostgreSQL
	// 4. producer.PublishFileDeleted(ctx, event) — Consumer удалит объект из MinIO
	//    Такой паттерн называется "eventual consistency": физическое удаление происходит позже,
	//    но пользователь видит файл как удалённый немедленно.

	slog.Info("удаление файла", "file_id", fileID, "user_id", userID)
	return nil
}

// List возвращает файлы в директории.
func (s *fileService) List(ctx context.Context, dirID, userID string) ([]*model.FileResponse, error) {
	// 1. files.ListByDirectory(ctx, dirID, userID)
	// 2. Конвертировать каждый File -> FileResponse через file.ToResponse()
	// 3. Вернуть слайс ответов
	return nil, nil
}
