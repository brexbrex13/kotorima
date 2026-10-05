import { MouseEvent } from "react";
import { Category } from "./types";

export function CategoryTabs({
  categories,
  selected,
  all,
  onSelect,
  onManage,
  onContextMenu,
}: {
  categories: Category[];
  selected: number;
  all: boolean;
  onSelect: (id: number) => void;
  onManage: (id: number) => void;
  onContextMenu: (id: number, event: MouseEvent) => void;
}) {
  return (
    <div className="filters category-tabs" aria-label="カテゴリ">
      <button
        className={all ? "active" : ""}
        onClick={() => onSelect(-1)}
        data-tip="全カテゴリのタスクを表示"
      >
        全て
      </button>
      {categories.map((c) => (
        <button
          key={c.id}
          className={selected === c.id ? "active" : ""}
          title={c.name}
          onClick={() => onSelect(c.id)}
          onContextMenu={(e) => onContextMenu(c.id, e)}
        >
          {c.name}
        </button>
      ))}
      <button
        className="category-add-tab"
        aria-label="カテゴリを追加"
        data-tip="カテゴリを追加・管理"
        onClick={() => onManage(0)}
      >
        ＋
      </button>
    </div>
  );
}
