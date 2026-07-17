import { Bot, Eraser, Send, Sparkles } from "lucide-react";
import { useEffect, useRef, useState } from "react";
import { useNavigate } from "react-router-dom";
import { api, ApiError, loadUser } from "../api/client";

interface ChatMsg {
  role: "user" | "assistant";
  content: string;
}

const suggestions = [
  "Почему у меня такой индекс здоровья?",
  "Как избежать кассового разрыва?",
  "Как поднять средний чек?",
  "На чём можно сэкономить?",
];

const storageKey = () => `ap_chat_${loadUser()?.phone ?? "anon"}`;

/* AI-советник: чат с LLM, которая видит свежие цифры бизнеса. */
export default function Advisor() {
  const navigate = useNavigate();
  const [messages, setMessages] = useState<ChatMsg[]>(() => {
    try {
      return JSON.parse(localStorage.getItem(storageKey()) ?? "[]") as ChatMsg[];
    } catch {
      return [];
    }
  });
  const [input, setInput] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [offline, setOffline] = useState(false);
  const bottomRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    localStorage.setItem(storageKey(), JSON.stringify(messages.slice(-40)));
    bottomRef.current?.scrollIntoView({ behavior: "smooth", block: "end" });
  }, [messages, busy]);

  async function send(text: string) {
    const content = text.trim();
    if (!content || busy) return;
    setError("");
    setInput("");
    const next: ChatMsg[] = [...messages, { role: "user", content }];
    setMessages(next);
    setBusy(true);
    try {
      const resp = await api<{ reply: string }>("/api/chat", {
        method: "POST",
        body: JSON.stringify({ messages: next.slice(-12) }),
      });
      setMessages((m) => [...m, { role: "assistant", content: resp.reply }]);
    } catch (err) {
      if (err instanceof ApiError && err.status === 401) {
        navigate("/login", { replace: true });
        return;
      }
      if (err instanceof ApiError && err.status === 503) {
        setOffline(true);
      } else {
        setError(err instanceof ApiError ? err.message : "Советник недоступен, попробуйте ещё раз");
      }
      // возвращаем реплику в поле ввода, чтобы не потерялась
      setMessages(messages);
      setInput(content);
    } finally {
      setBusy(false);
    }
  }

  if (offline) {
    return (
      <div className="card rise-in flex flex-col items-center px-6 py-10 text-center">
        <div className="flex size-12 items-center justify-center rounded-2xl bg-bg text-ink-3">
          <Bot className="size-6" />
        </div>
        <div className="mt-3 text-sm font-bold">AI-советник не подключён</div>
        <p className="mt-1 max-w-sm text-xs leading-relaxed text-ink-3">
          Администратору нужно задать <code>LLM_API_URL</code> в конфигурации
          (подойдёт GigaChat, OpenAI или локальный ollama) и перезапустить сервис.
        </p>
      </div>
    );
  }

  return (
    <div className="flex min-h-[70dvh] flex-col">
      {/* шапка */}
      <div className="card rise-in mb-3 flex items-center gap-3 p-4">
        <div className="flex size-10 shrink-0 items-center justify-center rounded-xl bg-brand-soft text-brand">
          <Bot className="size-5" />
        </div>
        <div className="min-w-0 flex-1">
          <div className="text-sm font-bold">AI-советник</div>
          <div className="text-xs text-ink-3">
            видит ваши свежие показатели и отвечает по цифрам
          </div>
        </div>
        {messages.length > 0 && (
          <button
            className="rounded-lg p-2 text-ink-3 transition-colors hover:bg-bg hover:text-ink"
            title="Очистить диалог"
            onClick={() => setMessages([])}
          >
            <Eraser className="size-4" />
          </button>
        )}
      </div>

      {/* лента сообщений */}
      <div className="flex flex-1 flex-col gap-2.5 pb-3">
        {messages.length === 0 && (
          <div className="card rise-in p-4">
            <div className="mb-2 flex items-center gap-1.5 text-xs font-bold uppercase tracking-wide text-ink-3">
              <Sparkles className="size-3.5" /> С чего начать
            </div>
            <div className="flex flex-wrap gap-2">
              {suggestions.map((s) => (
                <button key={s} className="chip" onClick={() => void send(s)}>
                  {s}
                </button>
              ))}
            </div>
          </div>
        )}

        {messages.map((m, i) => (
          <div
            key={i}
            className={`max-w-[85%] whitespace-pre-wrap rounded-2xl px-4 py-2.5 text-sm leading-relaxed ${
              m.role === "user"
                ? "self-end bg-brand text-white"
                : "card self-start"
            }`}
          >
            {m.content}
          </div>
        ))}

        {busy && (
          <div className="card flex items-center gap-2 self-start px-4 py-2.5 text-sm text-ink-3">
            <span className="size-1.5 animate-pulse rounded-full bg-ink-3" />
            <span className="size-1.5 animate-pulse rounded-full bg-ink-3 [animation-delay:150ms]" />
            <span className="size-1.5 animate-pulse rounded-full bg-ink-3 [animation-delay:300ms]" />
            советник думает…
          </div>
        )}
        {error && <div className="self-start text-xs text-crit">{error}</div>}
        <div ref={bottomRef} />
      </div>

      {/* ввод */}
      <form
        className="sticky bottom-20 flex gap-2 md:bottom-4"
        onSubmit={(e) => {
          e.preventDefault();
          void send(input);
        }}
      >
        <input
          className="input flex-1 !bg-card shadow-card"
          placeholder="Спросите про свой бизнес…"
          value={input}
          maxLength={2000}
          onChange={(e) => setInput(e.target.value)}
        />
        <button className="btn-primary !px-4" disabled={busy || !input.trim()} title="Отправить">
          <Send className="size-4.5" />
        </button>
      </form>
      <p className="mt-2 text-center text-[10px] leading-relaxed text-ink-3">
        Советы носят рекомендательный характер и не гарантируют результат.
        Не является инвестиционной, налоговой или юридической консультацией.
      </p>
    </div>
  );
}
