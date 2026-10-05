import { useState } from "react";
import { api } from "./api";
import { localISO, Snapshot } from "./types";
export function NotificationPause({
  current,
  onClose,
  onSaved,
  onError,
}: {
  current: string;
  onClose: () => void;
  onSaved: () => Promise<void>;
  onError: (e: unknown) => void;
}) {
  const [until, setUntil] = useState(current.slice(0, 16)),
    [busy, setBusy] = useState(false);
  const save = async (value: string) => {
    setBusy(true);
    try {
      const latest = await api<Snapshot>("GetSnapshot");
      await api("SaveSettings", { ...latest.settings, pause_until: value });
      await onSaved();
      onClose();
    } catch (e) {
      onError(e);
    } finally {
      setBusy(false);
    }
  };
  return (
    <div className="overlay">
      <section className="modal pause-dialog" aria-label="通知を一時停止">
        <header>
          <h2>通知を一時停止</h2>
          <button aria-label="一時停止の設定を閉じる" onClick={onClose}>
            ×
          </button>
        </header>
        <div className="modal-body">
          <div className="pause-presets">
            {[10, 30, 60, 360, 1440].map((n) => (
              <button
                disabled={busy}
                key={n}
                onClick={() =>
                  void save(localISO(new Date(Date.now() + n * 60000)))
                }
              >
                {n < 60 ? n + "分" : n < 1440 ? n / 60 + "時間" : "1日"}
              </button>
            ))}
          </div>
          <label>
            再開日時
            <input
              type="datetime-local"
              value={until}
              onChange={(e) => setUntil(e.target.value)}
            />
          </label>
          <button disabled={busy} onClick={() => void save("")}>
            一時停止を解除
          </button>
        </div>
        <footer>
          <button
            className="primary"
            disabled={busy || !until}
            onClick={() => void save(until)}
          >
            この日時まで停止
          </button>
        </footer>
      </section>
    </div>
  );
}
