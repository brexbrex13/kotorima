import { useEffect, useRef, useState } from "react";
import { Task } from "./types";
const fold = (v: string) =>
  v
    .trim()
    .normalize("NFKC")
    .toLocaleLowerCase()
    .replace(/[ァ-ヶ]/g, (c) => String.fromCharCode(c.charCodeAt(0) - 0x60));
export function suggestions(
  tasks: Task[],
  query: string,
  minimum: number,
  category = -1,
): string[] {
  const q = fold(query);
  if (minimum === 0 || query.includes("\n")) return [];
  const counts = new Map<
    string,
    { title: string; count: number; recent: string; category: boolean }
  >();
  for (const t of tasks) {
    if (t.series_id || t.deleted_at) continue;
    const key = fold(t.title);
    if (!key.startsWith(q) || (q && key === q)) continue;
    const old = counts.get(key);
    if (old) {
      old.count++;
      old.category ||= t.category_id === category;
      if (t.created_at > old.recent) {
        old.recent = t.created_at;
        old.title = t.title;
      }
    } else
      counts.set(key, {
        title: t.title,
        count: 1,
        recent: t.created_at,
        category: t.category_id === category,
      });
  }
  return [...counts.values()]
    .filter((v) => v.count >= minimum)
    .sort(
      (a, b) =>
        Number(b.category) - Number(a.category) ||
        b.count - a.count ||
        b.recent.localeCompare(a.recent),
    )
    .slice(0, 5)
    .map((v) => v.title);
}
export function useSuggestions(
  tasks: Task[],
  query: string,
  minimum: number,
  onChoose: (title: string) => void,
  category = -1,
) {
  const listRef = useRef<HTMLDivElement>(null);
  const composing = useRef(false),
    commitEnter = useRef(0);
  const [imeQuery, setImeQuery] = useState<string | null>(null);
  const [closed, setClosed] = useState<string | null>(null),
    [index, setIndex] = useState(-1),
    [active, setActive] = useState(false);
  const matching = suggestions(tasks, imeQuery ?? query, minimum, category);
  const items =
    !active || closed === query
      ? []
      : matching.length || imeQuery === null
        ? matching
        : suggestions(tasks, "", minimum, category);
  useEffect(() => {
    listRef.current
      ?.querySelector<HTMLElement>(".active")
      ?.scrollIntoView?.({ block: "nearest" });
  }, [index]);
  const choose = (title: string) => {
    onChoose(title);
    setClosed(title);
    setIndex(-1);
  };
  const keyDown = (e: React.KeyboardEvent, enabled = true) => {
    if (
      composing.current ||
      e.nativeEvent.isComposing ||
      e.keyCode === 229 ||
      (performance.now() < commitEnter.current && e.key === "Enter")
    )
      return true;
    if (!enabled || !items.length) return false;
    if (e.key === "ArrowDown" || e.key === "ArrowUp") {
      e.preventDefault();
      setIndex((i) =>
        i < 0
          ? e.key === "ArrowDown"
            ? 0
            : items.length - 1
          : (i + (e.key === "ArrowDown" ? 1 : -1) + items.length) %
            items.length,
      );
      return true;
    }
    if ((e.key === "Tab" && !e.shiftKey) || (e.key === "Enter" && index >= 0)) {
      e.preventDefault();
      choose(items[Math.min(Math.max(index, 0), items.length - 1)]);
      return true;
    }
    if (e.key === "Escape") {
      e.preventDefault();
      e.stopPropagation();
      setClosed(query);
      setIndex(-1);
      return true;
    }
    return false;
  };
  const list = items.length ? (
    <div
      ref={listRef}
      className="task-suggestions"
      role="listbox"
      aria-label="タスク名の候補"
    >
      {items.map((title, i) => (
        <button
          key={title}
          role="option"
          aria-selected={i === index}
          className={i === index ? "active" : ""}
          onMouseDown={(e) => e.preventDefault()}
          onClick={() => choose(title)}
        >
          {title}
        </button>
      ))}
    </div>
  ) : null;
  return {
    list,
    items,
    index,
    choose,
    keyDown,
    compositionStart: () => {
      composing.current = true;
      commitEnter.current = 0;
      setImeQuery("");
    },
    compositionUpdate: (e: React.CompositionEvent) => {
      setImeQuery(e.data);
      setClosed(null);
    },
    compositionEnd: () => {
      composing.current = false;
      commitEnter.current = performance.now() + 80;
      setImeQuery(null);
    },
    keyUp: () => {
      if (!composing.current) commitEnter.current = 0;
    },
    open: items.length > 0,
    focus: () => {
      commitEnter.current = 0;
      setActive(true);
      setClosed(null);
      setIndex(-1);
    },
    blur: () => {
      commitEnter.current = 0;
      setActive(false);
    },
    reset: () => {
      setActive(true);
      setClosed(null);
      setIndex(-1);
    },
  };
}
