import { fmtTime } from "../lib/format";

const zones = {
  ok: { color: "var(--color-good)", soft: "var(--color-good-soft)", label: "Всё хорошо", emoji: "🟢" },
  warning: { color: "var(--color-warn)", soft: "var(--color-warn-soft)", label: "Стоит задуматься", emoji: "🟡" },
  critical: { color: "var(--color-crit)", soft: "var(--color-crit-soft)", label: "Нужны срочные меры", emoji: "🔴" },
} as const;

interface Props {
  value: number;
  status: keyof typeof zones;
  calculatedAt?: string;
  model?: string;
}

// Полукруглый гейдж ИЖБ. Цвет зоны никогда не «один»: рядом всегда число,
// текстовый ярлык и эмодзи-иконка (доступность: не только цветом).
export default function HealthGauge({ value, status, calculatedAt, model }: Props) {
  const zone = zones[status];
  const R = 82;
  const LEN = Math.PI * R; // длина полудуги
  const filled = (Math.min(100, Math.max(0, value)) / 100) * LEN;

  return (
    <div className="card rise-in flex flex-col items-center px-6 pb-6 pt-5">
      <div className="self-start text-sm font-semibold text-ink-2">
        Индекс здоровья бизнеса
      </div>

      <div className="relative mt-2">
        <svg width="220" height="124" viewBox="0 0 220 124" role="img"
          aria-label={`Индекс здоровья ${value} из 100 — ${zone.label}`}>
          {/* трек */}
          <path
            d={`M ${110 - R} 112 A ${R} ${R} 0 0 1 ${110 + R} 112`}
            fill="none" stroke="var(--color-line)" strokeWidth="14" strokeLinecap="round"
          />
          {/* значение */}
          <path
            d={`M ${110 - R} 112 A ${R} ${R} 0 0 1 ${110 + R} 112`}
            fill="none" stroke={zone.color} strokeWidth="14" strokeLinecap="round"
            strokeDasharray={`${filled} ${LEN}`}
            style={{ transition: "stroke-dasharray 0.9s cubic-bezier(0.22, 1, 0.36, 1)" }}
          />
        </svg>
        <div className="absolute inset-x-0 bottom-0 flex flex-col items-center">
          <div className="text-5xl font-bold tabular-nums leading-none">{value}</div>
          <div className="mt-1 text-xs font-medium text-ink-3">из 100</div>
        </div>
      </div>

      <div
        className="mt-3 inline-flex items-center gap-1.5 rounded-full px-3.5 py-1.5 text-sm font-semibold"
        style={{ background: zone.soft, color: "var(--color-ink)" }}
      >
        <span aria-hidden>{zone.emoji}</span>
        {zone.label}
      </div>

      {calculatedAt && (
        <div className="mt-3 text-[11px] text-ink-3">
          обновлено в {fmtTime(calculatedAt)}
          {model && ` · модель: ${model === "prophet" ? "Prophet (ML)" : "Хольт-Винтерс"}`}
        </div>
      )}
    </div>
  );
}
