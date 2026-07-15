package service

import "github.com/shopspring/decimal"

// Расчёт Индекса здоровья бизнеса (ИЖБ) по формуле ТЗ:
//
//	health_index = 100 · (current_balance + forecasted_revenue_30d) / (monthly_expenses · 1.2)
//
// с ограничением в диапазон [1, 100].

var (
	dec30  = decimal.NewFromInt(30)
	dec16  = decimal.NewFromInt(16)
	dec100 = decimal.NewFromInt(100)
	dec12  = decimal.NewFromFloat(1.2)
	dec02  = decimal.NewFromFloat(0.2)
)

// CurrentBalance оценивает баланс счёта: накопленная чистая выручка минус
// фиксированные расходы, пропорционально прошедшим дням истории
// (реальных остатков в MVP нет — см. ТЗ, п. 5).
func CurrentBalance(netTotal, monthlyExpenses decimal.Decimal, historyDays int) decimal.Decimal {
	if historyDays <= 0 {
		return netTotal
	}
	spent := monthlyExpenses.Mul(decimal.NewFromInt(int64(historyDays))).Div(dec30)
	return netTotal.Sub(spent)
}

// Forecast30 — прогнозная выручка на 30 дней: сумма 14-дневного прогноза,
// оставшиеся 16 дней экстраполируются средним фактом последних 7 дней.
func Forecast30(yhat []decimal.Decimal, avgLast7 decimal.Decimal) decimal.Decimal {
	sum := decimal.Zero
	for _, v := range yhat {
		sum = sum.Add(v)
	}
	return sum.Add(avgLast7.Mul(dec16))
}

// HealthIndex — итоговый индекс 1..100.
func HealthIndex(balance, forecast30, monthlyExpenses decimal.Decimal) int {
	numerator := balance.Add(forecast30)
	if monthlyExpenses.IsZero() {
		// Обязательных платежей нет: банкротить нечем.
		if numerator.IsPositive() {
			return 100
		}
		return 1
	}
	idx := dec100.Mul(numerator).Div(monthlyExpenses.Mul(dec12))
	n := int(idx.Round(0).IntPart())
	if n < 1 {
		return 1
	}
	if n > 100 {
		return 100
	}
	return n
}

// AdjustHealthIndex — поправки поверх базовой формулы ТЗ (наше улучшение):
// базовая формула сравнивает месячный запас денег с месячными платежами и
// легко даёт 100 даже при кассовом разрыве через неделю. Штрафуем индекс за
// краткосрочные риски, чтобы он честно отражал состояние:
//   - разрыв в горизонте 14 дней  → −25;
//   - выручка недели упала >15% к предыдущей → −10.
func AdjustHealthIndex(base int, gapWithinHorizon, revenueTrendDrop bool) int {
	idx := base
	if gapWithinHorizon {
		idx -= 25
	}
	if revenueTrendDrop {
		idx -= 10
	}
	if idx < 1 {
		return 1
	}
	if idx > 100 {
		return 100
	}
	return idx
}

// WeeklyTrendDrop: средняя выручка последних 7 дней упала более чем на 15%
// к предыдущим 7 дням. Меньше 14 точек — считаем, что спада нет.
func WeeklyTrendDrop(values []float64) bool {
	if len(values) < 14 {
		return false
	}
	last := values[len(values)-7:]
	prev := values[len(values)-14 : len(values)-7]
	sumLast, sumPrev := 0.0, 0.0
	for i := 0; i < 7; i++ {
		sumLast += last[i]
		sumPrev += prev[i]
	}
	return sumPrev > 0 && sumLast < sumPrev*0.85
}

// HealthStatus — зона индекса для UI/уведомлений.
func HealthStatus(index int) string {
	switch {
	case index < 40:
		return "critical"
	case index < 70:
		return "warning"
	default:
		return "ok"
	}
}
