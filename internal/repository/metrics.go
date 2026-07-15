package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"alfa-pulse/internal/models"
)

// AddDailyDelta накапливает агрегаты дня (UPSERT): при повторном импорте за ту же
// дату суммы и число транзакций прибавляются, средний чек пересчитывается.
func (r *Repository) AddDailyDelta(ctx context.Context, pid uuid.UUID, date time.Time,
	revenue, returns decimal.Decimal, txCount int) error {

	_, err := r.pool.Exec(ctx, `
		INSERT INTO daily_metrics (participant_id, date, total_revenue, return_amount, transaction_count, avg_check)
		VALUES ($1, $2, $3, $4, $5,
			CASE WHEN $5::int > 0 THEN ROUND($3::numeric / $5::int, 2) ELSE 0 END)
		ON CONFLICT (participant_id, date) DO UPDATE SET
			total_revenue     = daily_metrics.total_revenue + EXCLUDED.total_revenue,
			return_amount     = daily_metrics.return_amount + EXCLUDED.return_amount,
			transaction_count = daily_metrics.transaction_count + EXCLUDED.transaction_count,
			avg_check = CASE
				WHEN daily_metrics.transaction_count + EXCLUDED.transaction_count > 0
				THEN ROUND((daily_metrics.total_revenue + EXCLUDED.total_revenue)
					/ (daily_metrics.transaction_count + EXCLUDED.transaction_count), 2)
				ELSE 0 END`,
		pid, date, revenue, returns, txCount)
	return err
}

// GetMetricsRange — метрики за период включительно, по возрастанию даты.
func (r *Repository) GetMetricsRange(ctx context.Context, pid uuid.UUID, from, to time.Time) ([]models.DailyMetric, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT participant_id, date, total_revenue, return_amount, transaction_count, avg_check
		FROM daily_metrics
		WHERE participant_id = $1 AND date BETWEEN $2 AND $3
		ORDER BY date`, pid, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanMetrics(rows)
}

// GetLastMetrics — последние n дней с данными, по возрастанию даты.
func (r *Repository) GetLastMetrics(ctx context.Context, pid uuid.UUID, n int) ([]models.DailyMetric, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT participant_id, date, total_revenue, return_amount, transaction_count, avg_check
		FROM (
			SELECT * FROM daily_metrics
			WHERE participant_id = $1
			ORDER BY date DESC
			LIMIT $2
		) t ORDER BY date`, pid, n)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanMetrics(rows)
}

type MetricTotals struct {
	NetTotal  decimal.Decimal // Σ(total_revenue − return_amount) за всю историю
	FirstDate *time.Time
	LastDate  *time.Time
}

func (r *Repository) GetMetricTotals(ctx context.Context, pid uuid.UUID) (MetricTotals, error) {
	var t MetricTotals
	err := r.pool.QueryRow(ctx, `
		SELECT COALESCE(SUM(total_revenue - return_amount), 0), MIN(date), MAX(date)
		FROM daily_metrics WHERE participant_id = $1`, pid).
		Scan(&t.NetTotal, &t.FirstDate, &t.LastDate)
	return t, err
}

func scanMetrics(rows interface {
	Next() bool
	Scan(dest ...any) error
	Err() error
}) ([]models.DailyMetric, error) {
	var out []models.DailyMetric
	for rows.Next() {
		var m models.DailyMetric
		if err := rows.Scan(&m.ParticipantID, &m.Date, &m.TotalRevenue, &m.ReturnAmount, &m.TransactionCount, &m.AvgCheck); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}
