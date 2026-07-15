import { Activity, ArrowLeft } from "lucide-react";
import { useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import { api, ApiError, saveUser } from "../api/client";
import type { Participant } from "../api/types";
import { ErrorNote } from "../components/Bits";

export default function Login() {
  const navigate = useNavigate();
  const [step, setStep] = useState<"phone" | "code">("phone");
  const [phone, setPhone] = useState("");
  const [code, setCode] = useState("");
  const [debugCode, setDebugCode] = useState<string | null>(null);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  async function requestCode(e: React.FormEvent) {
    e.preventDefault();
    setError("");
    setBusy(true);
    try {
      const resp = await api<{ message: string; debug_code?: string }>(
        "/api/auth/request-code",
        { method: "POST", body: JSON.stringify({ phone }) },
      );
      setDebugCode(resp.debug_code ?? null);
      setStep("code");
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Не удалось отправить код");
    } finally {
      setBusy(false);
    }
  }

  async function verify(e: React.FormEvent) {
    e.preventDefault();
    setError("");
    setBusy(true);
    try {
      const resp = await api<{ participant: Participant }>("/api/auth/verify", {
        method: "POST",
        body: JSON.stringify({ phone, code }),
      });
      saveUser(resp.participant);
      navigate("/", { replace: true });
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Не удалось войти");
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="flex min-h-dvh flex-col items-center justify-center px-6 py-10">
      <div className="w-full max-w-sm">
        <div className="mb-8 flex flex-col items-center text-center">
          <div className="relative flex size-16 items-center justify-center rounded-3xl bg-brand shadow-lg shadow-brand/25">
            <Activity className="pulse-dot size-8 text-white" strokeWidth={2.5} />
          </div>
          <h1 className="mt-4 text-2xl font-bold">Альфа-Пульс</h1>
          <p className="mt-1 text-sm text-ink-2">
            Личный финансовый доктор вашего бизнеса
          </p>
        </div>

        <div className="card rise-in p-6">
          {step === "phone" ? (
            <form onSubmit={requestCode} className="flex flex-col gap-4">
              <div>
                <label htmlFor="phone" className="mb-1.5 block text-sm font-semibold">
                  Телефон участника пилота
                </label>
                <input
                  id="phone"
                  className="input tabular-nums"
                  type="tel"
                  inputMode="tel"
                  placeholder="+7 900 123-45-67"
                  value={phone}
                  onChange={(e) => setPhone(e.target.value)}
                  autoFocus
                  required
                />
              </div>
              {error && <ErrorNote message={error} />}
              <button className="btn-primary" disabled={busy || phone.trim().length < 10}>
                {busy ? "Отправляем…" : "Получить код"}
              </button>
              <p className="text-center text-xs leading-relaxed text-ink-3">
                Код придёт в Telegram-бот, если он привязан
              </p>
            </form>
          ) : (
            <form onSubmit={verify} className="flex flex-col gap-4">
              <button
                type="button"
                className="inline-flex items-center gap-1 self-start text-xs font-medium text-ink-3 hover:text-ink"
                onClick={() => { setStep("phone"); setCode(""); setError(""); }}
              >
                <ArrowLeft className="size-3.5" /> изменить номер
              </button>
              <div>
                <label htmlFor="code" className="mb-1.5 block text-sm font-semibold">
                  Код из сообщения
                </label>
                <input
                  id="code"
                  className="input text-center text-2xl font-bold tracking-[0.5em]"
                  inputMode="numeric"
                  autoComplete="one-time-code"
                  maxLength={4}
                  placeholder="••••"
                  value={code}
                  onChange={(e) => setCode(e.target.value.replace(/\D/g, ""))}
                  autoFocus
                  required
                />
              </div>
              {debugCode && (
                <div className="rounded-xl bg-bg px-4 py-3 text-center text-xs text-ink-2">
                  демо-режим: код <b className="tabular-nums">{debugCode}</b>
                </div>
              )}
              {error && <ErrorNote message={error} />}
              <button className="btn-primary" disabled={busy || code.length !== 4}>
                {busy ? "Проверяем…" : "Войти"}
              </button>
            </form>
          )}
        </div>

        <div className="mt-6 text-center">
          <Link to="/admin" className="text-xs text-ink-3 hover:text-ink">
            Админка пилота
          </Link>
        </div>
      </div>
    </div>
  );
}
