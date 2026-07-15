package service

import (
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
)

func d(v float64) decimal.Decimal { return decimal.NewFromFloat(v) }

func TestCurrentBalance(t *testing.T) {
	// 90 дней истории = ровно 3 месяца расходов
	got := CurrentBalance(d(1_620_000), d(495_000), 90)
	assert.True(t, got.Equal(d(135_000)), "got %s", got)

	// без истории — просто накопленная выручка
	assert.True(t, CurrentBalance(d(500), d(100), 0).Equal(d(500)))
}

func TestForecast30(t *testing.T) {
	yhat := []decimal.Decimal{d(10_000), d(12_000)} // Σ = 22 000
	got := Forecast30(yhat, d(1_000))               // + 16 × 1 000
	assert.True(t, got.Equal(d(38_000)), "got %s", got)
}

func TestHealthIndexExactValues(t *testing.T) {
	// 100·(0 + 300 000)/(500 000·1.2) = 50
	assert.Equal(t, 50, HealthIndex(d(0), d(300_000), d(500_000)))

	// ровно на границе: 100·600 000/600 000 = 100
	assert.Equal(t, 100, HealthIndex(d(100_000), d(500_000), d(500_000)))
}

func TestHealthIndexClamps(t *testing.T) {
	// глубоко отрицательный числитель → минимум 1
	assert.Equal(t, 1, HealthIndex(d(-1_000_000), d(0), d(100_000)))
	// сверхприбыльный бизнес → максимум 100
	assert.Equal(t, 100, HealthIndex(d(10_000_000), d(10_000_000), d(100_000)))
}

func TestHealthIndexZeroExpenses(t *testing.T) {
	assert.Equal(t, 100, HealthIndex(d(1), d(0), d(0)), "нет платежей и деньги есть — здоров")
	assert.Equal(t, 1, HealthIndex(d(-1), d(0), d(0)), "нет платежей, но баланс в минусе")
}

func TestAdjustHealthIndex(t *testing.T) {
	assert.Equal(t, 100, AdjustHealthIndex(100, false, false), "без рисков — без штрафов")
	assert.Equal(t, 75, AdjustHealthIndex(100, true, false), "разрыв в горизонте: −25")
	assert.Equal(t, 90, AdjustHealthIndex(100, false, true), "спад недели: −10")
	assert.Equal(t, 65, AdjustHealthIndex(100, true, true))
	assert.Equal(t, 1, AdjustHealthIndex(20, true, true), "не опускаемся ниже 1")
}

func TestWeeklyTrendDrop(t *testing.T) {
	stable := make([]float64, 14)
	falling := make([]float64, 14)
	for i := range stable {
		stable[i] = 10_000
		falling[i] = 10_000
		if i >= 7 {
			falling[i] = 8_000 // −20% ко второй неделе
		}
	}
	assert.False(t, WeeklyTrendDrop(stable))
	assert.True(t, WeeklyTrendDrop(falling))
	assert.False(t, WeeklyTrendDrop(falling[:10]), "мало данных — спада не фиксируем")
}

func TestHealthStatusZones(t *testing.T) {
	assert.Equal(t, "critical", HealthStatus(39))
	assert.Equal(t, "warning", HealthStatus(40))
	assert.Equal(t, "warning", HealthStatus(69))
	assert.Equal(t, "ok", HealthStatus(70))
}
