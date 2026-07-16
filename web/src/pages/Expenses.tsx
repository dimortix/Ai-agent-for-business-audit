import { CalendarClock, Plus, Repeat, Trash2, Wallet } from "lucide-react";
import { useCallback, useEffect, useState } from "react";
import { useNavigate } from "react-router-dom";
import { api, ApiError } from "../api/client";
import type { MyExpenses } from "../api/types";
import { ErrorNote, Spinner } from "../components/Bits";
import { fmtMoney, fmtDateShort, todayISO } from "../lib/format";

const categories = [
  { key: "rent", label: "Аренда" },
  { key: "salary", label: "Зарплаты" },
  { key: "supplies", label: "Закупки" },
  { key: "taxes", label: "Налоги" },
  { key: "loan", label: "Кредиты" },
  { key: "utilities", label: "Коммуналка" },
  { key: "other", label: "Прочее" },
];

const catLabel = (k: string) => categories.find((c) => c.key === k)?.label ?? "Прочее";

export default function Expenses() {
  const navigate = useNavigate();
  const [data, setData] = useState<MyExpenses | null>(null);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  const [fixed, setFixed] = useState({ description: "", amount: "", due: "", category: "rent" });
  const [oneOff, setOneOff] = useState({ description: "", amount: "", date: todayISO(), category: "supplies" });

  const load = useCallback(() => {
    setError("");
    api<MyExpenses>("/api/my/expenses")
      .then(setData)
      .catch((err) => {
        if (err instanceof ApiError && err.status === 401) {
          navigate("/login", { replace: true });
          return;
        }
        setError(err instanceof ApiError ? err.message : "Не удалось загрузить расходы");
      });
  }, [navigate]);

  useEffect(load, [load]);

  async function addFixed(e: React.FormEvent) {
    e.preventDefault();
    setBusy(true);
    setError("");
    try {
      await api("/api/my/expenses/fixed", {
        method: "POST",
        body: JSON.stringify({
          description: fixed.description.trim(),
          amount: Number(fixed.amount),
          due_day_of_month: Number(fixed.due),
          category: fixed.category,
        }),
      });
      setFixed({ description: "", amount: "", due: "", category: "rent" });
      load();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Не удалось сохранить");
    } finally {
      setBusy(false);
    }
  }

  async function addOneOff(e: React.FormEvent) {
    e.preventDefault();
    setBusy(true);
    setError("");
    try {
      await api("/api/my/expenses/one-off", {
        method: "POST",
        body: JSON.stringify({
          description: oneOff.description.trim(),
          amount: Number(oneOff.amount),
          date: oneOff.date,
          category: oneOff.category,
        }),
      });
      setOneOff({ description: "", amount: "", date: todayISO(), category: "supplies" });
      load();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Не удалось сохранить");
    } finally {
      setBusy(false);
    }
  }

  async function delFixed(description: string) {
    await api(`/api/my/expenses/fixed?description=${encodeURIComponent(description)}`, { method: "DELETE" });
    load();
  }
  async function delOneOff(id: number) {
    await api(`/api/my/expenses/one-off/${id}`, { method: "DELETE" });
    load();
  }

  if (!data && !error) return <Spinner />;

  const monthly = data?.monthly_total ?? 0;

  return (
    <div className="flex flex-col gap-3">
      <div className="card rise-in flex items-center gap-3 p-4">
        <div className="flex size-10 shrink-0 items-center justify-center rounded-xl bg-brand-soft text-brand">
          <Wallet className="size-5" />
        </div>
        <div>
          <div className="text-sm font-bold">Мои расходы</div>
          <div className="text-xs text-ink-3">
            Обязательные платежи: <b>{fmtMoney(monthly)}/мес</b>. Учтены в прогнозе разрыва и прибыли.
          </div>
        </div>
      </div>

      {error && <ErrorNote message={error} />}

      <div className="flex flex-col gap-3 md:grid md:grid-cols-2 md:items-start">
        {/* Регулярные платежи */}
        <div className="card rise-in rise-in-1 p-4 sm:p-5">
          <div className="mb-3 flex items-center gap-2 text-sm font-semibold text-ink-2">
            <Repeat className="size-4" /> Регулярные платежи
          </div>
          <div className="flex flex-col">
            {(data?.fixed ?? []).map((e, i) => (
              <div key={e.description} className={`flex items-center gap-2 py-2.5 text-sm ${i > 0 ? "border-t border-line" : ""}`}>
                <div className="min-w-0 flex-1">
                  <div className="truncate font-medium">{e.description}</div>
                  <div className="text-xs text-ink-3">{catLabel(e.category)} · до {e.due_day_of_month} числа</div>
                </div>
                <div className="shrink-0 tabular-nums">{fmtMoney(e.amount)}</div>
                <button className="rounded-lg p-1.5 text-ink-3 hover:bg-crit-soft hover:text-crit" onClick={() => void delFixed(e.description)}>
                  <Trash2 className="size-4" />
                </button>
              </div>
            ))}
            {(!data?.fixed || data.fixed.length === 0) && (
              <div className="py-3 text-center text-xs text-ink-3">пока нет — добавьте аренду, зарплаты, кредит…</div>
            )}
          </div>
          <form onSubmit={addFixed} className="mt-3 flex flex-col gap-2">
            <input className="input !py-2 text-sm" placeholder="Название (Аренда, Кредит…)" required maxLength={200}
              value={fixed.description} onChange={(e) => setFixed({ ...fixed, description: e.target.value })} />
            <div className="flex gap-2">
              <input className="input !py-2 text-sm" placeholder="Сумма ₽" type="number" min="1" required
                value={fixed.amount} onChange={(e) => setFixed({ ...fixed, amount: e.target.value })} />
              <input className="input !py-2 text-sm !w-28" placeholder="День" type="number" min="1" max="31" required
                value={fixed.due} onChange={(e) => setFixed({ ...fixed, due: e.target.value })} />
            </div>
            <div className="flex gap-2">
              <select className="input !py-2 text-sm" value={fixed.category} onChange={(e) => setFixed({ ...fixed, category: e.target.value })}>
                {categories.map((c) => <option key={c.key} value={c.key}>{c.label}</option>)}
              </select>
              <button className="btn-primary !py-2 text-sm" disabled={busy}>
                <Plus className="size-4" /> Добавить
              </button>
            </div>
          </form>
        </div>

        {/* Разовые расходы */}
        <div className="card rise-in rise-in-2 p-4 sm:p-5">
          <div className="mb-3 flex items-center gap-2 text-sm font-semibold text-ink-2">
            <CalendarClock className="size-4" /> Разовые расходы
          </div>
          <div className="flex flex-col">
            {(data?.one_off ?? []).map((e, i) => (
              <div key={e.id} className={`flex items-center gap-2 py-2.5 text-sm ${i > 0 ? "border-t border-line" : ""}`}>
                <div className="min-w-0 flex-1">
                  <div className="truncate font-medium">{e.description}</div>
                  <div className="text-xs text-ink-3">{catLabel(e.category)} · {fmtDateShort(e.date)}</div>
                </div>
                <div className="shrink-0 tabular-nums">{fmtMoney(e.amount)}</div>
                <button className="rounded-lg p-1.5 text-ink-3 hover:bg-crit-soft hover:text-crit" onClick={() => void delOneOff(e.id)}>
                  <Trash2 className="size-4" />
                </button>
              </div>
            ))}
            {(!data?.one_off || data.one_off.length === 0) && (
              <div className="py-3 text-center text-xs text-ink-3">пока нет — закупка товара, ремонт…</div>
            )}
          </div>
          <form onSubmit={addOneOff} className="mt-3 flex flex-col gap-2">
            <input className="input !py-2 text-sm" placeholder="Что (закупка зерна, ремонт…)" required maxLength={200}
              value={oneOff.description} onChange={(e) => setOneOff({ ...oneOff, description: e.target.value })} />
            <div className="flex gap-2">
              <input className="input !py-2 text-sm" placeholder="Сумма ₽" type="number" min="1" required
                value={oneOff.amount} onChange={(e) => setOneOff({ ...oneOff, amount: e.target.value })} />
              <input className="input !py-2 text-sm" type="date" required
                value={oneOff.date} onChange={(e) => setOneOff({ ...oneOff, date: e.target.value })} />
            </div>
            <div className="flex gap-2">
              <select className="input !py-2 text-sm" value={oneOff.category} onChange={(e) => setOneOff({ ...oneOff, category: e.target.value })}>
                {categories.map((c) => <option key={c.key} value={c.key}>{c.label}</option>)}
              </select>
              <button className="btn-primary !py-2 text-sm" disabled={busy}>
                <Plus className="size-4" /> Добавить
              </button>
            </div>
          </form>
        </div>
      </div>
    </div>
  );
}
