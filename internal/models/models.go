// Package models — структуры данных, соответствующие схеме БД.
package models

import (
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

func init() {
	// Деньги в JSON — числами (15000.5), а не строками ("15000.5"): так в примерах ТЗ.
	decimal.MarshalJSONWithoutQuotes = true
}

type Participant struct {
	ID             uuid.UUID `json:"id"`
	Phone          string    `json:"phone"`
	AccountID      string    `json:"account_id"`
	Name           string    `json:"name"`
	GroupType      string    `json:"group_type"` // A — контрольная, B — полный функционал
	TelegramChatID *int64    `json:"telegram_chat_id,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
}

type DailyMetric struct {
	ParticipantID    uuid.UUID       `json:"-"`
	Date             time.Time       `json:"date"`
	TotalRevenue     decimal.Decimal `json:"total_revenue"`
	ReturnAmount     decimal.Decimal `json:"return_amount"`
	TransactionCount int             `json:"transaction_count"`
	AvgCheck         decimal.Decimal `json:"avg_check"`
}

// Net — чистая выручка дня (поступления минус возвраты).
func (m DailyMetric) Net() decimal.Decimal {
	return m.TotalRevenue.Sub(m.ReturnAmount)
}

type FixedExpense struct {
	ParticipantID uuid.UUID       `json:"-"`
	Description   string          `json:"description"`
	Amount        decimal.Decimal `json:"amount"`
	DueDayOfMonth int             `json:"due_day_of_month"`
}

type Prediction struct {
	ParticipantID    uuid.UUID       `json:"-"`
	CalculatedAt     time.Time       `json:"calculated_at"`
	ForecastDate     time.Time       `json:"forecast_date"`
	PredictedRevenue decimal.Decimal `json:"predicted_revenue"`
	PredictedLower   decimal.Decimal `json:"predicted_lower"`
	PredictedUpper   decimal.Decimal `json:"predicted_upper"`
	ModelUsed        string          `json:"model_used"`
	HealthIndex      *int            `json:"health_index,omitempty"`
	CashGapDate      *time.Time      `json:"cash_gap_date,omitempty"`
}

type Recommendation struct {
	ID             int64     `json:"id"`
	ParticipantID  uuid.UUID `json:"-"`
	CreatedAt      time.Time `json:"created_at"`
	RuleCode       string    `json:"rule_code"`
	Message        string    `json:"message"`
	WasActionTaken bool      `json:"was_action_taken"`
}

type PushSubscription struct {
	ParticipantID uuid.UUID
	Endpoint      string
	P256dh        string
	Auth          string
}
