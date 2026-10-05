import { useEffect, useState } from "react";
import { api, on } from "./api";
export function UndoToast({ onError }: { onError: (e: unknown) => void }) {
  const [completion, setCompletion] = useState<{
      token: string;
      title: string;
    } | null>(null),
    [busy, setBusy] = useState(false);
  useEffect(
    () =>
      on("board:completed", (data) =>
        setCompletion(data as { token: string; title: string }),
      ),
    [],
  );
  useEffect(() => {
    if (!completion) return;
    const timer = setTimeout(() => setCompletion(null), 10000);
    return () => clearTimeout(timer);
  }, [completion]);
  if (!completion) return null;
  return (
    <div className="undo-toast" role="status">
      <span>{completion.title} を完了</span>
      <button
        disabled={busy}
        onClick={async () => {
          setBusy(true);
          try {
            await api("UndoCompletion", completion.token);
            setCompletion(null);
          } catch (e) {
            onError(e);
          } finally {
            setBusy(false);
          }
        }}
      >
        元に戻す
      </button>
      <button
        aria-label="取り消し表示を閉じる"
        onClick={() => setCompletion(null)}
      >
        ×
      </button>
    </div>
  );
}
