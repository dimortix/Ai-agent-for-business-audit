import { Table2, ChartColumn } from "lucide-react";
import { useCallback, useEffect, useState } from "react";
import { useNavigate } from "react-router-dom";
import { api, ApiError } from "../api/client";
import type { Analytics as AnalyticsData } from "../api/types";
import { ErrorNote, Spinner, StatCard } from "../components/Bits";
import MetricChart from "../components/MetricChart";
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
  const [view, setView] = useState<"charts" | "table">("charts");
  const [error, setError] = useState("");

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
      </div>

      {!data ? (
        <Spinner />
      ) : days.length === 0 ? (
        <ErrorNote message="За выбранный период данных нет" />
      ) : (
        <>
          <div className="grid grid-cols-2 gap-3 sm:grid-cols-4">
            <StatCard label="Чистая выручка" value={fmtMoneyCompact(total) + " ₽"} delay={1} />
            <StatCard label="Покупок" value={txTotal.toLocaleString("ru-RU")} delay={1} />
            <StatCard label="Средний чек" value={fmtMoney(avgCheck)} delay={2} />
            <StatCard label="Возвраты" value={fmtMoneyCompact(returnsTotal) + " ₽"} delay={2} />
          </div>

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
