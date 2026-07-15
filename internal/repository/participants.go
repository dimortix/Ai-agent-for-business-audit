package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"alfa-pulse/internal/models"
)

const participantCols = `id, phone, account_id, name, group_type, telegram_chat_id, created_at`

func scanParticipant(row pgx.Row) (*models.Participant, error) {
	var p models.Participant
	err := row.Scan(&p.ID, &p.Phone, &p.AccountID, &p.Name, &p.GroupType, &p.TelegramChatID, &p.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// UpsertParticipant создаёт участника или обновляет его данные по телефону.
func (r *Repository) UpsertParticipant(ctx context.Context, p models.Participant) (uuid.UUID, error) {
	var id uuid.UUID
	err := r.pool.QueryRow(ctx, `
		INSERT INTO participants (phone, account_id, name, group_type)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (phone) DO UPDATE SET
			account_id = EXCLUDED.account_id,
			name       = EXCLUDED.name,
			group_type = EXCLUDED.group_type
		RETURNING id`,
		p.Phone, p.AccountID, p.Name, p.GroupType).Scan(&id)
	if err != nil {
		return uuid.Nil, fmt.Errorf("upsert участника %s: %w", p.Phone, err)
	}
	return id, nil
}

func (r *Repository) GetParticipantByID(ctx context.Context, id uuid.UUID) (*models.Participant, error) {
	return scanParticipant(r.pool.QueryRow(ctx,
		`SELECT `+participantCols+` FROM participants WHERE id = $1`, id))
}

func (r *Repository) GetParticipantByPhone(ctx context.Context, phone string) (*models.Participant, error) {
	return scanParticipant(r.pool.QueryRow(ctx,
		`SELECT `+participantCols+` FROM participants WHERE phone = $1`, phone))
}

func (r *Repository) GetParticipantByAccountID(ctx context.Context, accountID string) (*models.Participant, error) {
	return scanParticipant(r.pool.QueryRow(ctx,
		`SELECT `+participantCols+` FROM participants WHERE account_id = $1`, accountID))
}

func (r *Repository) GetParticipantByChatID(ctx context.Context, chatID int64) (*models.Participant, error) {
	return scanParticipant(r.pool.QueryRow(ctx,
		`SELECT `+participantCols+` FROM participants WHERE telegram_chat_id = $1`, chatID))
}

func (r *Repository) SetTelegramChatID(ctx context.Context, id uuid.UUID, chatID int64) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE participants SET telegram_chat_id = $2 WHERE id = $1`, id, chatID)
	return err
}

func (r *Repository) ListParticipants(ctx context.Context) ([]models.Participant, error) {
	return r.listParticipants(ctx,
		`SELECT `+participantCols+` FROM participants ORDER BY created_at`)
}

// ListParticipantsByGroup — участники заданной группы (для пересчёта берём 'B').
func (r *Repository) ListParticipantsByGroup(ctx context.Context, group string) ([]models.Participant, error) {
	return r.listParticipants(ctx,
		`SELECT `+participantCols+` FROM participants WHERE group_type = $1 ORDER BY created_at`, group)
}

func (r *Repository) listParticipants(ctx context.Context, sql string, args ...any) ([]models.Participant, error) {
	rows, err := r.pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []models.Participant
	for rows.Next() {
		var p models.Participant
		if err := rows.Scan(&p.ID, &p.Phone, &p.AccountID, &p.Name, &p.GroupType, &p.TelegramChatID, &p.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}
