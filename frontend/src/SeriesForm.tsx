import { useEffect, useState } from "react";
import { api } from "./api";
import { Category, Series, date } from "./types";
import { Editor } from "./Editor";
export function SeriesForm({
  initial,
  categories,
  defaultShowDays,
  onClose,
  onSaved,
  onError,
}: {
  initial: Series;
  categories: Category[];
  defaultShowDays: number;
  onClose: () => void;
  onSaved: () => void;
  onError: (e: unknown) => void;
}) {
  const [v, setV] = useState({
      ...initial,
      category_id: initial.category_id || categories[0]?.id || 0,
      show_days: initial.show_days < 0 ? defaultShowDays : initial.show_days,
      due_time: "",
      end_date: "",
      notify_mode:
        initial.notify_mode === "hours" ? "days" : initial.notify_mode,
    }),
    [busy, setBusy] = useState(false),
    [uploading, setUploading] = useState(false);
  const patch = (p: Partial<Series>) => setV((x) => ({ ...x, ...p }));
  const lead = v.show_days < 0 ? defaultShowDays : v.show_days;
  const save = async () => {
    setBusy(true);
    try {
      await api("SaveSeries", v);
      onSaved();
      onClose();
    } catch (e) {
      onError(e);
    } finally {
      setBusy(false);
    }
  };
  const [preview, setPreview] = useState<{
    deadline: string;
    add_at: string;
    reminder_at: string;
  } | null>(null);
  useEffect(() => {
    let current = true;
    const timer = setTimeout(() => {
      void api<{ deadline: string; add_at: string; reminder_at: string }>(
        "PreviewSeries",
        v,
      )
        .then((p) => {
          if (current) setPreview(p);
        })
        .catch(() => {
          if (current) setPreview(null);
        });
    }, 200);
    return () => {
      current = false;
      clearTimeout(timer);
    };
  }, [v, defaultShowDays]);
  return (
    <div className="overlay">
      <section className="modal large" aria-label="定期タスク設定">
        <header>
          <h2>{v.id ? "定期設定の編集" : "定期タスクを追加"}</h2>
          <button
            disabled={busy || uploading}
            onClick={onClose}
            aria-label="定期設定を閉じる"
          >
            ×
          </button>
        </header>
        <div className="modal-body">
          <fieldset className="form-section">
            <legend>内容</legend>
            <label>
              タスク名
              <input
                value={v.title}
                onChange={(e) => patch({ title: e.target.value })}
              />
            </label>
            <div className="formgrid">
              <label>
                カテゴリ
                <select
                  value={v.category_id}
                  onChange={(e) => patch({ category_id: +e.target.value })}
                >
                  {categories.map((c) => (
                    <option key={c.id} value={c.id}>
                      {c.name}
                    </option>
                  ))}
                </select>
              </label>
              <label className="check">
                <input
                  type="checkbox"
                  checked={v.important}
                  onChange={(e) => patch({ important: e.target.checked })}
                />
                重要
              </label>
            </div>
          </fieldset>
          <fieldset className="form-section">
            <legend>期限・繰り返し</legend>
            <label>
              繰り返し
              <select
                value={v.period + ":" + v.interval}
                onChange={(e) => {
                  const [period, interval] = e.target.value.split(":");
                  patch({ period, interval: +interval });
                }}
              >
                <option value="daily:1">毎日</option>
                <option value="weekly:1">毎週</option>
                <option value="weekly:2">隔週</option>
                <option value="monthly:1">毎月</option>
                <option value="monthly:2">2か月ごと</option>
                <option value="monthly:3">3か月ごと</option>
                <option value="yearly:1">毎年</option>
              </select>
            </label>
            {v.period === "weekly" && (
              <div className="weekdays" role="group" aria-label="期限の曜日">
                {["日", "月", "火", "水", "木", "金", "土"].map((n, i) => (
                  <button
                    key={i}
                    aria-pressed={v.weekdays.includes(i)}
                    onClick={() =>
                      patch({
                        weekdays: v.weekdays.includes(i)
                          ? v.weekdays.filter((x) => x !== i)
                          : [...v.weekdays, i].sort(),
                      })
                    }
                  >
                    {n}
                  </button>
                ))}
              </div>
            )}
            {(v.period === "monthly" || v.period === "yearly") && (
              <div className="formgrid">
                {v.period === "yearly" && (
                  <label>
                    月
                    <select
                      value={v.month}
                      onChange={(e) => patch({ month: +e.target.value })}
                    >
                      {Array.from({ length: 12 }, (_, i) => (
                        <option key={i} value={i + 1}>
                          {i + 1}月
                        </option>
                      ))}
                    </select>
                  </label>
                )}
                <label>
                  期限日
                  <select
                    value={v.month_day}
                    onChange={(e) => patch({ month_day: +e.target.value })}
                  >
                    <option value={0}>月末</option>
                    {Array.from({ length: 31 }, (_, i) => (
                      <option key={i} value={i + 1}>
                        {i + 1}日
                      </option>
                    ))}
                  </select>
                </label>
              </div>
            )}
            <label
              className="inline-field"
              data-tip="期限まで残り0〜指定日数になったら追加します。登録時も同じ判定です。"
            >
              タスクリストへの追加
              <input
                aria-label="タスクリストへの追加"
                type="number"
                min={0}
                max={366}
                value={lead}
                onChange={(e) => patch({ show_days: +e.target.value })}
              />
              日前
            </label>
          </fieldset>
          <fieldset className="form-section">
            <legend>通知</legend>
            <label
              className="check"
              data-tip="期限になっただけでは通知しません。この設定で通知する日時を決めます。"
            >
              <input
                type="checkbox"
                checked={v.notify_mode !== "off"}
                onChange={(e) =>
                  patch({ notify_mode: e.target.checked ? "days" : "off" })
                }
              />
              通知する
            </label>
            {v.notify_mode !== "off" && (
              <div className="formgrid">
                <label className="inline-field">
                  期限の
                  <input
                    aria-label="通知する日数"
                    type="number"
                    min={0}
                    max={366}
                    value={v.notify_days}
                    onChange={(e) => patch({ notify_days: +e.target.value })}
                  />
                  日前
                </label>
                <label>
                  通知時刻
                  <input
                    type="time"
                    value={v.notify_time}
                    onChange={(e) => patch({ notify_time: e.target.value })}
                  />
                </label>
              </div>
            )}
            {v.notify_mode !== "off" && v.notify_days > lead && (
              <p className="danger" role="alert">
                タスクの追加を通知より前に設定してください。
              </p>
            )}
          </fieldset>
          <details>
            <summary>メモ</summary>
            <label className="check notice-memo-option">
              <input
                type="checkbox"
                checked={!!v.show_memo_in_notice}
                onChange={(e) =>
                  patch({ show_memo_in_notice: e.target.checked })
                }
              />
              通知にメモを表示する
            </label>
            <Editor
              value={v.memo}
              onChange={(memo) => patch({ memo })}
              onError={onError}
              onBusy={setUploading}
            />
          </details>
          {preview && (
            <p
              className="schedule-preview"
              data-tip="編集は作成済みのタスクには反映しません。再開時も過去分を補充しません。"
            >
              追加 {preview.add_at <= date() ? "保存時" : preview.add_at} · 期限{" "}
              {preview.deadline}
              {preview.reminder_at &&
                " · 通知 " + preview.reminder_at.replace("T", " ").slice(0, 16)}
            </p>
          )}
        </div>
        <footer>
          <button
            className="primary"
            disabled={
              busy ||
              uploading ||
              !v.title.trim() ||
              (v.notify_mode !== "off" && v.notify_days > lead)
            }
            onClick={() => void save()}
          >
            {busy ? "保存中…" : "今すぐ保存"}
          </button>
        </footer>
      </section>
    </div>
  );
}
