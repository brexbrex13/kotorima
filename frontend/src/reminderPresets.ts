import { localISO } from "./types";
export function presetTime(
  kind: string,
  time: string,
  now = new Date(),
): string {
  const minutes: Record<string, number> = {
    "30m": 30,
    "1h": 60,
    "4h": 240,
    "24h": 1440,
    "1w": 10080,
  };
  if (minutes[kind])
    return localISO(new Date(now.getTime() + minutes[kind] * 60000));
  const next = new Date(now);
  if (kind === "tomorrow") next.setDate(next.getDate() + 1);
  if (kind === "next-week")
    next.setDate(next.getDate() + ((8 - next.getDay()) % 7 || 7));
  if (kind === "next-month") {
    next.setDate(1);
    next.setMonth(next.getMonth() + 1);
  }
  const [h, m] = time.split(":").map(Number);
  next.setHours(h, m, 0, 0);
  return localISO(next);
}
