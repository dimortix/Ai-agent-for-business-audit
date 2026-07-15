package forecast

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Чистый недельный паттерн без шума: прогноз должен воспроизводить сезонность.
func TestHoltWintersReproducesWeeklySeasonality(t *testing.T) {
	pattern := []float64{100, 120, 90, 110, 130, 180, 160}
	series := make([]float64, 0, 8*7)
	for week := 0; week < 8; week++ {
		series = append(series, pattern...)
	}

	res := HoltWinters(series, 14)
	require.Len(t, res.YHat, 14)

	// len(series)=56 кратно 7, поэтому прогнозный день h продолжает паттерн с индекса (h-1)%7
	for h := 1; h <= 14; h++ {
		want := pattern[(h-1)%7]
		assert.InDeltaf(t, want, res.YHat[h-1], want*0.05,
			"день %d: ожидали ~%.0f, получили %.1f", h, want, res.YHat[h-1])
	}
}

// Растущий ряд: прогноз должен продолжать тренд вверх.
func TestHoltWintersFollowsTrend(t *testing.T) {
	series := make([]float64, 56)
	for i := range series {
		series[i] = 1000 + 10*float64(i) // +10 в день, без сезонности
	}

	res := HoltWinters(series, 14)
	require.Len(t, res.YHat, 14)

	last := series[len(series)-1]
	assert.Greater(t, res.YHat[0], last*0.98, "прогноз не должен провалиться ниже уровня ряда")
	assert.Greater(t, res.YHat[13], res.YHat[0], "тренд вверх должен сохраняться")
	// за 13 дней при +10/день рост ≈ 130
	assert.InDelta(t, 130, res.YHat[13]-res.YHat[0], 40)
}

// Резко падающий ряд: значения и нижние границы не уходят в минус.
func TestHoltWintersClampsNegative(t *testing.T) {
	series := make([]float64, 28)
	for i := range series {
		series[i] = 500 - 20*float64(i) // уходит в минус в пределах горизонта
		if series[i] < 0 {
			series[i] = 0
		}
	}

	res := HoltWinters(series, 14)
	for i := 0; i < 14; i++ {
		assert.GreaterOrEqual(t, res.YHat[i], 0.0)
		assert.GreaterOrEqual(t, res.Lower[i], 0.0)
		assert.GreaterOrEqual(t, res.Upper[i], res.YHat[i])
	}
}

// Короткий ряд (< 2 сезонов): наивный прогноз средним, правильной длины.
func TestHoltWintersShortSeries(t *testing.T) {
	series := []float64{100, 200, 300}
	res := HoltWinters(series, 14)

	require.Len(t, res.YHat, 14)
	for _, v := range res.YHat {
		assert.InDelta(t, 200, v, 0.001)
	}
}

// Пустой ряд и нулевой горизонт не паникуют.
func TestHoltWintersEdgeCases(t *testing.T) {
	res := HoltWinters(nil, 5)
	require.Len(t, res.YHat, 5)
	assert.Equal(t, 0.0, res.YHat[0])

	empty := HoltWinters([]float64{1, 2, 3}, 0)
	assert.Empty(t, empty.YHat)
}

// Интервал должен расширяться с горизонтом (неопределённость растёт).
func TestHoltWintersIntervalWidens(t *testing.T) {
	series := make([]float64, 56)
	for i := range series {
		series[i] = 1000 + 50*float64(i%2) // лёгкий «шум», σ > 0
	}
	res := HoltWinters(series, 14)

	w1 := res.Upper[0] - res.Lower[0]
	w14 := res.Upper[13] - res.Lower[13]
	assert.Greater(t, w14, w1)
}
