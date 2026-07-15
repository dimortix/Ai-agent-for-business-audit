import { CloudOff, FlaskConical, Lightbulb, ChevronRight } from "lucide-react";
import { useEffect, useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import { api, ApiError } from "../api/client";
import type { Advice, Dashboard as DashboardData } from "../api/types";
import AdviceCard from "../components/AdviceCard";
import { EmptyState, ErrorNote, Spinner, StatCard } from "../components/Bits";
import CashGapAlert from "../components/CashGapAlert";
import ForecastChart from "../components/ForecastChart";
import HealthGauge from "../components/HealthGauge";
import IntroCard from "../components/IntroCard";
import MetricChart from "../components/MetricChart";
import PushButton from "../components/PushButton";
import TelegramCard from "../components/TelegramCard";
import { fmtDateShort, fmtMoney, fmtMoneyCompact } from "../lib/format";

export default function Dashboard() {
  const navigate = useNavigate();
  const [data, setData] = useState<DashboardData | null>(null);
  const [advice, setAdvice] = useState<Advice[]>([]);
  const [error, setError] = useState("");

  useEffect(() => {
    api<DashboardData>("/api/dashboard")
      .then((d) => {
        setData(d);
        if (d.participant.group_type === "B") {
          void api<{ items: Advice[] }>("/api/advice?status=active")
            .then((r) => setAdvice(r.items ?? []))
            .catch(() => setAdvice([]));
        }
      })
      .catch((err) => {
        if (err instanceof ApiError && err.status === 401) {
          navigate("/login", { replace: true });
          return;
        }
        setError(err instanceof ApiError ? err.message : "Не удалось загрузить данные");
      });
  }, [navigate]);

  if (error) return <ErrorNote message={error} />;
  if (!data) return <Spinner />;

  const lastDay = data.last_7_days.at(-1);

  const stats = lastDay && (
    <div className="grid grid-cols-3 gap-3">
      <StatCard
        label={`Выручка ${fmtDateShort(lastDay.date)}`}
        value={fmtMoneyCompact(lastDay.revenue) + " ₽"}
        delay={2}
      />
      <StatCard label="Покупок" value={String(lastDay.transactions)} delay={3} />
      <StatCard label="Средний чек" value={fmtMoney(lastDay.avg_check)} delay={3} />
    </div>
  );

  // Контрольная группа A: только факт, без прогнозов и советов.
  if (data.control_group) {
    return (
      <div className="flex flex-col gap-3">
        <div className="card rise-in flex items-center gap-3 p-4">
          <div className="flex size-9 shrink-0 items-center justify-center rounded-xl bg-bg text-ink-2">
            <FlaskConical className="size-4.5" />
          </div>
          <p className="text-sm text-ink-2">
            Вы в <b>контрольной группе</b> пилота: доступна статистика без
            прогнозов и советов.
          </p>
        </div>
        {stats}
        {data.last_7_days.length > 0 && (
          <MetricChart
            title="Выручка за неделю"
            data={data.last_7_days.map((d) => ({ date: d.date, revenue: d.revenue }))}
            dataKey="revenue"
            format={(v) => fmtMoney(v)}
            formatAxis={fmtMoneyCompact}
            delay={2}
          />
        )}
      </div>
    );
  }

  if (!data.has_forecast) {
    return (
      <div className="flex flex-col gap-3">
        <EmptyState
          icon={<CloudOff className="size-6" />}
          title="Прогноз ещё не рассчитан"
          text="Ждём первых данных от банка. Обычно это занимает несколько минут после подключения — загляните чуть позже."
        />
        {stats}
      </div>
    );
  }

  return (
    <div className="flex flex-col gap-3">
      <IntroCard />

      {/* на десктопе: гейдж слева, тревога/статистика/подключения справа */}
      <div className="flex flex-col gap-3 md:grid md:grid-cols-2 md:items-start">
        {data.health_index !== undefined && data.health_status && (
          <HealthGauge
            value={data.health_index}
            status={data.health_status}
            calculatedAt={data.calculated_at}
            model={data.model_used}
          />
        )}
        <div className="flex flex-col gap-3">
          {data.cash_gap_date && <CashGapAlert date={data.cash_gap_date} />}
          {stats}
          <PushButton />
          <TelegramCard bot={data.telegram_bot} linked={data.telegram_linked} />
        </div>
      </div>

      <ForecastChart
        fact={data.last_7_days}
        forecast={data.forecast ?? []}
        cashGapDate={data.cash_gap_date}
      />

      {advice.length > 0 && (
        <>
          <div className="mt-2 flex items-center justify-between px-1">
            <h2 className="text-sm font-bold">Что сделать сейчас</h2>
            <Link
              to="/advice"
              className="inline-flex items-center gap-0.5 text-xs font-semibold text-brand"
            >
              все советы <ChevronRight className="size-3.5" />
            </Link>
          </div>
          <div className="flex flex-col gap-3 md:grid md:grid-cols-2 md:items-start">
            {advice.slice(0, 2).map((a, i) => (
              <AdviceCard key={a.id} advice={a} delay={i + 2} />
            ))}
          </div>
        </>
      )}
      {advice.length === 0 && (
        <div className="card rise-in rise-in-3 flex items-center gap-3 p-4">
          <div className="flex size-9 items-center justify-center rounded-xl bg-good-soft text-good">
            <Lightbulb className="size-4.5" />
          </div>
          <p className="text-sm text-ink-2">Срочных дел нет — показатели в норме.</p>
        </div>
      )}
    </div>
  );
}
