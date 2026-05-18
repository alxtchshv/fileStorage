package model

import "time"

// Kafka события — структуры сообщений публикуемых в топики.
// Они сериализуются в JSON и отправляются в Kafka.
// Потребители (consumers) в других сервисах или в этом же сервисе читают и обрабатывают их.

// EventFileUploaded публикуется в топик "file.uploaded" после успешной загрузки файла.
// Consumer может использовать это событие для: virus scan, thumbnail generation, indexing.
type EventFileUploaded struct {
	EventID   string    `json:"event_id"` // UUID для идемпотентной обработки
	FileID    string    `json:"file_id"`
	UserID    string    `json:"user_id"`
	FileName  string    `json:"file_name"`
	SizeBytes int64     `json:"size_bytes"`
	MimeType  string    `json:"mime_type"`
	OccuredAt time.Time `json:"occured_at"`
}

// EventFileDeleted публикуется в топик "file.deleted" после мягкого удаления файла.
// Consumer занимается реальным удалением из MinIO через задержку (grace period).
type EventFileDeleted struct {
	EventID    string    `json:"event_id"`
	FileID     string    `json:"file_id"`
	UserID     string    `json:"user_id"`
	StorageKey string    `json:"storage_key"` // ключ объекта в MinIO для удаления
	OccuredAt  time.Time `json:"occured_at"`
}

// EventAuditLog публикуется в топик "audit.log" для каждого важного действия пользователя.
// Используется для аудита, безопасности, дебаггинга.
type EventAuditLog struct {
	EventID   string    `json:"event_id"`
	UserID    string    `json:"user_id"`
	Action    string    `json:"action"`    // "login", "logout", "upload", "delete", "download"
	Resource  string    `json:"resource"`  // "file:uuid", "directory:uuid"
	IPAddress string    `json:"ip_address"`
	UserAgent string    `json:"user_agent"`
	Success   bool      `json:"success"`
	Error     string    `json:"error,omitempty"`
	OccuredAt time.Time `json:"occured_at"`
}

// AuditLog — запись аудита в базе данных (таблица audit_logs).
// Consumer читает EventAuditLog из Kafka и сохраняет в эту таблицу.
type AuditLog struct {
	ID        string
	UserID    string
	Action    string
	Resource  string
	IPAddress string
	UserAgent string
	Success   bool
	Error     string
	CreatedAt time.Time
}
