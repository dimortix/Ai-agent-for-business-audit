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
	file := flag.String("file", "", "путь к CSV с транзакциями (account_id,date,amount,type)")
	recalc := flag.Bool("recalc", true, "пересчитать прогнозы затронутых участников группы B")
	flag.Parse()

	if *file == "" {
		fmt.Fprintln(os.Stderr, "использование: collector -file <transactions.csv> [-recalc=false]")
		os.Exit(2)
	}

	log := slog.New(slog.NewTextHandler(os.Stderr, nil))
	if err := run(*file, *recalc, log); err != nil {
		log.Error("импорт не удался", "err", err)
		os.Exit(1)
	}
}

func run(file string, recalc bool, log *slog.Logger) error {
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

	f, err := os.Open(file)
	if err != nil {
		return err
	}
	defer f.Close()

	report, err := svc.ImportTransactionsCSV(ctx, f)
	if err != nil {
		return err
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
