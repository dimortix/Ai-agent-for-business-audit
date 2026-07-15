import { Send } from "lucide-react";

/* Привязка Telegram из UI: ссылка на бота (коды входа + тревоги в мессенджер). */
export default function TelegramCard({ bot, linked }: { bot?: string; linked?: boolean }) {
  if (!bot || linked) return null;

  return (
    <div className="card rise-in rise-in-4 flex items-center gap-3 p-4">
      <div className="flex size-9 shrink-0 items-center justify-center rounded-xl bg-bg text-ink-2">
        <Send className="size-4.5" />
      </div>
      <div className="min-w-0 flex-1 text-sm text-ink-2">
        Привяжите Telegram — коды входа и тревожные сигналы будут приходить в мессенджер
      </div>
      <a
        className="btn-primary shrink-0 !px-3.5 !py-2 text-sm"
        href={`https://t.me/${bot}?start=link`}
        target="_blank"
        rel="noreferrer"
      >
        Привязать
      </a>
    </div>
  );
}
