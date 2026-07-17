import { Table2, ChartColumn, Download, FileText } from "lucide-react";
import { useCallback, useEffect, useState } from "react";
import { useNavigate } from "react-router-dom";
import { api, ApiError, loadUser } from "../api/client";
import type { Analytics as AnalyticsData, Insights } from "../api/types";
import { ErrorNote, Spinner, StatCard } from "../components/Bits";
import { BenchmarkCards, CashCalendarCard, InsightsStrip } from "../components/InsightBlocks";
import MetricChart from "../components/MetricChart";
import MonthSummaryCard from "../components/MonthSummaryCard";
import { daysAgoISO, fmtDateShort, fmtMoney, fmtMoneyCompact, todayISO } from "../lib/format";

const presets = [
  { label: "7 дней", days: 7 },
  { label: "30 дней", days: 30 },
  { label: "90 дней", days: 90 },
];

export default function Analytics() {
  const navigate = useNavigate();
  const [rangeDays, setRangeDays] = useState(30);
  const [data, setData] = useState<AnalyticsData | null>(null);
  const [insights, setInsights] = useState<Insights | null>(null);
  const [view, setView] = useState<"charts" | "table">("charts");
  const [error, setError] = useState("");
  const isGroupB = loadUser()?.group_type === "B";

  useEffect(() => {
    void api<Insights>("/api/insights").then(setInsights).catch(() => setInsights(null));
  }, []);

  const load = useCallback(
    (days: number) => {
      setData(null);
      setError("");
      api<AnalyticsData>(`/api/analytics?from=${daysAgoISO(days - 1)}&to=${todayISO()}`)
        .then(setData)
        .catch((err) => {
          if (err instanceof ApiError && err.status === 401) {
            navigate("/login", { replace: true });
            return;
          }
          setError(err instanceof ApiError ? err.message : "Не удалось загрузить аналитику");
        });
    },
    [navigate],
  );

  useEffect(() => load(rangeDays), [rangeDays, load]);

  if (error) return <ErrorNote message={error} />;

  const days = data?.days ?? [];
  const total = days.reduce((s, d) => s + d.net, 0);
  const txTotal = days.reduce((s, d) => s + d.transactions, 0);
  const returnsTotal = days.reduce((s, d) => s + d.returns, 0);
  const avgCheck = txTotal > 0 ? days.reduce((s, d) => s + d.avg_check * d.transactions, 0) / txTotal : 0;

  return (
    <div className="flex flex-col gap-3">
      {/* фильтры — одной строкой над графиками */}
      <div className="flex flex-wrap items-center gap-2">
        {presets.map((p) => (
          <button
            key={p.days}
            className="chip"
            data-active={rangeDays === p.days}
            onClick={() => setRangeDays(p.days)}
          >
            {p.label}
          </button>
        ))}
        <div className="flex-1" />
        <button
          className="chip"
          data-active={view === "table"}
          onClick={() => setView(view === "charts" ? "table" : "charts")}
          title={view === "charts" ? "Показать таблицу" : "Показать графики"}
        >
          {view === "charts" ? <Table2 className="size-3.5" /> : <ChartColumn className="size-3.5" />}
          {view === "charts" ? "таблица" : "графики"}
        </button>
        <a
          className="chip"
          href={`/api/analytics/export?from=${daysAgoISO(rangeDays - 1)}&to=${todayISO()}`}
          download
          title="Скачать сырые метрики CSV за выбранный период"
        >
          <Download className="size-3.5" />
          CSV
        </a>
        {isGroupB && (
          <a
            className="chip"
            href={`/api/report/monthly?month=${todayISO().slice(0, 7)}`}
            download
            title="Подробный отчёт за месяц (для бухгалтерии и кредита)"
          >
            <FileText className="size-3.5" />
            отчёт за месяц
          </a>
        )}
      </div>

      {!data ? (
        <Spinner />
      ) : days.length === 0 ? (
        <ErrorNote message="За выбранный период данных нет" />
      ) : (
        <>
          {/* дельты приходят для окна 30 дней — показываем их на этом пресете */}
          <div className="grid grid-cols-2 gap-3 sm:grid-cols-4">
            <StatCard
              label="Чистая выручка"
              value={fmtMoneyCompact(total) + " ₽"}
              delta={rangeDays === 30 ? insights?.period?.revenue_delta : undefined}
              hint={rangeDays === 30 ? "к прошлым 30 дням" : undefined}
              delay={1}
            />
            <StatCard
              label="Покупок"
              value={txTotal.toLocaleString("ru-RU")}
              delta={rangeDays === 30 ? insights?.period?.tx_delta : undefined}
              delay={1}
            />
            <StatCard
              label="Средний чек"
              value={fmtMoney(avgCheck)}
              delta={rangeDays === 30 ? insights?.period?.avg_check_delta : undefined}
              delay={2}
            />
            <StatCard
              label="Возвраты"
              value={fmtMoneyCompact(returnsTotal) + " ₽"}
              hint={
                insights?.period && insights.period.current.returns_rate > 0
                  ? `${(insights.period.current.returns_rate * 100).toFixed(1)}% от оборота`
                  : undefined
              }
              delay={2}
            />
          </div>

          {/* итог за 30 дней: сколько остаётся после обязательных расходов */}
          {isGroupB && insights?.period && insights.monthly_expenses !== undefined && (
            <MonthSummaryCard
              earned={insights.period.current.revenue}
              expenses={insights.monthly_expenses}
              delay={2}
            />
          )}

          {/* экономика с учётом расходов (группа B) */}
          {isGroupB && insights && (insights.margin_pct !== undefined || insights.break_even_daily !== undefined) && (
            <div className="grid grid-cols-2 gap-3 sm:grid-cols-3">
              {insights.margin_pct !== undefined && (
                <StatCard
                  label="Операционная маржа"
                  value={`${(insights.margin_pct * 100).toFixed(0)}%`}
                  hint="после обязательных расходов"
                  delta={insights.margin_pct}
                  delay={2}
                />
              )}
              {insights.break_even_daily !== undefined && (
                <StatCard
                  label="Точка безубыточности"
                  value={fmtMoneyCompact(insights.break_even_daily) + " ₽/день"}
                  hint="нужно зарабатывать в день"
                  delay={2}
                />
              )}
              {insights.days_above_break_even !== undefined && (
                <StatCard
                  label="Прибыльных дней"
                  value={`${insights.days_above_break_even} из 30`}
                  hint="выручка выше расходов"
                  delay={3}
                />
              )}
            </div>
          )}

          {view === "charts" ? (
            <>
              <MetricChart
                title="Чистая выручка по дням"
                data={days}
                dataKey="net"
                format={fmtMoney}
                formatAxis={fmtMoneyCompact}
                delay={2}
              />
              <div className="flex flex-col gap-3 md:grid md:grid-cols-2">
                <MetricChart
                  title="Средний чек"
                  data={days}
                  dataKey="avg_check"
                  kind="line"
                  format={fmtMoney}
                  formatAxis={fmtMoneyCompact}
                  delay={3}
                />
                <MetricChart
                  title="Покупки в день"
                  data={days}
                  dataKey="transactions"
                  format={(v) => `${v} покупок`}
                  delay={4}
                />
              </div>

              {insights?.weekday_profile && (
                <div className="flex flex-col gap-3 md:grid md:grid-cols-2 md:items-start">
                  <MetricChart
                    title="Профиль недели: средняя выручка по дням"
                    data={insights.weekday_profile.map((w) => ({
                      label: w.label,
                      avg: w.avg_revenue,
                    }))}
                    dataKey="avg"
                    xKey="label"
                    format={fmtMoney}
                    formatAxis={fmtMoneyCompact}
                    delay={3}
                  />
                  <div className="flex flex-col gap-3">
                    {isGroupB && (
                      <BenchmarkCards
                        benchmark={insights.benchmark}
                        mape={insights.forecast_mape}
                        delay={3}
                      />
                    )}
                    <InsightsStrip texts={insights.insights} delay={4} />
                  </div>
                </div>
              )}

              {isGroupB && insights?.profit_series && insights.profit_series.length > 0 && (
                <MetricChart
                  title="Чистая прибыль по дням (выручка − расходы)"
                  data={insights.profit_series.map((p) => ({ date: p.date, profit: p.profit }))}
                  dataKey="profit"
                  kind="line"
                  format={fmtMoney}
                  formatAxis={fmtMoneyCompact}
                  color="var(--color-good)"
                  delay={3}
                />
              )}

              {isGroupB && insights?.balance_projection && insights.balance_projection.length > 0 && (
                <MetricChart
                  title="Прогноз денег на счёте · 30 дней"
                  data={insights.balance_projection.map((b) => ({ date: b.date, balance: b.balance }))}
                  dataKey="balance"
                  kind="line"
                  format={fmtMoney}
                  formatAxis={fmtMoneyCompact}
                  delay={4}
                />
              )}

              {isGroupB && insights?.cash_calendar && (
                <CashCalendarCard events={insights.cash_calendar} delay={4} />
              )}
            </>
          ) : (
            <div className="card rise-in overflow-x-auto">
              <table className="w-full text-sm">
                <thead>
                  <tr className="border-b border-line text-left text-xs text-ink-3">
                    <th className="px-4 py-3 font-semibold">Дата</th>
                    <th className="px-4 py-3 text-right font-semibold">Выручка</th>
                    <th className="px-4 py-3 text-right font-semibold">Возвраты</th>
                    <th className="px-4 py-3 text-right font-semibold">Покупки</th>
                    <th className="px-4 py-3 text-right font-semibold">Ср. чек</th>
                  </tr>
                </thead>
                <tbody>
                  {[...days].reverse().map((d) => (
                    <tr key={d.date} className="border-b border-line last:border-0">
                      <td className="px-4 py-2.5">{fmtDateShort(d.date)}</td>
                      <td className="px-4 py-2.5 text-right tabular-nums">{fmtMoney(d.net)}</td>
                      <td className="px-4 py-2.5 text-right tabular-nums text-ink-3">
                        {d.returns > 0 ? fmtMoney(d.returns) : "—"}
                      </td>
                      <td className="px-4 py-2.5 text-right tabular-nums">{d.transactions}</td>
                      <td className="px-4 py-2.5 text-right tabular-nums">{fmtMoney(d.avg_check)}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </>
      )}
    </div>
  );
}
