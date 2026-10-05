import { useEffect, useRef, useState } from "react";
import { presetTime } from "./reminderPresets";
export type ReminderValue = {
  deadline: string;
  reminder_at: string;
  reminder_mode?: string;
  reminder_time?: string;
};
export function ReminderFields({
  value,
  onChange,
  defaultTime = "09:00",
}: {
  defaultTime?: string;
  value: ReminderValue;
  onChange: (patch: Partial<ReminderValue>) => void;
}) {
  const [mode, setMode] = useState(
    value.reminder_mode === "deadline"
      ? "deadline"
      : value.reminder_at
        ? "custom"
        : "off",
  );
  const presetAt = useRef("");
  useEffect(() => {
    if (value.reminder_mode === "deadline") setMode("deadline");
    else if (value.reminder_at && value.reminder_at !== presetAt.current)
      setMode("custom");
    else if (!value.reminder_at)
      setMode((current) => (current === "custom" ? current : "off"));
  }, [value.reminder_mode, value.reminder_at]);
  const choose = (next: string) => {
    setMode(next);
    const at = ["off", "deadline", "custom"].includes(next)
      ? next === "custom"
        ? value.reminder_at
        : ""
      : presetTime(next, defaultTime);
    presetAt.current = at;
    onChange({
      reminder_mode: next === "deadline" ? "deadline" : "",
      reminder_time: next === "deadline" ? defaultTime : "",
      reminder_at: at,
    });
  };
  const select = useRef<HTMLSelectElement>(null);
  const chooseRef = useRef(choose);
  chooseRef.current = choose;
  useEffect(() => {
    const el = select.current;
    if (!el) return;
    let delta = 0;
    const wheel = (e: WheelEvent) => {
      if (document.activeElement !== el || !e.deltaY) return;
      e.preventDefault();
      delta +=
        e.deltaY * (e.deltaMode === 1 ? 16 : e.deltaMode === 2 ? 120 : 1);
      if (Math.abs(delta) < 40) return;
      const options = Array.from(el.options).filter((o) => !o.disabled);
      const current = options.findIndex((o) => o.value === el.value);
      const next =
        options[
          Math.max(
            0,
            Math.min(options.length - 1, current + (delta > 0 ? 1 : -1)),
          )
        ];
      delta = 0;
      if (next && next.value !== el.value) chooseRef.current(next.value);
    };
    el.addEventListener("wheel", wheel, { passive: false });
    return () => el.removeEventListener("wheel", wheel);
  }, []);
  const at =
    value.reminder_mode === "deadline" && value.deadline
      ? value.deadline.slice(0, 10) + "T" + (value.reminder_time || defaultTime)
      : value.reminder_at;
  return (
    <>
      <label>
        期限日
        <input
          type="date"
          value={value.deadline.slice(0, 10)}
          onChange={(e) => {
            const deadline = e.target.value;
            if (!deadline && mode === "deadline") {
              setMode("off");
              onChange({
                deadline,
                reminder_at: "",
                reminder_mode: "",
                reminder_time: "",
              });
            } else onChange({ deadline });
          }}
        />
      </label>
      <label>
        通知
        <select
          aria-label="通知方法"
          value={mode}
          ref={select}
          onChange={(e) => choose(e.target.value)}
        >
          <option value="off">通知なし</option>

          <option value="deadline" disabled={!value.deadline}>
            期限日と同じ
          </option>
          <option value="tomorrow">明日</option>
          <option value="30m">30分後</option>
          <option value="1h">1時間後</option>
          <option value="4h">4時間後</option>
          <option value="24h">24時間後</option>
          <option value="1w">1週間後</option>
          <option value="next-week">来週（月曜日）</option>
          <option value="next-month">来月（1日）</option>
          <option value="custom">日時指定</option>
        </select>
      </label>
      {mode === "custom" && (
        <label>
          通知時刻
          <input
            type="datetime-local"
            value={value.reminder_at.slice(0, 16)}
            onChange={(e) => onChange({ reminder_at: e.target.value })}
          />
        </label>
      )}
      {mode === "deadline" && (
        <label>
          期限日の通知時刻
          <input
            type="time"
            value={value.reminder_time || defaultTime}
            onChange={(e) =>
              onChange({ reminder_time: e.target.value || defaultTime })
            }
          />
        </label>
      )}
      <span className="reminder-status" role="status">
        {at ? "通知：" + at.replace("T", " ").slice(0, 16) : "通知なし"}
      </span>
    </>
  );
}
