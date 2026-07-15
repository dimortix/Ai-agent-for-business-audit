package repository

import "context"

// HasImportBatch — импортировался ли уже файл с таким sha256.
func (r *Repository) HasImportBatch(ctx context.Context, hash string) (bool, error) {
	var exists bool
	err := r.pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM import_batches WHERE hash = $1)`, hash).Scan(&exists)
	return exists, err
}

// RecordImportBatch фиксирует успешный импорт файла.
func (r *Repository) RecordImportBatch(ctx context.Context, hash string, rows int) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO import_batches (hash, rows_count) VALUES ($1, $2)
		ON CONFLICT (hash) DO NOTHING`, hash, rows)
	return err
}
