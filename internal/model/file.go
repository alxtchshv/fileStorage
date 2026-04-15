package model

import "time"

type File struct {
	ID           string // UUID
	UserID       string // UUID — чей файл
	DirectoryID  string // UUID — в каком каталоге
	OriginalName string // видит пользователь
	StoredName   string // имя на диске
	SizeBytes    int64
	MimeType     string
	CreatedAt    time.Time
	UpdatedAt    time.Time
	DeletedAt    *time.Time // nil = файл живой
}

// для JSON ответов
type FileResponse struct {
	ID           string    `json:"id"`
	DirectoryID  string    `json:"directory_id"`
	OriginalName string    `json:"original_name"`
	SizeBytes    int64     `json:"size_bytes"`
	MimeType     string    `json:"mime_type"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// для HEAD запроса — только метаданные
type FileMeta struct {
	OriginalName string
	SizeBytes    int64
	MimeType     string
	UpdatedAt    time.Time
}
