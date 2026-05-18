package repository

import (
	"context"
	"errors"
	"managerFiles/internal/model"

	"github.com/jackc/pgx"
	"github.com/jackc/pgx/v5/pgxpool"
)

type UserRepository struct {
	db *pgxpool.Pool
}

func NewUserRepository(db *pgxpool.Pool) *UserRepository {
	return &UserRepository{db: db}
}

// Create вставляет нового пользователя.
//
// SQL: INSERT INTO users (id, username, email, password_hash)
//
//	VALUES ($1, $2, $3, $4)
//
// Если email уже занят — PostgreSQL вернёт ошибку с кодом "23505" (unique violation).
// Поймай её через errors.As(err, &pgErr) и верни model.ErrEmailAlreadyExists.
func (r *UserRepository) Create(ctx context.Context, user *model.User) error {

	if _, err := r.db.Exec(ctx, `INSERT INTO users (id, username, email, password_hash)
	  VALUES ($1, $2, $3, $4)`, user.ID, user.Username, user.Email, user.PasswordHash); err != nil {

		var pgErr *pgx.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return model.ErrEmailAlreadyExists
		}

		return err
	}

	return nil
}

// GetByEmail ищет пользователя по email. Нужен при логине.
//
// SQL: SELECT id, username, email, password_hash, created_at
//
//	FROM users WHERE email = $1
//
// Если строк нет — pgx вернёт pgx.ErrNoRows.
// Преобразуй его в model.ErrUserNotFound.
func (r *UserRepository) GetByEmail(ctx context.Context, email string) (*model.User, error) {

	row := r.db.QueryRow(ctx, `SELECT id, username, email, password_hash, created_at
	  FROM users WHERE email = $1`, email)

	var user model.User

	err := row.Scan(&user.ID, &user.Username, &user.Email, &user.PasswordHash, &user.CreatedAt)

	if err != nil {

		if errors.Is(err, pgx.ErrNoRows) {
			return nil, model.ErrUserNotFound
		}

		return nil, err
	}

	return &user, nil
}

// GetByID ищет пользователя по UUID. Нужен при валидации JWT токена.
//
// SQL: SELECT id, username, email, password_hash, created_at
//
//	FROM users WHERE id = $1
func (r *UserRepository) GetByID(ctx context.Context, id string) (*model.User, error) {

	row := r.db.QueryRow(ctx, `SELECT id, username, email, password_hash, created_at
	  FROM users WHERE id = $1`, id)

	var user model.User

	err := row.Scan(&user.ID, &user.Username, &user.Email, &user.PasswordHash, &user.CreatedAt)

	if err != nil {

		if errors.Is(err, pgx.ErrNoRows) {
			return nil, model.ErrUserNotFound
		}

		return nil, err
	}

	return &user, nil
}
