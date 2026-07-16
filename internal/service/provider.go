package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/shopspring/decimal"
)

// TransactionProvider — абстракция источника транзакций (ТЗ V2, п. 5).
// Коллектор не знает, откуда приходят данные:
//   - CSVProvider — демо и тесты (файлы);
//   - AcquiringAPIProvider — боевой режим: REST-шлюз процессинга банка.
//
// На демо-стенде используется синтетический CSV-поток, имитирующий реальный;
// для боевого включения достаточно сконфигурировать ACQUIRING_API_URL/TOKEN —
// код коллектора не меняется.
type TransactionProvider interface {
	Name() string
	GetTransactions(ctx context.Context, from, to time.Time) ([]ParsedTx, error)
}

// --- CSV (демо) ---------------------------------------------------------------

type CSVProvider struct {
	Path string
}

func (p *CSVProvider) Name() string { return "csv:" + p.Path }

func (p *CSVProvider) GetTransactions(_ context.Context, from, to time.Time) ([]ParsedTx, error) {
	raw, err := os.ReadFile(p.Path)
	if err != nil {
		return nil, err
	}
	report := &ImportReport{Errors: []string{}}
	all := parseTransactionsCSV(raw, report)
	if len(report.Errors) > 0 {
		return nil, fmt.Errorf("ошибки парсинга CSV: %v", report.Errors[0])
	}

	var out []ParsedTx
	for _, tx := range all {
		if !tx.Date.Before(from) && !tx.Date.After(to) {
			out = append(out, tx)
		}
	}
	return out, nil
}

// --- API эквайринга (боевой шлюз) ----------------------------------------------

// ErrProviderNotConfigured — провайдер эквайринга не настроен (нет URL/токена).
var ErrProviderNotConfigured = errors.New(
	"провайдер эквайринга не сконфигурирован: задайте ACQUIRING_API_URL и ACQUIRING_API_TOKEN")

// AcquiringAPIProvider обращается к внутреннему API процессинга.
// Ожидаемый контракт: GET {base}/v1/transactions?from=YYYY-MM-DD&to=YYYY-MM-DD
// с Bearer-авторизацией → [{"account_id","date","amount","type"}].
type AcquiringAPIProvider struct {
	BaseURL string
	Token   string
	httpc   *http.Client
}

func NewAcquiringAPIProvider(baseURL, token string) *AcquiringAPIProvider {
	return &AcquiringAPIProvider{
		BaseURL: baseURL,
		Token:   token,
		httpc:   &http.Client{Timeout: 30 * time.Second},
	}
}

func (p *AcquiringAPIProvider) Name() string { return "acquiring-api" }

func (p *AcquiringAPIProvider) GetTransactions(ctx context.Context, from, to time.Time) ([]ParsedTx, error) {
	if p.BaseURL == "" || p.Token == "" {
		return nil, ErrProviderNotConfigured
	}

	url := fmt.Sprintf("%s/v1/transactions?from=%s&to=%s",
		p.BaseURL, from.Format("2006-01-02"), to.Format("2006-01-02"))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+p.Token)

	resp, err := p.httpc.Do(req)
	if err != nil {
		return nil, fmt.Errorf("API эквайринга недоступен: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("API эквайринга вернул %d: %s", resp.StatusCode, body)
	}

	var rows []struct {
		AccountID string          `json:"account_id"`
		Date      string          `json:"date"`
		Amount    decimal.Decimal `json:"amount"`
		Type      string          `json:"type"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxCSVSize)).Decode(&rows); err != nil {
		return nil, fmt.Errorf("некорректный ответ API эквайринга: %w", err)
	}

	out := make([]ParsedTx, 0, len(rows))
	for _, r := range rows {
		date, err := parseDate(r.Date)
		if err != nil {
			continue
		}
		txType, err := parseTxType(r.Type)
		if err != nil {
			continue
		}
		out = append(out, ParsedTx{AccountID: r.AccountID, Date: date, Amount: r.Amount, Type: txType})
	}
	return out, nil
}
