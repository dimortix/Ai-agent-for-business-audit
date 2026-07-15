import { HelpCircle, X } from "lucide-react";
import { useState } from "react";

const SEEN_KEY = "ap_intro_seen";

/* Онбординг: короткое объяснение ИЖБ при первом входе (закрывается навсегда). */
export default function IntroCard() {
  const [hidden, setHidden] = useState(() => localStorage.getItem(SEEN_KEY) === "1");
  if (hidden) return null;

  return (
    <div className="card rise-in relative p-4 pr-10">
      <button
        aria-label="Скрыть подсказку"
        className="absolute right-2.5 top-2.5 rounded-lg p-1.5 text-ink-3 transition-colors hover:bg-bg hover:text-ink"
        onClick={() => {
          localStorage.setItem(SEEN_KEY, "1");
          setHidden(true);
        }}
      >
        <X className="size-4" />
      </button>
      <div className="flex items-start gap-3">
        <div className="flex size-9 shrink-0 items-center justify-center rounded-xl bg-bg text-ink-2">
          <HelpCircle className="size-4.5" />
        </div>
        <div className="text-sm leading-relaxed text-ink-2">
          <b className="text-ink">Что такое индекс здоровья?</b> Это запас
          прочности бизнеса от 1 до 100: деньги на счету плюс прогноз выручки на
          месяц против обязательных платежей. <b className="text-ink">🟢 ≥ 70</b> — всё
          хорошо, <b className="text-ink">🟡 40–69</b> — стоит задуматься,{" "}
          <b className="text-ink">🔴 &lt; 40</b> — нужны срочные меры. Прогноз строит
          ML-модель по вашим реальным оборотам — вы ничего не заполняете.
        </div>
      </div>
    </div>
  );
}
