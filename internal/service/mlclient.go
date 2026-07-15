package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// MLClient — HTTP-клиент Python ML-сервиса (Prophet).
type MLClient struct {
	baseURL string
	httpc   *http.Client
}

func NewMLClient(baseURL string) *MLClient {
	return &MLClient{
		baseURL: baseURL,
		// Prophet обучается на 90 точках за ~1–3 секунды; 15с — с запасом.
		httpc: &http.Client{Timeout: 15 * time.Second},
	}
}

type mlSeriesPoint struct {
	DS string  `json:"ds"`
	Y  float64 `json:"y"`
}

type MLForecastPoint struct {
	DS    string  `json:"ds"`
	YHat  float64 `json:"yhat"`
	Lower float64 `json:"yhat_lower"`
	Upper float64 `json:"yhat_upper"`
}

// Forecast отправляет ряд в ML-сервис. Ошибка любого рода означает,
// что вызывающий код должен перейти на fallback-модель Хольта-Винтерса.
func (c *MLClient) Forecast(ctx context.Context, dates []time.Time, values []float64, horizon int) ([]MLForecastPoint, error) {
	if len(dates) != len(values) {
		return nil, fmt.Errorf("несогласованные длины ряда: %d дат, %d значений", len(dates), len(values))
	}
	series := make([]mlSeriesPoint, len(values))
	for i := range values {
		series[i] = mlSeriesPoint{DS: dates[i].Format("2006-01-02"), Y: values[i]}
	}
	body, err := json.Marshal(map[string]any{"series": series, "horizon": horizon})
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/forecast", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpc.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ML-сервис недоступен: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("ML-сервис вернул %d: %s", resp.StatusCode, msg)
	}

	var out struct {
		Forecast []MLForecastPoint `json:"forecast"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&out); err != nil {
		return nil, fmt.Errorf("некорректный ответ ML-сервиса: %w", err)
	}
	if len(out.Forecast) != horizon {
		return nil, fmt.Errorf("ML-сервис вернул %d точек вместо %d", len(out.Forecast), horizon)
	}
	return out.Forecast, nil
}
