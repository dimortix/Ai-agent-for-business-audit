export interface Participant {
  name: string;
  phone: string;
  group_type: "A" | "B";
}

export interface ForecastPoint {
  date: string;
  yhat: number;
  lower: number;
  upper: number;
}

export interface DayFact {
  date: string;
  revenue: number;
  transactions: number;
  avg_check: number;
}

export interface Operation {
  paid_at: string;
  amount: number;
  type: "income" | "return";
}

export interface Dashboard {
  participant: Participant;
  last_7_days: DayFact[];
  has_forecast: boolean;
  control_group?: boolean;
  telegram_bot?: string;
  telegram_linked?: boolean;
  recent_operations?: Operation[];
  health_index?: number;
  health_status?: "ok" | "warning" | "critical";
  cash_gap_date?: string | null;
  forecast?: ForecastPoint[];
  model_used?: string;
  calculated_at?: string;
  current_balance?: number;
  monthly_expenses?: number;
  active_advice_count?: number;
}

export interface AnalyticsDay {
  date: string;
  revenue: number;
  returns: number;
  net: number;
  transactions: number;
  avg_check: number;
}

export interface Analytics {
  from: string;
  to: string;
  days: AnalyticsDay[];
}

export interface Advice {
  id: number;
  created_at: string;
  rule_code: string;
  message: string;
  was_action_taken: boolean;
}

export interface PeriodStats {
  revenue: number;
  transactions: number;
  avg_check: number;
  returns_rate: number;
}

export interface PeriodComparison {
  days: number;
  current: PeriodStats;
  previous: PeriodStats;
  revenue_delta?: number;
  tx_delta?: number;
  avg_check_delta?: number;
}

export interface WeekdayAvg {
  label: string;
  avg_revenue: number;
}

export interface HealthPoint {
  at: string;
  index: number;
}

export interface Scenario {
  gap_date: string | null;
  total_14d: number;
}

export interface CalendarEvent {
  date: string;
  description: string;
  amount: number;
  balance_after: number;
  risk: boolean;
}

export interface ProfitPoint {
  date: string;
  profit: number;
}

export interface BalancePoint {
  date: string;
  balance: number;
}

export interface Insights {
  period?: PeriodComparison;
  weekday_profile?: WeekdayAvg[];
  insights: string[];
  profit_series?: ProfitPoint[];
  margin_pct?: number;
  break_even_daily?: number;
  days_above_break_even?: number;
  monthly_expenses?: number;
  runway_days?: number;
  health_history?: HealthPoint[];
  forecast_mape?: number;
  scenarios?: { pessimistic: Scenario; base: Scenario; optimistic: Scenario };
  cash_calendar?: CalendarEvent[];
  benchmark?: { rank: number; total: number; growth_pct?: number };
  balance_projection?: BalancePoint[];
}

export interface FixedExpenseItem {
  description: string;
  amount: number;
  due_day_of_month: number;
  category: string;
}

export interface OneOffExpenseItem {
  id: number;
  date: string;
  amount: number;
  description: string;
  category: string;
}

export interface MyExpenses {
  fixed: FixedExpenseItem[] | null;
  one_off: OneOffExpenseItem[] | null;
  monthly_total: number;
}

export interface Expense {
  description: string;
  amount: number;
  due_day_of_month: number;
}

export interface AdminParticipant {
  id: string;
  phone: string;
  account_id: string;
  name: string;
  group_type: "A" | "B";
  telegram_linked: boolean;
  first_data_date?: string;
  last_data_date?: string;
  health_index?: number;
}
