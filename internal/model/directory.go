package model

import (
	"strings"
	"time"
)

const (
	MaxDirNameLen     = 255
	forbiddenDirChars = `/\:*?"<>|`
)

type Directory struct {
	ID        string
	UserID    string
	ParentID  *string
	Name      string
	CreatedAt time.Time
}

type DirectoryResponse struct {
	ID        string    `json:"id"`
	ParentID  *string   `json:"parent_id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
}

type DirectoryContents struct {
	Directory   DirectoryResponse   `json:"directory"`
	Directories []DirectoryResponse `json:"directories"`
	Files       []FileResponse      `json:"files"`
}

type CreateDirInput struct {
	Name     string  `json:"name"`
	ParentID *string `json:"parent_id"`
}

func (i *CreateDirInput) Validate() error {
	i.Name = strings.TrimSpace(i.Name)

	if i.Name == "" {
		return ErrEmptyDirName
	}
	if len(i.Name) > MaxDirNameLen {
		return ErrDirNameTooLong
	}
	if strings.ContainsAny(i.Name, forbiddenDirChars) {
		return ErrInvalidDirName
	}
	if i.Name == "." || i.Name == ".." {
		return ErrInvalidDirName
	}
	return nil
}

func (d *Directory) ToResponse() *DirectoryResponse {
	return &DirectoryResponse{
		ID:        d.ID,
		ParentID:  d.ParentID,
		Name:      d.Name,
		CreatedAt: d.CreatedAt,
	}
}
