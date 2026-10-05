import { SmartAdd } from "./useSmartAdd";
import { useEffect, useRef } from "react";
// Chips for automatic values, the similar-task warning and the invalid-key notice.
export function SmartHints({
  smart,
  onOpen,
  onHeight,
}: {
  smart: SmartAdd;
  onOpen: (taskId: number) => void;
  onHeight?: (height: number) => void;
}) {
  const root = useRef<HTMLDivElement>(null);
  useEffect(() => {
    if (!onHeight || !root.current || typeof ResizeObserver === "undefined")
      return;
    const observer = new ResizeObserver(() =>
      onHeight(root.current?.getBoundingClientRect().height ?? 0),
    );
    observer.observe(root.current);
    return () => observer.disconnect();
  }, [onHeight]);
  return (
    <div className="smart-hints" ref={root}>
      {smart.chips.length > 0 && (
        <div className="smart-chips" aria-label="推定した初期値">
          {smart.chips.map((c) => (
            <span key={c.field} className="smart-chip">
              {c.label}
              <button
                aria-label={`${c.label}を外す`}
                onClick={() => smart.drop(c.field)}
              >
                ×
              </button>
            </span>
          ))}
        </div>
      )}
      {smart.duplicate && (
        <button
          className="smart-duplicate"
          onClick={() => onOpen(smart.duplicate!.task_id)}
        >
          ⚠ 似たタスクがあります: {smart.duplicate.title} ›
        </button>
      )}
      {smart.invalid && (
        <p className="smart-invalid" role="status">
          AIのキーが無効です（設定で確認）
        </p>
      )}
      {smart.unavailable && !smart.invalid && (
        <p className="smart-invalid" role="status">
          AIに接続できないため通常登録に切り替えています。設定の「接続確認」で再接続できます。
        </p>
      )}
    </div>
  );
}
