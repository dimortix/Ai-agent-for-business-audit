package handler

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"alfa-pulse/internal/auth"
)

// GET /api/advice?status=active|done|all
func (d Deps) listAdvice(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	pid := auth.ParticipantID(ctx)

	status := r.URL.Query().Get("status")
	switch status {
	case "", "active":
		status = "active"
	case "done", "all":
	default:
		writeErr(w, http.StatusBadRequest, "status: active | done | all")
		return
	}

	recs, err := d.Repo.ListRecommendations(ctx, pid, status)
	if err != nil {
		d.Log.Error("список советов", "err", err)
		writeErr(w, http.StatusInternalServerError, "внутренняя ошибка")
		return
	}

	items := make([]map[string]any, len(recs))
	for i, rec := range recs {
		items[i] = map[string]any{
			"id":               rec.ID,
			"created_at":       rec.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
			"rule_code":        rec.RuleCode,
			"message":          rec.Message,
			"was_action_taken": rec.WasActionTaken,
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

// POST /api/advice/{id}/done — участник отметил совет выполненным.
func (d Deps) adviceDone(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	pid := auth.ParticipantID(ctx)

	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "некорректный id")
		return
	}

	ok, err := d.Repo.MarkRecommendationDone(ctx, id, pid)
	if err != nil {
		d.Log.Error("отметка совета", "err", err)
		writeErr(w, http.StatusInternalServerError, "внутренняя ошибка")
		return
	}
	if !ok {
		writeErr(w, http.StatusNotFound, "совет не найден")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
