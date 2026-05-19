package repository

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"managerFiles/internal/config"

	"github.com/golang-migrate/migrate/v4"
	"github.com/jackc/pgx/v5/pgxpool"

	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

// NewPool создаёт пул соединений PostgreSQL через pgxpool.
// pgxpool — конкурентно-безопасный пул: несколько горутин одновременно
// берут свободное соединение из пула, вместо ожидания одного общего.
func NewPool(ctx context.Context, cfg *config.Config) *pgxpool.Pool {
	connStr := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=disable pool_max_conns=%d",
		cfg.DBHost, cfg.DBPort, cfg.DBUser, cfg.DBPassword, cfg.DBName, cfg.DBMaxConns,
	)

	poolConfig, err := pgxpool.ParseConfig(connStr)
	if err != nil {
		panic(fmt.Errorf("проблемы с созданием poolConfig: %w", err))
	}

	poolConn, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		panic(fmt.Errorf("проблемы с созданием poolConn: %w", err))
	}

	tmpCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	if err := poolConn.Ping(tmpCtx); err != nil {
		poolConn.Close()
		panic(fmt.Errorf("проблемы с подключением pool к DB: %w", err))
	}

	slog.Info("Постгре подклбчен", "host", cfg.DBHost)

	return poolConn
}

// RunMigrations применяет SQL миграции из папки migration/.
// golang-migrate хранит состояние в таблице schema_migrations —
// уже применённые миграции пропускаются, безопасно вызывать при каждом старте.
func RunMigrations(cfg *config.Config) {
	dbURL := fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s?sslmode=disable",
		cfg.DBUser, cfg.DBPassword, cfg.DBHost, cfg.DBPort, cfg.DBName,
	)

	migration, err := migrate.New("file://migration", dbURL)
	if err != nil {
		panic(fmt.Errorf("проблемы с созданием миграции: %w", err))
	}
	defer func() {
		srcConn, destConn := migration.Close()
		if srcConn != nil {
			slog.Warn("migrate source close", "err", srcConn)
		}
		if destConn != nil {
			slog.Warn("migrate source close", "err", destConn)
		}
	}()

	if err := migration.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		panic(fmt.Errorf("проблемы с применением миграции: %w", err))
	}

	slog.Info("RunMigrations реалиовано")
}
