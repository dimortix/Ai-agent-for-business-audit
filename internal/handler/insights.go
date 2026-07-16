package handler

import (
	"encoding/csv"
	"fmt"
	"net/http"
	"time"

	"alfa-pulse/internal/auth"
)

// GET /api/insights — расширенная аналитика (сценарии, календарь, бенчмарк…).
func (d Deps) insights(w http.ResponseWriter, r *http.Request) {
	data, err := d.Svc.BuildInsights(r.Context(), auth.ParticipantID(r.Context()))
	if err != nil {
		d.Log.Error("insights", "err", err)
		writeErr(w, http.StatusInternalServerError, "внутренняя ошибка")
		return
	}
	writeJSON(w, http.StatusOK, data)
}

// GET /api/analytics/export?from=&to= — выгрузка метрик за период в CSV.
func (d Deps) exportCSV(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	pid := auth.ParticipantID(ctx)

	to := time.Now().UTC().Truncate(24 * time.Hour)
	from := to.AddDate(0, 0, -29)
	if s := r.URL.Query().Get("from"); s != "" {
		if t, err := time.Parse("2006-01-02", s); err == nil {
			from = t
		}
	}
	if s := r.URL.Query().Get("to"); s != "" {
		if t, err := time.Parse("2006-01-02", s); err == nil {
			to = t
		}
	}
	if from.After(to) || to.Sub(from) > maxAnalyticsRangeDays*24*time.Hour {
		writeErr(w, http.StatusBadRequest, "некорректный период")
		return
	}

	metrics, err := d.Repo.GetMetricsRange(ctx, pid, from, to)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "внутренняя ошибка")
		return
	}

	filename := fmt.Sprintf("alfa-pulse_%s_%s.csv", from.Format("2006-01-02"), to.Format("2006-01-02"))
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="`+filename+`"`)
	_, _ = w.Write([]byte{0xEF, 0xBB, 0xBF}) // BOM: Excel корректно откроет кириллицу

	cw := csv.NewWriter(w)
	_ = cw.Write([]string{"Дата", "Выручка", "Возвраты", "Чистая выручка", "Покупок", "Средний чек"})
	for _, m := range metrics {
		_ = cw.Write([]string{
			m.Date.Format("2006-01-02"),
			m.TotalRevenue.String(),
			m.ReturnAmount.String(),
			m.Net().String(),
			fmt.Sprint(m.TransactionCount),
			m.AvgCheck.String(),
		})
	}
	cw.Flush()
}
