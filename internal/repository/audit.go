package repository

import (
	"context"
	"managerFiles/internal/model"

	"github.com/jackc/pgx/v5/pgxpool"
)

type AuditLogRepository struct {
	db *pgxpool.Pool
}

func NewAuditLogRepository(db *pgxpool.Pool) *AuditLogRepository {
	return &AuditLogRepository{db: db}
}

// Create записывает одно событие аудита. Вызывается из Kafka consumer.
//
// SQL: INSERT INTO audit_logs
//        (id, user_id, action, resource, ip_address, user_agent, success, error)
//      VALUES ($1, $2, $3, $4, $5::inet, $6, $7, $8)
//
// Приведение $5::inet нужно потому что колонка имеет тип INET, а мы передаём строку.
func (r *AuditLogRepository) Create(ctx context.Context, entry *model.AuditLog) error {
	// r.db.Exec(ctx, sql, entry.ID, entry.UserID, entry.Action, ...)
	return nil
}

// ListByUser возвращает историю действий пользователя с пагинацией.
//
// SQL: SELECT id, user_id, action, resource, ip_address::text, user_agent, success, error, created_at
//      FROM audit_logs
//      WHERE user_id = $1
//      ORDER BY created_at DESC
//      LIMIT $2 OFFSET $3
//
// LIMIT + OFFSET — простая пагинация. limit и offset передаёт клиент через query params.
func (r *AuditLogRepository) ListByUser(ctx context.Context, userID string, limit, offset int) ([]*model.AuditLog, error) {
	// rows, err := r.db.Query(ctx, sql, userID, limit, offset)
	// defer rows.Close()
	// for rows.Next() { scan... }
	return nil, nil
}
