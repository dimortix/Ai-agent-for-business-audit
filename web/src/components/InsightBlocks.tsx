// Блоки расширенной аналитики: авто-инсайты, сценарии, денежный календарь,
// бенчмарк и точность модели.
import { AlertTriangle, CalendarDays, Sparkles, Target, Trophy } from "lucide-react";
import type { CalendarEvent, Insights, Scenario } from "../api/types";
import { fmtDateShort, fmtMoney, fmtMoneyCompact } from "../lib/format";

/* Авто-инсайты: короткие выводы, сгенерированные из данных. */
export function InsightsStrip({ texts, delay = 0 }: { texts: string[]; delay?: number }) {
  if (!texts.length) return null;
  return (
    <div className={`flex flex-col gap-2 md:grid md:grid-cols-2 rise-in ${delay ? `rise-in-${delay}` : ""}`}>
      {texts.slice(0, 4).map((t) => (
        <div key={t} className="card flex items-start gap-3 p-4">
          <div className="flex size-8 shrink-0 items-center justify-center rounded-lg bg-brand-soft text-brand">
            <Sparkles className="size-4" />
          </div>
          <p className="text-sm leading-relaxed text-ink-2">{t}</p>
        </div>
      ))}
    </div>
  );
}

const scenarioMeta = [
  { key: "optimistic", label: "Оптимистичный", color: "var(--color-good)" },
  { key: "base", label: "Базовый", color: "var(--color-series-fact)" },
  { key: "pessimistic", label: "Пессимистичный", color: "var(--color-crit)" },
] as const;

/* Сценарии из доверительного интервала Prophet: верх/центр/низ. */
export function ScenariosCard({ scenarios, delay = 0 }: {
  scenarios: NonNullable<Insights["scenarios"]>;
  delay?: number;
}) {
  return (
    <div className={`card rise-in p-4 sm:p-5 ${delay ? `rise-in-${delay}` : ""}`}>
      <div className="mb-1 text-sm font-semibold text-ink-2">Сценарии на 14 дней</div>
      <p className="mb-3 text-[11px] leading-relaxed text-ink-3">
        Границы доверительного интервала ML-прогноза: что будет при удачном и
        неудачном раскладе
      </p>
      <div className="flex flex-col gap-2.5">
        {scenarioMeta.map(({ key, label, color }) => {
          const s: Scenario = scenarios[key];
          return (
            <div key={key} className="flex items-center gap-2.5 text-sm">
              <span className="size-2.5 shrink-0 rounded-full" style={{ background: color }} />
              <span className="w-32 shrink-0 font-medium">{label}</span>
              <span className="tabular-nums text-ink-2">{fmtMoneyCompact(s.total_14d)} ₽</span>
              <span className="ml-auto text-right text-xs">
                {s.gap_date ? (
                  <span className="font-semibold" style={{ color: "var(--color-crit)" }}>
                    разрыв {fmtDateShort(s.gap_date)}
                  </span>
                ) : (
                  <span className="font-semibold" style={{ color: "var(--color-good)" }}>
                    без разрыва
                  </span>
                )}
              </span>
            </div>
          );
        })}
      </div>
    </div>
  );
}

/* Денежный календарь: обязательные платежи и расчётный баланс после каждого. */
export function CashCalendarCard({ events, delay = 0 }: {
  events: CalendarEvent[];
  delay?: number;
}) {
  if (!events.length) return null;
  return (
    <div className={`card rise-in p-4 sm:p-5 ${delay ? `rise-in-${delay}` : ""}`}>
      <div className="mb-1 flex items-center gap-2 text-sm font-semibold text-ink-2">
        <CalendarDays className="size-4" /> Денежный календарь · 30 дней
      </div>
      <p className="mb-3 text-[11px] text-ink-3">
        Обязательные платежи и остаток после каждого (с учётом прогноза поступлений)
      </p>
      <div className="flex flex-col">
        {events.slice(0, 8).map((e, i) => (
          <div
            key={e.date + e.description}
            className={`flex items-center gap-3 py-2.5 text-sm ${i > 0 ? "border-t border-line" : ""}`}
          >
            <div className="w-14 shrink-0 text-xs font-semibold text-ink-3">
              {fmtDateShort(e.date)}
            </div>
            <div className="min-w-0 flex-1 truncate">{e.description}</div>
            <div className="shrink-0 tabular-nums text-ink-2">−{fmtMoneyCompact(e.amount)} ₽</div>
            <div
              className="w-24 shrink-0 text-right text-xs font-semibold tabular-nums"
              style={{ color: e.risk ? "var(--color-crit)" : "var(--color-ink-3)" }}
              title={`Остаток после платежа: ${fmtMoney(e.balance_after)}`}
            >
              {e.risk && <AlertTriangle className="mr-1 inline size-3.5" />}
              {fmtMoneyCompact(e.balance_after)} ₽
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}

/* Бенчмарк внутри пилота + точность модели. */
export function BenchmarkCards({ benchmark, mape, delay = 0 }: {
  benchmark?: Insights["benchmark"];
  mape?: number;
  delay?: number;
}) {
  if (!benchmark && mape === undefined) return null;
  return (
    <div className={`grid gap-3 sm:grid-cols-2 rise-in ${delay ? `rise-in-${delay}` : ""}`}>
      {benchmark && (
        <div className="card flex items-center gap-3 p-4">
          <div className="flex size-10 shrink-0 items-center justify-center rounded-xl bg-warn-soft" style={{ color: "var(--color-warn)" }}>
            <Trophy className="size-5" />
          </div>
          <div>
            <div className="text-sm font-bold">
              №{benchmark.rank} из {benchmark.total} в пилоте
            </div>
            <div className="text-xs text-ink-3">
              по росту выручки за 30 дней
              {benchmark.growth_pct !== undefined &&
                ` (${benchmark.growth_pct >= 0 ? "+" : ""}${(benchmark.growth_pct * 100).toFixed(0)}%)`}
            </div>
          </div>
        </div>
      )}
      {mape !== undefined && (
        <div className="card flex items-center gap-3 p-4">
          <div className="flex size-10 shrink-0 items-center justify-center rounded-xl bg-good-soft text-good">
            <Target className="size-5" />
          </div>
          <div>
            <div className="text-sm font-bold">Точность прогноза {(100 - mape * 100).toFixed(0)}%</div>
            <div className="text-xs text-ink-3">
              средняя ошибка {(mape * 100).toFixed(0)}% — прогноз сверяется с фактом ежедневно
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
