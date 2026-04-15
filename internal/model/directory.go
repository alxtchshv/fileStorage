package model

import "time"

type Directory struct {
	ID        string  // UUID
	UserID    string  // UUID — чей каталог
	ParentID  *string // UUID или nil (nil = корневой каталог)
	Name      string
	CreatedAt time.Time
}

type DirectoryResponse struct {
	ID        string    `json:"id"`
	ParentID  *string   `json:"parent_id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
}

// содержимое каталога — и папки и файлы вместе
type DirectoryContents struct {
	Directory   DirectoryResponse   `json:"directory"`
	Directories []DirectoryResponse `json:"directories"`
	Files       []FileResponse      `json:"files"`
}
