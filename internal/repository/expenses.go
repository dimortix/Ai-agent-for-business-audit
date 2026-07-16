package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"alfa-pulse/internal/models"
)

func (r *Repository) ListExpenses(ctx context.Context, pid uuid.UUID) ([]models.FixedExpense, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT participant_id, description, amount, COALESCE(due_day_of_month, 1), category
		FROM fixed_expenses
		WHERE participant_id = $1
		ORDER BY description`, pid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []models.FixedExpense
	for rows.Next() {
		var e models.FixedExpense
		if err := rows.Scan(&e.ParticipantID, &e.Description, &e.Amount, &e.DueDayOfMonth, &e.Category); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// UpsertExpense добавляет или обновляет фиксированный расход участника.
func (r *Repository) UpsertExpense(ctx context.Context, e models.FixedExpense) error {
	if e.Category == "" {
		e.Category = "other"
	}
	_, err := r.pool.Exec(ctx, `
		INSERT INTO fixed_expenses (participant_id, description, amount, due_day_of_month, category)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (participant_id, description) DO UPDATE SET
			amount = EXCLUDED.amount,
			due_day_of_month = EXCLUDED.due_day_of_month,
			category = EXCLUDED.category`,
		e.ParticipantID, e.Description, e.Amount, e.DueDayOfMonth, e.Category)
	return err
}

// DeleteExpense удаляет расход; false — такого не было.
func (r *Repository) DeleteExpense(ctx context.Context, pid uuid.UUID, description string) (bool, error) {
	tag, err := r.pool.Exec(ctx, `
		DELETE FROM fixed_expenses WHERE participant_id = $1 AND description = $2`,
		pid, description)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}

// MonthlyExpensesTotal — сумма всех фиксированных платежей за месяц.
func (r *Repository) MonthlyExpensesTotal(ctx context.Context, pid uuid.UUID) (decimal.Decimal, error) {
	var total decimal.Decimal
	err := r.pool.QueryRow(ctx,
		`SELECT COALESCE(SUM(amount), 0) FROM fixed_expenses WHERE participant_id = $1`, pid).
		Scan(&total)
	return total, err
}
