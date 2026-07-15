package service

import (
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"alfa-pulse/internal/models"
)

func day(y int, m time.Month, dd int) time.Time {
	return time.Date(y, m, dd, 0, 0, 0, 0, time.UTC)
}

func forecastDays(from time.Time, n int) ([]time.Time, []decimal.Decimal) {
	dates := make([]time.Time, n)
	yhat := make([]decimal.Decimal, n)
	for i := 0; i < n; i++ {
		dates[i] = from.AddDate(0, 0, i+1)
		yhat[i] = d(5_000)
	}
	return dates, yhat
}

func TestDetectCashGapNone(t *testing.T) {
	last := day(2026, 7, 13)
	dates, yhat := forecastDays(last, 14)
	// платежей нет, баланс большой — разрыва нет
	gap, amount := DetectCashGap(d(500_000), yhat, dates, nil, d(100_000))
	assert.Nil(t, gap)
	assert.True(t, amount.IsZero())
}

func TestDetectCashGapOnRentDay(t *testing.T) {
	last := day(2026, 7, 13)
	dates, yhat := forecastDays(last, 14) // +5 000/день
	payments := []Payment{{Date: last.AddDate(0, 0, 6), Amount: d(150_000), Description: "Аренда"}}

	// порог = 0.2 × 300 000 = 60 000; старт 100 000
	gap, amount := DetectCashGap(d(100_000), yhat, dates, payments, d(300_000))

	require.NotNil(t, gap)
	// день 6: 100 000 + 6×5 000 − 150 000 = −20 000 < 60 000
	assert.Equal(t, last.AddDate(0, 0, 6), *gap)
	assert.True(t, amount.Equal(d(80_000)), "не хватает до порога: got %s", amount)
}

func TestDetectCashGapFirstDay(t *testing.T) {
	last := day(2026, 7, 13)
	dates, yhat := forecastDays(last, 14)
	// баланс уже почти на нуле — разрыв в первый же день
	gap, _ := DetectCashGap(d(1_000), yhat, dates, nil, d(300_000))
	require.NotNil(t, gap)
	assert.Equal(t, dates[0], *gap)
}

func TestDuePaymentsMonthEnd(t *testing.T) {
	expenses := []models.FixedExpense{
		{Description: "Аренда", Amount: d(100_000), DueDayOfMonth: 31},
	}
	// апрель — 30 дней: платёж «31-го» должен лечь на 30 апреля
	payments := DuePayments(expenses, day(2026, 4, 20), 14)

	require.Len(t, payments, 1)
	assert.Equal(t, day(2026, 4, 30), payments[0].Date)
}

func TestDuePaymentsSpansTwoMonths(t *testing.T) {
	expenses := []models.FixedExpense{
		{Description: "ФОТ", Amount: d(50_000), DueDayOfMonth: 5},
	}
	// окно 28.06–11.07 захватывает 5 июля
	payments := DuePayments(expenses, day(2026, 6, 27), 14)

	require.Len(t, payments, 1)
	assert.Equal(t, day(2026, 7, 5), payments[0].Date)
}
