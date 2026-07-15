// Package db — подключение к PostgreSQL и применение миграций.
package db

import (
	"context"
	"fmt"
	"time"

	pgxdecimal "github.com/jackc/pgx-shopspring-decimal"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Connect открывает пул соединений и ждёт готовности БД (до ~30 секунд —
// на случай, когда контейнер postgres ещё стартует).
func Connect(ctx context.Context, url string) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		return nil, fmt.Errorf("парсинг DATABASE_URL: %w", err)
	}
	cfg.MaxConns = 8
	// Нативная поддержка shopspring/decimal для колонок NUMERIC/DECIMAL.
	cfg.AfterConnect = func(ctx context.Context, conn *pgx.Conn) error {
		pgxdecimal.Register(conn.TypeMap())
		return nil
	}

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, err
	}

	deadline := time.Now().Add(30 * time.Second)
	for {
		if err = pool.Ping(ctx); err == nil {
			return pool, nil
		}
		if time.Now().After(deadline) {
			pool.Close()
			return nil, fmt.Errorf("БД недоступна: %w", err)
		}
		time.Sleep(time.Second)
	}
}
