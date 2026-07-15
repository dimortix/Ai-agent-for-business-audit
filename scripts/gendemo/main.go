// Генератор демо-данных Альфа-Пульс.
//
// Создаёт в data/:
//   - participants.csv        — 2 участника (B «Демо Кофе», A «Контроль»)
//   - transactions.csv        — кофейня: 83 дня истории (заканчивается D-8),
//     последние 2 недели — деградация (чек, трафик, выручка вниз)
//   - transactions_crisis.csv — «кризисные» дни D-7…D-1 (провал −60%)
//   - expenses.sql            — фиксированные расходы 495 000 ₽/мес,
//     аренда с due-day = (сегодня+2), чтобы разрыв попадал в горизонт
//
// Числа подобраны так, чтобы после make demo индекс был в жёлтой зоне (~45)
// с кассовым разрывом через ~10 дней, а после make crisis падал в красную
// (<40) и триггерил push/Telegram-уведомление.
package main

import (
	"fmt"
	"math/rand"
	"os"
	"strings"
	"time"
)

const (
	accountB = "ACC-001"
	accountA = "ACC-002"
)

var weekdayMult = map[time.Weekday]float64{
	time.Monday: 0.80, time.Tuesday: 0.95, time.Wednesday: 1.00,
	time.Thursday: 1.05, time.Friday: 1.35, time.Saturday: 1.30, time.Sunday: 1.15,
}

func main() {
	outDir := "data"
	if len(os.Args) > 1 {
		outDir = os.Args[1]
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		fatal(err)
	}

	rng := rand.New(rand.NewSource(42)) // детерминированные демо-данные
	today := time.Now().UTC().Truncate(24 * time.Hour)

	writeFile(outDir+"/participants.csv", participantsCSV())
	writeFile(outDir+"/transactions.csv", mainTransactions(rng, today))
	writeFile(outDir+"/transactions_crisis.csv", crisisTransactions(rng, today))
	writeFile(outDir+"/expenses.sql", expensesSQL(today))

	fmt.Println("демо-данные созданы в", outDir+"/")
	fmt.Println("  участник группы B: +79001234567 (кофейня «Демо Кофе»)")
	fmt.Println("  участник группы A: +79007654321 (контрольная группа)")
}

func participantsCSV() string {
	return "phone,account_id,group_type,name\n" +
		"+79001234567," + accountB + ",B,Кофейня «Демо Кофе»\n" +
		"+79007654321," + accountA + ",A,Пекарня «Контроль»\n"
}

// mainTransactions: 83 дня, D-90 … D-8.
func mainTransactions(rng *rand.Rand, today time.Time) string {
	var b strings.Builder
	b.WriteString("account_id,date,amount,type\n")

	start := today.AddDate(0, 0, -90)
	end := today.AddDate(0, 0, -8)
	total := int(end.Sub(start).Hours()/24) + 1 // 83 дня

	for i := 0; i < total; i++ {
		day := start.AddDate(0, 0, i)
		left := total - i // 14..1 — хвост деградации

		base := 18200.0 * weekdayMult[day.Weekday()] * (0.94 + rng.Float64()*0.12)
		check := 450.0

		if left <= 14 {
			// плавный спад выручки и чека в последние 2 недели
			k := 1.0 - 0.035*float64(15-left) // 0.965 … 0.51
			base *= k
			check = 450 - 5.5*float64(15-left) // 445 … 373
		}
		if left <= 4 {
			// последние 4 дня строго убывают (правило REVENUE_DECLINE_3D)
			base = 14200 - 1050*float64(4-left) // 14200, 13150, 12100, 11050
			check = 380 - 4*float64(4-left)
		}

		writeDay(&b, rng, accountB, day, base, check)

		// изредка возвраты
		if rng.Float64() < 0.25 {
			ret := 300 + rng.Float64()*600
			fmt.Fprintf(&b, "%s,%s,%.2f,return\n", accountB, day.Format("2006-01-02"), ret)
		}
	}

	// Группа A: пекарня, 30 ровных дней (D-30 … D-1)
	for i := 30; i >= 1; i-- {
		day := today.AddDate(0, 0, -i)
		writeDay(&b, rng, accountA, day, 5600*(0.9+rng.Float64()*0.2), 260)
	}
	return b.String()
}

// crisisTransactions: D-7 … D-1, выручка ~7к и строго вниз (−60% к норме).
func crisisTransactions(rng *rand.Rand, today time.Time) string {
	var b strings.Builder
	b.WriteString("account_id,date,amount,type\n")
	for i := 7; i >= 1; i-- {
		day := today.AddDate(0, 0, -i)
		revenue := 7800 - 250*float64(7-i) // 7800 … 6300, строго убывает
		writeDay(&b, rng, accountB, day, revenue, 310)
	}
	return b.String()
}

// writeDay генерирует count чеков со средним около check на сумму ~revenue.
func writeDay(b *strings.Builder, rng *rand.Rand, account string, day time.Time, revenue, check float64) {
	count := int(revenue / check)
	if count < 1 {
		count = 1
	}
	date := day.Format("2006-01-02")
	for t := 0; t < count; t++ {
		amount := check * (0.80 + rng.Float64()*0.40)
		fmt.Fprintf(b, "%s,%s,%.2f,income\n", account, date, amount)
	}
}

func expensesSQL(today time.Time) string {
	rentDue := today.AddDate(0, 0, 2).Day() // аренда через 2 дня → разрыв в горизонте

	rows := []struct {
		desc string
		amt  int
		due  int
	}{
		{"Аренда помещения", 170000, rentDue},
		{"Зарплаты: расчёт", 115000, 5},
		{"Зарплаты: аванс", 115000, 20},
		{"Закупка кофе и молока", 60000, 15},
		{"Коммуналка и интернет", 20000, 25},
		{"Эквайринг и обслуживание", 15000, 28},
	} // итого 495 000 ₽/мес

	var b strings.Builder
	b.WriteString("-- Фиксированные расходы демо-кофейни (генерируется gendemo)\n")
	for _, r := range rows {
		fmt.Fprintf(&b,
			"INSERT INTO fixed_expenses (participant_id, description, amount, due_day_of_month)\n"+
				"SELECT id, '%s', %d, %d FROM participants WHERE account_id = '%s'\n"+
				"ON CONFLICT (participant_id, description) DO UPDATE SET amount = EXCLUDED.amount, due_day_of_month = EXCLUDED.due_day_of_month;\n",
			r.desc, r.amt, r.due, accountB)
	}
	return b.String()
}

func writeFile(path, content string) {
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		fatal(err)
	}
	fmt.Println("создан:", path)
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "gendemo:", err)
	os.Exit(1)
}
