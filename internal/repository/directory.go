package repository

import (
	"context"
	"errors"
	"managerFiles/internal/model"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type DirectoryRepository struct {
	db *pgxpool.Pool
}

func NewDirectoryRepository(db *pgxpool.Pool) *DirectoryRepository {
	return &DirectoryRepository{db: db}
}

// Create вставляет новую директорию. RETURNING возвращает created_at без лишнего SELECT.
// parent_id = nil — корневая директория (pgx передаёт nil как NULL автоматически).
func (r *DirectoryRepository) Create(ctx context.Context, dir *model.Directory) error {
	return r.db.QueryRow(ctx,
		`INSERT INTO directories (id, user_id, parent_id, name) VALUES ($1, $2, $3, $4) RETURNING created_at`,
		dir.ID, dir.UserID, dir.ParentID, dir.Name,
	).Scan(&dir.CreatedAt)
}

// GetByID достаёт директорию по id. Проверка прав — в сервисном слое.
func (r *DirectoryRepository) GetByID(ctx context.Context, id string) (*model.Directory, error) {
	var dir model.Directory
	err := r.db.QueryRow(ctx,
		`SELECT id, user_id, parent_id, name, created_at FROM directories WHERE id = $1`, id,
	).Scan(&dir.ID, &dir.UserID, &dir.ParentID, &dir.Name, &dir.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, model.ErrDirNotFound
		}
		return nil, err
	}
	return &dir, nil
}

// GetRootDirs возвращает корневые директории пользователя (parent_id IS NULL).
func (r *DirectoryRepository) GetRootDirs(ctx context.Context, userID string) ([]*model.Directory, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, user_id, parent_id, name, created_at
		 FROM directories WHERE user_id = $1 AND parent_id IS NULL ORDER BY name ASC`, userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanDirs(rows)
}

// ListSubDirs возвращает вложенные директории.
func (r *DirectoryRepository) ListSubDirs(ctx context.Context, userID, parentID string) ([]*model.Directory, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, user_id, parent_id, name, created_at
		 FROM directories WHERE user_id = $1 AND parent_id = $2 ORDER BY name ASC`, userID, parentID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanDirs(rows)
}

// Delete удаляет директорию. ON DELETE CASCADE удалит все вложенные.
// Если RowsAffected == 0 — директория не найдена или чужая.
func (r *DirectoryRepository) Delete(ctx context.Context, id, userID string) error {
	tag, err := r.db.Exec(ctx,
		`DELETE FROM directories WHERE id = $1 AND user_id = $2`, id, userID,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return model.ErrDirNotFound
	}
	return nil
}

func scanDirs(rows pgx.Rows) ([]*model.Directory, error) {
	var dirs []*model.Directory
	for rows.Next() {
		var dir model.Directory
		if err := rows.Scan(&dir.ID, &dir.UserID, &dir.ParentID, &dir.Name, &dir.CreatedAt); err != nil {
			return nil, err
		}
		dirs = append(dirs, &dir)
	}
	return dirs, rows.Err()
}
