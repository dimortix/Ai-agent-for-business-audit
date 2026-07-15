import { AlertTriangle, Check, Receipt, TrendingDown, Users, Lightbulb } from "lucide-react";
import { useState } from "react";
import type { Advice } from "../api/types";
import { fmtDateShort } from "../lib/format";

const ruleMeta: Record<string, { icon: typeof Lightbulb; title: string }> = {
  AVG_CHECK_DROP: { icon: Receipt, title: "Средний чек" },
  REVENUE_DECLINE_3D: { icon: TrendingDown, title: "Выручка падает" },
  TRAFFIC_DROP: { icon: Users, title: "Меньше клиентов" },
  CASH_GAP_SOON: { icon: AlertTriangle, title: "Кассовый разрыв" },
};

interface Props {
  advice: Advice;
  onDone?: (id: number) => Promise<void>;
  delay?: number;
}

export default function AdviceCard({ advice, onDone, delay = 0 }: Props) {
  const [busy, setBusy] = useState(false);
  const meta = ruleMeta[advice.rule_code] ?? { icon: Lightbulb, title: "Совет" };
  const Icon = meta.icon;
  const urgent = advice.rule_code === "CASH_GAP_SOON" && !advice.was_action_taken;

  return (
    <div className={`card rise-in p-4 ${delay ? `rise-in-${delay}` : ""} ${advice.was_action_taken ? "opacity-60" : ""}`}>
      <div className="flex items-start gap-3">
        <div
          className="flex size-9 shrink-0 items-center justify-center rounded-xl"
          style={{
            background: urgent ? "var(--color-crit-soft)" : "var(--color-bg)",
            color: urgent ? "var(--color-crit)" : "var(--color-ink-2)",
          }}
        >
          <Icon className="size-4.5" />
        </div>
        <div className="min-w-0 flex-1">
          <div className="flex items-baseline justify-between gap-2">
            <span className="text-xs font-bold uppercase tracking-wide text-ink-3">
              {meta.title}
            </span>
            <span className="shrink-0 text-[11px] text-ink-3">{fmtDateShort(advice.created_at.slice(0, 10))}</span>
          </div>
          <p className="mt-1 text-sm leading-relaxed text-ink">{advice.message}</p>

          {onDone && !advice.was_action_taken && (
            <button
              className="btn-ghost mt-3 !py-2 text-sm"
              disabled={busy}
              onClick={() => {
                setBusy(true);
                void onDone(advice.id).finally(() => setBusy(false));
              }}
            >
              <Check className="size-4" />
              Сделано
            </button>
          )}
          {advice.was_action_taken && (
            <div className="mt-2 inline-flex items-center gap-1 text-xs font-semibold text-good">
              <Check className="size-3.5" /> выполнено
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
