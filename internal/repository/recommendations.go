package repository

import (
	"context"
	"time"

	"github.com/google/uuid"

	"alfa-pulse/internal/models"
)

func (r *Repository) InsertRecommendation(ctx context.Context, pid uuid.UUID, ruleCode, message string) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO recommendations (participant_id, rule_code, message)
		VALUES ($1, $2, $3)`, pid, ruleCode, message)
	return err
}

// ListRecommendations: status = active | done | all.
func (r *Repository) ListRecommendations(ctx context.Context, pid uuid.UUID, status string) ([]models.Recommendation, error) {
	q := `SELECT id, participant_id, created_at, rule_code, message, was_action_taken
	      FROM recommendations WHERE participant_id = $1`
	switch status {
	case "active":
		q += ` AND was_action_taken = FALSE`
	case "done":
		q += ` AND was_action_taken = TRUE`
	}
	q += ` ORDER BY created_at DESC LIMIT 100`

	rows, err := r.pool.Query(ctx, q, pid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []models.Recommendation
	for rows.Next() {
		var rec models.Recommendation
		if err := rows.Scan(&rec.ID, &rec.ParticipantID, &rec.CreatedAt, &rec.RuleCode, &rec.Message, &rec.WasActionTaken); err != nil {
			return nil, err
		}
		out = append(out, rec)
	}
	return out, rows.Err()
}

// MarkRecommendationDone помечает совет выполненным; false — не найден/чужой.
func (r *Repository) MarkRecommendationDone(ctx context.Context, id int64, pid uuid.UUID) (bool, error) {
	tag, err := r.pool.Exec(ctx, `
		UPDATE recommendations SET was_action_taken = TRUE
		WHERE id = $1 AND participant_id = $2`, id, pid)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}

// HasRecentRecommendation — был ли совет того же правила новее since, включая
// выполненные (иначе после «Сделано» ближайший пересчёт создаёт дубль и
// система «пилит» пользователя тем же советом).
func (r *Repository) HasRecentRecommendation(ctx context.Context, pid uuid.UUID, ruleCode string, since time.Time) (bool, error) {
	var exists bool
	err := r.pool.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM recommendations
			WHERE participant_id = $1 AND rule_code = $2 AND created_at > $3
		)`, pid, ruleCode, since).Scan(&exists)
	return exists, err
}

func (r *Repository) CountActiveRecommendations(ctx context.Context, pid uuid.UUID) (int, error) {
	var n int
	err := r.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM recommendations
		WHERE participant_id = $1 AND was_action_taken = FALSE`, pid).Scan(&n)
	return n, err
}
