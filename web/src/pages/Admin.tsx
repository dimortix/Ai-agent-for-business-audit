import { ArrowLeft, Plus, RefreshCw, Trash2, Upload } from "lucide-react";
import { useCallback, useEffect, useState } from "react";
import { Link } from "react-router-dom";
import type { AdminParticipant, Expense } from "../api/types";
import { ErrorNote } from "../components/Bits";
import { fmtMoney } from "../lib/format";

const TOKEN_KEY = "ap_admin_token";

/* Служебная страница организаторов пилота: доступ по X-Admin-Token. */
export default function Admin() {
  const [token, setToken] = useState(localStorage.getItem(TOKEN_KEY) ?? "");
  const [authed, setAuthed] = useState(false);
  const [participants, setParticipants] = useState<AdminParticipant[]>([]);
  const [log, setLog] = useState("");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  const adminFetch = useCallback(
    async (path: string, init?: RequestInit): Promise<unknown> => {
      const res = await fetch(path, {
        ...init,
        headers: { "X-Admin-Token": token, ...(init?.headers ?? {}) },
      });
      const body: unknown = await res.json().catch(() => ({}));
      if (!res.ok) {
        const msg =
          (body as { error?: string }).error ?? `ошибка ${res.status}`;
        throw new Error(msg);
      }
      return body;
    },
    [token],
  );

  const loadParticipants = useCallback(async () => {
    setError("");
    try {
      const data = (await adminFetch("/api/admin/participants")) as {
        items: AdminParticipant[];
      };
      setParticipants(data.items ?? []);
      setAuthed(true);
      localStorage.setItem(TOKEN_KEY, token);
    } catch (err) {
      setAuthed(false);
      setError(err instanceof Error ? err.message : "не удалось получить список");
    }
  }, [adminFetch, token]);

  useEffect(() => {
    if (token) void loadParticipants();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  async function upload(path: string, file: File) {
    setBusy(true);
    setError("");
    setLog("");
    try {
      const form = new FormData();
      form.append("file", file);
      const res = await adminFetch(path, { method: "POST", body: form });
      setLog(JSON.stringify(res, null, 2));
      await loadParticipants();
    } catch (err) {
      setError(err instanceof Error ? err.message : "ошибка импорта");
    } finally {
      setBusy(false);
    }
  }

  async function recalc(id: string) {
    setBusy(true);
    setError("");
    try {
      const res = await adminFetch(`/api/admin/recalculate/${id}`, { method: "POST" });
      setLog(JSON.stringify(res, null, 2));
      await loadParticipants();
    } catch (err) {
      setError(err instanceof Error ? err.message : "ошибка пересчёта");
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="mx-auto min-h-dvh max-w-3xl px-4 py-6">
      <Link to="/login" className="mb-4 inline-flex items-center gap-1 text-xs text-ink-3 hover:text-ink">
        <ArrowLeft className="size-3.5" /> к входу
      </Link>
      <h1 className="mb-4 text-xl font-bold">Админка пилота</h1>

      <div className="card mb-4 flex flex-col gap-3 p-4 sm:flex-row sm:items-center">
        <input
          id="admin-token"
          name="admin-token"
          className="input flex-1"
          type="password"
          autoComplete="off"
          placeholder="X-Admin-Token"
          value={token}
          onChange={(e) => setToken(e.target.value)}
        />
        <button className="btn-primary" onClick={() => void loadParticipants()} disabled={!token}>
          Подключиться
        </button>
      </div>

      {error && <div className="mb-4"><ErrorNote message={error} /></div>}

      {authed && (
        <>
          <div className="card mb-4 overflow-x-auto">
            <table className="w-full text-sm">
              <thead>
                <tr className="border-b border-line text-left text-xs text-ink-3">
                  <th className="px-4 py-3 font-semibold">Участник</th>
                  <th className="px-4 py-3 font-semibold">Группа</th>
                  <th className="px-4 py-3 font-semibold">Данные</th>
                  <th className="px-4 py-3 font-semibold">ИЖБ</th>
                  <th className="px-4 py-3" />
                </tr>
              </thead>
              <tbody>
                {participants.map((p) => (
                  <tr key={p.id} className="border-b border-line last:border-0">
                    <td className="px-4 py-2.5">
                      <div className="font-medium">{p.name || p.account_id}</div>
                      <div className="text-xs text-ink-3">
                        {p.phone} · {p.account_id}
                        {p.telegram_linked && " · TG ✓"}
                      </div>
                    </td>
                    <td className="px-4 py-2.5">
                      <span className="chip !cursor-default" data-active={p.group_type === "B"}>
                        {p.group_type}
                      </span>
                    </td>
                    <td className="px-4 py-2.5 text-xs text-ink-2">
                      {p.last_data_date ? `по ${p.last_data_date}` : "нет"}
                    </td>
                    <td className="px-4 py-2.5 font-bold tabular-nums">
                      {p.health_index ?? "—"}
                    </td>
                    <td className="px-4 py-2.5 text-right">
                      {p.group_type === "B" && (
                        <button
                          className="btn-ghost !p-2"
                          title="Пересчитать прогноз"
                          disabled={busy}
                          onClick={() => void recalc(p.id)}
                        >
                          <RefreshCw className="size-4" />
                        </button>
                      )}
                    </td>
                  </tr>
                ))}
                {participants.length === 0 && (
                  <tr>
                    <td colSpan={5} className="px-4 py-6 text-center text-sm text-ink-3">
                      Участников нет — импортируйте CSV ниже
                    </td>
                  </tr>
                )}
              </tbody>
            </table>
          </div>

          <ExpensesSection participants={participants} adminFetch={adminFetch} />

          <div className="grid gap-4 sm:grid-cols-2">
            <UploadCard
              title="Импорт участников"
              hint="CSV: phone,account_id,group_type,name"
              disabled={busy}
              onFile={(f) => void upload("/api/participants/import", f)}
            />
            <UploadCard
              title="Импорт транзакций"
              hint="CSV: account_id,date,amount,type — с автопересчётом"
              disabled={busy}
              onFile={(f) => void upload("/api/admin/import-transactions", f)}
            />
          </div>

          {log && (
            <pre className="card mt-4 max-h-72 overflow-auto p-4 text-xs leading-relaxed">
              {log}
            </pre>
          )}
        </>
      )}
    </div>
  );
}

/* Управление фиксированными расходами: от них зависят ИЖБ и дата разрыва. */
function ExpensesSection({ participants, adminFetch }: {
  participants: AdminParticipant[];
  adminFetch: (path: string, init?: RequestInit) => Promise<unknown>;
}) {
  const [pid, setPid] = useState("");
  const [items, setItems] = useState<Expense[]>([]);
  const [total, setTotal] = useState(0);
  const [err, setErr] = useState("");
  const [form, setForm] = useState({ description: "", amount: "", due: "" });

  const load = useCallback(async (id: string) => {
    if (!id) return;
    setErr("");
    try {
      const data = (await adminFetch(`/api/admin/expenses/${id}`)) as {
        items: Expense[] | null;
        monthly_total: number;
      };
      setItems(data.items ?? []);
      setTotal(data.monthly_total ?? 0);
    } catch (e) {
      setErr(e instanceof Error ? e.message : "не удалось загрузить расходы");
    }
  }, [adminFetch]);

  useEffect(() => {
    if (!pid && participants.length > 0) {
      const first = participants.find((p) => p.group_type === "B") ?? participants[0];
      setPid(first.id);
      void load(first.id);
    }
  }, [participants, pid, load]);

  async function add(e: React.FormEvent) {
    e.preventDefault();
    setErr("");
    try {
      await adminFetch(`/api/admin/expenses/${pid}`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          description: form.description.trim(),
          amount: Number(form.amount),
          due_day_of_month: Number(form.due),
        }),
      });
      setForm({ description: "", amount: "", due: "" });
      await load(pid);
    } catch (e2) {
      setErr(e2 instanceof Error ? e2.message : "не удалось сохранить");
    }
  }

  async function remove(description: string) {
    setErr("");
    try {
      await adminFetch(
        `/api/admin/expenses/${pid}?description=${encodeURIComponent(description)}`,
        { method: "DELETE" },
      );
      await load(pid);
    } catch (e) {
      setErr(e instanceof Error ? e.message : "не удалось удалить");
    }
  }

  return (
    <div className="card mb-4 p-4">
      <div className="mb-3 flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
        <div className="text-sm font-bold">
          Фиксированные расходы
          {total > 0 && <span className="ml-2 font-normal text-ink-3">{fmtMoney(total)}/мес</span>}
        </div>
        <select
          className="input !w-auto !py-2 text-sm"
          value={pid}
          onChange={(e) => {
            setPid(e.target.value);
            void load(e.target.value);
          }}
        >
          {participants.map((p) => (
            <option key={p.id} value={p.id}>
              {p.name || p.account_id} ({p.group_type})
            </option>
          ))}
        </select>
      </div>

      {err && <div className="mb-3"><ErrorNote message={err} /></div>}

      <table className="w-full text-sm">
        <tbody>
          {items.map((e) => (
            <tr key={e.description} className="border-b border-line last:border-0">
              <td className="py-2 pr-2">{e.description}</td>
              <td className="py-2 pr-2 text-right tabular-nums">{fmtMoney(e.amount)}</td>
              <td className="py-2 pr-2 text-right text-xs text-ink-3">до {e.due_day_of_month} числа</td>
              <td className="py-2 text-right">
                <button
                  className="rounded-lg p-1.5 text-ink-3 transition-colors hover:bg-crit-soft hover:text-crit"
                  title="Удалить"
                  onClick={() => void remove(e.description)}
                >
                  <Trash2 className="size-4" />
                </button>
              </td>
            </tr>
          ))}
          {items.length === 0 && (
            <tr><td className="py-3 text-center text-xs text-ink-3">расходов нет</td></tr>
          )}
        </tbody>
      </table>

      <form onSubmit={add} className="mt-3 flex flex-col gap-2 sm:flex-row">
        <input
          className="input flex-1 !py-2 text-sm" placeholder="Название (Аренда…)"
          value={form.description} required maxLength={200}
          onChange={(e) => setForm({ ...form, description: e.target.value })}
        />
        <input
          className="input !py-2 text-sm sm:!w-32" placeholder="Сумма ₽" type="number" min="1"
          value={form.amount} required
          onChange={(e) => setForm({ ...form, amount: e.target.value })}
        />
        <input
          className="input !py-2 text-sm sm:!w-28" placeholder="День (1–31)" type="number" min="1" max="31"
          value={form.due} required
          onChange={(e) => setForm({ ...form, due: e.target.value })}
        />
        <button className="btn-primary !py-2 text-sm" type="submit">
          <Plus className="size-4" /> Добавить
        </button>
      </form>
      <p className="mt-2 text-[11px] text-ink-3">
        После изменения расходов прогноз и индекс участника пересчитываются автоматически.
      </p>
    </div>
  );
}

function UploadCard({ title, hint, disabled, onFile }: {
  title: string;
  hint: string;
  disabled: boolean;
  onFile: (f: File) => void;
}) {
  return (
    <label className={`card flex cursor-pointer items-center gap-3 p-4 transition-shadow hover:shadow-md ${disabled ? "pointer-events-none opacity-50" : ""}`}>
      <div className="flex size-10 shrink-0 items-center justify-center rounded-xl bg-brand-soft text-brand">
        <Upload className="size-5" />
      </div>
      <div className="min-w-0">
        <div className="text-sm font-bold">{title}</div>
        <div className="text-xs text-ink-3">{hint}</div>
      </div>
      <input
        type="file"
        accept=".csv,text/csv"
        className="hidden"
        onChange={(e) => {
          const f = e.target.files?.[0];
          if (f) onFile(f);
          e.target.value = "";
        }}
      />
    </label>
  );
}
