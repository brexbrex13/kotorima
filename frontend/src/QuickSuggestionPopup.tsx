import { useEffect, useState } from "react";
import { api, on } from "./api";
import { useTheme } from "./theme";
import { Snapshot } from "./types";
export function QuickSuggestionPopup() {
  const [v, setV] = useState<{ items: string[]; index: number }>({
    items: [],
    index: -1,
  });
  const [theme, setTheme] = useState<Snapshot["settings"]["theme"]>();
  useTheme(theme);
  useEffect(() => {
    let sequence = 0,
      live = true;
    const reload = () => {
      const current = ++sequence;
      void api<typeof v>("GetQuickSuggestions").then((value) => {
        if (live && sequence === current) setV(value);
      });
    };
    const reloadTheme = () =>
      void api<Snapshot>("GetSnapshot").then((value) => {
        if (live) setTheme(value.settings.theme);
      });
    const off = [
      on("board:mini-suggestions", reload),
      on("board:changed", reloadTheme),
    ];
    reload();
    reloadTheme();
    void api("Ready", "quick-suggestions");
    return () => {
      live = false;
      off.forEach((f) => f());
    };
  }, []);
  return (
    <div
      className="quick-suggestion-window"
      role="listbox"
      aria-label="タスク名の候補"
    >
      {v.items.map((title, i) => (
        <button
          key={title}
          role="option"
          aria-selected={i === v.index}
          className={i === v.index ? "active" : ""}
          onMouseDown={(e) => e.preventDefault()}
          onClick={() => void api("ChooseQuickSuggestion", title)}
        >
          {title}
        </button>
      ))}
    </div>
  );
}
