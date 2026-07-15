import { PartyPopper } from "lucide-react";
import { useCallback, useEffect, useState } from "react";
import { useNavigate } from "react-router-dom";
import { api, ApiError } from "../api/client";
import type { Advice as AdviceItem } from "../api/types";
import AdviceCard from "../components/AdviceCard";
import { EmptyState, ErrorNote, Spinner } from "../components/Bits";

const filters = [
  { key: "active", label: "Актуальные" },
  { key: "done", label: "Выполненные" },
  { key: "all", label: "Все" },
] as const;

type FilterKey = (typeof filters)[number]["key"];

export default function Advice() {
  const navigate = useNavigate();
  const [filter, setFilter] = useState<FilterKey>("active");
  const [items, setItems] = useState<AdviceItem[] | null>(null);
  const [error, setError] = useState("");

  const load = useCallback(
    (f: FilterKey) => {
      setItems(null);
      setError("");
      api<{ items: AdviceItem[] }>(`/api/advice?status=${f}`)
        .then((r) => setItems(r.items ?? []))
        .catch((err) => {
          if (err instanceof ApiError && err.status === 401) {
            navigate("/login", { replace: true });
            return;
          }
          setError(err instanceof ApiError ? err.message : "Не удалось загрузить советы");
        });
    },
    [navigate],
  );

  useEffect(() => load(filter), [filter, load]);

  async function markDone(id: number) {
    await api(`/api/advice/${id}/done`, { method: "POST" });
    load(filter);
  }

  return (
    <div className="flex flex-col gap-3">
      <div className="flex gap-2">
        {filters.map((f) => (
          <button
            key={f.key}
            className="chip"
            data-active={filter === f.key}
            onClick={() => setFilter(f.key)}
          >
            {f.label}
          </button>
        ))}
      </div>

      {error && <ErrorNote message={error} />}
      {!error && items === null && <Spinner />}
      {items?.length === 0 && (
        <EmptyState
          icon={<PartyPopper className="size-6" />}
          title={filter === "active" ? "Актуальных советов нет" : "Здесь пока пусто"}
          text={
            filter === "active"
              ? "Показатели в норме. Мы следим за пульсом и подскажем, если что-то пойдёт не так."
              : undefined
          }
        />
      )}
      {items?.map((a, i) => (
        <AdviceCard
          key={a.id}
          advice={a}
          onDone={markDone}
          delay={Math.min(i + 1, 4)}
        />
      ))}
    </div>
  );
}
