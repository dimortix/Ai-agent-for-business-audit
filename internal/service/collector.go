package service

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"alfa-pulse/internal/models"
	"alfa-pulse/internal/repository"
)

// Коллектор транзакций (ТЗ, п. 5): CSV «account_id,date,amount,type» →
// агрегация по дням → UPSERT в daily_metrics (повторный импорт накапливает).

const (
	maxCSVSize   = 10 << 20 // 10 МБ
	maxErrorList = 20       // сколько ошибок строк показывать в отчёте
)

type ImportReport struct {
	Imported    int      `json:"imported"`
	Skipped     int      `json:"skipped"`
	Errors      []string `json:"errors"`
	AffectedB   []uuid.UUID `json:"-"` // участники группы B — их нужно пересчитать
	DaysUpdated int      `json:"days_updated"`
}

type dayAgg struct {
	income  decimal.Decimal
	returns decimal.Decimal
	count   int
}

// ErrDuplicateImport — файл с таким содержимым уже импортировался.
var ErrDuplicateImport = errors.New(
	"этот файл уже импортирован ранее: повторная загрузка удвоила бы метрики (защита от двойного импорта)")

// ImportTransactionsCSV читает CSV и накапливает дневные агрегаты.
// Формат строки: account_id, date (2006-01-02 или 02.01.2006), amount > 0,
// type (income|приход / return|возврат).
// Повторный импорт файла с тем же содержимым отклоняется по sha256.
func (s *Service) ImportTransactionsCSV(ctx context.Context, r io.Reader) (*ImportReport, error) {
	raw, err := io.ReadAll(io.LimitReader(r, maxCSVSize))
	if err != nil {
		return nil, fmt.Errorf("чтение файла: %w", err)
	}
	sum := sha256.Sum256(raw)
	hash := hex.EncodeToString(sum[:])
	if dup, err := s.repo.HasImportBatch(ctx, hash); err != nil {
		return nil, err
	} else if dup {
		return nil, ErrDuplicateImport
	}

	reader := csv.NewReader(bytes.NewReader(raw))
	reader.FieldsPerRecord = -1
	reader.TrimLeadingSpace = true

	report := &ImportReport{Errors: []string{}}
	participants := map[string]*models.Participant{} // кэш поиска по account_id
	agg := map[uuid.UUID]map[time.Time]*dayAgg{}
	affected := map[uuid.UUID]bool{}

	line := 0
	for {
		rec, err := reader.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		line++
		if err != nil {
			report.addError(line, "не удалось разобрать строку CSV")
			report.Skipped++
			continue
		}
		// Пропускаем заголовок.
		if line == 1 && len(rec) > 0 && strings.EqualFold(strings.TrimSpace(rec[0]), "account_id") {
			continue
		}
		if len(rec) < 4 {
			report.addError(line, "ожидались 4 колонки: account_id,date,amount,type")
			report.Skipped++
			continue
		}

		accountID := strings.TrimSpace(rec[0])
		date, err := parseDate(strings.TrimSpace(rec[1]))
		if err != nil {
			report.addError(line, "некорректная дата «"+rec[1]+"»")
			report.Skipped++
			continue
		}
		amount, err := decimal.NewFromString(strings.TrimSpace(strings.ReplaceAll(rec[2], ",", ".")))
		if err != nil || !amount.IsPositive() {
			report.addError(line, "некорректная сумма «"+rec[2]+"»")
			report.Skipped++
			continue
		}
		txType, err := parseTxType(rec[3])
		if err != nil {
			report.addError(line, err.Error())
			report.Skipped++
			continue
		}

		p, ok := participants[accountID]
		if !ok {
			p, err = s.repo.GetParticipantByAccountID(ctx, accountID)
			if err != nil && !errors.Is(err, repository.ErrNotFound) {
				return nil, err
			}
			participants[accountID] = p // nil тоже кэшируем
		}
		if p == nil {
			report.addError(line, "неизвестный account_id «"+accountID+"»")
			report.Skipped++
			continue
		}

		if agg[p.ID] == nil {
			agg[p.ID] = map[time.Time]*dayAgg{}
		}
		a := agg[p.ID][date]
		if a == nil {
			a = &dayAgg{}
			agg[p.ID][date] = a
		}
		switch txType {
		case "income":
			a.income = a.income.Add(amount)
			a.count++
		case "return":
			a.returns = a.returns.Add(amount)
		}
		if p.GroupType == "B" {
			affected[p.ID] = true
		}
		report.Imported++
	}

	// Запись агрегатов.
	for pid, days := range agg {
		for date, a := range days {
			if err := s.repo.AddDailyDelta(ctx, pid, date, a.income, a.returns, a.count); err != nil {
				return report, fmt.Errorf("запись метрик %s / %s: %w", pid, date.Format("2006-01-02"), err)
			}
			report.DaysUpdated++
		}
	}
	for pid := range affected {
		report.AffectedB = append(report.AffectedB, pid)
	}
	if report.Imported > 0 {
		if err := s.repo.RecordImportBatch(ctx, hash, report.Imported); err != nil {
			s.log.Warn("не удалось записать отпечаток импорта", "err", err)
		}
	}
	return report, nil
}

// ImportParticipantsCSV: «phone,account_id,group_type[,name]».
func (s *Service) ImportParticipantsCSV(ctx context.Context, r io.Reader) (*ImportReport, error) {
	reader := csv.NewReader(io.LimitReader(r, maxCSVSize))
	reader.FieldsPerRecord = -1
	reader.TrimLeadingSpace = true

	report := &ImportReport{Errors: []string{}}
	line := 0
	for {
		rec, err := reader.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		line++
		if err != nil {
			report.addError(line, "не удалось разобрать строку CSV")
			report.Skipped++
			continue
		}
		if line == 1 && len(rec) > 0 && strings.EqualFold(strings.TrimSpace(rec[0]), "phone") {
			continue
		}
		if len(rec) < 3 {
			report.addError(line, "ожидались колонки: phone,account_id,group_type[,name]")
			report.Skipped++
			continue
		}

		phone, err := NormalizePhone(rec[0])
		if err != nil {
			report.addError(line, "телефон «"+rec[0]+"»: "+err.Error())
			report.Skipped++
			continue
		}
		accountID := strings.TrimSpace(rec[1])
		group := strings.ToUpper(strings.TrimSpace(rec[2]))
		if group != "A" && group != "B" {
			report.addError(line, "группа должна быть A или B, получено «"+rec[2]+"»")
			report.Skipped++
			continue
		}
		name := ""
		if len(rec) > 3 {
			name = strings.TrimSpace(rec[3])
		}

		if _, err := s.repo.UpsertParticipant(ctx, models.Participant{
			Phone: phone, AccountID: accountID, GroupType: group, Name: name,
		}); err != nil {
			report.addError(line, "не удалось сохранить участника: "+err.Error())
			report.Skipped++
			continue
		}
		report.Imported++
	}
	return report, nil
}

func (r *ImportReport) addError(line int, msg string) {
	if len(r.Errors) < maxErrorList {
		r.Errors = append(r.Errors, fmt.Sprintf("строка %d: %s", line, msg))
	} else if len(r.Errors) == maxErrorList {
		r.Errors = append(r.Errors, "…и другие ошибки")
	}
}

func parseDate(s string) (time.Time, error) {
	for _, layout := range []string{"2006-01-02", "02.01.2006"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, errors.New("bad date")
}

func parseTxType(s string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "income", "приход", "sale", "продажа":
		return "income", nil
	case "return", "возврат", "refund":
		return "return", nil
	}
	return "", errors.New("тип операции должен быть income/приход или return/возврат, получено «" + s + "»")
}
