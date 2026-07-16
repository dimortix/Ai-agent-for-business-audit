package service

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"alfa-pulse/internal/models"
	"alfa-pulse/internal/repository"
)

func TestComparePeriods(t *testing.T) {
	today := day(2026, 7, 15)
	var metrics []models.DailyMetric
	// предыдущие 30 дней: по 10 000, текущие 30: по 12 000 (+20%)
	for i := 59; i >= 0; i-- {
		rev := 10_000.0
		if i < 30 {
			rev = 12_000
		}
		metrics = append(metrics, metric(-i, rev, 20, rev/20))
	}
	// metric() создаёт даты от 2026-07-01; пересоберём с нужными датами
	for i := range metrics {
		metrics[i].Date = today.AddDate(0, 0, -(59 - i))
	}

	p := comparePeriods(metrics, today, 30)
	require.NotNil(t, p.RevenueDelta)
	assert.InDelta(t, 0.20, *p.RevenueDelta, 0.01)
	assert.Equal(t, 600, p.Current.Transaction)
}

func TestWeekdayProfileOrdersMonFirst(t *testing.T) {
	var metrics []models.DailyMetric
	base := day(2026, 7, 6) // понедельник
	for i := 0; i < 28; i++ {
		d := base.AddDate(0, 0, i)
		rev := 10_000.0
		if d.Weekday() == time.Friday {
			rev = 20_000 // пятница — пик
		}
		metrics = append(metrics, models.DailyMetric{
			Date: d, TotalRevenue: decimal.NewFromFloat(rev), TransactionCount: 30,
		})
	}
	profile := weekdayProfile(metrics)
	require.Len(t, profile, 7)
	assert.Equal(t, "Пн", profile[0].Label)
	assert.Equal(t, "Пт", profile[4].Label)
	assert.True(t, profile[4].AvgRevenue.GreaterThan(profile[0].AvgRevenue))

	text := bestWeekday(profile)
	assert.Contains(t, text, "Пятница")
}

func TestRunwayDays(t *testing.T) {
	lastFact := day(2026, 7, 14)
	yhat := make([]decimal.Decimal, 14)
	for i := range yhat {
		yhat[i] = d(5_000)
	}
	expenses := []models.FixedExpense{
		{Description: "Аренда", Amount: d(150_000), DueDayOfMonth: 20}, // через 6 дней
	}
	monthly := d(300_000) // порог 60 000

	// старт 100к, +5к/день, 20-го −150к → риск на 6-й день
	runway := RunwayDays(d(100_000), yhat, d(5_000), expenses, lastFact, monthly)
	assert.Equal(t, 6, runway)

	// без платежей и с запасом — риска нет в горизонте
	safe := RunwayDays(d(1_000_000), yhat, d(5_000), nil, lastFact, d(10_000))
	assert.Equal(t, runwayCapDays+1, safe)

	// нет обязательных платежей вовсе
	assert.Equal(t, runwayCapDays+1, RunwayDays(d(0), yhat, d(0), nil, lastFact, decimal.Zero))
}

func TestComputeMAPE(t *testing.T) {
	pairs := []repository.ForecastFactPair{
		{Fact: d(100), Predicted: d(90)},  // 10%
		{Fact: d(200), Predicted: d(220)}, // 10%
		{Fact: d(100), Predicted: d(110)}, // 10%
	}
	m := computeMAPE(pairs)
	require.NotNil(t, m)
	assert.InDelta(t, 0.10, *m, 0.001)

	assert.Nil(t, computeMAPE(pairs[:2]), "меньше 3 пар — нет оценки")
}

func TestComputeBenchmark(t *testing.T) {
	me := uuid.New()
	other1, other2 := uuid.New(), uuid.New()
	growth := []repository.ParticipantGrowth{
		{ParticipantID: other1, Current30: d(110), Previous30: d(100)}, // +10%
		{ParticipantID: me, Current30: d(150), Previous30: d(100)},     // +50% → №1
		{ParticipantID: other2, Current30: d(90), Previous30: d(100)},  // −10%
	}
	bm := computeBenchmark(growth, me)
	require.NotNil(t, bm)
	assert.Equal(t, 1, bm.Rank)
	assert.Equal(t, 3, bm.Total)
	assert.InDelta(t, 0.5, *bm.GrowthPct, 0.001)

	assert.Nil(t, computeBenchmark(growth[:1], other1), "один участник — сравнивать не с кем")
}

func TestBalanceProjection(t *testing.T) {
	lastFact := day(2026, 7, 14)
	yhat := make([]decimal.Decimal, 14)
	for i := range yhat {
		yhat[i] = d(5_000)
	}
	// платёж 100к на 5-й день
	payments := []Payment{{Date: lastFact.AddDate(0, 0, 5), Amount: d(100_000)}}

	proj := balanceProjection(d(50_000), yhat, d(5_000), payments, lastFact, 10)
	require.Len(t, proj, 10)
	// день 1: 50к + 5к = 55к
	assert.True(t, proj[0].Balance.Equal(d(55_000)), "день1: %s", proj[0].Balance)
	// день 5: 50к + 25к − 100к = −25к
	assert.True(t, proj[4].Balance.Equal(d(-25_000)), "день5: %s", proj[4].Balance)
	// день 10: −25к + 25к = 0
	assert.True(t, proj[9].Balance.Equal(d(0)), "день10: %s", proj[9].Balance)
}

func TestRunwayDaysFromPayments(t *testing.T) {
	lastFact := day(2026, 7, 14)
	yhat := make([]decimal.Decimal, 14)
	for i := range yhat {
		yhat[i] = d(3_000)
	}
	// порог = 0.2×300к = 60к; старт 100к, платёж 150к на 4-й день
	payments := []Payment{{Date: lastFact.AddDate(0, 0, 4), Amount: d(150_000)}}
	runway := RunwayDaysFromPayments(d(100_000), yhat, d(3_000), payments, lastFact, d(300_000))
	// день4: 100к + 12к − 150к = −38к < 60к
	assert.Equal(t, 4, runway)
}

func TestCashCalendarMarksRisk(t *testing.T) {
	lastFact := day(2026, 7, 14)
	yhat := []decimal.Decimal{d(5_000)}
	expenses := []models.FixedExpense{
		{Description: "Мелкий платёж", Amount: d(10_000), DueDayOfMonth: 16},
		{Description: "Аренда", Amount: d(200_000), DueDayOfMonth: 25},
	}
	events := cashCalendar(d(120_000), yhat, d(5_000), expenses, lastFact, d(300_000), 30)

	require.Len(t, events, 2)
	assert.Equal(t, "Мелкий платёж", events[0].Description)
	assert.False(t, events[0].Risk, "после мелкого платежа подушка цела")
	assert.Equal(t, "Аренда", events[1].Description)
	assert.True(t, events[1].Risk, "после аренды баланс ниже подушки")
}
