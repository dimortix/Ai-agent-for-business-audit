// Package forecast — fallback-модель прогнозирования: аддитивный метод
// Хольта-Винтерса (тройное экспоненциальное сглаживание) без внешних библиотек.
// Используется, когда Python ML-сервис недоступен или вернул ошибку.
package forecast

import "math"

const (
	season = 7 // недельная сезонность дневной выручки

	// Коэффициенты сглаживания подобраны один раз под дневную выручку
	// малой розницы: уровень реагирует умеренно, тренд — консервативно,
	// сезонность — заметно (см. ТЗ, п. 5).
	alpha = 0.35 // уровень
	beta  = 0.05 // тренд
	gamma = 0.25 // сезонность

	z80 = 1.28 // квантиль N(0,1) для 80% доверительного интервала
)

// Result — прогноз и границы 80% доверительного интервала.
// Все значения неотрицательны (выручка не бывает меньше нуля).
type Result struct {
	YHat  []float64
	Lower []float64
	Upper []float64
}

// HoltWinters строит прогноз на horizon дней вперёд по дневному ряду series.
// Ряд короче двух сезонов (14 точек) прогнозируется наивно — средним.
func HoltWinters(series []float64, horizon int) Result {
	if horizon <= 0 {
		return Result{}
	}
	n := len(series)
	if n < 2*season {
		return naive(series, horizon)
	}

	// Инициализация по первым двум сезонам.
	avg1 := mean(series[:season])
	avg2 := mean(series[season : 2*season])
	level := avg1
	trend := (avg2 - avg1) / season

	// Начальные сезонные отклонения — среднее по всем полным сезонам.
	seasonal := make([]float64, season)
	fullSeasons := n / season
	for i := 0; i < season; i++ {
		sum := 0.0
		for k := 0; k < fullSeasons; k++ {
			seasonMean := mean(series[k*season : (k+1)*season])
			sum += series[k*season+i] - seasonMean
		}
		seasonal[i] = sum / float64(fullSeasons)
	}

	// Проход по ряду с накоплением остатков one-step-ahead для оценки σ.
	residuals := make([]float64, 0, n)
	for t := 0; t < n; t++ {
		idx := t % season
		fitted := level + trend + seasonal[idx]
		residuals = append(residuals, series[t]-fitted)

		prevLevel := level
		level = alpha*(series[t]-seasonal[idx]) + (1-alpha)*(level+trend)
		trend = beta*(level-prevLevel) + (1-beta)*trend
		seasonal[idx] = gamma*(series[t]-level) + (1-gamma)*seasonal[idx]
	}
	sigma := stddev(residuals)

	res := Result{
		YHat:  make([]float64, horizon),
		Lower: make([]float64, horizon),
		Upper: make([]float64, horizon),
	}
	for h := 1; h <= horizon; h++ {
		idx := (n + h - 1) % season
		f := level + float64(h)*trend + seasonal[idx]
		spread := z80 * sigma * math.Sqrt(float64(h)) // интервал расширяется с горизонтом
		res.YHat[h-1] = math.Max(0, f)
		res.Lower[h-1] = math.Max(0, f-spread)
		res.Upper[h-1] = math.Max(0, f+spread)
	}
	return res
}

// naive — прогноз средним значением ряда (для коротких рядов).
func naive(series []float64, horizon int) Result {
	m, s := 0.0, 0.0
	if len(series) > 0 {
		m = mean(series)
		s = stddev(series)
	}
	res := Result{
		YHat:  make([]float64, horizon),
		Lower: make([]float64, horizon),
		Upper: make([]float64, horizon),
	}
	for h := 1; h <= horizon; h++ {
		spread := z80 * s * math.Sqrt(float64(h))
		res.YHat[h-1] = math.Max(0, m)
		res.Lower[h-1] = math.Max(0, m-spread)
		res.Upper[h-1] = math.Max(0, m+spread)
	}
	return res
}

func mean(xs []float64) float64 {
	if len(xs) == 0 {
		return 0
	}
	sum := 0.0
	for _, x := range xs {
		sum += x
	}
	return sum / float64(len(xs))
}

func stddev(xs []float64) float64 {
	if len(xs) < 2 {
		return 0
	}
	m := mean(xs)
	sum := 0.0
	for _, x := range xs {
		d := x - m
		sum += d * d
	}
	return math.Sqrt(sum / float64(len(xs)-1))
}
