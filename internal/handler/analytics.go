package handler

import (
	"net/http"
	"time"

	"alfa-pulse/internal/auth"
)

const maxAnalyticsRangeDays = 400

// GET /api/analytics?from=2026-06-01&to=2026-07-01 — история метрик.
func (d Deps) analytics(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	pid := auth.ParticipantID(ctx)

	to := time.Now().UTC().Truncate(24 * time.Hour)
	from := to.AddDate(0, 0, -29)

	if s := r.URL.Query().Get("to"); s != "" {
		t, err := time.Parse("2006-01-02", s)
		if err != nil {
			writeErr(w, http.StatusBadRequest, "параметр to: ожидается дата YYYY-MM-DD")
			return
		}
		to = t
	}
	if s := r.URL.Query().Get("from"); s != "" {
		t, err := time.Parse("2006-01-02", s)
		if err != nil {
			writeErr(w, http.StatusBadRequest, "параметр from: ожидается дата YYYY-MM-DD")
			return
		}
		from = t
	}
	if from.After(to) {
		writeErr(w, http.StatusBadRequest, "from позже to")
		return
	}
	if to.Sub(from) > maxAnalyticsRangeDays*24*time.Hour {
		writeErr(w, http.StatusBadRequest, "слишком большой период (максимум 400 дней)")
		return
	}

	metrics, err := d.Repo.GetMetricsRange(ctx, pid, from, to)
	if err != nil {
		d.Log.Error("аналитика", "err", err)
		writeErr(w, http.StatusInternalServerError, "внутренняя ошибка")
		return
	}

	rows := make([]map[string]any, len(metrics))
	for i, m := range metrics {
		rows[i] = map[string]any{
			"date":         m.Date.Format("2006-01-02"),
			"revenue":      m.TotalRevenue,
			"returns":      m.ReturnAmount,
			"net":          m.Net(),
			"transactions": m.TransactionCount,
			"avg_check":    m.AvgCheck,
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"from": from.Format("2006-01-02"),
		"to":   to.Format("2006-01-02"),
		"days": rows,
	})
}
