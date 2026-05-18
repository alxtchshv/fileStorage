package repository

import (
	"context"
	"time"

	"managerFiles/internal/model"
)

// UserRepo — интерфейс доступа к пользователям.
type UserRepo interface {
	Create(ctx context.Context, user *model.User) error
	GetByEmail(ctx context.Context, email string) (*model.User, error)
	GetByID(ctx context.Context, id string) (*model.User, error)
}

// FileRepo — интерфейс доступа к метаданным файлов.
type FileRepo interface {
	Create(ctx context.Context, file *model.File) (*model.File, error)
	GetByID(ctx context.Context, id string) (*model.File, error)
	GetStorageKeyByID(ctx context.Context, id, userID string) (string, error)
	ListByDirectory(ctx context.Context, dirID, userID string) ([]*model.File, error)
	SoftDelete(ctx context.Context, id, userID string) error
	ListAllRecursive(ctx context.Context, dirID, userID string) ([]*model.File, error)
}

// DirectoryRepo — интерфейс доступа к директориям.
type DirectoryRepo interface {
	Create(ctx context.Context, dir *model.Directory) error
	GetByID(ctx context.Context, id string) (*model.Directory, error)
	GetRootDirs(ctx context.Context, userID string) ([]*model.Directory, error)
	ListSubDirs(ctx context.Context, userID, parentID string) ([]*model.Directory, error)
	Delete(ctx context.Context, id, userID string) error
}

// TokenStore — интерфейс NoSQL хранилища (Redis) для JWT токенов и кэша.
// Отделён от PostgreSQL репозиториев — другое хранилище, другой слой.
type TokenStore interface {
	// Blacklist добавляет JTI в чёрный список с TTL.
	Blacklist(ctx context.Context, jti string, ttl time.Duration) error
	// IsBlacklisted проверяет наличие JTI в чёрном списке.
	IsBlacklisted(ctx context.Context, jti string) (bool, error)
	// CacheSet сохраняет произвольное значение в кэш с TTL.
	CacheSet(ctx context.Context, key string, value any, ttl time.Duration) error
	// CacheGet читает значение из кэша. false если ключ не найден (cache miss).
	CacheGet(ctx context.Context, key string, dest any) (bool, error)
	// CacheDelete удаляет ключи из кэша (инвалидация).
	CacheDelete(ctx context.Context, keys ...string) error
}

// AuditLogRepo — интерфейс для записи аудит логов.
type AuditLogRepo interface {
	Create(ctx context.Context, entry *model.AuditLog) error
	ListByUser(ctx context.Context, userID string, limit, offset int) ([]*model.AuditLog, error)
}
