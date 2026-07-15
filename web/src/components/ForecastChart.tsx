import {
  Area,
  ComposedChart,
  CartesianGrid,
  Line,
  ReferenceLine,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from "recharts";
import type { DayFact, ForecastPoint } from "../api/types";
import { fmtDateLong, fmtDateShort, fmtMoney, fmtMoneyCompact } from "../lib/format";

const FACT = "var(--color-series-fact)";
const FORECAST = "var(--color-series-forecast)";

interface Row {
  date: string;
  fact?: number;
  yhat?: number;
  band?: [number, number];
}

function buildRows(fact: DayFact[], forecast: ForecastPoint[]): Row[] {
  const rows: Row[] = fact.map((f) => ({ date: f.date, fact: f.revenue }));
  // мостик: пунктир прогноза начинается от последней фактической точки
  if (fact.length > 0 && forecast.length > 0) {
    const last = fact[fact.length - 1];
    rows[rows.length - 1] = {
      ...rows[rows.length - 1],
      yhat: last.revenue,
      band: [last.revenue, last.revenue],
    };
  }
  for (const p of forecast) {
    rows.push({ date: p.date, yhat: p.yhat, band: [p.lower, p.upper] });
  }
  return rows;
}

function ChartTooltip({ active, payload, label }: {
  active?: boolean;
  payload?: Array<{ dataKey?: string; value?: number | [number, number] }>;
  label?: string;
}) {
  if (!active || !payload?.length || !label) return null;
  const by = (k: string) => payload.find((p) => p.dataKey === k)?.value;
  const fact = by("fact") as number | undefined;
  const yhat = by("yhat") as number | undefined;
  const band = by("band") as [number, number] | undefined;

  return (
    <div className="card border border-line px-3 py-2 text-xs shadow-lg">
      <div className="mb-1 font-semibold">{fmtDateLong(label)}</div>
      {fact !== undefined && (
        <div className="flex items-center gap-1.5">
          <span className="inline-block h-0.5 w-3 rounded" style={{ background: FACT }} />
          Факт: <b className="tabular-nums">{fmtMoney(fact)}</b>
        </div>
      )}
      {yhat !== undefined && fact === undefined && (
        <div className="flex items-center gap-1.5">
          <span className="inline-block h-0.5 w-3 rounded" style={{ background: FORECAST }} />
          Прогноз: <b className="tabular-nums">{fmtMoney(yhat)}</b>
        </div>
      )}
      {band && band[0] !== band[1] && (
        <div className="mt-0.5 text-ink-3">
          интервал: {fmtMoneyCompact(band[0])} – {fmtMoneyCompact(band[1])} ₽
        </div>
      )}
    </div>
  );
}

interface Props {
  fact: DayFact[];
  forecast: ForecastPoint[];
  cashGapDate?: string | null;
}

export default function ForecastChart({ fact, forecast, cashGapDate }: Props) {
  const rows = buildRows(fact, forecast);

  return (
    <div className="card rise-in rise-in-2 p-4 sm:p-5">
      <div className="mb-1 flex items-baseline justify-between">
        <div className="text-sm font-semibold text-ink-2">Выручка: факт и прогноз 14 дней</div>
      </div>

      {/* легенда: ≥2 серии — обязательна */}
      <div className="mb-3 flex flex-wrap items-center gap-x-4 gap-y-1 text-xs text-ink-2">
        <span className="inline-flex items-center gap-1.5">
          <span className="inline-block h-0.5 w-4 rounded" style={{ background: FACT }} />
          факт
        </span>
        <span className="inline-flex items-center gap-1.5">
          <svg width="16" height="2" aria-hidden>
            <line x1="0" y1="1" x2="16" y2="1" stroke={FORECAST} strokeWidth="2" strokeDasharray="4 3" />
          </svg>
          прогноз
        </span>
        <span className="inline-flex items-center gap-1.5">
          <span className="inline-block h-3 w-4 rounded-sm" style={{ background: FORECAST, opacity: 0.12 }} />
          интервал 80%
        </span>
      </div>

      <div className="h-56 sm:h-64">
        <ResponsiveContainer width="100%" height="100%">
          <ComposedChart data={rows} margin={{ top: 8, right: 8, bottom: 0, left: 0 }}>
            <CartesianGrid vertical={false} stroke="var(--color-line)" strokeDasharray="0" />
            <XAxis
              dataKey="date"
              tickFormatter={fmtDateShort}
              tick={{ fontSize: 11, fill: "var(--color-ink-3)" }}
              axisLine={false}
              tickLine={false}
              minTickGap={28}
            />
            <YAxis
              tickFormatter={fmtMoneyCompact}
              tick={{ fontSize: 11, fill: "var(--color-ink-3)" }}
              axisLine={false}
              tickLine={false}
              width={44}
            />
            <Tooltip content={<ChartTooltip />} cursor={{ stroke: "var(--color-ink-3)", strokeDasharray: "3 3" }} />
            <Area
              dataKey="band"
              fill={FORECAST}
              fillOpacity={0.1}
              stroke="none"
              connectNulls
              isAnimationActive={false}
            />
            <Line
              dataKey="fact"
              stroke={FACT}
              strokeWidth={2}
              dot={false}
              activeDot={{ r: 4, strokeWidth: 2, stroke: "var(--color-card)" }}
              connectNulls
              isAnimationActive={false}
            />
            <Line
              dataKey="yhat"
              stroke={FORECAST}
              strokeWidth={2}
              strokeDasharray="6 4"
              dot={false}
              activeDot={{ r: 4, strokeWidth: 2, stroke: "var(--color-card)" }}
              connectNulls
              isAnimationActive={false}
            />
            {cashGapDate && (
              <ReferenceLine
                x={cashGapDate}
                stroke={FORECAST}
                strokeDasharray="4 4"
                label={{
                  value: "разрыв",
                  position: "insideTopRight",
                  fontSize: 10,
                  fill: FORECAST,
                  fontWeight: 700,
                }}
              />
            )}
          </ComposedChart>
        </ResponsiveContainer>
      </div>
    </div>
  );
}
