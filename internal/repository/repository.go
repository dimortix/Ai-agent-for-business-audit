// Package repository — доступ к данным (все SQL-запросы приложения).
package repository

import (
	"errors"

	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrNotFound возвращается, когда запись не найдена.
var ErrNotFound = errors.New("запись не найдена")

type Repository struct {
	pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}
