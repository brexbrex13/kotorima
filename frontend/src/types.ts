export type Task = {
  id: number;
  version: number;
  title: string;
  memo: string;
  show_memo_in_notice?: boolean;
  status: string;
  deadline: string;
  reminder_at: string;
  reminder_mode?: string;
  reminder_time?: string;
  today_date?: string;
  notified_at: string;
  category_id: number;
  important: boolean;
  sort_order: number;
  created_at: string;
  done_at: string;
  deleted_at: string;
  series_id: number;
  occurrence: string;
  near_days: number;
};
export type Category = {
  id: number;
  name: string;
  color: string;
  sort_order: number;
  text_color?: string;
  dormant?: boolean;
};
export type Series = {
  id: number;
  version: number;
  title: string;
  memo: string;
  show_memo_in_notice?: boolean;
  category_id: number;
  important: boolean;
  period: string;
  interval: number;
  weekdays: number[];
  month_day: number;
  month: number;
  first_due: string;
  next_due: string;
  due_time: string;
  show_days: number;
  near_days: number;
  notify_mode: string;
  notify_days: number;
  notify_time: string;
  notify_hours: number;
  active: boolean;
  deleted: boolean;
  end_date: string;
};
export type Notice = {
  id: number;
  key: string;
  task_id: number;
  kind: string;
  title: string;
  fired_at: string;
  acknowledged: boolean;
};
export type ColorPreset = {
  name: string;
  background: string;
  foreground: string;
};
export type Settings = {
  reminder_default_time?: string;
  quick_shortcut?: string;
  suggest_min_count?: number;
  theme?: "light" | "dark" | "system";
  series_show_days: number;
  custom_colors: ColorPreset[];
  near_days: number;
  workdays: boolean;
  notify_times: string[];
  notify_weekdays: number[];
  sound: boolean;
  native_toast: boolean;
  private: boolean;
  pause_until: string;
  font_size: number;
  compact: boolean;
  collapsed: number[];
  monitor: string;
  smart_add?: boolean;
  ai_timeout_seconds?: number;
  jev_provider?: string;
  jev_cloudflare_account?: string;
  jev_custom_url?: string;
  jev_custom_model?: string;
};
export type Snapshot = {
  tasks: Task[];
  categories: Category[];
  series: Series[];
  notifications: Notice[];
  settings: Settings;
};
export const localISO = (d = new Date()) =>
  `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, "0")}-${String(d.getDate()).padStart(2, "0")}T${String(d.getHours()).padStart(2, "0")}:${String(d.getMinutes()).padStart(2, "0")}:${String(d.getSeconds()).padStart(2, "0")}`;
export const date = (d = new Date()) => localISO(d).slice(0, 10);
export const emptyTask = (category_id = 0): Task => ({
  id: 0,
  version: 0,
  title: "",
  memo: "",
  show_memo_in_notice: false,
  status: "pending",
  deadline: "",
  reminder_at: "",
  notified_at: "",
  category_id,
  important: false,
  sort_order: 0,
  created_at: "",
  done_at: "",
  deleted_at: "",
  series_id: 0,
  occurrence: "",
  near_days: 3,
});
export const emptySeries = (): Series => ({
  id: 0,
  version: 0,
  title: "",
  memo: "",
  show_memo_in_notice: false,
  category_id: 0,
  important: false,
  period: "weekly",
  interval: 1,
  weekdays: [new Date().getDay()],
  month_day: new Date().getDate(),
  month: new Date().getMonth() + 1,
  first_due: "",
  next_due: "",
  due_time: "",
  show_days: -1,
  near_days: 3,
  notify_mode: "days",
  notify_days: 1,
  notify_time: "09:00",
  notify_hours: 24,
  active: true,
  deleted: false,
  end_date: "",
});
export function urgency(t: Task, s: Settings, now = new Date()): string {
  if (t.status !== "pending" || t.deleted_at || !t.deadline) return "";
  const due = new Date(
    t.deadline.length === 10 ? t.deadline + "T23:59:59" : t.deadline,
  );
  if (due < now) return "overdue";
  if (date(due) === date(now)) return "today";
  const n = s.near_days;
  let days = 0;
  const c = new Date(date(now) + "T00:00");
  while (date(c) < date(due) && days <= n) {
    c.setDate(c.getDate() + 1);
    if (!s.workdays || (c.getDay() !== 0 && c.getDay() !== 6)) days++;
  }
  return days <= n ? "near" : "";
}
export const urgencyLabel: Record<string, string> = {
  overdue: "期限超過",
  today: "今日期限",
  near: "期限が近い",
};
export const textOnly = (html: string) =>
  html
    .replace(/<[^>]+>/g, " ")
    .replace(/&nbsp;/g, " ")
    .trim();
