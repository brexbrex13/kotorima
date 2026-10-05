import { useEffect, useRef, useState } from "react";
import { api } from "./api";
import { editable, mergeDraft } from "./drafts";
import { Category, Snapshot, Task } from "./types";
import { Editor } from "./Editor";
import { ReminderFields } from "./ReminderFields";
import { Icon } from "./Icons";
export type DraftHandle = { leave: () => Promise<boolean> };
export function TaskDetail({
  task,
  defaultTime,
  categories,
  onClose,
  onSaved,
  onError,
  handle,
}: {
  defaultTime?: string;
  task: Task;
  categories: Category[];
  onClose: () => void;
  onSaved: () => void;
  onError: (e: unknown) => void;
  handle: React.MutableRefObject<DraftHandle | null>;
}) {
  const [draft, setDraft] = useState(task),
    [status, setStatus] = useState("保存済み"),
    [busy, setBusy] = useState(false),
    [leaveDialog, setLeaveDialog] = useState(false);
  const pendingLeave = useRef<((result: boolean) => void) | null>(null);
  const current = useRef(task),
    saved = useRef(task),
    chain = useRef(Promise.resolve()),
    uploading = useRef(false);
  const change = (patch: Partial<Task>) => {
    const v = { ...current.current, ...patch };
    current.current = v;
    setDraft(v);
    setStatus("未保存");
    localStorage.setItem(
      "draft:" + task.id,
      JSON.stringify({ draft: v, base: saved.current }),
    );
  };
  const flush = () => {
    chain.current = chain.current
      .catch(() => {})
      .then(async () => {
        if (uploading.current)
          throw new Error("画像を保存中です。完了後に操作してください");
        const v = current.current;
        if (
          JSON.stringify({ ...v, version: 0 }) ===
          JSON.stringify({ ...saved.current, version: 0 })
        )
          return;
        setStatus("保存中…");
        try {
          const base = saved.current;
          let result: Task;
          try {
            result = await api<Task>("SaveTask", {
              ...v,
              version: base.version,
            });
          } catch (e) {
            if (!String(e).includes("別の画面")) throw e;
            const snapshot = await api<Snapshot>("GetSnapshot");
            const latest = snapshot.tasks.find((t) => t.id === task.id);
            if (!latest) throw e;
            result = await api<Task>("SaveTask", mergeDraft(base, v, latest));
          }
          const during = current.current;
          current.current = { ...result };
          for (const key of editable) {
            if (during[key] !== v[key])
              Object.assign(current.current, { [key]: during[key] });
          }
          saved.current = result;
          setDraft(current.current);
          if (
            JSON.stringify({ ...current.current, version: 0 }) ===
            JSON.stringify({ ...result, version: 0 })
          ) {
            localStorage.removeItem("draft:" + task.id);
            setStatus("保存済み");
          } else setStatus("未保存");
          onSaved();
        } catch (e) {
          setStatus("保存失敗 — 入力はこの画面に残っています");
          throw e;
        }
      });
    return chain.current;
  };
  const dirty = () =>
    editable.some((key) => current.current[key] !== saved.current[key]);
  const discard = () => {
    localStorage.removeItem("draft:" + task.id);
    current.current = { ...saved.current };
    setDraft(current.current);
    setStatus("保存済み");
  };
  const requestLeave = () => {
    if (busy || uploading.current) return Promise.resolve(false);
    if (!dirty()) return Promise.resolve(true);
    if (pendingLeave.current) return Promise.resolve(false);
    setLeaveDialog(true);
    return new Promise<boolean>((resolve) => {
      pendingLeave.current = resolve;
    });
  };
  const finishLeave = (result: boolean) => {
    setLeaveDialog(false);
    const resolve = pendingLeave.current;
    pendingLeave.current = null;
    resolve?.(result);
  };
  handle.current = { leave: requestLeave };
  useEffect(() => {
    const raw = localStorage.getItem("draft:" + task.id);
    if (raw) {
      try {
        const parsed = JSON.parse(raw) as { draft?: Task; base?: Task } & Task;
        const v = parsed.draft ?? parsed;
        if (v.id === task.id) {
          current.current = mergeDraft(parsed.base ?? task, v, task);
          setDraft(current.current);
          setStatus("未保存の入力を復元しました");
        }
      } catch (e) {
        onError(e);
        setStatus("保存失敗 — 未保存の入力を別の場所へコピーしてください");
      }
    }
    return () => {
      handle.current = null;
      pendingLeave.current?.(false);
    };
  }, []);
  const saveAndClose = async () => {
    setBusy(true);
    try {
      await flush();
      onClose();
    } catch (e) {
      onError(e);
    } finally {
      setBusy(false);
    }
  };
  const leave = async () => {
    if (await requestLeave()) onClose();
  };
  const saveBeforeLeave = async () => {
    setBusy(true);
    try {
      await flush();
      finishLeave(true);
    } catch (e) {
      onError(e);
    } finally {
      setBusy(false);
    }
  };
  const state = async (value: string) => {
    if (!(await requestLeave())) return;
    setBusy(true);
    try {
      await api("SetState", task.id, value);
      onSaved();
      onClose();
    } catch (e) {
      onError(e);
    } finally {
      setBusy(false);
    }
  };
  return (
    <aside className="detail task-detail" aria-label="タスクの詳細">
      <header>
        <div className="detail-heading">
          <h2>{task.series_id ? "定期タスク" : "タスクの詳細"}</h2>
          <span
            role="status"
            className={status.startsWith("保存失敗") ? "danger" : "muted"}
            data-tip={status.startsWith("保存失敗") ? status : undefined}
          >
            {status}
          </span>
        </div>
        <div className="detail-tools">
          <button
            aria-label="重要"
            aria-pressed={draft.important}
            data-tip="重要を切り替える"
            disabled={busy}
            onClick={() => change({ important: !draft.important })}
          >
            <Icon name="star" filled={draft.important} />
          </button>
          <button
            aria-label={draft.deleted_at ? "ごみ箱から戻す" : "ごみ箱へ"}
            data-tip={draft.deleted_at ? "ごみ箱から戻す" : "ごみ箱へ"}
            className="danger"
            disabled={busy}
            onClick={() => void state(draft.deleted_at ? "restore" : "trash")}
          >
            <Icon name={draft.deleted_at ? "undo" : "trash"} />
          </button>
          {!draft.deleted_at && (
            <button
              aria-label={draft.status === "pending" ? "完了" : "再開する"}
              data-tip={draft.status === "pending" ? "完了" : "再開する"}
              className="complete-task"
              disabled={busy}
              onClick={() =>
                void state(draft.status === "pending" ? "done" : "pending")
              }
            >
              <Icon name={draft.status === "pending" ? "check" : "undo"} />
            </button>
          )}
          <button
            aria-label="詳細を閉じる"
            data-tip="詳細を閉じる"
            disabled={busy}
            onClick={() => void leave()}
          >
            <Icon name="close" />
          </button>
        </div>
      </header>
      <div className="detail-body">
        <fieldset className="form-section">
          <legend>内容</legend>
          <textarea
            aria-label="タスク名"
            className="detail-title"
            value={draft.title}
            onChange={(e) => change({ title: e.target.value })}
          />
          <label className="check notice-memo-option">
            <input
              type="checkbox"
              checked={!!draft.show_memo_in_notice}
              onChange={(e) =>
                change({ show_memo_in_notice: e.target.checked })
              }
            />
            通知にメモを表示する
          </label>
          <Editor
            value={draft.memo}
            onChange={(memo) => change({ memo })}
            onError={onError}
            onBusy={(v) => {
              uploading.current = v;
              setBusy(v);
            }}
          />
        </fieldset>
        <fieldset className="form-section">
          <legend>期限・通知</legend>
          <div className="formgrid">
            <ReminderFields
              value={draft}
              onChange={change}
              defaultTime={defaultTime}
            />
          </div>
        </fieldset>
        <fieldset className="form-section category-section">
          <legend>カテゴリ</legend>
          <select
            aria-label="カテゴリ"
            value={draft.category_id}
            onChange={(e) => change({ category_id: +e.target.value })}
          >
            {categories.map((c) => (
              <option key={c.id} value={c.id}>
                {c.name}
              </option>
            ))}
          </select>
        </fieldset>
      </div>
      <footer>
        <button
          disabled={busy}
          onClick={() => {
            discard();
            onClose();
          }}
        >
          変更を破棄
        </button>
        <button
          className="primary"
          disabled={busy}
          onClick={() => void saveAndClose()}
        >
          今すぐ保存
        </button>
      </footer>
      {leaveDialog && (
        <div className="overlay">
          <section
            className="modal unsaved-dialog"
            role="dialog"
            aria-modal="true"
            aria-label="未保存の変更"
            onKeyDown={(e) => {
              if (e.key === "Escape") {
                e.stopPropagation();
                finishLeave(false);
              }
            }}
          >
            <h2>変更はまだ保存されていません</h2>
            <p>保存するまで、期限・通知などの変更は反映されません。</p>
            <div className="actions">
              <button
                disabled={busy}
                autoFocus
                onClick={() => finishLeave(false)}
              >
                編集に戻る
              </button>
              <button
                disabled={busy}
                onClick={() => {
                  discard();
                  finishLeave(true);
                }}
              >
                破棄して続ける
              </button>
              <button
                className="primary"
                disabled={busy}
                onClick={() => void saveBeforeLeave()}
              >
                保存して続ける
              </button>
            </div>
          </section>
        </div>
      )}
    </aside>
  );
}
