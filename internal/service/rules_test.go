package service

import (
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"alfa-pulse/internal/models"
)

// metric — метрика дня для тестов: выручка, число покупок, средний чек.
func metric(offset int, revenue float64, count int, avgCheck float64) models.DailyMetric {
	return models.DailyMetric{
		Date:             day(2026, 7, 1).AddDate(0, 0, offset),
		TotalRevenue:     d(revenue),
		TransactionCount: count,
		AvgCheck:         d(avgCheck),
	}
}

func codes(hits []RuleHit) []string {
	out := make([]string, len(hits))
	for i, h := range hits {
		out[i] = h.Code
	}
	return out
}

// Стабильные 14 дней — ни одно правило не должно сработать.
func TestRulesQuietOnStableMetrics(t *testing.T) {
	var metrics []models.DailyMetric
	for i := 0; i < 14; i++ {
		metrics = append(metrics, metric(i, 15_000, 40, 420))
	}
	hits := EvaluateRules(metrics, nil, decimal.Zero)
	assert.Empty(t, hits)
}

func TestAvgCheckDropTriggersAt10Percent(t *testing.T) {
	var metrics []models.DailyMetric
	for i := 0; i < 5; i++ {
		metrics = append(metrics, metric(i, 15_000, 40, 500)) // было 500
	}
	for i := 5; i < 10; i++ {
		metrics = append(metrics, metric(i, 15_000, 40, 430)) // стало 430 (−14%)
	}
	hits := EvaluateRules(metrics, nil, decimal.Zero)

	require.Contains(t, codes(hits), RuleAvgCheckDrop)
	for _, h := range hits {
		if h.Code == RuleAvgCheckDrop {
			assert.Contains(t, h.Message, "Средний чек")
			assert.Contains(t, h.Message, "14%")
		}
	}
}

func TestAvgCheckSmallDropIgnored(t *testing.T) {
	var metrics []models.DailyMetric
	for i := 0; i < 5; i++ {
		metrics = append(metrics, metric(i, 15_000, 40, 500))
	}
	for i := 5; i < 10; i++ {
		metrics = append(metrics, metric(i, 15_000, 40, 475)) // −5% — в пределах шума
	}
	assert.NotContains(t, codes(EvaluateRules(metrics, nil, decimal.Zero)), RuleAvgCheckDrop)
}

func TestRevenueDeclineThreeDays(t *testing.T) {
	metrics := []models.DailyMetric{
		metric(0, 20_000, 40, 500),
		metric(1, 18_000, 40, 450),
		metric(2, 16_000, 40, 400),
		metric(3, 14_000, 40, 350),
	}
	assert.Contains(t, codes(EvaluateRules(metrics, nil, decimal.Zero)), RuleRevenueDecline)
}

func TestRevenueDeclineNotStrictIgnored(t *testing.T) {
	metrics := []models.DailyMetric{
		metric(0, 20_000, 40, 500),
		metric(1, 18_000, 40, 450),
		metric(2, 18_000, 40, 450), // плато — серия прервана
		metric(3, 14_000, 40, 350),
	}
	assert.NotContains(t, codes(EvaluateRules(metrics, nil, decimal.Zero)), RuleRevenueDecline)
}

func TestTrafficDropTriggersAt20Percent(t *testing.T) {
	var metrics []models.DailyMetric
	for i := 0; i < 7; i++ {
		metrics = append(metrics, metric(i, 15_000, 50, 300)) // прошлая неделя: 350 покупок
	}
	for i := 7; i < 14; i++ {
		metrics = append(metrics, metric(i, 10_000, 35, 300)) // эта неделя: 245 (−30%)
	}
	hits := EvaluateRules(metrics, nil, decimal.Zero)
	require.Contains(t, codes(hits), RuleTrafficDrop)
}

func TestTrafficSmallDropIgnored(t *testing.T) {
	var metrics []models.DailyMetric
	for i := 0; i < 7; i++ {
		metrics = append(metrics, metric(i, 15_000, 50, 300))
	}
	for i := 7; i < 14; i++ {
		metrics = append(metrics, metric(i, 14_000, 46, 300)) // −8% — не повод
	}
	assert.NotContains(t, codes(EvaluateRules(metrics, nil, decimal.Zero)), RuleTrafficDrop)
}

func TestCashGapRecommendation(t *testing.T) {
	gap := day(2026, 7, 25)
	hits := EvaluateRules(nil, &gap, d(15_000))

	require.Len(t, hits, 1)
	assert.Equal(t, RuleCashGapSoon, hits[0].Code)
	assert.Contains(t, hits[0].Message, "25 июля")
	assert.Contains(t, hits[0].Message, "15 000 ₽")
}

func TestFormatMoney(t *testing.T) {
	assert.Equal(t, "15 000 ₽", FormatMoney(d(15_000)))
	assert.Equal(t, "1 234 568 ₽", FormatMoney(d(1_234_567.6)))
	assert.Equal(t, "999 ₽", FormatMoney(d(999)))
	assert.Equal(t, "−5 000 ₽", FormatMoney(d(-5_000)))
}
