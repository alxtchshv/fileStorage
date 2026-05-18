package repository

import (
	"context"
	"managerFiles/internal/model"

	"github.com/jackc/pgx/v5/pgxpool"
)

type DirectoryRepository struct {
	db *pgxpool.Pool
}

func NewDirectoryRepository(db *pgxpool.Pool) *DirectoryRepository {
	return &DirectoryRepository{db: db}
}

// Create вставляет новую директорию.
//
// SQL: INSERT INTO directories (id, user_id, parent_id, name)
//
//	VALUES ($1, $2, $3, $4)
//	RETURNING created_at
//
// parent_id может быть NULL (корневая директория) — pgx передаёт *string как NULL автоматически.
func (r *DirectoryRepository) Create(ctx context.Context, dir *model.Directory) error {
	r.db.QueryRow(ctx, `INSERT INTO directories (id, user_id, parent_id, name)
	  VALUES ($1, $2, $3, $4)
	  RETURNING created_at`, dir.ID, dir.UserID, dir.ParentID, dir.Name).Scan(&dir.CreatedAt)

	return nil
}

// GetByID достаёт директорию по id.
//
// SQL: SELECT id, user_id, parent_id, name, created_at
//
//	FROM directories WHERE id = $1
//
// Проверка прав (user_id) — в сервисном слое, не здесь.
func (r *DirectoryRepository) GetByID(ctx context.Context, id string) (*model.Directory, error) {
	row := r.db.QueryRow(ctx, `SELECT id, user_id, parent_id, name, created_at
	  FROM directories WHERE id = $1`, id)

	var dir model.Directory
	err := row.Scan(&dir.ID, &dir.UserID, &dir.ParentID, &dir.Name, &dir.CreatedAt)
	if err != nil {
		if err.Error() == "no rows in result set" {
			return nil, model.ErrDirNotFound
		}
		return nil, err
	}

	return &dir, nil
}

// GetRootDirs возвращает корневые директории пользователя (parent_id IS NULL).
//
// SQL: SELECT id, user_id, parent_id, name, created_at
//
//	FROM directories
//	WHERE user_id = $1 AND parent_id IS NULL
//	ORDER BY name ASC
func (r *DirectoryRepository) GetRootDirs(ctx context.Context, userID string) ([]*model.Directory, error) {
	rows, err := r.db.Query(ctx, `SELECT id, user_id, parent_id, name, created_at
	  FROM directories
	  WHERE user_id = $1 AND parent_id IS NULL
	  ORDER BY name ASC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var dirs []*model.Directory
	for rows.Next() {
		var dir model.Directory
		err := rows.Scan(&dir.ID, &dir.UserID, &dir.ParentID, &dir.Name, &dir.CreatedAt)
		if err != nil {
			return nil, err
		}
		dirs = append(dirs, &dir)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return dirs, nil
}

// ListSubDirs возвращает вложенные директории.
//
// SQL: SELECT id, user_id, parent_id, name, created_at
//
//	FROM directories
//	WHERE user_id = $1 AND parent_id = $2
//	ORDER BY name ASC
func (r *DirectoryRepository) ListSubDirs(ctx context.Context, userID, parentID string) ([]*model.Directory, error) {
	rows, err := r.db.Query(ctx, `SELECT id, user_id, parent_id, name, created_at
	  FROM directories
	  WHERE user_id = $1 AND parent_id = $2
	  ORDER BY name ASC`, userID, parentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var dirs []*model.Directory
	for rows.Next() {
		var dir model.Directory
		err := rows.Scan(&dir.ID, &dir.UserID, &dir.ParentID, &dir.Name, &dir.CreatedAt)
		if err != nil {
			return nil, err
		}
		dirs = append(dirs, &dir)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return dirs, nil
}

// Delete удаляет директорию. Каскадно удалит все вложенные (ON DELETE CASCADE в миграции).
//
// SQL: DELETE FROM directories WHERE id = $1 AND user_id = $2
//
// Если RowsAffected() == 0 — директория не найдена -> model.ErrDirNotFound.
func (r *DirectoryRepository) Delete(ctx context.Context, id, userID string) error {
	tag, err := r.db.Exec(ctx, `DELETE FROM directories WHERE id = $1 AND user_id = $2`, id, userID)
	if err != nil {
		return err
	}

	if tag.RowsAffected() == 0 {
		return model.ErrDirNotFound
	}

	return nil
}
