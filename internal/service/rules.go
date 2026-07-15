package service

import (
	"fmt"
	"strings"
	"time"

	"github.com/shopspring/decimal"

	"alfa-pulse/internal/models"
)

// Правила рекомендаций (ТЗ, п. 5). Каждое правило — чистая функция от
// последних метрик; тексты — конкретные действия, а не констатация.

const (
	RuleAvgCheckDrop   = "AVG_CHECK_DROP"
	RuleRevenueDecline = "REVENUE_DECLINE_3D"
	RuleTrafficDrop    = "TRAFFIC_DROP"
	RuleCashGapSoon    = "CASH_GAP_SOON"
)

type RuleHit struct {
	Code    string
	Message string
}

// EvaluateRules принимает метрики по возрастанию даты (обычно последние 14 дней)
// и результат детектора разрыва.
func EvaluateRules(metrics []models.DailyMetric, gapDate *time.Time, gapAmount decimal.Decimal) []RuleHit {
	var hits []RuleHit

	if h := checkAvgCheckDrop(metrics); h != nil {
		hits = append(hits, *h)
	}
	if h := checkRevenueDecline(metrics); h != nil {
		hits = append(hits, *h)
	}
	if h := checkTrafficDrop(metrics); h != nil {
		hits = append(hits, *h)
	}
	if gapDate != nil {
		hits = append(hits, RuleHit{
			Code: RuleCashGapSoon,
			Message: fmt.Sprintf(
				"Если ничего не менять, %s на счету может не хватить около %s на обязательные платежи. "+
					"Срочно: перенесите необязательные закупки, договоритесь об отсрочке аренды "+
					"или запустите акцию со скидкой 15%% в дневные часы, чтобы быстро добрать выручку.",
				FormatDateRU(*gapDate), FormatMoney(gapAmount)),
		})
	}
	return hits
}

// Средний чек: среднее за последние 5 торговых дней против предыдущих 5.
func checkAvgCheckDrop(metrics []models.DailyMetric) *RuleHit {
	trading := tradingDays(metrics)
	if len(trading) < 10 {
		return nil
	}
	last5 := avgCheckMean(trading[len(trading)-5:])
	prev5 := avgCheckMean(trading[len(trading)-10 : len(trading)-5])
	if !prev5.IsPositive() {
		return nil
	}
	drop := decimal.NewFromInt(1).Sub(last5.Div(prev5)) // доля падения
	if drop.LessThan(decimal.NewFromFloat(0.10)) {
		return nil
	}
	return &RuleHit{
		Code: RuleAvgCheckDrop,
		Message: fmt.Sprintf(
			"Средний чек снизился на %s%% (было %s, стало %s). "+
				"Предложите гостям десерт к кофе, комбо-наборы или позицию «к этому обычно берут» — "+
				"это поднимает чек без роста трафика.",
			drop.Mul(dec100).Round(0), FormatMoney(prev5), FormatMoney(last5)),
	}
}

// Выручка падает 3 дня подряд (3 убывающих перехода = 4 точки).
func checkRevenueDecline(metrics []models.DailyMetric) *RuleHit {
	n := len(metrics)
	if n < 4 {
		return nil
	}
	last4 := metrics[n-4:]
	for i := 1; i < 4; i++ {
		if !last4[i].Net().LessThan(last4[i-1].Net()) {
			return nil
		}
	}
	base := last4[0].Net()
	var dropPct decimal.Decimal
	if base.IsPositive() {
		dropPct = decimal.NewFromInt(1).Sub(last4[3].Net().Div(base)).Mul(dec100).Round(0)
	}
	return &RuleHit{
		Code: RuleRevenueDecline,
		Message: fmt.Sprintf(
			"Выручка снижается три дня подряд (−%s%% к уровню трёх дней назад). "+
				"Проверьте отзывы и рейтинг на Яндекс Картах и 2ГИС — возможно, что-то отпугивает гостей. "+
				"Напомните о себе в соцсетях.",
			dropPct),
	}
}

// Число транзакций (аппроксимация числа клиентов) за 7 дней упало на 20%+.
func checkTrafficDrop(metrics []models.DailyMetric) *RuleHit {
	n := len(metrics)
	if n < 14 {
		return nil
	}
	var last7, prev7 int
	for _, m := range metrics[n-7:] {
		last7 += m.TransactionCount
	}
	for _, m := range metrics[n-14 : n-7] {
		prev7 += m.TransactionCount
	}
	if prev7 == 0 || float64(last7) >= float64(prev7)*0.8 {
		return nil
	}
	drop := int((1 - float64(last7)/float64(prev7)) * 100)
	return &RuleHit{
		Code: RuleTrafficDrop,
		Message: fmt.Sprintf(
			"Поток клиентов упал на %d%% за неделю (%d покупок против %d). "+
				"Запустите акцию для постоянных гостей: «шестой кофе в подарок» или скидка в «мёртвые» часы.",
			drop, last7, prev7),
	}
}

// tradingDays — дни, когда были продажи (для среднего чека дни без продаж не считаем).
func tradingDays(metrics []models.DailyMetric) []models.DailyMetric {
	out := make([]models.DailyMetric, 0, len(metrics))
	for _, m := range metrics {
		if m.TransactionCount > 0 {
			out = append(out, m)
		}
	}
	return out
}

func avgCheckMean(metrics []models.DailyMetric) decimal.Decimal {
	if len(metrics) == 0 {
		return decimal.Zero
	}
	sum := decimal.Zero
	for _, m := range metrics {
		sum = sum.Add(m.AvgCheck)
	}
	return sum.Div(decimal.NewFromInt(int64(len(metrics))))
}

// FormatMoney — «12 345 ₽» (разделитель тысяч — пробел, без копеек).
func FormatMoney(d decimal.Decimal) string {
	s := d.Round(0).String()
	neg := strings.HasPrefix(s, "-")
	s = strings.TrimPrefix(s, "-")
	var b strings.Builder
	for i, r := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			b.WriteRune(' ')
		}
		b.WriteRune(r)
	}
	res := b.String() + " ₽"
	if neg {
		return "−" + res
	}
	return res
}

var monthsRU = [...]string{
	"января", "февраля", "марта", "апреля", "мая", "июня",
	"июля", "августа", "сентября", "октября", "ноября", "декабря",
}

// FormatDateRU — «25 июля».
func FormatDateRU(t time.Time) string {
	return fmt.Sprintf("%d %s", t.Day(), monthsRU[t.Month()-1])
}
