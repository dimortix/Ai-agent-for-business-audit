// Мелкие переиспользуемые блоки: статистика, пустые состояния, спиннер.

export function StatCard({ label, value, hint, delay = 0 }: {
  label: string;
  value: string;
  hint?: string;
  delay?: number;
}) {
  return (
    <div className={`card rise-in p-4 ${delay ? `rise-in-${delay}` : ""}`}>
      <div className="text-xs font-medium text-ink-3">{label}</div>
      <div className="mt-1 text-xl font-bold tabular-nums leading-tight">{value}</div>
      {hint && <div className="mt-0.5 text-[11px] text-ink-3">{hint}</div>}
    </div>
  );
}

export function EmptyState({ icon, title, text }: {
  icon: React.ReactNode;
  title: string;
  text?: string;
}) {
  return (
    <div className="card rise-in flex flex-col items-center px-6 py-10 text-center">
      <div className="flex size-12 items-center justify-center rounded-2xl bg-bg text-ink-3">
        {icon}
      </div>
      <div className="mt-3 text-sm font-bold">{title}</div>
      {text && <div className="mt-1 max-w-xs text-xs leading-relaxed text-ink-3">{text}</div>}
    </div>
  );
}

export function Spinner() {
  return (
    <div className="flex justify-center py-16" role="status" aria-label="Загрузка">
      <div className="size-7 animate-spin rounded-full border-[3px] border-line border-t-brand" />
    </div>
  );
}

export function ErrorNote({ message }: { message: string }) {
  return (
    <div
      className="rounded-xl px-4 py-3 text-sm font-medium"
      style={{ background: "var(--color-crit-soft)", color: "var(--color-ink)" }}
      role="alert"
    >
      {message}
    </div>
  );
}
