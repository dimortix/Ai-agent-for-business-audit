package handler

import (
	"encoding/csv"
	"fmt"
	"net/http"
	"time"

	"github.com/shopspring/decimal"

	"alfa-pulse/internal/auth"
)

// GET /api/report/monthly?month=2026-07 — детализированный отчёт за месяц (ТЗ V2, п. 1).
// Поступления, расходы по категориям, средний чек, число операций, итоговый
// баланс, отклонение факта от прогноза. Для бухгалтерии и заявки на кредит.
func (d Deps) monthlyReport(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	pid := auth.ParticipantID(ctx)

	// Месяц: параметр month=YYYY-MM или текущий.
	now := time.Now().UTC()
	from := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	if s := r.URL.Query().Get("month"); s != "" {
		if t, err := time.Parse("2006-01", s); err == nil {
			from = t
		}
	}
	to := from.AddDate(0, 1, -1) // последний день месяца

	metrics, err := d.Repo.GetMetricsRange(ctx, pid, from, to)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "внутренняя ошибка")
		return
	}

	// Агрегаты по метрикам.
	var revenue, returns decimal.Decimal
	txCount := 0
	for _, m := range metrics {
		revenue = revenue.Add(m.TotalRevenue)
		returns = returns.Add(m.ReturnAmount)
		txCount += m.TransactionCount
	}
	net := revenue.Sub(returns)
	avgCheck := decimal.Zero
	if txCount > 0 {
		avgCheck = revenue.Div(decimal.NewFromInt(int64(txCount))).Round(2)
	}

	// Расходы по категориям: регулярные (за месяц) + разовые (в месяце).
	fixed, _ := d.Repo.ListExpenses(ctx, pid)
	byCat := map[string]decimal.Decimal{}
	fixedTotal := decimal.Zero
	for _, e := range fixed {
		byCat[e.Category] = byCat[e.Category].Add(e.Amount)
		fixedTotal = fixedTotal.Add(e.Amount)
	}
	oneOffCat, _ := d.Repo.MonthOneOffByCategory(ctx, pid, from, to)
	oneOffTotal := decimal.Zero
	for cat, sum := range oneOffCat {
		byCat[cat] = byCat[cat].Add(sum)
		oneOffTotal = oneOffTotal.Add(sum)
	}
	expensesTotal := fixedTotal.Add(oneOffTotal)
	balance := net.Sub(expensesTotal)

	// Отклонение от прогноза: факт vs предсказание за дни месяца.
	var forecastErr *decimal.Decimal
	if pairs, err := d.Repo.PastForecastAccuracy(ctx, pid, 60); err == nil && len(pairs) > 0 {
		var predSum, factSum decimal.Decimal
		for _, p := range pairs {
			pd := p.Date
			if !pd.Before(from) && !pd.After(to) {
				predSum = predSum.Add(p.Predicted)
				factSum = factSum.Add(p.Fact)
			}
		}
		if predSum.IsPositive() {
			dev := factSum.Sub(predSum)
			forecastErr = &dev
		}
	}

	filename := fmt.Sprintf("alfa-pulse-report_%s.csv", from.Format("2006-01"))
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="`+filename+`"`)
	_, _ = w.Write([]byte{0xEF, 0xBB, 0xBF}) // BOM для Excel

	cw := csv.NewWriter(w)
	_ = cw.Write([]string{"Показатель", "Значение, ₽"})
	_ = cw.Write([]string{"Месяц", from.Format("2006-01")})
	_ = cw.Write([]string{"Поступления (оборот)", revenue.StringFixed(2)})
	_ = cw.Write([]string{"Возвраты", returns.StringFixed(2)})
	_ = cw.Write([]string{"Чистая выручка", net.StringFixed(2)})
	_ = cw.Write([]string{"Количество операций", fmt.Sprint(txCount)})
	_ = cw.Write([]string{"Средний чек", avgCheck.StringFixed(2)})
	_ = cw.Write([]string{"", ""})
	_ = cw.Write([]string{"Расходы по категориям", ""})
	for _, cat := range []string{"rent", "salary", "supplies", "taxes", "loan", "utilities", "other"} {
		if v, ok := byCat[cat]; ok && v.IsPositive() {
			_ = cw.Write([]string{categoryRU(cat), v.StringFixed(2)})
		}
	}
	_ = cw.Write([]string{"Итого расходы", expensesTotal.StringFixed(2)})
	_ = cw.Write([]string{"", ""})
	_ = cw.Write([]string{"Итоговый баланс (выручка − расходы)", balance.StringFixed(2)})
	if forecastErr != nil {
		_ = cw.Write([]string{"Отклонение факта от прогноза", forecastErr.StringFixed(2)})
	}
	cw.Flush()
}

func categoryRU(cat string) string {
	switch cat {
	case "rent":
		return "Аренда"
	case "salary":
		return "Зарплаты"
	case "supplies":
		return "Закупки"
	case "taxes":
		return "Налоги"
	case "loan":
		return "Кредиты"
	case "utilities":
		return "Коммуналка"
	default:
		return "Прочее"
	}
}
