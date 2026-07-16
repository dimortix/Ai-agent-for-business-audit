package service

import (
	"context"
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"alfa-pulse/internal/models"
	"alfa-pulse/internal/repository"
)

// Расширенная аналитика «одним экраном» (как у банковских сервисов аналитики
// бизнеса): сравнение периодов, профиль недели, запас прочности, сценарии,
// денежный календарь, точность модели, бенчмарк внутри пилота, авто-инсайты.

type PeriodStats struct {
	Revenue     decimal.Decimal `json:"revenue"`
	Transaction int             `json:"transactions"`
	AvgCheck    decimal.Decimal `json:"avg_check"`
	ReturnsRate float64         `json:"returns_rate"` // доля возвратов, 0.013 = 1.3%
}

type PeriodComparison struct {
	Days          int         `json:"days"`
	Current       PeriodStats `json:"current"`
	Previous      PeriodStats `json:"previous"`
	RevenueDelta  *float64    `json:"revenue_delta,omitempty"`   // +0.12 = +12%
	TxDelta       *float64    `json:"tx_delta,omitempty"`
	AvgCheckDelta *float64    `json:"avg_check_delta,omitempty"`
}

type WeekdayAvg struct {
	Label      string          `json:"label"` // Пн … Вс
	AvgRevenue decimal.Decimal `json:"avg_revenue"`
}

type HealthPoint struct {
	At    time.Time `json:"at"`
	Index int       `json:"index"`
}

type Scenario struct {
	GapDate *string         `json:"gap_date"` // null — разрыва нет
	Total14 decimal.Decimal `json:"total_14d"`
}

type Scenarios struct {
	Pessimistic Scenario `json:"pessimistic"` // нижняя граница интервала
	Base        Scenario `json:"base"`
	Optimistic  Scenario `json:"optimistic"` // верхняя граница
}

type CalendarEvent struct {
	Date         string          `json:"date"`
	Description  string          `json:"description"`
	Amount       decimal.Decimal `json:"amount"`
	BalanceAfter decimal.Decimal `json:"balance_after"`
	Risk         bool            `json:"risk"` // баланс после платежа ниже «подушки»
}

type Benchmark struct {
	Rank      int      `json:"rank"`
	Total     int      `json:"total"`
	GrowthPct *float64 `json:"growth_pct,omitempty"` // рост выручки 30д, +0.18 = +18%
}

type ProfitPoint struct {
	Date   string          `json:"date"`
	Profit decimal.Decimal `json:"profit"` // net − доля фикс. расходов − разовые расходы дня
}

type BalancePoint struct {
	Date    string          `json:"date"`
	Balance decimal.Decimal `json:"balance"`
}

type Insights struct {
	Period         *PeriodComparison `json:"period,omitempty"`
	WeekdayProfile []WeekdayAvg      `json:"weekday_profile,omitempty"`
	Texts          []string          `json:"insights"`

	// Экономика с учётом расходов (доступна всем группам):
	ProfitSeries       []ProfitPoint    `json:"profit_series,omitempty"`
	MarginPct          *float64         `json:"margin_pct,omitempty"`      // операционная маржа 30д
	BreakEvenDaily     *decimal.Decimal `json:"break_even_daily,omitempty"` // фикс. расходы / 30
	DaysAboveBreakEven *int             `json:"days_above_break_even,omitempty"`

	// только группа B (прогнозные):
	RunwayDays        *int            `json:"runway_days,omitempty"` // 121 = «120+»
	HealthHistory     []HealthPoint   `json:"health_history,omitempty"`
	ForecastMAPE      *float64        `json:"forecast_mape,omitempty"` // 0.07 = 7%
	Scenarios         *Scenarios      `json:"scenarios,omitempty"`
	CashCalendar      []CalendarEvent `json:"cash_calendar,omitempty"`
	Benchmark         *Benchmark      `json:"benchmark,omitempty"`
	BalanceProjection []BalancePoint  `json:"balance_projection,omitempty"` // прогноз денег 30д
}

const runwayCapDays = 120

var weekdayLabels = [...]string{"Вс", "Пн", "Вт", "Ср", "Чт", "Пт", "Сб"} // индекс = time.Weekday

// BuildInsights собирает всю расширенную аналитику участника.
func (s *Service) BuildInsights(ctx context.Context, pid uuid.UUID) (*Insights, error) {
	p, err := s.repo.GetParticipantByID(ctx, pid)
	if err != nil {
		return nil, err
	}

	today := dateOnly(time.Now().UTC())
	metrics60, err := s.repo.GetMetricsRange(ctx, pid, today.AddDate(0, 0, -60), today)
	if err != nil {
		return nil, err
	}

	out := &Insights{Texts: []string{}}
	out.Period = comparePeriods(metrics60, today, 30)
	out.WeekdayProfile = weekdayProfile(metrics60)

	if best := bestWeekday(out.WeekdayProfile); best != "" {
		out.Texts = append(out.Texts, best)
	}
	if t := revenueTrendText(out.Period); t != "" {
		out.Texts = append(out.Texts, t)
	}
	if t := returnsText(out.Period); t != "" {
		out.Texts = append(out.Texts, t)
	}

	// Экономика с учётом расходов: прибыль по дням, маржа, безубыточность.
	monthlyAll, _ := s.repo.MonthlyExpensesTotal(ctx, pid)
	oneOffByDay, _ := s.repo.DailyOneOffTotals(ctx, pid, today.AddDate(0, 0, -30), today)
	s.fillProfitEconomics(out, metrics60, monthlyAll, oneOffByDay, today)

	if p.GroupType != "B" {
		return out, nil // контрольной группе — только фактическая аналитика
	}

	// История индекса
	history, err := s.repo.HealthHistory(ctx, pid, 30)
	if err == nil {
		for _, h := range history {
			out.HealthHistory = append(out.HealthHistory, HealthPoint{At: h.CalculatedAt, Index: *h.HealthIndex})
		}
	}

	// Точность модели: MAPE по «честным» прогнозам (сделанным до наступления даты)
	if pairs, err := s.repo.PastForecastAccuracy(ctx, pid, 14); err == nil {
		if mape := computeMAPE(pairs); mape != nil {
			out.ForecastMAPE = mape
			if *mape < 0.15 {
				out.Texts = append(out.Texts, fmt.Sprintf(
					"Средняя ошибка прогноза — %.0f%%: модели можно доверять.", *mape*100))
			}
		} else {
			out.Texts = append(out.Texts,
				"Модель ежедневно сверяет прогноз с фактом — оценка точности появится после первых суток работы.")
		}
	}

	// Прогноз, сценарии, запас прочности, календарь
	preds, err := s.repo.GetLatestPredictionBatch(ctx, pid)
	if err != nil || len(preds) == 0 {
		return out, nil
	}
	expenses, err := s.repo.ListExpenses(ctx, pid)
	if err != nil {
		return out, nil
	}
	monthly := decimal.Zero
	for _, e := range expenses {
		monthly = monthly.Add(e.Amount)
	}
	fin, err := s.ComputeFinancials(ctx, pid)
	if err != nil {
		return out, nil
	}

	dates := make([]time.Time, len(preds))
	yhat := make([]decimal.Decimal, len(preds))
	lower := make([]decimal.Decimal, len(preds))
	upper := make([]decimal.Decimal, len(preds))
	for i, pr := range preds {
		dates[i] = dateOnly(pr.ForecastDate)
		yhat[i] = pr.PredictedRevenue
		lower[i] = pr.PredictedLower
		upper[i] = pr.PredictedUpper
	}
	lastFact := dates[0].AddDate(0, 0, -1)
	payments := DuePayments(expenses, lastFact, horizonDays)

	out.Scenarios = &Scenarios{
		Pessimistic: buildScenario(fin.CurrentBalance, lower, dates, payments, monthly),
		Base:        buildScenario(fin.CurrentBalance, yhat, dates, payments, monthly),
		Optimistic:  buildScenario(fin.CurrentBalance, upper, dates, payments, monthly),
	}

	avg7 := avgLastNDec(metrics60, 7)
	// разовые расходы тоже участвуют в разрыве/календаре/runway
	future, _ := s.repo.FutureOneOffExpenses(ctx, pid, lastFact, 120)
	allPayments := append(DuePayments(expenses, lastFact, 120), oneOffsAsPayments(future)...)

	runway := RunwayDaysFromPayments(fin.CurrentBalance, yhat, avg7, allPayments, lastFact, monthly)
	out.RunwayDays = &runway
	if runway <= 30 {
		out.Texts = append(out.Texts, fmt.Sprintf(
			"Запаса прочности — около %d дней: держите руку на пульсе платежей.", runway))
	}

	out.CashCalendar = cashCalendarFromPayments(fin.CurrentBalance, yhat, avg7, allPayments, lastFact, monthly, 30)
	out.BalanceProjection = balanceProjection(fin.CurrentBalance, yhat, avg7, allPayments, lastFact, 30)

	// Бенчмарк внутри пилота (обезличенно)
	if growth, err := s.repo.GrowthByGroup(ctx, "B"); err == nil {
		if bm := computeBenchmark(growth, pid); bm != nil {
			out.Benchmark = bm
			if bm.Rank == 1 && bm.Total > 1 {
				out.Texts = append(out.Texts, fmt.Sprintf(
					"Вы №1 по росту выручки среди %d бизнесов пилота 🏆", bm.Total))
			}
		}
	}

	return out, nil
}

// --- чистые функции (покрыты тестами) ---------------------------------------

func comparePeriods(metrics []models.DailyMetric, today time.Time, days int) *PeriodComparison {
	curFrom := today.AddDate(0, 0, -days+1)
	prevFrom := curFrom.AddDate(0, 0, -days)

	var cur, prev PeriodStats
	var curReturns, prevReturns, curRev, prevRev decimal.Decimal
	for _, m := range metrics {
		d := dateOnly(m.Date)
		switch {
		case !d.Before(curFrom):
			cur.Revenue = cur.Revenue.Add(m.Net())
			cur.Transaction += m.TransactionCount
			curReturns = curReturns.Add(m.ReturnAmount)
			curRev = curRev.Add(m.TotalRevenue)
		case !d.Before(prevFrom):
			prev.Revenue = prev.Revenue.Add(m.Net())
			prev.Transaction += m.TransactionCount
			prevReturns = prevReturns.Add(m.ReturnAmount)
			prevRev = prevRev.Add(m.TotalRevenue)
		}
	}
	if cur.Transaction > 0 {
		cur.AvgCheck = curRev.Div(decimal.NewFromInt(int64(cur.Transaction))).Round(2)
	}
	if prev.Transaction > 0 {
		prev.AvgCheck = prevRev.Div(decimal.NewFromInt(int64(prev.Transaction))).Round(2)
	}
	cur.ReturnsRate = rate(curReturns, curRev)
	prev.ReturnsRate = rate(prevReturns, prevRev)

	pc := &PeriodComparison{Days: days, Current: cur, Previous: prev}
	pc.RevenueDelta = deltaPct(cur.Revenue, prev.Revenue)
	if prev.Transaction > 0 {
		d := float64(cur.Transaction-prev.Transaction) / float64(prev.Transaction)
		pc.TxDelta = &d
	}
	pc.AvgCheckDelta = deltaPct(cur.AvgCheck, prev.AvgCheck)
	return pc
}

func weekdayProfile(metrics []models.DailyMetric) []WeekdayAvg {
	sums := map[time.Weekday]decimal.Decimal{}
	counts := map[time.Weekday]int{}
	for _, m := range metrics {
		if m.TransactionCount == 0 {
			continue // нерабочие дни не искажают профиль
		}
		wd := m.Date.Weekday()
		sums[wd] = sums[wd].Add(m.Net())
		counts[wd]++
	}
	// порядок: Пн … Вс
	order := []time.Weekday{time.Monday, time.Tuesday, time.Wednesday, time.Thursday,
		time.Friday, time.Saturday, time.Sunday}
	out := make([]WeekdayAvg, 0, 7)
	for _, wd := range order {
		avg := decimal.Zero
		if counts[wd] > 0 {
			avg = sums[wd].Div(decimal.NewFromInt(int64(counts[wd]))).Round(0)
		}
		out = append(out, WeekdayAvg{Label: weekdayLabels[wd], AvgRevenue: avg})
	}
	return out
}

// RunwayDays — через сколько дней баланс опустится ниже «подушки»
// (0.2 месячных платежей) при базовом прогнозе; дальше 14 дней — среднее 7 дней.
// Возвращает runwayCapDays+1, если в горизонте 120 дней риска нет.
func RunwayDays(balance decimal.Decimal, yhat []decimal.Decimal, avg7 decimal.Decimal,
	expenses []models.FixedExpense, lastFact time.Time, monthly decimal.Decimal) int {
	return RunwayDaysFromPayments(balance, yhat, avg7,
		DuePayments(expenses, lastFact, runwayCapDays), lastFact, monthly)
}

// RunwayDaysFromPayments — как RunwayDays, но платежи уже развёрнуты
// (фиксированные + разовые).
func RunwayDaysFromPayments(balance decimal.Decimal, yhat []decimal.Decimal, avg7 decimal.Decimal,
	payments []Payment, lastFact time.Time, monthly decimal.Decimal) int {

	if monthly.IsZero() {
		return runwayCapDays + 1
	}
	threshold := monthly.Mul(dec02)
	byDay := paymentsByDay(payments)

	bal := balance
	for i := 0; i < runwayCapDays; i++ {
		day := lastFact.AddDate(0, 0, i+1)
		income := avg7
		if i < len(yhat) {
			income = yhat[i]
		}
		bal = bal.Add(income).Sub(byDay[day.Format("2006-01-02")])
		if bal.LessThan(threshold) {
			return i + 1
		}
	}
	return runwayCapDays + 1
}

// balanceProjection — прогноз денег на счёте на days дней вперёд (график накоплений).
func balanceProjection(balance decimal.Decimal, yhat []decimal.Decimal, avg7 decimal.Decimal,
	payments []Payment, lastFact time.Time, days int) []BalancePoint {

	byDay := paymentsByDay(payments)
	out := make([]BalancePoint, 0, days)
	bal := balance
	for i := 0; i < days; i++ {
		day := lastFact.AddDate(0, 0, i+1)
		income := avg7
		if i < len(yhat) {
			income = yhat[i]
		}
		bal = bal.Add(income).Sub(byDay[day.Format("2006-01-02")])
		out = append(out, BalancePoint{Date: day.Format("2006-01-02"), Balance: bal.Round(0)})
	}
	return out
}

func paymentsByDay(payments []Payment) map[string]decimal.Decimal {
	byDay := map[string]decimal.Decimal{}
	for _, p := range payments {
		k := p.Date.Format("2006-01-02")
		byDay[k] = byDay[k].Add(p.Amount)
	}
	return byDay
}

// fillProfitEconomics считает прибыль по дням, операционную маржу и
// точку безубыточности (все группы, факт — расходы известны).
func (s *Service) fillProfitEconomics(out *Insights, metrics []models.DailyMetric,
	monthly decimal.Decimal, oneOffByDay map[string]decimal.Decimal, today time.Time) {

	dailyFixed := decimal.Zero
	if monthly.IsPositive() {
		dailyFixed = monthly.Div(dec30)
	}
	from := today.AddDate(0, 0, -29)

	var series []ProfitPoint
	var revenue30, profit30 decimal.Decimal
	daysAbove := 0
	for _, m := range metrics {
		d := dateOnly(m.Date)
		if d.Before(from) {
			continue
		}
		key := d.Format("2006-01-02")
		profit := m.Net().Sub(dailyFixed).Sub(oneOffByDay[key])
		series = append(series, ProfitPoint{Date: key, Profit: profit.Round(0)})
		revenue30 = revenue30.Add(m.Net())
		profit30 = profit30.Add(profit)
		if m.Net().GreaterThanOrEqual(dailyFixed) {
			daysAbove++
		}
	}
	out.ProfitSeries = series

	if dailyFixed.IsPositive() {
		be := dailyFixed.Round(0)
		out.BreakEvenDaily = &be
		da := daysAbove
		out.DaysAboveBreakEven = &da
	}
	if revenue30.IsPositive() {
		m, _ := profit30.Div(revenue30).Float64()
		out.MarginPct = &m
		if m > 0 {
			out.Texts = append(out.Texts, fmt.Sprintf(
				"Операционная маржа за 30 дней — %.0f%%: после обязательных расходов бизнес в плюсе.", m*100))
		} else {
			out.Texts = append(out.Texts, fmt.Sprintf(
				"Операционная маржа за 30 дней отрицательная (%.0f%%): расходы превышают выручку — пересмотрите траты.", m*100))
		}
	}
}

func cashCalendar(balance decimal.Decimal, yhat []decimal.Decimal, avg7 decimal.Decimal,
	expenses []models.FixedExpense, lastFact time.Time, monthly decimal.Decimal, days int) []CalendarEvent {
	return cashCalendarFromPayments(balance, yhat, avg7,
		DuePayments(expenses, lastFact, days), lastFact, monthly, days)
}

func cashCalendarFromPayments(balance decimal.Decimal, yhat []decimal.Decimal, avg7 decimal.Decimal,
	payments []Payment, lastFact time.Time, monthly decimal.Decimal, days int) []CalendarEvent {

	threshold := monthly.Mul(dec02)
	byDay := map[string][]Payment{}
	for _, p := range payments {
		k := p.Date.Format("2006-01-02")
		byDay[k] = append(byDay[k], p)
	}

	var out []CalendarEvent
	bal := balance
	for i := 0; i < days; i++ {
		day := lastFact.AddDate(0, 0, i+1)
		income := avg7
		if i < len(yhat) {
			income = yhat[i]
		}
		bal = bal.Add(income)
		for _, p := range byDay[day.Format("2006-01-02")] {
			bal = bal.Sub(p.Amount)
			out = append(out, CalendarEvent{
				Date:         day.Format("2006-01-02"),
				Description:  p.Description,
				Amount:       p.Amount,
				BalanceAfter: bal.Round(0),
				Risk:         bal.LessThan(threshold),
			})
		}
	}
	return out
}

func buildScenario(balance decimal.Decimal, series []decimal.Decimal, dates []time.Time,
	payments []Payment, monthly decimal.Decimal) Scenario {

	gap, _ := DetectCashGap(balance, series, dates, payments, monthly)
	total := decimal.Zero
	for _, v := range series {
		total = total.Add(v)
	}
	sc := Scenario{Total14: total.Round(0)}
	if gap != nil {
		s := gap.Format("2006-01-02")
		sc.GapDate = &s
	}
	return sc
}

// computeMAPE — средняя абсолютная процентная ошибка прогноза.
// Меньше 3 пар «прогноз/факт» — оценка ненадёжна, возвращаем nil.
func computeMAPE(pairs []repository.ForecastFactPair) *float64 {
	sum, n := 0.0, 0
	for _, p := range pairs {
		fact, _ := p.Fact.Float64()
		pred, _ := p.Predicted.Float64()
		if fact > 0 {
			sum += math.Abs(fact-pred) / fact
			n++
		}
	}
	if n < 3 {
		return nil
	}
	m := sum / float64(n)
	return &m
}

// computeBenchmark — место участника по росту выручки 30д/30д среди группы.
func computeBenchmark(growth []repository.ParticipantGrowth, pid uuid.UUID) *Benchmark {
	type entry struct {
		id uuid.UUID
		g  float64
	}
	var list []entry
	for _, x := range growth {
		cur, _ := x.Current30.Float64()
		prev, _ := x.Previous30.Float64()
		if prev > 0 {
			list = append(list, entry{x.ParticipantID, (cur - prev) / prev})
		}
	}
	if len(list) < 2 {
		return nil // сравнивать не с кем
	}
	sort.Slice(list, func(i, j int) bool { return list[i].g > list[j].g })
	for i, e := range list {
		if e.id == pid {
			g := e.g
			return &Benchmark{Rank: i + 1, Total: len(list), GrowthPct: &g}
		}
	}
	return nil
}

// --- вспомогательные ---------------------------------------------------------

func rate(part, total decimal.Decimal) float64 {
	if !total.IsPositive() {
		return 0
	}
	f, _ := part.Div(total).Float64()
	return f
}

func deltaPct(cur, prev decimal.Decimal) *float64 {
	if !prev.IsPositive() {
		return nil
	}
	f, _ := cur.Sub(prev).Div(prev).Float64()
	return &f
}

func avgLastNDec(metrics []models.DailyMetric, n int) decimal.Decimal {
	if len(metrics) == 0 {
		return decimal.Zero
	}
	if len(metrics) < n {
		n = len(metrics)
	}
	sum := decimal.Zero
	for _, m := range metrics[len(metrics)-n:] {
		sum = sum.Add(m.Net())
	}
	return sum.Div(decimal.NewFromInt(int64(n)))
}

func bestWeekday(profile []WeekdayAvg) string {
	if len(profile) == 0 {
		return ""
	}
	total := decimal.Zero
	nonZero := 0
	best := profile[0]
	for _, w := range profile {
		total = total.Add(w.AvgRevenue)
		if w.AvgRevenue.IsPositive() {
			nonZero++
		}
		if w.AvgRevenue.GreaterThan(best.AvgRevenue) {
			best = w
		}
	}
	if nonZero < 3 || !best.AvgRevenue.IsPositive() {
		return ""
	}
	mean := total.Div(decimal.NewFromInt(int64(nonZero)))
	if !mean.IsPositive() {
		return ""
	}
	upliftDec := best.AvgRevenue.Sub(mean).Div(mean).Mul(dec100).Round(0)
	uplift := upliftDec.IntPart()
	if uplift < 10 {
		return ""
	}
	return fmt.Sprintf("%s — ваш сильнейший день: в среднем +%d%% к обычной выручке.", fullWeekday(best.Label), uplift)
}

func fullWeekday(short string) string {
	m := map[string]string{
		"Пн": "Понедельник", "Вт": "Вторник", "Ср": "Среда", "Чт": "Четверг",
		"Пт": "Пятница", "Сб": "Суббота", "Вс": "Воскресенье",
	}
	if f, ok := m[short]; ok {
		return f
	}
	return short
}

func revenueTrendText(p *PeriodComparison) string {
	if p == nil || p.RevenueDelta == nil {
		return ""
	}
	d := *p.RevenueDelta
	switch {
	case d >= 0.1:
		return fmt.Sprintf("Выручка за %d дней выросла на %.0f%% к прошлому периоду — отличный темп!", p.Days, d*100)
	case d <= -0.1:
		return fmt.Sprintf("Выручка за %d дней снизилась на %.0f%% к прошлому периоду.", p.Days, -d*100)
	}
	return ""
}

func returnsText(p *PeriodComparison) string {
	if p == nil {
		return ""
	}
	r := p.Current.ReturnsRate
	if r >= 0.015 {
		return fmt.Sprintf("Доля возвратов — %.1f%%: стоит разобраться в причинах.", r*100)
	}
	return ""
}

