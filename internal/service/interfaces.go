package service

import (
	"context"
	"io"

	"managerFiles/internal/model"
	"managerFiles/pkg/jwt"
)

// AuthService — интерфейс аутентификации.
type AuthService interface {
	// Register регистрирует нового пользователя.
	// Хеширует пароль через bcrypt перед сохранением.
	Register(ctx context.Context, input *model.RegisterInput) (*model.UserResponse, error)

	// Login проверяет credentials и возвращает пару JWT токенов.
	Login(ctx context.Context, input *model.LoginInput) (*jwt.TokenPair, error)

	// Refresh обменивает refresh токен на новую пару access+refresh токенов.
	// Старый refresh токен инвалидируется (добавляется в Redis blacklist).
	Refresh(ctx context.Context, refreshToken string) (*jwt.TokenPair, error)

	// Logout инвалидирует access токен (добавляет в Redis blacklist).
	// Клиент также должен удалить refresh токен на своей стороне.
	Logout(ctx context.Context, accessToken string) error
}

// FileService — интерфейс управления файлами.
type FileService interface {
	// Upload принимает поток данных файла, шифрует и сохраняет в MinIO.
	// Публикует событие EventFileUploaded в Kafka.
	// Задача шифрования отправляется в воркер пул — не блокирует HTTP горутину.
	Upload(ctx context.Context, userID string, input *model.UploadInput, reader io.Reader) (*model.FileResponse, error)

	// Download достаёт файл из MinIO, расшифровывает и стримит в writer.
	// Проверяет что файл принадлежит пользователю (userID).
	Download(ctx context.Context, fileID, userID string, writer io.Writer) error

	// GetMeta возвращает метаданные файла без скачивания (для HEAD запроса).
	GetMeta(ctx context.Context, fileID, userID string) (*model.FileMeta, error)

	// Delete помечает файл удалённым (soft delete в PostgreSQL).
	// Публикует событие EventFileDeleted в Kafka для последующей очистки MinIO.
	Delete(ctx context.Context, fileID, userID string) error

	// List возвращает список файлов в директории.
	List(ctx context.Context, dirID, userID string) ([]*model.FileResponse, error)
}

// DirectoryService — интерфейс управления директориями.
type DirectoryService interface {
	Create(ctx context.Context, userID string, input *model.CreateDirInput) (*model.DirectoryResponse, error)
	Get(ctx context.Context, dirID, userID string) (*model.DirectoryContents, error)
	GetRoot(ctx context.Context, userID string) ([]*model.DirectoryResponse, error)
	Delete(ctx context.Context, dirID, userID string) error
}

// EncryptionService — интерфейс шифрования.
type EncryptionService interface {
	Encrypt(dst io.Writer, src io.Reader) error
	Decrypt(dst io.Writer, src io.Reader) error
}
