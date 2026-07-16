import { ArrowDownLeft, ArrowUpRight } from "lucide-react";
import type { Operation } from "../api/types";
import { fmtMoney } from "../lib/format";

const timeFmt = new Intl.DateTimeFormat("ru-RU", {
  day: "numeric",
  month: "short",
  hour: "2-digit",
  minute: "2-digit",
});

function relative(iso: string): string {
  const d = new Date(iso);
  const now = new Date();
  const sameDay = d.toDateString() === now.toDateString();
  const yest = new Date(now);
  yest.setDate(now.getDate() - 1);
  const hm = d.toLocaleTimeString("ru-RU", { hour: "2-digit", minute: "2-digit" });
  if (sameDay) return `сегодня, ${hm}`;
  if (d.toDateString() === yest.toDateString()) return `вчера, ${hm}`;
  return timeFmt.format(d);
}

/* Лента последних операций — «живой» бизнес прямо на экране (ТЗ V2, п. 6). */
export default function OperationsCard({ items, delay = 0 }: {
  items: Operation[];
  delay?: number;
}) {
  if (!items.length) return null;
  return (
    <div className={`card rise-in p-4 sm:p-5 ${delay ? `rise-in-${delay}` : ""}`}>
      <div className="mb-3 text-sm font-semibold text-ink-2">Последние операции</div>
      <div className="flex flex-col">
        {items.map((op, i) => {
          const isReturn = op.type === "return";
          return (
            <div
              key={op.paid_at + i}
              className={`flex items-center gap-3 py-2.5 ${i > 0 ? "border-t border-line" : ""}`}
            >
              <div
                className="flex size-8 shrink-0 items-center justify-center rounded-lg"
                style={{
                  background: isReturn ? "var(--color-crit-soft)" : "var(--color-good-soft)",
                  color: isReturn ? "var(--color-crit)" : "var(--color-good)",
                }}
              >
                {isReturn ? <ArrowDownLeft className="size-4" /> : <ArrowUpRight className="size-4" />}
              </div>
              <div className="min-w-0 flex-1">
                <div className="text-sm font-medium">
                  {isReturn ? "Возврат" : "Оплата картой"}
                </div>
                <div className="text-xs text-ink-3">{relative(op.paid_at)}</div>
              </div>
              <div
                className="shrink-0 text-sm font-semibold tabular-nums"
                style={{ color: isReturn ? "var(--color-crit)" : "var(--color-ink)" }}
              >
                {isReturn ? "−" : "+"}
                {fmtMoney(op.amount)}
              </div>
            </div>
          );
        })}
      </div>
    </div>
  );
}
