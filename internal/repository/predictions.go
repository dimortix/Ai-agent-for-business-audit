package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

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
