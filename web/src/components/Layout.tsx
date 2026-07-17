import { Activity, BarChart3, Bot, Lightbulb, LogOut, Wallet } from "lucide-react";
import { NavLink, useNavigate } from "react-router-dom";
import { loadUser, logout } from "../api/client";

const tabs = [
  { to: "/", label: "Пульс", icon: Activity, bOnly: false },
  { to: "/analytics", label: "Аналитика", icon: BarChart3, bOnly: false },
  { to: "/expenses", label: "Расходы", icon: Wallet, bOnly: true },
  { to: "/advice", label: "Советы", icon: Lightbulb, bOnly: true },
  { to: "/advisor", label: "Советник", icon: Bot, bOnly: true },
];

export default function Layout({ children }: { children: React.ReactNode }) {
  const user = loadUser();
  const navigate = useNavigate();
  // разделы «Расходы» и «Советы» — только для группы B (полный функционал)
  const visibleTabs = tabs.filter((t) => !t.bOnly || user?.group_type === "B");

  return (
    <div className="min-h-dvh pb-24 md:pb-10">
      <header className="sticky top-0 z-20 border-b border-line bg-card/90 backdrop-blur">
        <div className="mx-auto flex max-w-5xl items-center gap-3 px-4 py-3">
          <div className="flex size-9 items-center justify-center rounded-xl bg-brand">
            <Activity className="size-5 text-white" strokeWidth={2.5} />
          </div>
          <div className="min-w-0 flex-1">
            <div className="text-sm font-bold leading-tight">Альфа.Пульс</div>
            <div className="truncate text-xs text-ink-3">
              {user?.name || user?.phone || "пульс вашего бизнеса"}
            </div>
          </div>

          {/* навигация на десктопе */}
          <nav className="hidden items-center gap-1 md:flex">
            {visibleTabs.map(({ to, label }) => (
              <NavLink
                key={to}
                to={to}
                className={({ isActive }) =>
                  `rounded-lg px-3 py-1.5 text-sm font-medium transition-colors ${
                    isActive ? "bg-bg text-ink" : "text-ink-2 hover:text-ink"
                  }`
                }
              >
                {label}
              </NavLink>
            ))}
          </nav>

          <button
            title="Выйти"
            className="rounded-lg p-2 text-ink-3 transition-colors hover:bg-bg hover:text-ink"
            onClick={() => {
              void logout().then(() => navigate("/login"));
            }}
          >
            <LogOut className="size-4.5" />
          </button>
        </div>
      </header>

      <main className="mx-auto max-w-5xl px-4 pt-4">{children}</main>

      {/* нижняя навигация на мобильных */}
      <nav className="fixed inset-x-0 bottom-0 z-20 border-t border-line bg-card pb-[env(safe-area-inset-bottom)] md:hidden">
        <div className="mx-auto flex max-w-5xl">
          {visibleTabs.map(({ to, label, icon: Icon }) => (
            <NavLink
              key={to}
              to={to}
              className={({ isActive }) =>
                `flex flex-1 flex-col items-center gap-0.5 py-2.5 text-[11px] font-medium transition-colors ${
                  isActive ? "text-brand" : "text-ink-3"
                }`
              }
            >
              <Icon className="size-5" />
              {label}
            </NavLink>
          ))}
        </div>
      </nav>
    </div>
  );
}
