package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// OneOffExpense — разовый расход, внесённый предпринимателем.
type OneOffExpense struct {
	ID            int64           `json:"id"`
	ParticipantID uuid.UUID       `json:"-"`
	Date          time.Time       `json:"-"`
	DateStr       string          `json:"date"`
	Amount        decimal.Decimal `json:"amount"`
	Description   string          `json:"description"`
	Category      string          `json:"category"`
}

func (r *Repository) InsertOneOffExpense(ctx context.Context, e OneOffExpense) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO one_off_expenses (participant_id, date, amount, description, category)
		VALUES ($1, $2, $3, $4, $5)`,
		e.ParticipantID, e.Date, e.Amount, e.Description, e.Category)
	return err
}

func (r *Repository) ListOneOffExpenses(ctx context.Context, pid uuid.UUID, limit int) ([]OneOffExpense, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, participant_id, date, amount, description, category
		FROM one_off_expenses
		WHERE participant_id = $1
		ORDER BY date DESC, id DESC
		LIMIT $2`, pid, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []OneOffExpense
	for rows.Next() {
		var e OneOffExpense
		if err := rows.Scan(&e.ID, &e.ParticipantID, &e.Date, &e.Amount, &e.Description, &e.Category); err != nil {
			return nil, err
		}
		e.DateStr = e.Date.Format("2006-01-02")
		out = append(out, e)
	}
	return out, rows.Err()
}

func (r *Repository) DeleteOneOffExpense(ctx context.Context, id int64, pid uuid.UUID) (bool, error) {
	tag, err := r.pool.Exec(ctx, `
		DELETE FROM one_off_expenses WHERE id = $1 AND participant_id = $2`, id, pid)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}

// SumOneOffExpenses — сумма разовых расходов до даты включительно (для баланса).
func (r *Repository) SumOneOffExpenses(ctx context.Context, pid uuid.UUID, until time.Time) (decimal.Decimal, error) {
	var total decimal.Decimal
	err := r.pool.QueryRow(ctx, `
		SELECT COALESCE(SUM(amount), 0) FROM one_off_expenses
		WHERE participant_id = $1 AND date <= $2`, pid, until).Scan(&total)
	return total, err
}

// FutureOneOffExpenses — будущие разовые расходы в окне (для календаря/разрыва).
func (r *Repository) FutureOneOffExpenses(ctx context.Context, pid uuid.UUID, after time.Time, days int) ([]OneOffExpense, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, participant_id, date, amount, description, category
		FROM one_off_expenses
		WHERE participant_id = $1 AND date > $2 AND date <= $3
		ORDER BY date`, pid, after, after.AddDate(0, 0, days))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []OneOffExpense
	for rows.Next() {
		var e OneOffExpense
		if err := rows.Scan(&e.ID, &e.ParticipantID, &e.Date, &e.Amount, &e.Description, &e.Category); err != nil {
			return nil, err
		}
		e.DateStr = e.Date.Format("2006-01-02")
		out = append(out, e)
	}
	return out, rows.Err()
}

// DailyOneOffTotals — разовые расходы по дням периода (для графика прибыли).
func (r *Repository) DailyOneOffTotals(ctx context.Context, pid uuid.UUID, from, to time.Time) (map[string]decimal.Decimal, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT date, COALESCE(SUM(amount), 0) FROM one_off_expenses
		WHERE participant_id = $1 AND date BETWEEN $2 AND $3
		GROUP BY date`, pid, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := map[string]decimal.Decimal{}
	for rows.Next() {
		var d time.Time
		var sum decimal.Decimal
		if err := rows.Scan(&d, &sum); err != nil {
			return nil, err
		}
		out[d.Format("2006-01-02")] = sum
	}
	return out, rows.Err()
}

// MonthOneOffByCategory — разовые расходы месяца по категориям (для отчёта).
func (r *Repository) MonthOneOffByCategory(ctx context.Context, pid uuid.UUID, from, to time.Time) (map[string]decimal.Decimal, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT category, COALESCE(SUM(amount), 0) FROM one_off_expenses
		WHERE participant_id = $1 AND date BETWEEN $2 AND $3
		GROUP BY category`, pid, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := map[string]decimal.Decimal{}
	for rows.Next() {
		var cat string
		var sum decimal.Decimal
		if err := rows.Scan(&cat, &sum); err != nil {
			return nil, err
		}
		out[cat] = sum
	}
	return out, rows.Err()
}
