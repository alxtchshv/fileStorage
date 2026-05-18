package kafka

// Топики Kafka — именованные каналы для обмена событиями.
// Нейминг: "<domain>.<action>" — понятно что произошло и в какой области.
// Партиционирование: по user_id — события одного пользователя идут в одну партицию (порядок гарантирован).
const (
	// TopicFileUploaded — файл успешно загружен и зашифрован.
	// Consumers: virus-scan worker, thumbnail generator, search indexer.
	TopicFileUploaded = "file.uploaded"

	// TopicFileDeleted — файл помечен удалённым в БД (soft delete).
	// Consumer: удаляет реальный объект из MinIO после grace period (например 24 часа).
	TopicFileDeleted = "file.deleted"

	// TopicAuditLog — аудит событий пользователей (login, logout, upload, download, delete).
	// Consumer: записывает в таблицу audit_logs в PostgreSQL.
	TopicAuditLog = "audit.log"
)

// MessageHeader — метаданные Kafka сообщения.
// Хранятся в headers записи — не в payload. Используются для роутинга и фильтрации.
type MessageHeader struct {
	EventID   string // UUID, уникальный для каждого события (идемпотентность)
	EventType string // тип события, совпадает с топиком
	UserID    string // идентификатор пользователя (для партиционирования)
	Version   string // версия схемы события (например "v1") для обратной совместимости
}
