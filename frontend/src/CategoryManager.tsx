import { useEffect, useRef, useState } from "react";
import { api } from "./api";
import { Category, ColorPreset, Snapshot } from "./types";
export const presets: ColorPreset[] = [
  { name: "標準", background: "#fffdf8", foreground: "#302d25" },
  { name: "レモン", background: "#fff0aa", foreground: "#493b12" },
  { name: "ピーチ", background: "#f9d8d2", foreground: "#542e29" },
  { name: "ミント", background: "#dcebd5", foreground: "#29432e" },
  { name: "スカイ", background: "#d6e8f4", foreground: "#254457" },
  { name: "ラベンダー", background: "#e7ddf4", foreground: "#443357" },
];
export function CategoryManager({
  data,
  onClose,
  onSaved,
  onError,
  initialCategoryId,
}: {
  data: Snapshot;
  onClose: () => void;
  onSaved: () => Promise<void>;
  onError: (e: unknown) => void;
  initialCategoryId?: number;
}) {
  const inputs = useRef(new Map<number, HTMLInputElement>());
  const addInput = useRef<HTMLInputElement>(null);
  useEffect(() => {
    if (initialCategoryId === undefined) return;
    const input =
      initialCategoryId > 0
        ? inputs.current.get(initialCategoryId)
        : addInput.current;
    input?.scrollIntoView?.({ block: "nearest" });
    input?.focus();
    input?.select();
  }, [initialCategoryId]);
  const [palette, setPalette] = useState<number | null>(null),
    [custom, setCustom] = useState<{
      value: ColorPreset;
      index: number;
    } | null>(null),
    [menu, setMenu] = useState<number | null>(null),
    [name, setName] = useState(""),
    [busy, setBusy] = useState(false);
  useEffect(() => {
    const outside = (e: PointerEvent) => {
      if (
        !(e.target as Element).closest(
          ".color-picker, .category-manager [data-popup], .category-manager .overlay",
        )
      ) {
        setPalette(null);
        setMenu(null);
      }
    };
    document.addEventListener("pointerdown", outside);
    return () => document.removeEventListener("pointerdown", outside);
  }, []);
  const [optimistic, setOptimistic] = useState<Record<number, Category>>({});
  const categories = data.categories
    .map((c) => optimistic[c.id] ?? c)
    .slice()
    .sort((a, b) => a.sort_order - b.sort_order || a.id - b.id);
  const run = async (fn: () => Promise<unknown>) => {
    setBusy(true);
    try {
      await fn();
      await onSaved();
    } catch (e) {
      onError(e);
    } finally {
      setBusy(false);
    }
  };
  const save = async (c: Category) => {
    setOptimistic((x) => ({ ...x, [c.id]: c }));
    await run(() => api("SaveCategory", c));
    setOptimistic((x) => {
      const next = { ...x };
      delete next[c.id];
      return next;
    });
  };
  const colors = data.settings.custom_colors ?? [];
  const customSave = async (remove = false) => {
    if (!custom) return;
    await run(async () => {
      const latest = await api<Snapshot>("GetSnapshot");
      const next = [...(latest.settings.custom_colors ?? [])];
      if (remove) next.splice(custom.index, 1);
      else if (custom.index < 0) next.push(custom.value);
      else next[custom.index] = custom.value;
      await api("SaveSettings", { ...latest.settings, custom_colors: next });
      setCustom(null);
      setMenu(null);
    });
  };
  return (
    <aside className="detail category-manager" aria-label="カテゴリ管理">
      <header>
        <h2>カテゴリ管理</h2>
        <button aria-label="カテゴリ管理を閉じる" onClick={onClose}>
          ×
        </button>
      </header>
      <div
        className="detail-body"
        onClick={(e) => {
          if (!(e.target as Element).closest(".color-picker, [data-popup]")) {
            setPalette(null);
            setMenu(null);
          }
        }}
      >
        {categories.map((c, i) => (
          <section className="manage-category" key={c.id}>
            <div className="manage-row">
              <input
                ref={(el) => {
                  if (el) inputs.current.set(c.id, el);
                  else inputs.current.delete(c.id);
                }}
                aria-label={c.name + "の名前"}
                defaultValue={c.name}
                key={c.name}
                onBlur={(e) => {
                  if (e.target.value.trim() && e.target.value !== c.name)
                    void save({ ...c, name: e.target.value });
                }}
              />
              <button
                className="color-button"
                data-popup
                data-tip="背景色と文字色を選ぶ"
                aria-label={c.name + "の配色"}
                style={{
                  background: c.color,
                  color: c.text_color || "#302d25",
                }}
                onClick={() => setPalette(palette === c.id ? null : c.id)}
              >
                Aa
              </button>
              <label
                className="check"
                data-tip="非表示でも通知と定期タスクの生成は続きます"
              >
                <input
                  type="checkbox"
                  checked={!c.dormant}
                  disabled={busy || i === 0}
                  data-tip={
                    i === 0 ? "先頭は標準追加先のため常に表示します" : undefined
                  }
                  onChange={(e) =>
                    void save({ ...c, dormant: !e.target.checked })
                  }
                />
                表示
              </label>
              <button
                aria-label={c.name + "を上へ"}
                disabled={busy || i === 0}
                onClick={() =>
                  void run(() =>
                    api(
                      "ReorderCategories",
                      categories.map((x, n) =>
                        n === i - 1
                          ? c.id
                          : n === i
                            ? categories[i - 1].id
                            : x.id,
                      ),
                    ),
                  )
                }
              >
                ↑
              </button>
              <button
                aria-label={c.name + "を下へ"}
                disabled={busy || i === categories.length - 1}
                onClick={() =>
                  void run(() =>
                    api(
                      "ReorderCategories",
                      categories.map((x, n) =>
                        n === i
                          ? categories[i + 1].id
                          : n === i + 1
                            ? c.id
                            : x.id,
                      ),
                    ),
                  )
                }
              >
                ↓
              </button>
              <button
                className="danger"
                aria-label={c.name + "を削除"}
                data-tip="タスクと定期設定は残るカテゴリの先頭へ移動します"
                disabled={busy || categories.length === 1}
                onClick={() => {
                  if (
                    confirm(
                      "このカテゴリを削除しますか？タスクと定期設定は残るカテゴリの先頭へ移動します。",
                    )
                  )
                    void run(() => api("DeleteCategory", c.id));
                }}
              >
                ×
              </button>
            </div>
            {palette === c.id && (
              <div className="color-picker">
                <div className="color-row">
                  {presets.map((p) => (
                    <button
                      key={p.name}
                      aria-label={p.name}
                      data-tip={p.name}
                      style={{ background: p.background, color: p.foreground }}
                      onClick={() => {
                        void save({
                          ...c,
                          color: p.background,
                          text_color: p.foreground,
                        });
                        setPalette(null);
                      }}
                    >
                      Aa
                    </button>
                  ))}
                  <button
                    className="custom-add"
                    aria-label="カスタム配色"
                    data-tip="背景色と文字色を登録する"
                    onClick={() =>
                      setCustom({
                        index: -1,
                        value: {
                          name: "カスタム",
                          background: c.color,
                          foreground: c.text_color || "#302d25",
                        },
                      })
                    }
                  >
                    ＋
                  </button>
                </div>
                {colors.length > 0 && (
                  <div className="color-row">
                    {colors.map((p, n) => (
                      <div key={n}>
                        <button
                          data-tip={p.name + "（右クリックで編集・削除）"}
                          aria-label={p.name}
                          style={{
                            background: p.background,
                            color: p.foreground,
                          }}
                          onClick={() => {
                            void save({
                              ...c,
                              color: p.background,
                              text_color: p.foreground,
                            });
                            setPalette(null);
                          }}
                          onContextMenu={(e) => {
                            e.preventDefault();
                            setMenu(n);
                          }}
                        >
                          Aa
                        </button>
                        {menu === n && (
                          <div className="custom-menu">
                            <button
                              onClick={() =>
                                setCustom({ index: n, value: { ...p } })
                              }
                            >
                              編集
                            </button>
                            <button
                              onClick={() => {
                                setCustom({ index: n, value: { ...p } });
                                setMenu(null);
                              }}
                            >
                              削除・管理
                            </button>
                          </div>
                        )}
                      </div>
                    ))}
                  </div>
                )}
              </div>
            )}
          </section>
        ))}
        <form
          className="category-add"
          onSubmit={(e) => {
            e.preventDefault();
            if (name.trim())
              void run(async () => {
                await api("SaveCategory", {
                  id: 0,
                  name,
                  color: presets[0].background,
                  text_color: presets[0].foreground,
                  sort_order: 0,
                  dormant: false,
                });
                setName("");
              });
          }}
        >
          <input
            ref={addInput}
            disabled={busy}
            aria-label="新しいカテゴリ名"
            value={name}
            onChange={(e) => setName(e.target.value)}
            placeholder="カテゴリ名"
          />
          <button disabled={busy || !name.trim()}>追加</button>
        </form>
      </div>
      {custom && (
        <div className="overlay">
          <section className="modal small">
            <header>
              <h2>カスタム配色</h2>
              <button onClick={() => setCustom(null)}>×</button>
            </header>
            <div className="modal-body">
              <label>
                名前
                <input
                  value={custom.value.name}
                  onChange={(e) =>
                    setCustom({
                      ...custom,
                      value: { ...custom.value, name: e.target.value },
                    })
                  }
                />
              </label>
              <div className="formgrid">
                <label>
                  背景色
                  <input
                    type="color"
                    value={custom.value.background}
                    onChange={(e) =>
                      setCustom({
                        ...custom,
                        value: { ...custom.value, background: e.target.value },
                      })
                    }
                  />
                </label>
                <label>
                  文字色
                  <input
                    type="color"
                    value={custom.value.foreground}
                    onChange={(e) =>
                      setCustom({
                        ...custom,
                        value: { ...custom.value, foreground: e.target.value },
                      })
                    }
                  />
                </label>
              </div>
              <div
                className="color-preview"
                style={{
                  background: custom.value.background,
                  color: custom.value.foreground,
                }}
              >
                タスクの文字色
              </div>
              {custom.index >= 0 && (
                <button
                  className="danger"
                  disabled={busy}
                  onClick={() => void customSave(true)}
                >
                  この配色を削除
                </button>
              )}
            </div>
            <footer>
              <button
                className="primary"
                disabled={busy}
                onClick={() => void customSave()}
              >
                保存
              </button>
            </footer>
          </section>
        </div>
      )}
    </aside>
  );
}
