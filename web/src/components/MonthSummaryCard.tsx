import { fmtMoney } from "../lib/format";

/* «Итог за 30 дней»: заработали − обязательные расходы = сколько остаётся.
   Прямо отвечает на вопрос «сколько в итоге остаётся» — чистая выручка этого
   не показывает, так как не вычитает аренду и зарплаты. «Обязательные расходы» —
   полный месячный объём (как на странице «Расходы» и в календаре). */
export default function MonthSummaryCard({ earned, expenses, delay = 0 }: {
  earned: number;
  expenses: number;
  delay?: number;
}) {
  const profit = earned - expenses;
  const positive = profit >= 0;
  const color = positive ? "var(--color-good)" : "var(--color-crit)";

  return (
    <div className={`card rise-in p-4 sm:p-5 ${delay ? `rise-in-${delay}` : ""}`}>
      <div className="mb-3 text-sm font-semibold text-ink-2">Итог за 30 дней</div>

      <div className="flex items-center justify-between py-1.5 text-sm">
        <span className="text-ink-2">Заработали</span>
        <span className="tabular-nums font-medium">+{fmtMoney(earned)}</span>
      </div>
      <div className="flex items-center justify-between py-1.5 text-sm">
        <span className="text-ink-2">Обязательные расходы</span>
        <span className="tabular-nums font-medium">−{fmtMoney(expenses)}</span>
      </div>

      <div className="mt-1.5 flex items-center justify-between border-t border-line pt-3">
        <span className="text-sm font-bold">{positive ? "Остаётся" : "Убыток"}</span>
        <span className="text-lg font-bold tabular-nums" style={{ color }}>
          {positive ? "" : "−"}
          {fmtMoney(Math.abs(profit))}
        </span>
      </div>
      <p className="mt-1.5 text-[11px] leading-relaxed text-ink-3">
        {positive
          ? "Прибыль после всех обязательных трат за месяц."
          : "Расходы превышают выручку — бизнес живёт за счёт подушки. Смотрите советы."}
      </p>
    </div>
  );
}
