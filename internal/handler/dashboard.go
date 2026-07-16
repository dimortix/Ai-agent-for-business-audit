package handler

import (
	"net/http"
	"time"

	"alfa-pulse/internal/auth"
	"alfa-pulse/internal/service"
)

// GET /api/dashboard — главный экран: ИЖБ, прогноз, разрыв, факт за неделю.
func (d Deps) dashboard(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	pid := auth.ParticipantID(ctx)

	p, err := d.Repo.GetParticipantByID(ctx, pid)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "участник не найден")
		return
	}

	resp := map[string]any{
		"participant": map[string]string{
			"name":       p.Name,
			"phone":      p.Phone,
			"group_type": p.GroupType,
		},
		"has_forecast": false,
		// для карточки привязки Telegram в UI
		"telegram_bot":    d.BotUsername,
		"telegram_linked": p.TelegramChatID != nil,
	}

	// Факт за последние 7 дней с данными.
	last7, err := d.Repo.GetLastMetrics(ctx, pid, 7)
	if err != nil {
		d.Log.Error("метрики за неделю", "err", err)
		writeErr(w, http.StatusInternalServerError, "внутренняя ошибка")
		return
	}
	days := make([]map[string]any, len(last7))
	for i, m := range last7 {
		days[i] = map[string]any{
			"date":         m.Date.Format("2006-01-02"),
			"revenue":      m.Net(),
			"transactions": m.TransactionCount,
			"avg_check":    m.AvgCheck,
		}
	}
	resp["last_7_days"] = days

	// Группа A — контрольная: только факт.
	if p.GroupType != "B" {
		resp["control_group"] = true
		writeJSON(w, http.StatusOK, resp)
		return
	}

	fin, err := d.Svc.ComputeFinancials(ctx, pid)
	if err == nil {
		resp["current_balance"] = fin.CurrentBalance.Round(2)
		resp["monthly_expenses"] = fin.MonthlyExpenses.Round(2)
	}

	if n, err := d.Repo.CountActiveRecommendations(ctx, pid); err == nil {
		resp["active_advice_count"] = n
	}

	preds, err := d.Repo.GetLatestPredictionBatch(ctx, pid)
	if err != nil {
		d.Log.Error("чтение прогноза", "err", err)
		writeErr(w, http.StatusInternalServerError, "внутренняя ошибка")
		return
	}
	if len(preds) == 0 {
		// Данные ещё не импортированы или пересчёт не выполнялся.
		writeJSON(w, http.StatusOK, resp)
		return
	}

	forecast := make([]map[string]any, len(preds))
	for i, pr := range preds {
		forecast[i] = map[string]any{
			"date":  pr.ForecastDate.Format("2006-01-02"),
			"yhat":  pr.PredictedRevenue,
			"lower": pr.PredictedLower,
			"upper": pr.PredictedUpper,
		}
	}
	resp["has_forecast"] = true
	resp["forecast"] = forecast
	resp["model_used"] = preds[0].ModelUsed
	resp["calculated_at"] = preds[0].CalculatedAt.Format(time.RFC3339)

	// Лента последних операций (ТЗ V2, п. 6) — «живой» бизнес на главном экране.
	if txs, err := d.Repo.ListRecentTransactions(ctx, pid, 6); err == nil {
		ops := make([]map[string]any, len(txs))
		for i, t := range txs {
			ops[i] = map[string]any{
				"paid_at": t.PaidAt.Format(time.RFC3339),
				"amount":  t.Amount,
				"type":    t.Type,
			}
		}
		resp["recent_operations"] = ops
	}

	if preds[0].HealthIndex != nil {
		idx := *preds[0].HealthIndex
		resp["health_index"] = idx
		resp["health_status"] = service.HealthStatus(idx)
	}
	if preds[0].CashGapDate != nil {
		resp["cash_gap_date"] = preds[0].CashGapDate.Format("2006-01-02")
	} else {
		resp["cash_gap_date"] = nil
	}

	writeJSON(w, http.StatusOK, resp)
}
