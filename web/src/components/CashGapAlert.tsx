import { AlertTriangle, ChevronRight } from "lucide-react";
import { Link } from "react-router-dom";
import { fmtDateLong } from "../lib/format";

export default function CashGapAlert({ date }: { date: string }) {
  return (
    <Link
      to="/advice"
      className="card rise-in rise-in-1 flex items-center gap-3 border border-crit/20 p-4"
      style={{ background: "var(--color-crit-soft)" }}
    >
      <div className="flex size-10 shrink-0 items-center justify-center rounded-xl bg-crit">
        <AlertTriangle className="size-5 text-white" />
      </div>
      <div className="min-w-0 flex-1">
        <div className="text-sm font-bold text-ink">
          Возможен кассовый разрыв {fmtDateLong(date)}
        </div>
        <div className="mt-0.5 text-xs text-ink-2">
          Денег может не хватить на обязательные платежи — смотрите, что сделать
        </div>
      </div>
      <ChevronRight className="size-5 shrink-0 text-ink-3" />
    </Link>
  );
}
