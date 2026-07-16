package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/shopspring/decimal"

	"alfa-pulse/internal/models"
)

// InsertPredictionBatch сохраняет прогноз одним батчем (общий calculated_at).
func (r *Repository) InsertPredictionBatch(ctx context.Context, preds []models.Prediction) error {
	if len(preds) == 0 {
		return nil
	}
	batch := &pgx.Batch{}
	for _, p := range preds {
		batch.Queue(`
			INSERT INTO predictions
				(participant_id, calculated_at, forecast_date, predicted_revenue,
				 predicted_lower, predicted_upper, model_used, health_index, cash_gap_date)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
			p.ParticipantID, p.CalculatedAt, p.ForecastDate, p.PredictedRevenue,
			p.PredictedLower, p.PredictedUpper, p.ModelUsed, p.HealthIndex, p.CashGapDate)
	}
	return r.pool.SendBatch(ctx, batch).Close()
}

// HealthHistory — история значений ИЖБ по батчам пересчёта (по возрастанию времени).
func (r *Repository) HealthHistory(ctx context.Context, pid uuid.UUID, limit int) ([]models.Prediction, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT calculated_at, health_index FROM (
			SELECT DISTINCT ON (calculated_at) calculated_at, health_index
			FROM predictions
			WHERE participant_id = $1 AND health_index IS NOT NULL
			ORDER BY calculated_at DESC
			LIMIT $2
		) t ORDER BY calculated_at`, pid, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []models.Prediction
	for rows.Next() {
		var p models.Prediction
		if err := rows.Scan(&p.CalculatedAt, &p.HealthIndex); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// ForecastFactPair — «честный» прогноз (сделан до наступления даты) и факт.
type ForecastFactPair struct {
	Date      time.Time
	Predicted decimal.Decimal
	Fact      decimal.Decimal
}

// PastForecastAccuracy — пары прогноз/факт для оценки точности модели (MAPE).
func (r *Repository) PastForecastAccuracy(ctx context.Context, pid uuid.UUID, limit int) ([]ForecastFactPair, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT p.forecast_date, p.predicted_revenue, m.total_revenue - m.return_amount
		FROM (
			SELECT DISTINCT ON (forecast_date) forecast_date, predicted_revenue
			FROM predictions
			WHERE participant_id = $1
			  AND forecast_date <= CURRENT_DATE
			  AND calculated_at < forecast_date
			ORDER BY forecast_date, calculated_at DESC
		) p
		JOIN daily_metrics m ON m.participant_id = $1 AND m.date = p.forecast_date
		ORDER BY p.forecast_date DESC
		LIMIT $2`, pid, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []ForecastFactPair
	for rows.Next() {
		var p ForecastFactPair
		if err := rows.Scan(&p.Date, &p.Predicted, &p.Fact); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// GetLatestPredictionBatch — последний рассчитанный прогноз (все точки батча).
func (r *Repository) GetLatestPredictionBatch(ctx context.Context, pid uuid.UUID) ([]models.Prediction, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT participant_id, calculated_at, forecast_date, predicted_revenue,
		       COALESCE(predicted_lower, 0), COALESCE(predicted_upper, 0),
		       model_used, health_index, cash_gap_date
		FROM predictions
		WHERE participant_id = $1
		  AND calculated_at = (SELECT MAX(calculated_at) FROM predictions WHERE participant_id = $1)
		ORDER BY forecast_date`, pid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []models.Prediction
	for rows.Next() {
		var p models.Prediction
		if err := rows.Scan(&p.ParticipantID, &p.CalculatedAt, &p.ForecastDate, &p.PredictedRevenue,
			&p.PredictedLower, &p.PredictedUpper, &p.ModelUsed, &p.HealthIndex, &p.CashGapDate); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}
