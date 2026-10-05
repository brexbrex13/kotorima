import { presetTime } from "./reminderPresets";
import { date } from "./types";
export type Suggestion = {
  category_id: number;
  important: boolean;
  deadline: string;
  reminder: string;
  duplicate: { task_id: number; title: string } | null;
  invalid: boolean;
  unavailable?: boolean;
};
export type JevStatus = {
  unavailable?: boolean;
  revision?: number;
  key_configured?: boolean;
  supported: boolean;
  configured: boolean;
  hint: string;
  invalid: boolean;
};
export type Field = "category" | "important" | "deadline" | "reminder";
export type QuickOptions = {
  deadline: string;
  reminder_at: string;
  reminder_mode: string;
  reminder_time: string;
  important: boolean;
  category_id: number;
};
export type AutoFields = Partial<Record<Field, string>>;
export const blankOptions = (): QuickOptions => ({
  deadline: "",
  reminder_at: "",
  reminder_mode: "",
  reminder_time: "",
  important: false,
  category_id: 0,
});
const noReminder = { reminder_at: "", reminder_mode: "", reminder_time: "" };
// Weeks run Monday to Sunday; "this week" means Friday, or Sunday once Friday has passed.
export function deadlineDate(kind: string, now = new Date()): string {
  const day = now.getDay() || 7;
  const offsets: Record<string, number> = {
    today: 0,
    tomorrow: 1,
    this_week: day <= 5 ? 5 - day : 7 - day,
    next_week: 12 - day,
  };
  if (!Object.prototype.hasOwnProperty.call(offsets, kind)) return "";
  const d = new Date(now);
  d.setDate(d.getDate() + offsets[kind]);
  return date(d);
}
export function applySuggestion(
  options: QuickOptions,
  touched: ReadonlySet<Field>,
  auto: AutoFields,
  s: Suggestion | null,
  defaultTime = "09:00",
  now = new Date(),
): { options: QuickOptions; auto: AutoFields } {
  const next = { ...options };
  const nextAuto: AutoFields = {};
  const apply = (
    field: Field,
    value: string | undefined,
    write: () => void,
    clear: () => void,
  ) => {
    if (touched.has(field)) return;
    if (value) {
      write();
      nextAuto[field] = value;
    } else if (auto[field] !== undefined) clear();
  };
  apply(
    "category",
    s && s.category_id > 0 ? String(s.category_id) : undefined,
    () => (next.category_id = s!.category_id),
    () => (next.category_id = 0),
  );
  apply(
    "important",
    s?.important ? "1" : undefined,
    () => (next.important = true),
    () => (next.important = false),
  );
  const due = s ? deadlineDate(s.deadline, now) : "";
  apply(
    "deadline",
    due ? s!.deadline : undefined,
    () => (next.deadline = due),
    () => (next.deadline = ""),
  );
  const reminder =
    s?.reminder && (s.reminder !== "deadline" || next.deadline)
      ? s.reminder
      : undefined;
  apply(
    "reminder",
    reminder,
    () =>
      Object.assign(
        next,
        reminder === "deadline"
          ? {
              reminder_mode: "deadline",
              reminder_time: defaultTime,
              reminder_at: "",
            }
          : {
              ...noReminder,
              reminder_at: presetTime(reminder!, defaultTime, now),
            },
      ),
    () => Object.assign(next, noReminder),
  );
  if (!next.deadline && next.reminder_mode === "deadline") {
    Object.assign(next, noReminder);
    delete nextAuto.reminder;
  }
  return { options: next, auto: nextAuto };
}
export function dropChip(
  options: QuickOptions,
  auto: AutoFields,
  field: Field,
): { options: QuickOptions; auto: AutoFields } {
  const next = { ...options };
  const nextAuto = { ...auto };
  delete nextAuto[field];
  if (field === "category") next.category_id = 0;
  if (field === "important") next.important = false;
  if (field === "reminder") Object.assign(next, noReminder);
  if (field === "deadline") {
    next.deadline = "";
    if (next.reminder_mode === "deadline") {
      Object.assign(next, noReminder);
      delete nextAuto.reminder;
    }
  }
  return { options: next, auto: nextAuto };
}
const deadlineNames: Record<string, string> = {
  today: "今日",
  tomorrow: "明日",
  this_week: "今週中",
  next_week: "来週",
};
const shortTime = (iso: string) =>
  `${+iso.slice(5, 7)}/${+iso.slice(8, 10)} ${iso.slice(11, 16)}`;
export function chipLabels(
  auto: AutoFields,
  options: QuickOptions,
  categories: { id: number; name: string }[],
): { field: Field; label: string }[] {
  const out: { field: Field; label: string }[] = [];
  const category = categories.find((c) => c.id === options.category_id);
  if (auto.category && category)
    out.push({ field: "category", label: category.name });
  if (auto.important) out.push({ field: "important", label: "重要" });
  if (auto.deadline)
    out.push({
      field: "deadline",
      label: "期限 " + (deadlineNames[auto.deadline] ?? ""),
    });
  if (auto.reminder)
    out.push({
      field: "reminder",
      label:
        auto.reminder === "deadline"
          ? "期限日に通知"
          : "通知 " + shortTime(options.reminder_at),
    });
  return out;
}
export type JevEndpoint = {
  timeout_seconds?: number;
  provider: string;
  account: string;
  base_url: string;
  model: string;
};
export const jevProviders = [
  {
    id: "openai",
    name: "OpenAI互換API（Geminiなど）",
    key: "APIキー",
    sendTo: "指定したAIサービス",
  },
  {
    id: "local",
    name: "ローカルAI（LM Studio・Ollamaなど）",
    key: "APIキー（認証ありの場合のみ）",
    sendTo: "このPCのローカルAI",
  },
  {
    id: "typesafe",
    name: "TypeSafe",
    key: "TypeSafe APIキー",
    sendTo: "TypeSafe AI",
  },
  {
    id: "vercel",
    name: "Vercel AI Gateway",
    key: "AI Gateway APIキー",
    sendTo: "Vercel AI Gateway",
  },
  {
    id: "cloudflare",
    name: "Cloudflare Workers AI",
    key: "Cloudflare APIトークン",
    sendTo: "Cloudflare",
  },
  {
    id: "custom",
    name: "カスタム（TypeSafe互換）",
    key: "APIキー",
    sendTo: "指定したエンドポイント",
  },
];
export const jevProvider = (id: string) =>
  jevProviders.find((p) => p.id === (id || "typesafe")) ??
  jevProviders.find((p) => p.id === "typesafe")!;

export const aiKeySlot = (endpoint: JevEndpoint) => {
  const p = jevProvider(endpoint.provider).id;
  return p === "openai" || p === "local"
    ? p + "@" + endpoint.base_url.trim().replace(/\/+$/, "")
    : p;
};
