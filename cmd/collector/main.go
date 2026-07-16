// Коллектор транзакций (CLI): импорт CSV в daily_metrics + пересчёт прогнозов.
// Предназначен для запуска по крону или вручную:
//
//	collector -file data/transactions.csv
//	collector -file data/transactions.csv -recalc=false
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"time"

	"alfa-pulse/internal/config"
	"alfa-pulse/internal/db"
	"alfa-pulse/internal/repository"
	"alfa-pulse/internal/service"
)

func main() {
	source := flag.String("source", "csv", "источник: csv | api (эквайринг). По крону обычно api")
	file := flag.String("file", "", "путь к CSV (для -source=csv)")
	days := flag.Int("days", 1, "сколько последних дней тянуть из API (для -source=api)")
	recalc := flag.Bool("recalc", true, "пересчитать прогнозы затронутых участников группы B")
	flag.Parse()

	if *source == "csv" && *file == "" {
		fmt.Fprintln(os.Stderr, "использование: collector -file <transactions.csv>  |  collector -source api -days 1")
		os.Exit(2)
	}

	log := slog.New(slog.NewTextHandler(os.Stderr, nil))
	if err := run(*source, *file, *days, *recalc, log); err != nil {
		log.Error("импорт не удался", "err", err)
		os.Exit(1)
	}
}

func run(source, file string, days int, recalc bool, log *slog.Logger) error {
	cfg := config.Load()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	pool, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	repo := repository.New(pool)
	svc := service.New(repo, service.NewMLClient(cfg.MLServiceURL), log)

	var report *service.ImportReport
	if source == "api" {
		// Боевой режим: тянем транзакции из API эквайринга через провайдер.
		provider := service.NewAcquiringAPIProvider(cfg.AcquiringAPIURL, cfg.AcquiringAPIToken)
		to := time.Now().UTC()
		from := to.AddDate(0, 0, -days)
		log.Info("сбор из API эквайринга", "provider", provider.Name(), "from", from.Format("2006-01-02"))
		txs, err := provider.GetTransactions(ctx, from, to)
		if err != nil {
			return err
		}
		report = &service.ImportReport{Errors: []string{}}
		if err := svc.ImportParsedTransactions(ctx, txs, report); err != nil {
			return err
		}
	} else {
		f, ferr := os.Open(file)
		if ferr != nil {
			return ferr
		}
		defer f.Close()
		report, err = svc.ImportTransactionsCSV(ctx, f)
		if err != nil {
			return err
		}
	}

	out := map[string]any{
		"imported":     report.Imported,
		"skipped":      report.Skipped,
		"days_updated": report.DaysUpdated,
		"errors":       report.Errors,
	}

	if recalc {
		results := []map[string]any{}
		for _, pid := range report.AffectedB {
			res, err := svc.Recalculate(ctx, pid)
			if err != nil {
				if !errors.Is(err, service.ErrControlGroup) {
					log.Warn("пересчёт не удался", "participant", pid, "err", err)
				}
				continue
			}
			item := map[string]any{
				"participant_id": pid.String(),
				"health_index":   res.HealthIndex,
				"model_used":     res.ModelUsed,
			}
			if res.CashGapDate != nil {
				item["cash_gap_date"] = res.CashGapDate.Format("2006-01-02")
			}
			results = append(results, item)
		}
		out["recalculated"] = results
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}
