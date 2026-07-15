package handler

import (
	"crypto/subtle"
	"errors"
	"io"
	"mime"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"alfa-pulse/internal/service"
)

// adminOnly — сверка X-Admin-Token с ADMIN_TOKEN (константное время) +
// аудит-лог каждого админского действия (включая неудачные попытки входа).
func (d Deps) adminOnly(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := r.Header.Get("X-Admin-Token")
		if token == "" || subtle.ConstantTimeCompare([]byte(token), []byte(d.Cfg.AdminToken)) != 1 {
			d.Log.Warn("аудит: отклонён админ-запрос с неверным токеном",
				"method", r.Method, "path", r.URL.Path, "ip", r.RemoteAddr)
			writeErr(w, http.StatusUnauthorized, "нужен корректный заголовок X-Admin-Token")
			return
		}
		d.Log.Info("аудит: админ-действие",
			"method", r.Method, "path", r.URL.Path, "ip", r.RemoteAddr)
		next.ServeHTTP(w, r)
	})
}

// csvBody достаёт CSV из multipart-поля file либо из сырого тела запроса.
func csvBody(w http.ResponseWriter, r *http.Request) (io.ReadCloser, bool) {
	r.Body = http.MaxBytesReader(w, r.Body, 12<<20)

	ct, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if strings.HasPrefix(ct, "multipart/") {
		if err := r.ParseMultipartForm(12 << 20); err != nil {
			writeErr(w, http.StatusBadRequest, "не удалось разобрать multipart-форму")
			return nil, false
		}
		f, _, err := r.FormFile("file")
		if err != nil {
			writeErr(w, http.StatusBadRequest, "ожидается файл в поле form-data «file»")
			return nil, false
		}
		return f, true
	}
	return r.Body, true
}

// POST /api/participants/import — CSV: phone,account_id,group_type[,name]
func (d Deps) importParticipants(w http.ResponseWriter, r *http.Request) {
	body, ok := csvBody(w, r)
	if !ok {
		return
	}
	defer body.Close()

	report, err := d.Svc.ImportParticipantsCSV(r.Context(), body)
	if err != nil {
		d.Log.Error("импорт участников", "err", err)
		writeErr(w, http.StatusInternalServerError, "импорт прерван: "+err.Error())
		return
	}
	d.Log.Info("импорт участников", "imported", report.Imported, "skipped", report.Skipped)
	writeJSON(w, http.StatusOK, report)
}

// POST /api/admin/import-transactions — CSV: account_id,date,amount,type.
// После импорта синхронно пересчитывает всех затронутых участников группы B.
func (d Deps) importTransactions(w http.ResponseWriter, r *http.Request) {
	body, ok := csvBody(w, r)
	if !ok {
		return
	}
	defer body.Close()

	report, err := d.Svc.ImportTransactionsCSV(r.Context(), body)
	if errors.Is(err, service.ErrDuplicateImport) {
		writeErr(w, http.StatusConflict, err.Error())
		return
	}
	if err != nil {
		d.Log.Error("импорт транзакций", "err", err)
		writeErr(w, http.StatusInternalServerError, "импорт прерван: "+err.Error())
		return
	}

	recalcs := make([]map[string]any, 0, len(report.AffectedB))
	for _, pid := range report.AffectedB {
		res, err := d.Svc.Recalculate(r.Context(), pid)
		if err != nil {
			recalcs = append(recalcs, map[string]any{"participant_id": pid, "error": err.Error()})
			continue
		}
		d.notifyAfterRecalc(r, pid, res)
		item := map[string]any{
			"participant_id": pid,
			"health_index":   res.HealthIndex,
			"model_used":     res.ModelUsed,
			"new_advice":     res.NewAdvice,
		}
		if res.CashGapDate != nil {
			item["cash_gap_date"] = res.CashGapDate.Format("2006-01-02")
		}
		recalcs = append(recalcs, item)
	}

	d.Log.Info("импорт транзакций", "imported", report.Imported,
		"days", report.DaysUpdated, "recalculated", len(recalcs))
	writeJSON(w, http.StatusOK, map[string]any{
		"imported":     report.Imported,
		"skipped":      report.Skipped,
		"days_updated": report.DaysUpdated,
		"errors":       report.Errors,
		"recalculated": recalcs,
	})
}

// POST /api/admin/recalculate/{participantID} — внеочередной пересчёт.
func (d Deps) recalculate(w http.ResponseWriter, r *http.Request) {
	pid, err := uuid.Parse(chi.URLParam(r, "participantID"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "некорректный participant_id")
		return
	}

	res, err := d.Svc.Recalculate(r.Context(), pid)
	switch {
	case errors.Is(err, service.ErrControlGroup), errors.Is(err, service.ErrNoData):
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	case err != nil:
		d.Log.Error("пересчёт", "participant", pid, "err", err)
		writeErr(w, http.StatusInternalServerError, "пересчёт не удался")
		return
	}
	d.notifyAfterRecalc(r, pid, res)

	resp := map[string]any{
		"participant_id": pid,
		"health_index":   res.HealthIndex,
		"model_used":     res.ModelUsed,
		"new_advice":     res.NewAdvice,
	}
	if res.CashGapDate != nil {
		resp["cash_gap_date"] = res.CashGapDate.Format("2006-01-02")
	}
	writeJSON(w, http.StatusOK, resp)
}

func (d Deps) notifyAfterRecalc(r *http.Request, pid uuid.UUID, res *service.RecalcResult) {
	p, err := d.Repo.GetParticipantByID(r.Context(), pid)
	if err == nil && d.Notifier != nil {
		d.Notifier.NotifyIfCritical(r.Context(), p, res)
	}
}

// GET /api/admin/participants — сводка по участникам для админки.
func (d Deps) adminParticipants(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	participants, err := d.Repo.ListParticipants(ctx)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "внутренняя ошибка")
		return
	}

	items := make([]map[string]any, 0, len(participants))
	for _, p := range participants {
		item := map[string]any{
			"id":              p.ID,
			"phone":           p.Phone,
			"account_id":      p.AccountID,
			"name":            p.Name,
			"group_type":      p.GroupType,
			"telegram_linked": p.TelegramChatID != nil,
		}
		if totals, err := d.Repo.GetMetricTotals(ctx, p.ID); err == nil && totals.LastDate != nil {
			item["first_data_date"] = totals.FirstDate.Format("2006-01-02")
			item["last_data_date"] = totals.LastDate.Format("2006-01-02")
		}
		if preds, err := d.Repo.GetLatestPredictionBatch(ctx, p.ID); err == nil && len(preds) > 0 && preds[0].HealthIndex != nil {
			item["health_index"] = *preds[0].HealthIndex
		}
		items = append(items, item)
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}
