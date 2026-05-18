package repository

import (
	"context"
	"managerFiles/internal/model"

	"github.com/jackc/pgx/v5/pgxpool"
)

type FileRepository struct {
	db *pgxpool.Pool
}

func NewFileRepository(db *pgxpool.Pool) *FileRepository {
	return &FileRepository{db: db}
}

// Create вставляет метаданные файла после загрузки в MinIO.
//
// SQL: INSERT INTO files
//
//	  (id, user_id, directory_id, original_name, storage_key,
//	   size_bytes, mime_type, is_encrypted, checksum)
//	VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
//	RETURNING created_at, updated_at
//
// RETURNING позволяет получить серверное время без дополнительного SELECT.
func (r *FileRepository) Create(ctx context.Context, file *model.File) (*model.File, error) {

	r.db.QueryRow(ctx, `INSERT INTO files
	  (id, user_id, directory_id, original_name, storage_key,
	   size_bytes, mime_type, is_encrypted, checksum)
	  VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	  RETURNING created_at, updated_at`,
		file.ID, file.UserID, file.DirectoryID, file.OriginalName,
		file.StorageKey, file.SizeBytes, file.MimeType,
		file.IsEncrypted, file.Checksum).Scan(&file.CreatedAt, &file.UpdatedAt)

	return file, nil
}

// GetByID достаёт метаданные файла. Фильтрует soft-deleted (deleted_at IS NULL).
//
// SQL: SELECT id, user_id, directory_id, original_name, storage_key,
//
//	       size_bytes, mime_type, is_encrypted, checksum,
//	       created_at, updated_at, deleted_at
//	FROM files WHERE id = $1 AND deleted_at IS NULL
//
// Если не найден — pgx.ErrNoRows -> model.ErrFileNotFound.
func (r *FileRepository) GetByID(ctx context.Context, id string) (*model.File, error) {
	row := r.db.QueryRow(ctx, `SELECT id, user_id, directory_id, original_name, storage_key,
	  size_bytes, mime_type, is_encrypted, checksum,
	  created_at, updated_at, deleted_at
	  FROM files WHERE id = $1 AND deleted_at IS NULL`, id)

	var file model.File
	err := row.Scan(&file.ID, &file.UserID, &file.DirectoryID, &file.OriginalName,
		&file.StorageKey, &file.SizeBytes, &file.MimeType,
		&file.IsEncrypted, &file.Checksum,
		&file.CreatedAt, &file.UpdatedAt, &file.DeletedAt)

	if err != nil {
		if err.Error() == "no rows in result set" {
			return nil, model.ErrFileNotFound
		}
		return nil, err
	}

	return &file, nil
}

// GetStorageKeyByID достаёт только storage_key — для события Kafka при удалении.
//
// SQL: SELECT storage_key FROM files
//
//	WHERE id = $1 AND user_id = $2 AND deleted_at IS NULL
func (r *FileRepository) GetStorageKeyByID(ctx context.Context, id, userID string) (string, error) {
	var key string
	r.db.QueryRow(ctx, `SELECT storage_key FROM files
	  WHERE id = $1 AND user_id = $2 AND deleted_at IS NULL`, id, userID).Scan(&key)

	if key == "" {
		return "", model.ErrFileNotFound
	}

	return key, nil
}

// ListByDirectory возвращает все живые файлы директории конкретного пользователя.
//
// SQL: SELECT ... FROM files
//
//	WHERE directory_id = $1 AND user_id = $2 AND deleted_at IS NULL
//	ORDER BY created_at DESC
//
// user_id в WHERE — защита: пользователь не видит чужие файлы.
func (r *FileRepository) ListByDirectory(ctx context.Context, dirID, userID string) ([]*model.File, error) {

	rows, err := r.db.Query(ctx, `SELECT id, user_id, directory_id, original_name, storage_key,
	  size_bytes, mime_type, is_encrypted, checksum,
	  created_at, updated_at, deleted_at
	  FROM files
	  WHERE directory_id = $1 AND user_id = $2 AND deleted_at IS NULL
	  ORDER BY created_at DESC`, dirID, userID)

	if err != nil {
		return nil, err
	}

	defer rows.Close()

	var files []*model.File
	for rows.Next() {

		var file model.File
		err := rows.Scan(&file.ID, &file.UserID, &file.DirectoryID, &file.OriginalName,
			&file.StorageKey, &file.SizeBytes, &file.MimeType,
			&file.IsEncrypted, &file.Checksum,
			&file.CreatedAt, &file.UpdatedAt, &file.DeletedAt)

		if err != nil {
			return nil, err
		}

		files = append(files, &file)

	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return files, nil
}

// SoftDelete помечает файл удалённым без реального удаления строки.
//
// SQL: UPDATE files SET deleted_at = now()
//
//	WHERE id = $1 AND user_id = $2 AND deleted_at IS NULL
//
// Если RowsAffected() == 0 — файл не найден или уже удалён -> model.ErrFileNotFound.
func (r *FileRepository) SoftDelete(ctx context.Context, id, userID string) error {

	tag, err := r.db.Exec(ctx, `UPDATE files SET deleted_at = now()
	  WHERE id = $1 AND user_id = $2 AND deleted_at IS NULL`, id, userID)

	if err != nil {
		return err
	}

	if tag.RowsAffected() == 0 {
		return model.ErrFileNotFound
	}

	return nil
}

func (r *FileRepository) ListAllRecursive(ctx context.Context, dirID, userID string) ([]*model.File, error) {
	// Example implementation: recursively fetch all files in dirID and its subdirectories
	// You may need to adjust this to your DB schema and logic

	var files []*model.File

	query := `
        WITH RECURSIVE subdirs AS (
            SELECT id FROM directories WHERE id = $1 AND user_id = $2
            UNION ALL
            SELECT d.id FROM directories d
            INNER JOIN subdirs s ON d.parent_id = s.id
            WHERE d.user_id = $2
        )
        SELECT f.* FROM files f
        INNER JOIN subdirs s ON f.directory_id = s.id
        WHERE f.user_id = $2
    `

	rows, err := r.db.Query(ctx, query, dirID, userID)
	if err != nil {
		return nil, err
	}

	defer rows.Close()
	for rows.Next() {
		var f model.File

		if err := rows.Scan(
			&f.ID,
			&f.UserID,
			&f.DirectoryID,
			&f.OriginalName,
			&f.StorageKey,
			&f.SizeBytes,
			&f.MimeType,
			&f.IsEncrypted,
			&f.Checksum,
			&f.CreatedAt,
			&f.UpdatedAt,
			&f.DeletedAt,
		); err != nil {
			return nil, err
		}

		files = append(files, &f)
	}

	return files, nil
}
