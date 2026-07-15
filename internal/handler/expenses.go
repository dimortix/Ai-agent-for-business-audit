package handler

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"alfa-pulse/internal/models"
	"alfa-pulse/internal/service"
)

// Админ-CRUD фиксированных расходов: без них прогноз кассового разрыва
// «магический» — организаторы пилота управляют платежами из админки.

func participantIDParam(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	pid, err := uuid.Parse(chi.URLParam(r, "participantID"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "некорректный participant_id")
		return uuid.Nil, false
	}
	return pid, true
}

// GET /api/admin/expenses/{participantID}
func (d Deps) listExpenses(w http.ResponseWriter, r *http.Request) {
	pid, ok := participantIDParam(w, r)
	if !ok {
		return
	}
	expenses, err := d.Repo.ListExpenses(r.Context(), pid)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "внутренняя ошибка")
		return
	}
	total := decimal.Zero
	for _, e := range expenses {
		total = total.Add(e.Amount)
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": expenses, "monthly_total": total})
}

// POST /api/admin/expenses/{participantID}
// {"description":"Аренда","amount":170000,"due_day_of_month":16}
func (d Deps) upsertExpense(w http.ResponseWriter, r *http.Request) {
	pid, ok := participantIDParam(w, r)
	if !ok {
		return
	}
	var req struct {
		Description   string          `json:"description"`
		Amount        decimal.Decimal `json:"amount"`
		DueDayOfMonth int             `json:"due_day_of_month"`
	}
	if !decodeBody(w, r, &req) {
		return
	}
	if req.Description == "" || len(req.Description) > 200 {
		writeErr(w, http.StatusBadRequest, "description: 1–200 символов")
		return
	}
	if !req.Amount.IsPositive() {
		writeErr(w, http.StatusBadRequest, "amount должен быть больше нуля")
		return
	}
	if req.DueDayOfMonth < 1 || req.DueDayOfMonth > 31 {
		writeErr(w, http.StatusBadRequest, "due_day_of_month: 1–31")
		return
	}

	err := d.Repo.UpsertExpense(r.Context(), models.FixedExpense{
		ParticipantID: pid,
		Description:   req.Description,
		Amount:        req.Amount,
		DueDayOfMonth: req.DueDayOfMonth,
	})
	if err != nil {
		d.Log.Error("сохранение расхода", "err", err)
		writeErr(w, http.StatusInternalServerError, "внутренняя ошибка")
		return
	}
	d.recalcAfterExpenseChange(r, pid)
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// DELETE /api/admin/expenses/{participantID}?description=Аренда
func (d Deps) deleteExpense(w http.ResponseWriter, r *http.Request) {
	pid, ok := participantIDParam(w, r)
	if !ok {
		return
	}
	desc := r.URL.Query().Get("description")
	if desc == "" {
		writeErr(w, http.StatusBadRequest, "нужен query-параметр description")
		return
	}
	found, err := d.Repo.DeleteExpense(r.Context(), pid, desc)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "внутренняя ошибка")
		return
	}
	if !found {
		writeErr(w, http.StatusNotFound, "такого расхода нет")
		return
	}
	d.recalcAfterExpenseChange(r, pid)
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// Расходы влияют на ИЖБ и дату разрыва — после изменения сразу пересчитываем.
func (d Deps) recalcAfterExpenseChange(r *http.Request, pid uuid.UUID) {
	res, err := d.Svc.Recalculate(r.Context(), pid)
	if err != nil {
		if !errors.Is(err, service.ErrControlGroup) && !errors.Is(err, service.ErrNoData) {
			d.Log.Warn("пересчёт после изменения расходов не удался", "participant", pid, "err", err)
		}
		return
	}
	d.notifyAfterRecalc(r, pid, res)
}
