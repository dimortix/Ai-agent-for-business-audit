package service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLLMDisabled(t *testing.T) {
	// Без baseURL клиент выключен и молча возвращает nil.
	tips, err := NewLLMClient("", "", "").GenerateAdvice(context.Background(), AdviceContext{})
	assert.NoError(t, err)
	assert.Nil(t, tips)
}

func TestLLMGenerateAdviceOK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/chat/completions", r.URL.Path)
		require.Equal(t, "Bearer test-key", r.Header.Get("Authorization"))

		var req struct {
			Model    string `json:"model"`
			Messages []struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"messages"`
		}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
		require.Equal(t, "test-model", req.Model)
		require.Len(t, req.Messages, 2)
		// в промпте должен быть обезличенный контекст
		assert.Contains(t, req.Messages[1].Content, "кофейня")
		assert.NotContains(t, req.Messages[1].Content, "+7") // без телефона

		// имитируем ответ модели: JSON-массив советов
		resp := map[string]any{
			"choices": []map[string]any{{
				"message": map[string]string{
					"role":    "assistant",
					"content": `["Предложите десерт к кофе.","Запустите акцию в дневные часы."]`,
				},
			}},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	llm := NewLLMClient(srv.URL, "test-key", "test-model")
	tips, err := llm.GenerateAdvice(context.Background(), AdviceContext{
		BusinessType: "кофейня",
		HealthIndex:  52,
		AvgCheck:     d(430),
	})
	require.NoError(t, err)
	require.Len(t, tips, 2)
	assert.Equal(t, "Предложите десерт к кофе.", tips[0])
}

func TestParseAdviceList(t *testing.T) {
	// JSON-массив
	assert.Equal(t, []string{"Совет A", "Совет B"},
		parseAdviceList(`Вот советы: ["Совет A","Совет B"]`))

	// нумерованный список (фолбэк)
	got := parseAdviceList("1. Первый длинный совет тут\n2. Второй длинный совет тут")
	require.Len(t, got, 2)
	assert.Equal(t, "Первый длинный совет тут", got[0])

	// не больше трёх
	assert.Len(t, parseAdviceList(`["раз","два","три","четыре","пять"]`), 3)
}

func TestBusinessType(t *testing.T) {
	assert.Equal(t, "кофейня", businessType("Кофейня «Демо Кофе»"))
	assert.Equal(t, "пекарня", businessType("Пекарня «Тёплый хлеб»"))
	assert.Equal(t, "барбершоп", businessType("Барбершоп «Острый угол»"))
	assert.Equal(t, "малый бизнес", businessType("ООО Ромашка"))
}
