package model

import (
	"strings"
	"time"
)

const (
	MaxFileNameLen   = 255
	MaxFileSizeBytes = 100 * 1024 * 1024 // 100 MB
)

type File struct {
	ID           string
	UserID       string
	DirectoryID  string
	OriginalName string
	StorageKey   string
	SizeBytes    int64
	MimeType     string
	IsEncrypted  bool
	Checksum     string
	CreatedAt    time.Time
	UpdatedAt    time.Time
	DeletedAt    *time.Time
}

type FileResponse struct {
	ID           string    `json:"id"`
	DirectoryID  string    `json:"directory_id"`
	OriginalName string    `json:"original_name"`
	SizeBytes    int64     `json:"size_bytes"`
	MimeType     string    `json:"mime_type"`
	IsEncrypted  bool      `json:"is_encrypted"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type FileMeta struct {
	OriginalName string
	SizeBytes    int64
	MimeType     string
	UpdatedAt    time.Time
}

type UploadInput struct {
	FileName    string
	SizeBytes   int64
	DirectoryID string
	Encrypt     bool
}

func (i *UploadInput) Validate() error {
	i.FileName = strings.TrimSpace(i.FileName)

	if i.FileName == "" {
		return ErrEmptyFileName
	}
	if len(i.FileName) > MaxFileNameLen {
		return ErrFileNameTooLong
	}
	if i.SizeBytes <= 0 || i.SizeBytes > MaxFileSizeBytes {
		return ErrFileTooLarge
	}
	if strings.TrimSpace(i.DirectoryID) == "" {
		return ErrDirNotFound
	}
	return nil
}

func (f *File) ToResponse() *FileResponse {
	return &FileResponse{
		ID:           f.ID,
		DirectoryID:  f.DirectoryID,
		OriginalName: f.OriginalName,
		SizeBytes:    f.SizeBytes,
		MimeType:     f.MimeType,
		IsEncrypted:  f.IsEncrypted,
		CreatedAt:    f.CreatedAt,
		UpdatedAt:    f.UpdatedAt,
	}
}

func (f *File) ToMeta() *FileMeta {
	return &FileMeta{
		OriginalName: f.OriginalName,
		SizeBytes:    f.SizeBytes,
		MimeType:     f.MimeType,
		UpdatedAt:    f.UpdatedAt,
	}
}

func (f *File) IsDeleted() bool {
	return f.DeletedAt != nil
}
