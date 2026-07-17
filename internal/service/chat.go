package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Чат-советник: предприниматель разговаривает с LLM, которая видит свежий
// контекст его бизнеса (индекс, выручка, разрыв, расходы, советы). Контекст
// собирается на каждый запрос — модель всегда отвечает по актуальным цифрам.

// ErrLLMDisabled — LLM не подключён (LLM_API_URL пуст).
var ErrLLMDisabled = errors.New("AI-советник не подключён: задайте LLM_API_URL в конфигурации")

const (
	chatMaxTurns   = 12   // сколько последних реплик отправляем модели
	chatMaxContent = 2000 // максимум символов в одной реплике
)

// ChatReply отвечает на диалог участника с учётом контекста его бизнеса.
func (s *Service) ChatReply(ctx context.Context, pid uuid.UUID, history []ChatTurn) (string, error) {
	if s.llm == nil || !s.llm.Enabled() {
		return "", ErrLLMDisabled
	}

	bizCtx, err := s.businessContext(ctx, pid)
	if err != nil {
		return "", err
	}

	system := "Ты — «Альфа.Пульс», дружелюбный финансовый советник для владельца малого бизнеса. " +
		"Отвечай по-русски, коротко и по делу (обычно 2–6 предложений), на «вы». " +
		"Опирайся ТОЛЬКО на цифры из блока «Данные бизнеса» ниже — не выдумывай показатели, которых там нет; " +
		"если данных не хватает, честно скажи об этом. " +
		"Давай практичные шаги для маленькой розницы/услуг: акции, средний чек, работа с арендой и закупками, перенос платежей. " +
		"Не давай инвестиционных, налоговых и юридических рекомендаций — предлагай обратиться к специалисту. " +
		"Не гарантируй прибыль. Форматируй простым текстом, без markdown-заголовков.\n\n" +
		"=== Данные бизнеса (обновлены только что) ===\n" + bizCtx

	// Ограничиваем историю по длине и количеству.
	trimmed := history
	if len(trimmed) > chatMaxTurns {
		trimmed = trimmed[len(trimmed)-chatMaxTurns:]
	}
	safe := make([]ChatTurn, 0, len(trimmed))
	for _, t := range trimmed {
		if t.Role != "user" && t.Role != "assistant" {
			continue
		}
		content := strings.TrimSpace(t.Content)
		if content == "" {
			continue
		}
		if len([]rune(content)) > chatMaxContent {
			content = string([]rune(content)[:chatMaxContent])
		}
		safe = append(safe, ChatTurn{Role: t.Role, Content: content})
	}
	if len(safe) == 0 || safe[len(safe)-1].Role != "user" {
		return "", errors.New("последняя реплика должна быть от пользователя")
	}

	reply, err := s.llm.Chat(ctx, system, safe)
	if err != nil {
		return "", err
	}
	return stripReasoning(reply), nil
}

// businessContext — компактная текстовая сводка бизнеса для системного промпта.
func (s *Service) businessContext(ctx context.Context, pid uuid.UUID) (string, error) {
	p, err := s.repo.GetParticipantByID(ctx, pid)
	if err != nil {
		return "", err
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Бизнес: %s (%s).\n", p.Name, businessType(p.Name))

	// Финансовое положение
	if fin, err := s.ComputeFinancials(ctx, pid); err == nil {
		fmt.Fprintf(&b, "Оценка денег на счету: %s. Обязательные расходы: %s в месяц.\n",
			FormatMoney(fin.CurrentBalance), FormatMoney(fin.MonthlyExpenses))
	}

	// Индекс и разрыв из последнего прогноза
	if preds, err := s.repo.GetLatestPredictionBatch(ctx, pid); err == nil && len(preds) > 0 {
		if preds[0].HealthIndex != nil {
			fmt.Fprintf(&b, "Индекс здоровья: %d из 100 (%s).\n",
				*preds[0].HealthIndex, healthStatusRU(HealthStatus(*preds[0].HealthIndex)))
		}
		if preds[0].CashGapDate != nil {
			fmt.Fprintf(&b, "Прогнозируется кассовый разрыв %s.\n", FormatDateRU(*preds[0].CashGapDate))
		} else {
			b.WriteString("Кассовых разрывов в ближайшие 14 дней не прогнозируется.\n")
		}
		week := preds[0].PredictedRevenue
		for i := 1; i < len(preds) && i < 7; i++ {
			week = week.Add(preds[i].PredictedRevenue)
		}
		fmt.Fprintf(&b, "Прогноз выручки на ближайшие 7 дней: ~%s (модель %s).\n",
			FormatMoney(week), preds[0].ModelUsed)
	}

	// Сводка за 30 дней + профиль недели
	today := dateOnly(time.Now().UTC())
	if metrics, err := s.repo.GetMetricsRange(ctx, pid, today.AddDate(0, 0, -60), today); err == nil && len(metrics) > 0 {
		if pc := comparePeriods(metrics, today, 30); pc != nil {
			fmt.Fprintf(&b, "За последние 30 дней: выручка %s (%d покупок, средний чек %s)",
				FormatMoney(pc.Current.Revenue), pc.Current.Transaction, FormatMoney(pc.Current.AvgCheck))
			if pc.RevenueDelta != nil {
				fmt.Fprintf(&b, ", динамика к прошлому периоду %+.0f%%", *pc.RevenueDelta*100)
			}
			b.WriteString(".\n")
		}
		if txt := bestWeekday(weekdayProfile(metrics)); txt != "" {
			b.WriteString(txt + "\n")
		}
	}

	// Крупнейшие статьи расходов
	if expenses, err := s.repo.ListExpenses(ctx, pid); err == nil && len(expenses) > 0 {
		b.WriteString("Основные регулярные платежи: ")
		for i, e := range expenses {
			if i > 0 {
				b.WriteString("; ")
			}
			fmt.Fprintf(&b, "%s — %s (до %d числа)", e.Description, FormatMoney(e.Amount), e.DueDayOfMonth)
			if i == 4 {
				break
			}
		}
		b.WriteString(".\n")
	}

	// Активные советы системы
	if recs, err := s.repo.ListRecommendations(ctx, pid, "active"); err == nil && len(recs) > 0 {
		b.WriteString("Активные советы системы:\n")
		for i, r := range recs {
			if i == 4 {
				break
			}
			fmt.Fprintf(&b, "- %s\n", r.Message)
		}
	}

	return b.String(), nil
}

func healthStatusRU(status string) string {
	switch status {
	case "critical":
		return "красная зона, нужны срочные меры"
	case "warning":
		return "жёлтая зона, стоит задуматься"
	default:
		return "зелёная зона, всё хорошо"
	}
}

// stripReasoning вырезает «размышления» reasoning-моделей (<think>…</think>),
// чтобы пользователь видел только ответ.
func stripReasoning(s string) string {
	for {
		start := strings.Index(s, "<think>")
		if start < 0 {
			break
		}
		end := strings.Index(s, "</think>")
		if end < 0 {
			s = s[:start]
			break
		}
		s = s[:start] + s[end+len("</think>"):]
	}
	return strings.TrimSpace(s)
}
