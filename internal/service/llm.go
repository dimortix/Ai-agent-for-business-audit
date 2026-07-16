package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/shopspring/decimal"

	"alfa-pulse/internal/models"
)

// LLM-модуль генерации советов (ТЗ V2, п. 4). Провайдер-нейтральный клиент по
// OpenAI-совместимому протоколу chat/completions: работает с GigaChat,
// YandexGPT (через прокси), OpenAI и локальными ollama/llama.cpp — то есть с
// моделью, развёрнутой внутри контура банка. Если LLM не сконфигурирован,
// система молча остаётся на детерминированных правилах (rules.go).
//
// Данные обезличены: в промпт уходят только агрегаты (тип бизнеса, средний чек,
// тренды, индекс, дни до разрыва) — без имени, телефона и счёта.

const llmAdviceRule = "AI_ADVICE"

// LLMClient — конфигурируемый клиент чат-модели.
type LLMClient struct {
	baseURL string
	apiKey  string
	model   string
	httpc   *http.Client
}

func NewLLMClient(baseURL, apiKey, model string) *LLMClient {
	if model == "" {
		model = "gpt-4o-mini"
	}
	return &LLMClient{
		baseURL: strings.TrimRight(baseURL, "/"),
		apiKey:  apiKey,
		model:   model,
		httpc:   &http.Client{Timeout: 25 * time.Second},
	}
}

func (c *LLMClient) Enabled() bool { return c != nil && c.baseURL != "" }

// AdviceContext — обезличенный контекст для промпта.
type AdviceContext struct {
	BusinessType   string          // «кофейня», «барбершоп» …
	HealthIndex    int             // 1..100
	AvgCheck       decimal.Decimal // средний чек, ₽
	AvgCheckDelta  float64         // изменение к прошлому периоду, доля
	RevenueDelta   float64         // изменение выручки 30/30, доля
	DaysToCashGap  int             // -1 если разрыва нет
	BestWeekday    string          // сильнейший день недели
	MonthlyExpense decimal.Decimal
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatRequest struct {
	Model       string        `json:"model"`
	Messages    []chatMessage `json:"messages"`
	Temperature float64       `json:"temperature"`
	MaxTokens   int           `json:"max_tokens"`
}

type chatResponse struct {
	Choices []struct {
		Message chatMessage `json:"message"`
	} `json:"choices"`
}

// GenerateAdvice просит модель дать 2–3 конкретных совета. Возвращает список
// текстов; при любой ошибке — nil (вызывающий тихо остаётся на правилах).
func (c *LLMClient) GenerateAdvice(ctx context.Context, ac AdviceContext) ([]string, error) {
	if !c.Enabled() {
		return nil, nil
	}

	system := "Ты — финансовый наставник для малого бизнеса в банковском сервисе «Альфа.Пульс». " +
		"Дай короткие практичные советы, что предпринимателю сделать прямо сейчас, чтобы улучшить финансы. " +
		"Пиши по-русски, дружелюбно и конкретно, с опорой на цифры из контекста. " +
		"Каждый совет — одно-два предложения, начинается с действия. " +
		"Не давай инвестиционных рекомендаций и не гарантируй прибыль. " +
		"Верни СТРОГО JSON-массив из 2–3 строк, без пояснений вокруг, например: " +
		`["Совет один.","Совет два."]`

	req := chatRequest{
		Model:       c.model,
		Temperature: 0.7,
		MaxTokens:   500,
		Messages: []chatMessage{
			{Role: "system", Content: system},
			{Role: "user", Content: ac.prompt()},
		},
	}
	body, _ := json.Marshal(req)

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := c.httpc.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("LLM недоступен: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("LLM вернул %d: %s", resp.StatusCode, msg)
	}

	var out chatResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&out); err != nil {
		return nil, err
	}
	if len(out.Choices) == 0 {
		return nil, fmt.Errorf("LLM вернул пустой ответ")
	}
	return parseAdviceList(out.Choices[0].Message.Content), nil
}

func (ac AdviceContext) prompt() string {
	var b strings.Builder
	fmt.Fprintf(&b, "Бизнес: %s. Индекс здоровья: %d из 100.\n", ac.BusinessType, ac.HealthIndex)
	fmt.Fprintf(&b, "Средний чек: %s", FormatMoney(ac.AvgCheck))
	if ac.AvgCheckDelta != 0 {
		fmt.Fprintf(&b, " (%+.0f%% к прошлому периоду)", ac.AvgCheckDelta*100)
	}
	b.WriteString(".\n")
	if ac.RevenueDelta != 0 {
		fmt.Fprintf(&b, "Выручка за 30 дней: %+.0f%% к предыдущим 30 дням.\n", ac.RevenueDelta*100)
	}
	if ac.BestWeekday != "" {
		fmt.Fprintf(&b, "Сильнейший день недели: %s.\n", ac.BestWeekday)
	}
	if ac.MonthlyExpense.IsPositive() {
		fmt.Fprintf(&b, "Обязательные расходы: %s в месяц.\n", FormatMoney(ac.MonthlyExpense))
	}
	if ac.DaysToCashGap >= 0 {
		fmt.Fprintf(&b, "Внимание: кассовый разрыв ожидается примерно через %d дней.\n", ac.DaysToCashGap)
	}
	b.WriteString("Дай 2–3 совета, что сделать в ближайшие дни.")
	return b.String()
}

// parseAdviceList вытаскивает список советов из ответа модели: сначала пробует
// JSON-массив, при неудаче — построчный разбор (модели любят нумерованные списки).
func parseAdviceList(raw string) []string {
	raw = strings.TrimSpace(raw)
	if i := strings.Index(raw, "["); i >= 0 {
		if j := strings.LastIndex(raw, "]"); j > i {
			var arr []string
			if err := json.Unmarshal([]byte(raw[i:j+1]), &arr); err == nil {
				return cleanAdvice(arr)
			}
		}
	}
	// фолбэк: разбор по строкам, срезая маркеры списка
	var out []string
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		line = strings.TrimLeft(line, "-*•0123456789.) ")
		if len(line) > 10 {
			out = append(out, line)
		}
	}
	return cleanAdvice(out)
}

func cleanAdvice(items []string) []string {
	out := make([]string, 0, len(items))
	for _, s := range items {
		s = strings.TrimSpace(strings.Trim(s, `"`))
		if s != "" {
			out = append(out, s)
		}
		if len(out) == 3 {
			break
		}
	}
	return out
}

// buildAdviceContext собирает обезличенный контекст из метрик участника.
func buildAdviceContext(p *models.Participant, index int, monthly decimal.Decimal,
	last14 []models.DailyMetric, gapDate *time.Time, period *PeriodComparison, bestDay string) AdviceContext {

	ac := AdviceContext{
		BusinessType:   businessType(p.Name),
		HealthIndex:    index,
		MonthlyExpense: monthly,
		DaysToCashGap:  -1,
		BestWeekday:    bestDay,
	}
	if len(last14) > 0 {
		ac.AvgCheck = last14[len(last14)-1].AvgCheck
	}
	if period != nil {
		if period.AvgCheckDelta != nil {
			ac.AvgCheckDelta = *period.AvgCheckDelta
		}
		if period.RevenueDelta != nil {
			ac.RevenueDelta = *period.RevenueDelta
		}
	}
	if gapDate != nil {
		ac.DaysToCashGap = int(dateOnly(*gapDate).Sub(dateOnly(time.Now().UTC())).Hours() / 24)
		if ac.DaysToCashGap < 0 {
			ac.DaysToCashGap = 0
		}
	}
	return ac
}

// businessType вытаскивает тип бизнеса из названия («Кофейня «Демо Кофе»» → «кофейня»).
func businessType(name string) string {
	name = strings.ToLower(name)
	for _, kind := range []string{"кофейн", "пекарн", "барбершоп", "фудтрак", "цвет", "магазин", "кафе", "ресторан", "салон"} {
		if strings.Contains(name, kind) {
			switch kind {
			case "кофейн":
				return "кофейня"
			case "пекарн":
				return "пекарня"
			case "цвет":
				return "цветочный магазин"
			}
			return kind
		}
	}
	return "малый бизнес"
}
