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

export interface Dashboard {
  participant: Participant;
  last_7_days: DayFact[];
  has_forecast: boolean;
  control_group?: boolean;
  telegram_bot?: string;
  telegram_linked?: boolean;
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
