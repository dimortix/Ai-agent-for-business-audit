import {
  Bar,
  BarChart,
  CartesianGrid,
  Line,
  LineChart,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from "recharts";
import { fmtDateLong, fmtDateShort } from "../lib/format";

interface Props {
  title: string;
  data: Array<{ date: string }>;
  dataKey: string;
  kind?: "bar" | "line";
  color?: string;
  format: (v: number) => string;
  formatAxis?: (v: number) => string;
  delay?: number;
}

function MetricTooltip({ active, payload, label, format }: {
  active?: boolean;
  payload?: Array<{ value?: number }>;
  label?: string;
  format: (v: number) => string;
}) {
  if (!active || !payload?.length || label === undefined) return null;
  const v = payload[0]?.value;
  return (
    <div className="card border border-line px-3 py-2 text-xs shadow-lg">
      <div className="font-semibold">{fmtDateLong(label)}</div>
      <div className="mt-0.5 tabular-nums text-ink-2">{v !== undefined ? format(v) : "—"}</div>
    </div>
  );
}

/* Один показатель — один график, одна ось (никаких dual-axis). */
export default function MetricChart({
  title, data, dataKey, kind = "bar",
  color = "var(--color-series-fact)",
  format, formatAxis, delay = 0,
}: Props) {
  const axis = formatAxis ?? ((v: number) => String(v));

  return (
    <div className={`card rise-in p-4 sm:p-5 ${delay ? `rise-in-${delay}` : ""}`}>
      <div className="mb-3 text-sm font-semibold text-ink-2">{title}</div>
      <div className="h-44 sm:h-52">
        <ResponsiveContainer width="100%" height="100%">
          {kind === "bar" ? (
            <BarChart data={data} margin={{ top: 4, right: 8, bottom: 0, left: 0 }}>
              <CartesianGrid vertical={false} stroke="var(--color-line)" />
              <XAxis
                dataKey="date" tickFormatter={fmtDateShort} axisLine={false} tickLine={false}
                tick={{ fontSize: 11, fill: "var(--color-ink-3)" }} minTickGap={28}
              />
              <YAxis
                tickFormatter={axis} axisLine={false} tickLine={false}
                tick={{ fontSize: 11, fill: "var(--color-ink-3)" }} width={44}
              />
              <Tooltip
                content={<MetricTooltip format={format} />}
                cursor={{ fill: "var(--color-bg)" }}
              />
              <Bar dataKey={dataKey} fill={color} radius={[4, 4, 0, 0]} maxBarSize={22} isAnimationActive={false} />
            </BarChart>
          ) : (
            <LineChart data={data} margin={{ top: 4, right: 8, bottom: 0, left: 0 }}>
              <CartesianGrid vertical={false} stroke="var(--color-line)" />
              <XAxis
                dataKey="date" tickFormatter={fmtDateShort} axisLine={false} tickLine={false}
                tick={{ fontSize: 11, fill: "var(--color-ink-3)" }} minTickGap={28}
              />
              <YAxis
                tickFormatter={axis} axisLine={false} tickLine={false}
                tick={{ fontSize: 11, fill: "var(--color-ink-3)" }} width={44}
              />
              <Tooltip
                content={<MetricTooltip format={format} />}
                cursor={{ stroke: "var(--color-ink-3)", strokeDasharray: "3 3" }}
              />
              <Line
                dataKey={dataKey} stroke={color} strokeWidth={2} dot={false}
                activeDot={{ r: 4, strokeWidth: 2, stroke: "var(--color-card)" }}
                isAnimationActive={false}
              />
            </LineChart>
          )}
        </ResponsiveContainer>
      </div>
    </div>
  );
}
