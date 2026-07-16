package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/shopspring/decimal"
)

// RawTransaction — одна операция для ленты «Последние операции».
type RawTransaction struct {
	PaidAt time.Time       `json:"paid_at"`
	Amount decimal.Decimal `json:"amount"`
	Type   string          `json:"type"` // income | return
}

// BulkInsertTransactions — быстрая вставка сырых операций (CopyFrom).
func (r *Repository) BulkInsertTransactions(ctx context.Context, pid uuid.UUID, txs []RawTransaction) error {
	if len(txs) == 0 {
		return nil
	}
	rows := make([][]any, len(txs))
	for i, t := range txs {
		rows[i] = []any{pid, t.PaidAt, t.Amount, t.Type}
	}
	_, err := r.pool.CopyFrom(ctx,
		pgx.Identifier{"transactions"},
		[]string{"participant_id", "paid_at", "amount", "type"},
		pgx.CopyFromRows(rows))
	return err
}

// ListRecentTransactions — последние операции участника (новые первыми).
func (r *Repository) ListRecentTransactions(ctx context.Context, pid uuid.UUID, limit int) ([]RawTransaction, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT paid_at, amount, type FROM transactions
		WHERE participant_id = $1
		ORDER BY paid_at DESC
		LIMIT $2`, pid, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []RawTransaction
	for rows.Next() {
		var t RawTransaction
		if err := rows.Scan(&t.PaidAt, &t.Amount, &t.Type); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}
