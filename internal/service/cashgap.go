package service

import (
	"time"

	"github.com/shopspring/decimal"

	"alfa-pulse/internal/models"
)

// Обнаружение кассового разрыва (ТЗ, п. 5): день i в горизонте прогноза,
// когда расчётный баланс опускается ниже «подушки» 0.2 · monthly_expenses.

type Payment struct {
	Date        time.Time
	Amount      decimal.Decimal
	Description string
}

// DuePayments разворачивает фиксированные расходы в конкретные даты платежей
// в окне (after, after+days]. День 31 в коротком месяце — последний день месяца.
func DuePayments(expenses []models.FixedExpense, after time.Time, days int) []Payment {
	var out []Payment
	for i := 1; i <= days; i++ {
		d := after.AddDate(0, 0, i)
		lastDay := lastDayOfMonth(d.Year(), d.Month())
		for _, e := range expenses {
			due := e.DueDayOfMonth
			if due > lastDay {
				due = lastDay
			}
			if d.Day() == due {
				out = append(out, Payment{Date: d, Amount: e.Amount, Description: e.Description})
			}
		}
	}
	return out
}

// DetectCashGap идёт по дням прогноза, прибавляя прогнозную выручку и вычитая
// платежи дня. Первый день с балансом ниже порога — дата разрыва.
// Возвращает nil, если разрыва в горизонте нет; amount — сколько не хватает
// до безопасного уровня в день разрыва.
func DetectCashGap(balance decimal.Decimal, yhat []decimal.Decimal, dates []time.Time,
	payments []Payment, monthlyExpenses decimal.Decimal) (*time.Time, decimal.Decimal) {

	threshold := monthlyExpenses.Mul(dec02)

	paymentsByDay := make(map[string]decimal.Decimal, len(payments))
	for _, p := range payments {
		key := p.Date.Format("2006-01-02")
		paymentsByDay[key] = paymentsByDay[key].Add(p.Amount)
	}

	bal := balance
	for i := range yhat {
		if i >= len(dates) {
			break
		}
		bal = bal.Add(yhat[i])
		bal = bal.Sub(paymentsByDay[dates[i].Format("2006-01-02")])
		if bal.LessThan(threshold) {
			d := dates[i]
			return &d, threshold.Sub(bal)
		}
	}
	return nil, decimal.Zero
}

func lastDayOfMonth(year int, month time.Month) int {
	return time.Date(year, month+1, 0, 0, 0, 0, 0, time.UTC).Day()
}
