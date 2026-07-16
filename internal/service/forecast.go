package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"alfa-pulse/internal/forecast"
	"alfa-pulse/internal/models"
	"alfa-pulse/internal/repository"
)

const horizonDays = 14

var (
	// ErrControlGroup — участник группы A: прогнозы и советы не считаем (контроль пилота).
	ErrControlGroup = errors.New("участник контрольной группы A: прогноз не рассчитывается")
	// ErrNoData — нет ни одного дня метрик.
	ErrNoData = errors.New("нет данных о транзакциях: сначала импортируйте CSV")
)

type Service struct {
	repo *repository.Repository
	ml   *MLClient
	llm  *LLMClient
	log  *slog.Logger
}

func New(repo *repository.Repository, ml *MLClient, log *slog.Logger) *Service {
	return &Service{repo: repo, ml: ml, log: log}
}

// WithLLM подключает LLM-модуль для AI-советов (опционально).
func (s *Service) WithLLM(llm *LLMClient) *Service {
	s.llm = llm
	return s
}

func (s *Service) Repo() *repository.Repository { return s.repo }

// RecalcResult — итог пересчёта для участника.
type RecalcResult struct {
	ParticipantID uuid.UUID       `json:"participant_id"`
	HealthIndex   int             `json:"health_index"`
	CashGapDate   *time.Time      `json:"-"`
	CashGapAmount decimal.Decimal `json:"cash_gap_amount"`
	ModelUsed     string          `json:"model_used"`
	NewAdvice     []string        `json:"new_advice"` // коды созданных рекомендаций
}

// Recalculate — полный цикл для участника группы B:
// ряд 90 дней → прогноз (ML или fallback HW) → ИЖБ → кассовый разрыв →
// правила рекомендаций → сохранение батча predictions.
func (s *Service) Recalculate(ctx context.Context, pid uuid.UUID) (*RecalcResult, error) {
	p, err := s.repo.GetParticipantByID(ctx, pid)
	if err != nil {
		return nil, err
	}
	if p.GroupType != "B" {
		return nil, ErrControlGroup
	}

	totals, err := s.repo.GetMetricTotals(ctx, pid)
	if err != nil {
		return nil, err
	}
	if totals.LastDate == nil {
		return nil, ErrNoData
	}
	lastDate := dateOnly(*totals.LastDate)

	// Ряд: последние ≤90 дней, пропуски заполняем нулями.
	from := lastDate.AddDate(0, 0, -89)
	if first := dateOnly(*totals.FirstDate); first.After(from) {
		from = first
	}
	metrics, err := s.repo.GetMetricsRange(ctx, pid, from, lastDate)
	if err != nil {
		return nil, err
	}
	dates, values := fillSeries(metrics, from, lastDate)

	// Прогноз: сначала ML-сервис, при любой ошибке — Хольт-Винтерс.
	yhat, lower, upper, modelUsed := s.forecastSeries(ctx, dates, values)

	forecastDates := make([]time.Time, horizonDays)
	for i := range forecastDates {
		forecastDates[i] = lastDate.AddDate(0, 0, i+1)
	}

	// Финансовые показатели.
	expenses, err := s.repo.ListExpenses(ctx, pid)
	if err != nil {
		return nil, err
	}
	monthly := decimal.Zero
	for _, e := range expenses {
		monthly = monthly.Add(e.Amount)
	}
	historyDays := int(lastDate.Sub(dateOnly(*totals.FirstDate)).Hours()/24) + 1
	balance := CurrentBalance(totals.NetTotal, monthly, historyDays)

	// Разовые расходы: прошлые уменьшают баланс, будущие — платежи в горизонте.
	if paidOneOffs, err := s.repo.SumOneOffExpenses(ctx, pid, lastDate); err == nil {
		balance = balance.Sub(paidOneOffs)
	}
	payments := DuePayments(expenses, lastDate, horizonDays)
	if future, err := s.repo.FutureOneOffExpenses(ctx, pid, lastDate, horizonDays); err == nil {
		payments = append(payments, oneOffsAsPayments(future)...)
	}

	avg7 := avgLastN(values, 7)
	f30 := Forecast30(yhat, avg7)

	gapDate, gapAmount := DetectCashGap(balance, yhat, forecastDates, payments, monthly)

	// Базовая формула ТЗ + штрафы за краткосрочные риски (см. health.go).
	index := AdjustHealthIndex(
		HealthIndex(balance, f30, monthly),
		gapDate != nil,
		WeeklyTrendDrop(values),
	)

	// Сохраняем батч прогноза.
	calcAt := time.Now().UTC()
	preds := make([]models.Prediction, horizonDays)
	for i := 0; i < horizonDays; i++ {
		idx := index
		preds[i] = models.Prediction{
			ParticipantID:    pid,
			CalculatedAt:     calcAt,
			ForecastDate:     forecastDates[i],
			PredictedRevenue: yhat[i],
			PredictedLower:   lower[i],
			PredictedUpper:   upper[i],
			ModelUsed:        modelUsed,
			HealthIndex:      &idx,
			CashGapDate:      gapDate,
		}
	}
	if err := s.repo.InsertPredictionBatch(ctx, preds); err != nil {
		return nil, fmt.Errorf("сохранение прогноза: %w", err)
	}

	// Правила: генерируем советы с дедупликацией за 3 дня.
	newAdvice, err := s.applyRules(ctx, pid, gapDate, gapAmount)
	if err != nil {
		s.log.Warn("не удалось создать рекомендации", "participant", pid, "err", err)
	}

	// AI-советы от LLM (если подключён): свежий взгляд поверх правил, раз в сутки.
	if aiCodes := s.applyAIAdvice(ctx, p, index, monthly, gapDate); len(aiCodes) > 0 {
		newAdvice = append(newAdvice, aiCodes...)
	}

	s.log.Info("пересчёт завершён",
		"participant", pid, "index", index, "model", modelUsed,
		"cash_gap", gapDate, "new_advice", newAdvice)

	return &RecalcResult{
		ParticipantID: pid,
		HealthIndex:   index,
		CashGapDate:   gapDate,
		CashGapAmount: gapAmount,
		ModelUsed:     modelUsed,
		NewAdvice:     newAdvice,
	}, nil
}

// forecastSeries возвращает прогноз в decimal и имя сработавшей модели.
func (s *Service) forecastSeries(ctx context.Context, dates []time.Time, values []float64) (yhat, lower, upper []decimal.Decimal, model string) {
	if points, err := s.ml.Forecast(ctx, dates, values, horizonDays); err == nil {
		yhat = make([]decimal.Decimal, horizonDays)
		lower = make([]decimal.Decimal, horizonDays)
		upper = make([]decimal.Decimal, horizonDays)
		for i, pt := range points {
			yhat[i] = clampMoney(pt.YHat)
			lower[i] = clampMoney(pt.Lower)
			upper[i] = clampMoney(pt.Upper)
		}
		return yhat, lower, upper, "prophet"
	} else {
		s.log.Warn("ML-сервис недоступен, включаю fallback Хольта-Винтерса", "err", err)
	}

	res := forecast.HoltWinters(values, horizonDays)
	yhat = make([]decimal.Decimal, horizonDays)
	lower = make([]decimal.Decimal, horizonDays)
	upper = make([]decimal.Decimal, horizonDays)
	for i := 0; i < horizonDays; i++ {
		yhat[i] = clampMoney(res.YHat[i])
		lower[i] = clampMoney(res.Lower[i])
		upper[i] = clampMoney(res.Upper[i])
	}
	return yhat, lower, upper, "hw"
}

func (s *Service) applyRules(ctx context.Context, pid uuid.UUID, gapDate *time.Time, gapAmount decimal.Decimal) ([]string, error) {
	last14, err := s.repo.GetLastMetrics(ctx, pid, 14)
	if err != nil {
		return nil, err
	}
	hits := EvaluateRules(last14, gapDate, gapAmount)

	created := []string{}
	dedupSince := time.Now().Add(-72 * time.Hour)
	for _, h := range hits {
		exists, err := s.repo.HasRecentRecommendation(ctx, pid, h.Code, dedupSince)
		if err != nil {
			return created, err
		}
		if exists {
			continue
		}
		if err := s.repo.InsertRecommendation(ctx, pid, h.Code, h.Message); err != nil {
			return created, err
		}
		created = append(created, h.Code)
	}
	return created, nil
}

// applyAIAdvice запрашивает у LLM свежие советы (не чаще раза в сутки на участника)
// и сохраняет их как рекомендации с пометкой AI_ADVICE. Ошибки не критичны —
// система тихо остаётся на правилах.
func (s *Service) applyAIAdvice(ctx context.Context, p *models.Participant, index int,
	monthly decimal.Decimal, gapDate *time.Time) []string {

	if s.llm == nil || !s.llm.Enabled() {
		return nil
	}
	dedupSince := time.Now().Add(-24 * time.Hour)
	if exists, err := s.repo.HasRecentRecommendation(ctx, p.ID, llmAdviceRule, dedupSince); err != nil || exists {
		return nil
	}

	last14, err := s.repo.GetLastMetrics(ctx, p.ID, 14)
	if err != nil {
		return nil
	}
	metrics60, _ := s.repo.GetMetricsRange(ctx, p.ID,
		dateOnly(time.Now().UTC()).AddDate(0, 0, -60), dateOnly(time.Now().UTC()))
	period := comparePeriods(metrics60, dateOnly(time.Now().UTC()), 30)
	bestDay := ""
	if profile := weekdayProfile(metrics60); len(profile) > 0 {
		if txt := bestWeekday(profile); txt != "" {
			bestDay = fullWeekday(profile[0].Label) // грубо; текст всё равно в промпте
		}
	}

	ac := buildAdviceContext(p, index, monthly, last14, gapDate, period, bestDay)
	tips, err := s.llm.GenerateAdvice(ctx, ac)
	if err != nil {
		s.log.Warn("LLM-советы недоступны, остаюсь на правилах", "participant", p.ID, "err", err)
		return nil
	}
	for _, tip := range tips {
		if err := s.repo.InsertRecommendation(ctx, p.ID, llmAdviceRule, tip); err != nil {
			s.log.Warn("не удалось сохранить AI-совет", "err", err)
		}
	}
	if len(tips) > 0 {
		s.log.Info("AI-советы сгенерированы", "participant", p.ID, "count", len(tips))
		return []string{llmAdviceRule}
	}
	return nil
}

// Financials — текущие финансовые показатели для дашборда.
type Financials struct {
	CurrentBalance  decimal.Decimal
	MonthlyExpenses decimal.Decimal
}

func (s *Service) ComputeFinancials(ctx context.Context, pid uuid.UUID) (*Financials, error) {
	totals, err := s.repo.GetMetricTotals(ctx, pid)
	if err != nil {
		return nil, err
	}
	monthly, err := s.repo.MonthlyExpensesTotal(ctx, pid)
	if err != nil {
		return nil, err
	}
	f := &Financials{MonthlyExpenses: monthly, CurrentBalance: totals.NetTotal}
	if totals.FirstDate != nil && totals.LastDate != nil {
		days := int(dateOnly(*totals.LastDate).Sub(dateOnly(*totals.FirstDate)).Hours()/24) + 1
		f.CurrentBalance = CurrentBalance(totals.NetTotal, monthly, days)
	}
	if paidOneOffs, err := s.repo.SumOneOffExpenses(ctx, pid, dateOnly(time.Now().UTC())); err == nil {
		f.CurrentBalance = f.CurrentBalance.Sub(paidOneOffs)
	}
	return f, nil
}

// oneOffsAsPayments — будущие разовые расходы в формате платежей календаря/разрыва.
func oneOffsAsPayments(oneOffs []repository.OneOffExpense) []Payment {
	out := make([]Payment, len(oneOffs))
	for i, o := range oneOffs {
		out[i] = Payment{Date: dateOnly(o.Date), Amount: o.Amount, Description: o.Description}
	}
	return out
}

// fillSeries превращает метрики в непрерывный дневной ряд (пропуски → 0).
func fillSeries(metrics []models.DailyMetric, from, to time.Time) ([]time.Time, []float64) {
	byDate := make(map[string]decimal.Decimal, len(metrics))
	for _, m := range metrics {
		byDate[dateOnly(m.Date).Format("2006-01-02")] = m.Net()
	}
	var dates []time.Time
	var values []float64
	for d := from; !d.After(to); d = d.AddDate(0, 0, 1) {
		dates = append(dates, d)
		v, _ := byDate[d.Format("2006-01-02")]
		f, _ := v.Float64()
		values = append(values, f)
	}
	return dates, values
}

func avgLastN(values []float64, n int) decimal.Decimal {
	if len(values) == 0 {
		return decimal.Zero
	}
	if len(values) < n {
		n = len(values)
	}
	sum := 0.0
	for _, v := range values[len(values)-n:] {
		sum += v
	}
	return decimal.NewFromFloat(sum / float64(n))
}

func clampMoney(v float64) decimal.Decimal {
	if v < 0 {
		v = 0
	}
	return decimal.NewFromFloat(v).Round(2)
}

func dateOnly(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}
