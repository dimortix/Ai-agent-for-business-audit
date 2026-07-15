package repository

import (
	"context"

	"github.com/google/uuid"

	"alfa-pulse/internal/models"
)

func (r *Repository) UpsertPushSubscription(ctx context.Context, s models.PushSubscription) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO push_subscriptions (participant_id, endpoint, p256dh, auth)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (participant_id, endpoint) DO UPDATE SET
			p256dh = EXCLUDED.p256dh,
			auth   = EXCLUDED.auth`,
		s.ParticipantID, s.Endpoint, s.P256dh, s.Auth)
	return err
}

func (r *Repository) ListPushSubscriptions(ctx context.Context, pid uuid.UUID) ([]models.PushSubscription, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT participant_id, endpoint, p256dh, auth
		FROM push_subscriptions WHERE participant_id = $1`, pid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []models.PushSubscription
	for rows.Next() {
		var s models.PushSubscription
		if err := rows.Scan(&s.ParticipantID, &s.Endpoint, &s.P256dh, &s.Auth); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// DeletePushSubscription удаляет «мёртвую» подписку (endpoint вернул 404/410).
func (r *Repository) DeletePushSubscription(ctx context.Context, pid uuid.UUID, endpoint string) error {
	_, err := r.pool.Exec(ctx, `
		DELETE FROM push_subscriptions
		WHERE participant_id = $1 AND endpoint = $2`, pid, endpoint)
	return err
}
