package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testSeries(n int) ([]time.Time, []float64) {
	dates := make([]time.Time, n)
	values := make([]float64, n)
	for i := 0; i < n; i++ {
		dates[i] = day(2026, 4, 1).AddDate(0, 0, i)
		values[i] = 10_000 + float64(i)*10
	}
	return dates, values
}

func TestMLClientForecastOK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/forecast", r.URL.Path)

		var req struct {
			Series []struct {
				DS string  `json:"ds"`
				Y  float64 `json:"y"`
			} `json:"series"`
			Horizon int `json:"horizon"`
		}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
		require.Len(t, req.Series, 90)
		require.Equal(t, 14, req.Horizon)
		require.Equal(t, "2026-04-01", req.Series[0].DS)

		fc := make([]map[string]any, req.Horizon)
		for i := range fc {
			fc[i] = map[string]any{
				"ds": fmt.Sprintf("2026-07-%02d", i+1), "yhat": 12_000.5,
				"yhat_lower": 11_000.0, "yhat_upper": 13_000.0,
			}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"forecast": fc})
	}))
	defer srv.Close()

	dates, values := testSeries(90)
	points, err := NewMLClient(srv.URL).Forecast(context.Background(), dates, values, 14)

	require.NoError(t, err)
	require.Len(t, points, 14)
	assert.Equal(t, 12_000.5, points[0].YHat)
	assert.Equal(t, 11_000.0, points[0].Lower)
}

func TestMLClientErrorStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"detail":"series too short"}`, http.StatusBadRequest)
	}))
	defer srv.Close()

	dates, values := testSeries(5)
	_, err := NewMLClient(srv.URL).Forecast(context.Background(), dates, values, 14)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "400")
}

func TestMLClientWrongPointCount(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"forecast": []any{}})
	}))
	defer srv.Close()

	dates, values := testSeries(90)
	_, err := NewMLClient(srv.URL).Forecast(context.Background(), dates, values, 14)
	require.Error(t, err)
}

func TestMLClientUnreachable(t *testing.T) {
	dates, values := testSeries(90)
	// закрытый порт → мгновенная ошибка соединения
	_, err := NewMLClient("http://127.0.0.1:1").Forecast(context.Background(), dates, values, 14)
	require.Error(t, err)
}
