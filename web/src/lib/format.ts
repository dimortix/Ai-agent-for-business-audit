const moneyFmt = new Intl.NumberFormat("ru-RU", {
  style: "currency",
  currency: "RUB",
  maximumFractionDigits: 0,
});

const moneyFmtKop = new Intl.NumberFormat("ru-RU", {
  style: "currency",
  currency: "RUB",
  minimumFractionDigits: 0,
  maximumFractionDigits: 2,
});

export const fmtMoney = (v: number) => moneyFmt.format(v);
export const fmtMoneyExact = (v: number) => moneyFmtKop.format(v);

/** Компакт для осей: 12 500 → «12,5к». */
export function fmtMoneyCompact(v: number): string {
  const abs = Math.abs(v);
  if (abs >= 1_000_000) return `${(v / 1_000_000).toLocaleString("ru-RU", { maximumFractionDigits: 1 })}М`;
  if (abs >= 1_000) return `${(v / 1_000).toLocaleString("ru-RU", { maximumFractionDigits: 1 })}к`;
  return String(Math.round(v));
}

const dShort = new Intl.DateTimeFormat("ru-RU", { day: "numeric", month: "short" });
const dLong = new Intl.DateTimeFormat("ru-RU", { day: "numeric", month: "long" });
const dWeekday = new Intl.DateTimeFormat("ru-RU", { weekday: "short", day: "numeric", month: "short" });
const tShort = new Intl.DateTimeFormat("ru-RU", { hour: "2-digit", minute: "2-digit" });

export const fmtDateShort = (iso: string) => dShort.format(new Date(iso + "T00:00:00"));
export const fmtDateLong = (iso: string) => dLong.format(new Date(iso + "T00:00:00"));
export const fmtDateWeekday = (iso: string) => dWeekday.format(new Date(iso + "T00:00:00"));
export const fmtTime = (iso: string) => tShort.format(new Date(iso));

/** Локальная дата в YYYY-MM-DD (не UTC: toISOString ночью отдаёт «вчера»). */
function toLocalISO(d: Date): string {
  const m = String(d.getMonth() + 1).padStart(2, "0");
  const day = String(d.getDate()).padStart(2, "0");
  return `${d.getFullYear()}-${m}-${day}`;
}

export const todayISO = () => toLocalISO(new Date());

export function daysAgoISO(n: number): string {
  const d = new Date();
  d.setDate(d.getDate() - n);
  return toLocalISO(d);
}
