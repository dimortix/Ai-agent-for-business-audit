import { Bell, BellRing } from "lucide-react";
import { useEffect, useState } from "react";
import { getPushState, subscribePush, type PushState } from "../lib/push";

/* Кнопка подписки на тревожные уведомления (ИЖБ < 40). */
export default function PushButton() {
  const [state, setState] = useState<PushState | "loading">("loading");

  useEffect(() => {
    getPushState().then(setState).catch(() => setState("unsupported"));
  }, []);

  if (state === "loading" || state === "unsupported" || state === "disabled") return null;

  if (state === "subscribed") {
    return (
      <div className="card rise-in rise-in-4 flex items-center gap-3 p-4">
        <div className="flex size-9 items-center justify-center rounded-xl bg-good-soft text-good">
          <BellRing className="size-4.5" />
        </div>
        <div className="text-sm text-ink-2">
          Уведомления включены — предупредим, если бизнесу станет плохо
        </div>
      </div>
    );
  }

  return (
    <div className="card rise-in rise-in-4 flex items-center gap-3 p-4">
      <div className="flex size-9 shrink-0 items-center justify-center rounded-xl bg-bg text-ink-2">
        <Bell className="size-4.5" />
      </div>
      <div className="min-w-0 flex-1 text-sm text-ink-2">
        {state === "denied"
          ? "Уведомления запрещены в браузере — разрешите их в настройках сайта"
          : "Получайте тревожный сигнал, если индекс здоровья упадёт"}
      </div>
      {state === "ready" && (
        <button
          className="btn-primary shrink-0 !px-3.5 !py-2 text-sm"
          onClick={() => void subscribePush().then(setState)}
        >
          Включить
        </button>
      )}
    </div>
  );
}
