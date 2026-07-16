package handler

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"alfa-pulse/internal/auth"
	"alfa-pulse/internal/models"
	"alfa-pulse/internal/repository"
	"alfa-pulse/internal/service"
)

// Раздел «Мои расходы»: предприниматель сам ведёт регулярные и разовые траты
// (ТЗ V2, п. 2). Изменения сразу влияют на прогноз кассового разрыва и прибыль.

var validCategories = map[string]bool{
	"rent": true, "salary": true, "supplies": true, "taxes": true,
	"loan": true, "utilities": true, "other": true,
}

func normCategory(c string) string {
	if validCategories[c] {
		return c
	}
	return "other"
}

// GET /api/my/expenses — регулярные (fixed) + разовые (one-off) расходы участника.
func (d Deps) myExpenses(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	pid := auth.ParticipantID(ctx)

	fixed, err := d.Repo.ListExpenses(ctx, pid)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "внутренняя ошибка")
		return
	}
	oneOff, err := d.Repo.ListOneOffExpenses(ctx, pid, 100)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "внутренняя ошибка")
		return
	}
	monthly := decimal.Zero
	for _, e := range fixed {
		monthly = monthly.Add(e.Amount)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"fixed":         fixed,
		"one_off":       oneOff,
		"monthly_total": monthly,
	})
}

// POST /api/my/expenses/fixed — добавить/обновить регулярный платёж.
func (d Deps) addFixedExpense(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Description   string          `json:"description"`
		Amount        decimal.Decimal `json:"amount"`
		DueDayOfMonth int             `json:"due_day_of_month"`
		Category      string          `json:"category"`
	}
	if !decodeBody(w, r, &req) {
		return
	}
	if req.Description == "" || len(req.Description) > 200 {
		writeErr(w, http.StatusBadRequest, "название: 1–200 символов")
		return
	}
	if !req.Amount.IsPositive() {
		writeErr(w, http.StatusBadRequest, "сумма должна быть больше нуля")
		return
	}
	if req.DueDayOfMonth < 1 || req.DueDayOfMonth > 31 {
		writeErr(w, http.StatusBadRequest, "день платежа: 1–31")
		return
	}

	pid := auth.ParticipantID(r.Context())
	err := d.Repo.UpsertExpense(r.Context(), modelFixed(pid, req.Description, req.Amount, req.DueDayOfMonth, req.Category))
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "внутренняя ошибка")
		return
	}
	d.recalcSelf(r, pid)
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// DELETE /api/my/expenses/fixed?description=... — удалить регулярный платёж.
func (d Deps) deleteFixedExpense(w http.ResponseWriter, r *http.Request) {
	desc := r.URL.Query().Get("description")
	if desc == "" {
		writeErr(w, http.StatusBadRequest, "нужен параметр description")
		return
	}
	pid := auth.ParticipantID(r.Context())
	ok, err := d.Repo.DeleteExpense(r.Context(), pid, desc)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "внутренняя ошибка")
		return
	}
	if !ok {
		writeErr(w, http.StatusNotFound, "расход не найден")
		return
	}
	d.recalcSelf(r, pid)
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// POST /api/my/expenses/one-off — добавить разовый расход.
func (d Deps) addOneOffExpense(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Date        string          `json:"date"`
		Amount      decimal.Decimal `json:"amount"`
		Description  string          `json:"description"`
		Category    string          `json:"category"`
	}
	if !decodeBody(w, r, &req) {
		return
	}
	date, err := time.Parse("2006-01-02", req.Date)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "дата: ожидается YYYY-MM-DD")
		return
	}
	if !req.Amount.IsPositive() {
		writeErr(w, http.StatusBadRequest, "сумма должна быть больше нуля")
		return
	}
	if req.Description == "" || len(req.Description) > 200 {
		writeErr(w, http.StatusBadRequest, "описание: 1–200 символов")
		return
	}

	pid := auth.ParticipantID(r.Context())
	err = d.Repo.InsertOneOffExpense(r.Context(), repository.OneOffExpense{
		ParticipantID: pid,
		Date:          date,
		Amount:        req.Amount,
		Description:   req.Description,
		Category:      normCategory(req.Category),
	})
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "внутренняя ошибка")
		return
	}
	d.recalcSelf(r, pid)
	writeJSON(w, http.StatusCreated, map[string]string{"status": "ok"})
}

// DELETE /api/my/expenses/one-off/{id}
func (d Deps) deleteOneOffExpense(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "некорректный id")
		return
	}
	pid := auth.ParticipantID(r.Context())
	ok, err := d.Repo.DeleteOneOffExpense(r.Context(), id, pid)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "внутренняя ошибка")
		return
	}
	if !ok {
		writeErr(w, http.StatusNotFound, "расход не найден")
		return
	}
	d.recalcSelf(r, pid)
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// GET /api/my/operations — лента последних операций (ТЗ V2, п. 6).
func (d Deps) operations(w http.ResponseWriter, r *http.Request) {
	pid := auth.ParticipantID(r.Context())
	limit := 30
	if s := r.URL.Query().Get("limit"); s != "" {
		if n, err := strconv.Atoi(s); err == nil && n > 0 && n <= 100 {
			limit = n
		}
	}
	txs, err := d.Repo.ListRecentTransactions(r.Context(), pid, limit)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "внутренняя ошибка")
		return
	}
	items := make([]map[string]any, len(txs))
	for i, t := range txs {
		items[i] = map[string]any{
			"paid_at": t.PaidAt.Format(time.RFC3339),
			"amount":  t.Amount,
			"type":    t.Type,
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func modelFixed(pid uuid.UUID, desc string, amount decimal.Decimal, due int, category string) models.FixedExpense {
	return models.FixedExpense{
		ParticipantID: pid,
		Description:    desc,
		Amount:         amount,
		DueDayOfMonth:  due,
		Category:       normCategory(category),
	}
}

// recalcSelf — пересчёт участника после его правки расходов (прогноз/индекс
// зависят от расходов). Для группы A пересчёт неприменим — молча пропускаем.
func (d Deps) recalcSelf(r *http.Request, pid uuid.UUID) {
	res, err := d.Svc.Recalculate(r.Context(), pid)
	if err != nil {
		if !errors.Is(err, service.ErrControlGroup) && !errors.Is(err, service.ErrNoData) {
			d.Log.Warn("пересчёт после правки расходов не удался", "participant", pid, "err", err)
		}
		return
	}
	d.notifyAfterRecalc(r, pid, res)
}
