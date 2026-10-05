import {
  MouseEvent as ReactMouseEvent,
  useCallback,
  useEffect,
  useRef,
  useState,
} from "react";
import {
  DndContext,
  PointerSensor,
  KeyboardSensor,
  useSensor,
  useSensors,
  DragEndEvent,
  useDroppable,
  closestCenter,
  pointerWithin,
} from "@dnd-kit/core";
import {
  SortableContext,
  useSortable,
  verticalListSortingStrategy,
  sortableKeyboardCoordinates,
} from "@dnd-kit/sortable";
import { CSS } from "@dnd-kit/utilities";
import { api, on } from "./api";
import { useTheme } from "./theme";
import {
  Category,
  Series,
  Snapshot,
  Task,
  emptyTask,
  emptySeries,
  urgency,
  urgencyLabel,
  textOnly,
} from "./types";
import { TaskDetail, DraftHandle } from "./TaskDetail";
import { SeriesForm } from "./SeriesForm";
import { SettingsForm } from "./SettingsForm";
import { CategoryManager } from "./CategoryManager";
import { CategoryTabs } from "./CategoryTabs";
import { ContextMenu } from "./ContextMenu";
import { MemoPreview } from "./MemoPreview";
import { NotificationPause } from "./NotificationPause";
import { ReminderFields } from "./ReminderFields";
import { useSuggestions } from "./Suggestions";
import { blankOptions, JevStatus, Suggestion } from "./smartAdd";
import { useSmartAdd } from "./useSmartAdd";
import { SmartHints } from "./SmartHints";
import { UndoToast } from "./UndoToast";
import { date } from "./types";
import { Icon } from "./Icons";

function Bell() {
  return (
    <svg
      width="18"
      height="18"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.7"
      aria-hidden="true"
    >
      <path d="M18 8a6 6 0 0 0-12 0c0 7-3 7-3 9h18c0-2-3-2-3-9M10 21h4" />
    </svg>
  );
}
function Clock() {
  return (
    <svg
      width="17"
      height="17"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.7"
      aria-hidden="true"
    >
      <circle cx="12" cy="12" r="9" />
      <path d="M12 6v6l4 2" />
    </svg>
  );
}
function Row({
  t,
  u,
  open,
  complete,
  reopen,
  toggleImportant,
  toggleToday,
  drag,
  onContextMenu,
}: {
  t: Task;
  u: string;
  open: () => void;
  complete: () => void;
  reopen: () => void;
  toggleImportant: () => void;
  toggleToday: () => void;
  drag: boolean;
  onContextMenu: (event: ReactMouseEvent) => void;
}) {
  const s = useSortable({ id: t.id, disabled: !drag });
  return (
    <article
      ref={s.setNodeRef}
      onContextMenu={onContextMenu}
      style={{
        transform: CSS.Transform.toString(s.transform),
        transition: s.transition,
      }}
      className={"task-row " + u + (s.isDragging ? " dragging" : "")}
    >
      <button
        className="grip"
        aria-label={t.title + "を移動"}
        data-tip="ドラッグで並べ替え・カテゴリ移動。キーボードはSpace→矢印。"
        disabled={!drag}
        {...s.attributes}
        {...s.listeners}
      >
        ⠿
      </button>
      <button
        className="star-toggle"
        aria-label={t.title + "の重要マーク"}
        aria-pressed={t.important}
        data-tip="重要を切り替え"
        onClick={toggleImportant}
      >
        {t.important ? "★" : "☆"}
      </button>
      {t.status === "pending" && !t.deleted_at && (
        <button
          className="today-toggle"
          aria-label={t.title + "を今日やる"}
          aria-pressed={t.today_date === date()}
          data-tip="今日やるを切り替える。期限は変わりません。"
          onClick={toggleToday}
        >
          <Icon name="sun" filled={t.today_date === date()} />
        </button>
      )}
      <button className="card-content" onClick={open}>
        <span className="task-title">{t.title}</span>
        {t.series_id > 0 && (
          <span
            className="recurring-mark"
            data-tip="定期設定から追加されたタスク"
            aria-label="定期タスク"
          >
            ↻
          </span>
        )}
        {t.memo && (
          <span className="memo-mark" data-tip="メモあり">
            ▤
          </span>
        )}
      </button>
      {t.deadline && (
        <time
          className={"task-deadline " + u}
          data-tip={urgencyLabel[u] || "期限"}
        >
          {t.deadline.slice(0, 10).replace(/-/g, "/")}
        </time>
      )}
      {t.reminder_at && (
        <span data-tip={"通知 " + t.reminder_at.replace("T", " ").slice(0, 16)}>
          <Bell />
        </span>
      )}
      {t.status !== "pending" && !t.deleted_at && (
        <button
          className="row-reopen icon"
          aria-label={t.title + "を再開"}
          data-tip="未完了に戻す"
          onClick={reopen}
        >
          <Icon name="undo" />
        </button>
      )}
      {t.status === "pending" && !t.deleted_at && (
        <input
          type="checkbox"
          className="complete-check"
          aria-label={t.title + "を完了"}
          checked={false}
          onChange={complete}
        />
      )}
    </article>
  );
}
function Group({
  c,
  collapsed,
  toggle,
  count,
  children,
  onContextMenu,
}: {
  c: Category;
  collapsed: boolean;
  toggle: () => void;
  count: number;
  children: React.ReactNode;
  onContextMenu: (event: ReactMouseEvent) => void;
}) {
  const drop = useDroppable({ id: "category:" + c.id });
  return (
    <section
      ref={drop.setNodeRef}
      onContextMenu={onContextMenu}
      className={"category" + (drop.isOver ? " drop-over" : "")}
      style={{ background: c.color, color: c.text_color || "#302d25" }}
      aria-label={c.name}
    >
      <button
        className="category-caption"
        aria-label={c.name + (collapsed ? "を展開" : "を折り畳む")}
        aria-expanded={!collapsed}
        onClick={toggle}
      >
        <span aria-hidden="true">{collapsed ? "▸" : "▾"}</span> {c.name}
        {collapsed && <span className="category-count">{count}</span>}
      </button>
      {!collapsed && children}
    </section>
  );
}
export default function App() {
  const [data, setData] = useState<Snapshot | null>(null),
    [error, setError] = useState(""),
    [view, setView] = useState("board"),
    [todayOnly, setTodayOnly] = useState(false),
    [recurringOnly, setRecurringOnly] = useState(false),
    [category, setCategory] = useState(-1),
    [query, setQuery] = useState(""),
    [important, setImportant] = useState(false),
    [dated, setDated] = useState(false),
    [quick, setQuick] = useState(localStorage.getItem("quick-draft") ?? ""),
    [adding, setAdding] = useState(false),
    [quickOptionsOpen, setQuickOptionsOpen] = useState(false),
    [quickOptions, setQuickOptions] = useState(blankOptions),
    [jevStatus, setJevStatus] = useState<JevStatus | null>(null),
    [selected, setSelected] = useState<Task | null>(null),
    [series, setSeries] = useState<Series | null>(null),
    [togglingSeries, setTogglingSeries] = useState<number | null>(null),
    [settings, setSettings] = useState(false),
    [panel, setPanel] = useState(""),
    [menu, setMenu] = useState(false),
    [deadline, setDeadline] = useState(""),
    [search, setSearch] = useState(false);
  const [categoryFocus, setCategoryFocus] = useState<number | undefined>(
    undefined,
  );
  const [context, setContext] = useState<{
    x: number;
    y: number;
    categoryId?: number;
    task?: Task;
  } | null>(null);
  const showContext = (
    event: ReactMouseEvent,
    target: { categoryId?: number; task?: Task },
  ) => {
    event.preventDefault();
    event.stopPropagation();
    setContext({ x: event.clientX, y: event.clientY, ...target });
  };
  const manageCategory = (id: number) =>
    void navigate(() => {
      setCategoryFocus(id);
      setPanel("categories");
    });
  const [bulkBusy, setBulkBusy] = useState(false);
  const [temporaryCollapsed, setTemporaryCollapsed] = useState<
    Record<number, boolean>
  >({});
  const collapseContext = JSON.stringify([
    view,
    category,
    todayOnly,
    recurringOnly,
    important,
    dated,
    query,
  ]);
  const autoExpand =
    category >= 0 ||
    !!query.trim() ||
    todayOnly ||
    recurringOnly ||
    important ||
    dated ||
    view !== "board";
  useEffect(() => setTemporaryCollapsed({}), [collapseContext]);
  useTheme(data?.settings.theme);
  const handle = useRef<DraftHandle | null>(null),
    quickRef = useRef<HTMLTextAreaElement>(null),
    addLock = useRef(false),
    quickPointer = useRef(false),
    searchCompositionEnd = useRef(0);
  const suggest = useSuggestions(
    data?.tasks ?? [],
    quick,
    data?.settings.suggest_min_count ?? 3,
    setQuick,
    category,
  );
  // On "all categories" Jev may also pick the category; a selected tab always wins.
  const smart = useSmartAdd({
    text: quick,
    options: quickOptions,
    setOptions: setQuickOptions,
    enabled:
      !!data?.settings.smart_add &&
      !!jevStatus?.configured &&
      view === "board" &&
      !recurringOnly,
    defaultTime: data?.settings.reminder_default_time || "09:00",
    categories: data?.categories ?? [],
    existingTasks: data?.tasks ?? [],
    ask: (title) => api<Suggestion>("SuggestTask", title, category <= 0),
    scope: JSON.stringify([
      category,
      data?.settings.jev_provider,
      data?.settings.jev_custom_url,
      data?.settings.jev_custom_model,
      data?.settings.ai_timeout_seconds,
      jevStatus?.revision,
    ]),
    hide: [
      ...(category > 0 ? (["category"] as const) : []),
      ...(important ? (["important"] as const) : []),
    ],
  });
  useEffect(() => {
    const load = () =>
      void api<JevStatus>("GetJevStatus", "")
        .then((s) => setJevStatus(s ?? null))
        .catch(() => setJevStatus(null));
    load();
    return on("board:jev", load);
  }, []);
  const report = useCallback(
    (e: unknown) => setError(e instanceof Error ? e.message : String(e)),
    [],
  );
  const reloadSequence = useRef(0);
  const reload = useCallback(async () => {
    const sequence = ++reloadSequence.current;
    try {
      const snapshot = await api<Snapshot>("GetSnapshot");
      if (sequence === reloadSequence.current) setData(snapshot);
    } catch (e) {
      report(e);
    }
  }, [report]);
  const leaveDetail = () => handle.current?.leave() ?? Promise.resolve(true);
  const navigate = async (fn: () => void) => {
    try {
      if (!(await leaveDetail())) return;
      setSelected(null);
      setMenu(false);
      setDeadline("");
      fn();
    } catch (e) {
      report(e);
    }
  };
  const open = async (t: Task) => {
    try {
      if (!(await leaveDetail())) return;

      const latest = await api<Snapshot>("GetSnapshot");
      const target = latest.tasks.find((current) => current.id === t.id);
      if (!target) return;
      setData(latest);
      setPanel("");
      setDeadline("");
      setSelected(null);
      setTimeout(() => setSelected(target), 0);
    } catch (e) {
      report(e);
    }
  };
  const resetOperation = () => {
    setQuick("");
    localStorage.removeItem("quick-draft");
    setQuickOptions(blankOptions());
    smart.reset();
    setQuickOptionsOpen(false);
    suggest.blur();
    setView("board");
    setCategory(-1);
    setTodayOnly(false);
    setRecurringOnly(false);
    setQuery("");
    setSearch(false);
    setImportant(false);
    setDated(false);
    setSelected(null);
    setSeries(null);
    setSettings(false);
    setPanel("");
    setMenu(false);
    setDeadline("");
    setError("");
  };
  const closeBoard = async (mode = "hide") => {
    try {
      if (!(await leaveDetail())) return;
      resetOperation();
      await api("FinishClose", mode);
    } catch (e) {
      report(e);
    }
  };
  useEffect(() => {
    void reload();
    void api("Ready", "board").catch(report);
    const off = [
      on("board:notices", () => {
        void navigate(() => setPanel("notices"));
      }),
      on("board:changed", () => void reload()),
      on("board:error", report),
      on("board:quick", () => {
        setView("board");
        setRecurringOnly(false);
        setTodayOnly(false);
        suggest.focus();
        requestAnimationFrame(() => quickRef.current?.focus());
      }),
      on("board:open", (id) => {
        void api<Snapshot>("GetSnapshot")
          .then((v) => {
            setData(v);
            const t = v.tasks.find((x) => x.id === Number(id));
            if (t) {
              setView("board");
              setCategory(-1);
              setRecurringOnly(false);
              setTodayOnly(false);
              setQuery("");
              setImportant(false);
              setDated(false);
              void open(t);
            }
          })
          .catch(report);
      }),
      on("board:close-request", (mode) => {
        void closeBoard(String(mode));
      }),
    ];
    const timer = setInterval(() => void reload(), 60000);
    let resizeTimer: ReturnType<typeof setTimeout>;
    const resize = () => {
      clearTimeout(resizeTimer);
      resizeTimer = setTimeout(
        () => void api("SaveMainWindowSize").catch(report),
        400,
      );
    };
    window.addEventListener("resize", resize);
    const blur = () => {
      quickPointer.current = false;
      suggest.blur();
    };
    window.addEventListener("blur", blur);
    return () => {
      off.forEach((f) => f());
      clearInterval(timer);
      clearTimeout(resizeTimer);
      window.removeEventListener("resize", resize);
      window.removeEventListener("blur", blur);
    };
  }, []);
  useEffect(() => {
    localStorage.setItem("quick-draft", quick);
  }, [quick]);
  useEffect(() => {
    if (data?.categories.find((c) => c.id === category)?.dormant)
      setCategory(-1);
  }, [data, category]);
  useEffect(() => {
    const click = (e: PointerEvent) => {
      quickPointer.current = true;
      const target = e.target as Element;
      if (!target.closest('.app-menu,[aria-label="メニュー"]')) setMenu(false);
      if (!target.closest(".deadline-popup,[data-deadline-trigger]"))
        setDeadline("");
      if (!target.closest(".quick")) setQuickOptionsOpen(false);
      if (!target.closest(".inline-search")) setSearch(false);
    };
    const key = (e: KeyboardEvent) => {
      if (e.ctrlKey && e.key === "n") {
        e.preventDefault();
        setView("board");
        setRecurringOnly(false);
        setTodayOnly(false);
        suggest.focus();
        requestAnimationFrame(() => quickRef.current?.focus());
      }
      if (e.ctrlKey && e.key === "f") {
        e.preventDefault();
        setSearch(true);
        setTimeout(() => document.getElementById("search")?.focus(), 0);
      }
      if (e.key === "Escape") {
        setSearch(false);
        setMenu(false);
        setDeadline("");
        if (selected) void navigate(() => {});
        else setPanel("");
      }
    };
    const release = (e: MouseEvent) => {
      quickPointer.current = false;
      if (
        !(e.target as Element).closest(
          ".quick textarea,.quick .task-suggestions",
        )
      )
        suggest.blur();
    };
    const finishPointer = () => {
      quickPointer.current = false;
    };
    const cancelPointer = () => {
      quickPointer.current = false;
      suggest.blur();
    };
    document.addEventListener("pointerdown", click);
    document.addEventListener("pointerup", finishPointer);
    document.addEventListener("click", release);
    document.addEventListener("pointercancel", cancelPointer);
    window.addEventListener("keydown", key);
    return () => {
      document.removeEventListener("pointerdown", click);
      document.removeEventListener("pointerup", finishPointer);
      document.removeEventListener("click", release);
      document.removeEventListener("pointercancel", cancelPointer);
      window.removeEventListener("keydown", key);
    };
  }, [selected]);
  const sensors = useSensors(
    useSensor(PointerSensor, { activationConstraint: { distance: 6 } }),
    useSensor(KeyboardSensor, {
      coordinateGetter: sortableKeyboardCoordinates,
    }),
  );
  const add = async () => {
    if (addLock.current || view !== "board" || recurringOnly || !quick.trim())
      return;
    addLock.current = true;
    setAdding(true);
    const target =
      category > 0
        ? category
        : categories.some((c) => c.id === smart.currentOptions().category_id)
          ? smart.currentOptions().category_id
          : categories[0]?.id || 0;
    try {
      const currentOptions = smart.currentOptions();
      await api(
        "SaveSuggestedTask",
        {
          ...emptyTask(target),
          title: quick,
          ...currentOptions,
          category_id: target,
          important: important || currentOptions.important,
          today_date: todayOnly ? date() : "",
        },
        category <= 0 && smart.isAutomatic("category"),
      );
      setQuick("");
      setQuickOptions(blankOptions());
      smart.reset();
      suggest.blur();
      setQuickOptionsOpen(false);
      await reload();
    } catch (e) {
      report(e);
    } finally {
      setAdding(false);
      addLock.current = false;
      requestAnimationFrame(() => {
        if (
          document.activeElement === quickRef.current ||
          document.activeElement === document.body
        )
          quickRef.current?.focus();
      });
    }
  };
  const state = async (t: Task, value: string) => {
    try {
      if (selected?.id === t.id && !(await leaveDetail())) return;
      await api("SetState", t.id, value);
      if (selected?.id === t.id) setSelected(null);
      await reload();
    } catch (e) {
      report(e);
    }
  };
  const bulkDelete = async (permanent: boolean) => {
    if (bulkBusy) return;
    setBulkBusy(true);
    try {
      if (!(await leaveDetail())) return;
      const latest = await api<Snapshot>("GetSnapshot");
      const count = latest.tasks.filter((t) =>
        permanent ? !!t.deleted_at : !t.deleted_at && t.status === "done",
      ).length;
      if (
        !count ||
        !confirm(
          permanent
            ? `ごみ箱の${count}件を完全削除しますか？タスクと不要になった内部画像は元に戻せません。`
            : `完了済みの${count}件をすべてごみ箱へ移動しますか？検索・絞り込みに関係なく全件が対象です。`,
        )
      )
        return;
      await api(permanent ? "EmptyTrash" : "TrashCompleted");
      if (
        selected &&
        (permanent ? !!selected.deleted_at : selected.status === "done")
      ) {
        localStorage.removeItem("draft:" + selected.id);
        setSelected(null);
      }
    } catch (e) {
      report(e);
    } finally {
      await reload();
      setBulkBusy(false);
    }
  };
  if (!data)
    return (
      <main className="loading">
        <p>{error || "読み込み中…"}</p>
        <button onClick={() => void reload()}>再試行</button>
      </main>
    );
  const pending = data.tasks.filter(
    (t) => t.status === "pending" && !t.deleted_at,
  );
  const urgent = pending
    .filter((t) => urgency(t, data.settings))
    .sort((a, b) => a.deadline.localeCompare(b.deadline));
  const categories = data.categories
    .slice()
    .sort((a, b) => a.sort_order - b.sort_order || a.id - b.id);
  const visibleCategories = categories.filter((c) => !c.dormant);
  let tasks = data.tasks
    .filter((t) =>
      todayOnly
        ? !t.deleted_at && t.status === "pending"
        : view === "trash"
          ? !!t.deleted_at
          : view === "history"
            ? !t.deleted_at && t.status !== "pending"
            : !t.deleted_at && t.status === "pending",
    )
    .filter((t) => !recurringOnly || t.series_id > 0)
    .filter((t) => !todayOnly || t.today_date === date())
    .filter((t) => todayOnly || category < 0 || t.category_id === category)
    .filter((t) => !important || t.important)
    .filter((t) => !dated || t.deadline)
    .filter(
      (t) =>
        !query ||
        [
          t.title,
          textOnly(t.memo),
          categories.find((c) => c.id === t.category_id)?.name ?? "",
        ]
          .join("\n")
          .toLocaleLowerCase()
          .includes(query.toLocaleLowerCase()),
    );
  tasks = tasks.sort((a, b) => a.sort_order - b.sort_order || b.id - a.id);
  const groups = (
    view === "board" || todayOnly ? visibleCategories : categories
  )
    .filter((c) => todayOnly || category < 0 || c.id === category)
    .filter(
      (c) =>
        (view === "board" && !recurringOnly && !todayOnly) ||
        tasks.some((t) => t.category_id === c.id),
    );
  const drag = view === "board" && !todayOnly && !query && !important && !dated;
  const dropped = async (e: DragEndEvent) => {
    if (!e.over || e.active.id === e.over.id) return;
    const moving = tasks.find((t) => t.id === e.active.id);
    if (!moving) return;
    const target = String(e.over.id).startsWith("category:")
      ? Number(String(e.over.id).split(":")[1])
      : tasks.find((t) => t.id === e.over?.id)?.category_id;
    if (target === undefined) return;
    try {
      if (!(await leaveDetail())) return;
      if (moving.category_id !== target)
        await api("MoveTask", moving.id, target);
      const items = tasks.filter(
        (t) => t.category_id === target && t.id !== moving.id,
      );
      const index = items.findIndex((t) => t.id === e.over?.id);
      items.splice(index < 0 ? items.length : index, 0, moving);
      await api(
        "Reorder",
        items.map((t) => t.id),
      );
      await reload();
    } catch (e) {
      report(e);
      void reload();
    }
  };
  const popupTasks = pending
    .filter(
      (t) =>
        t.deadline &&
        (deadline === "all" || urgency(t, data.settings) === deadline),
    )
    .sort((a, b) => a.deadline.localeCompare(b.deadline));
  const setTab = (next: string, id = -1) =>
    void navigate(() => {
      setTodayOnly(false);
      setView(next);
      setCategory(id);
      setPanel("");
    });
  const pendingCount = data.notifications.filter((n) => !n.acknowledged).length;
  const inputAvailable = view === "board" && !recurringOnly;
  const addTraits = [todayOnly ? "今日やる" : "", important ? "重要" : ""]
    .filter(Boolean)
    .join("・");
  const categoryPrefix =
    category > 0
      ? `「${categories.find((c) => c.id === category)?.name ?? "選択カテゴリ"}」に`
      : "";
  const quickPlaceholder = !inputAvailable
    ? "この表示ではタスクを追加できません"
    : categoryPrefix +
      (addTraits
        ? addTraits + "のタスクとして登録"
        : "タスクを入力してEnterで追加");
  return (
    <div
      className={"app " + (data.settings.compact ? "compact" : "")}
      style={{ fontSize: data.settings.font_size }}
    >
      <header className="app-header">
        <h1 style={{ "--wails-draggable": "drag" } as React.CSSProperties}>
          Kotorima
        </h1>
        <div className="app-actions">
          <button
            className="icon"
            data-popup
            data-tip="期限の近い順に確認"
            data-deadline-trigger
            aria-label="期限順の一覧"
            onClick={() => {
              setMenu(false);
              setDeadline(deadline === "all" ? "" : "all");
            }}
          >
            <Bell />
          </button>
          <button
            className={"icon" + (view === "history" ? " active" : "")}
            aria-label="完了済み"
            data-tip="完了済み"
            aria-pressed={view === "history"}
            onClick={() => setTab(view === "history" ? "board" : "history")}
          >
            <Icon name="check" />
          </button>
          <button
            className="icon"
            data-popup
            aria-label="メニュー"
            onClick={() => {
              setDeadline("");
              setMenu(!menu);
            }}
          >
            ⋯
            {pendingCount > 0 && (
              <span className="notification-badge">{pendingCount}</span>
            )}
          </button>
          <button
            className="icon"
            aria-label="トレイに格納"
            data-tip="トレイに格納"
            onClick={() => void closeBoard()}
          >
            ×
          </button>
        </div>
      </header>
      {panel === "pause" && (
        <NotificationPause
          current={data.settings.pause_until}
          onClose={() => setPanel("")}
          onSaved={reload}
          onError={report}
        />
      )}
      {menu && (
        <nav className="popover app-menu" aria-label="メニュー項目">
          <button onClick={() => void navigate(() => setPanel("notices"))}>
            未確認の通知
            {pendingCount > 0 && (
              <span className="notification-badge">{pendingCount}</span>
            )}
          </button>
          <button onClick={() => setTab("trash")}>ごみ箱</button>
          <hr />
          <button onClick={() => void navigate(() => setPanel("pause"))}>
            通知を一時停止
          </button>
          <hr />
          <button
            onClick={() =>
              void navigate(() => {
                setCategoryFocus(undefined);
                setPanel("categories");
              })
            }
          >
            カテゴリ管理
          </button>
          <button onClick={() => void navigate(() => setPanel("series"))}>
            定期設定
          </button>
          <hr />
          <button
            aria-label="ダークモード"
            aria-pressed={data.settings.theme === "dark"}
            onClick={() =>
              void api("SaveSettings", {
                ...data.settings,
                theme: data.settings.theme === "dark" ? "light" : "dark",
              })
                .then(reload)
                .catch(report)
            }
          >
            ダークモード{" "}
            <span>{data.settings.theme === "dark" ? "✓" : ""}</span>
          </button>
          <button onClick={() => void navigate(() => setSettings(true))}>
            設定
          </button>
        </nav>
      )}
      {error && (
        <div className="error" role="alert">
          {error}
          <button onClick={() => setError("")}>×</button>
        </div>
      )}
      {data.settings.pause_until &&
        new Date(data.settings.pause_until) > new Date() && (
          <div className="pause">
            通知保留中
            <button
              onClick={() =>
                void api("SaveSettings", { ...data.settings, pause_until: "" })
                  .then(reload)
                  .catch(report)
              }
            >
              再開
            </button>
          </div>
        )}
      {urgent.length > 0 && (
        <section className="deadline-band" aria-label="期限の確認">
          {["overdue", "today", "near"].map((u) => {
            const items = urgent.filter((t) => urgency(t, data.settings) === u);
            return (
              items.length > 0 && (
                <div className={"band-group " + u} key={u}>
                  <button
                    className="band-label"
                    data-deadline-trigger
                    data-popup
                    onClick={() => {
                      setMenu(false);
                      setDeadline(deadline === u ? "" : u);
                    }}
                  >
                    {u === "today"
                      ? "今日"
                      : u === "near"
                        ? "近日"
                        : "期限超過"}{" "}
                    {items.length}
                  </button>
                  <div className="band-items">
                    {items.slice(0, 3).map((t) => (
                      <button
                        key={t.id}
                        data-tip={t.title + " · " + t.deadline}
                        onClick={() => void open(t)}
                      >
                        <span>{t.title}</span>
                      </button>
                    ))}
                    {items.length > 3 && (
                      <button
                        className="band-more"
                        data-deadline-trigger
                        data-popup
                        onClick={() => setDeadline(u)}
                      >
                        ＋{items.length - 3}
                      </button>
                    )}
                  </div>
                </div>
              )
            );
          })}
        </section>
      )}
      {deadline && (
        <section className="popover deadline-popup" aria-label="期限タスク一覧">
          <header>
            <strong>
              {deadline === "all" ? "期限順" : urgencyLabel[deadline]}
            </strong>
            <button
              aria-label="期限一覧を閉じる"
              onClick={() => setDeadline("")}
            >
              ×
            </button>
          </header>
          <div>
            {popupTasks.map((t) => (
              <button
                className="deadline-item"
                key={t.id}
                onClick={() => void open(t)}
              >
                <span>
                  {t.title}
                  <small>
                    {t.series_id ? "定期 · " : ""}
                    {categories.find((c) => c.id === t.category_id)?.name}
                  </small>
                </span>
                <time className={urgency(t, data.settings)}>
                  {t.deadline.slice(0, 10)}
                </time>
              </button>
            ))}
            {!popupTasks.length && (
              <p className="empty-small">該当するタスクはありません</p>
            )}
          </div>
        </section>
      )}
      <div className="workspace">
        <main className="board-main">
          {
            <div
              className={"quick" + (!inputAvailable ? " unavailable" : "")}
              data-popup
            >
              <textarea
                ref={quickRef}
                aria-label="新しい付箋"
                placeholder={quickPlaceholder}
                data-tip={
                  !suggest.open ? "Enterで追加、Shift+Enterで改行" : undefined
                }
                value={quick}
                readOnly={adding || !inputAvailable}
                aria-disabled={!inputAvailable}
                onCompositionStart={() => {
                  smart.compositionStart();
                  suggest.compositionStart();
                }}
                onCompositionUpdate={suggest.compositionUpdate}
                onCompositionEnd={() => {
                  suggest.compositionEnd();
                  smart.compositionEnd();
                }}
                onKeyUp={(e) => {
                  suggest.keyUp();
                  if (e.key === "Tab" && inputAvailable && !quick.trim())
                    suggest.focus();
                }}
                onClick={() => {
                  if (inputAvailable) suggest.focus();
                }}
                onBlur={() => {
                  if (!quickPointer.current) suggest.blur();
                }}
                onChange={(e) => {
                  setQuick(e.target.value);
                  suggest.reset();
                }}
                onKeyDown={(e) => {
                  if (!inputAvailable || adding) return;
                  if (suggest.keyDown(e)) return;
                  if (
                    e.key === "Enter" &&
                    !e.altKey &&
                    !e.shiftKey &&
                    !e.nativeEvent.isComposing &&
                    e.keyCode !== 229
                  ) {
                    e.preventDefault();
                    void add();
                  }
                }}
              />
              {inputAvailable && suggest.list}
              <button
                className="quick-clock"
                aria-label="登録時の期限・通知"
                aria-expanded={quickOptionsOpen}
                data-tip="期限・通知・重要を設定して登録"
                disabled={!inputAvailable || adding}
                onClick={() => setQuickOptionsOpen(!quickOptionsOpen)}
              >
                <Clock />
              </button>
              {smart.chips.length === 0 &&
                (quickOptions.deadline ||
                  quickOptions.reminder_at ||
                  quickOptions.important) && (
                  <small className="quick-options-summary">
                    {quickOptions.deadline && "期限 " + quickOptions.deadline}
                    {(quickOptions.reminder_at ||
                      quickOptions.reminder_mode === "deadline") &&
                      " · 通知あり"}
                    {quickOptions.important && " · ★"}
                  </small>
                )}
              {inputAvailable && (
                <SmartHints
                  smart={smart}
                  onOpen={(id) => void api("OpenTask", id).catch(report)}
                />
              )}
              {inputAvailable && quickOptionsOpen && (
                <div
                  className="popover quick-options"
                  aria-label="登録時の設定"
                >
                  <ReminderFields
                    value={quickOptions}
                    defaultTime={data.settings.reminder_default_time}
                    onChange={(p) => {
                      if ("deadline" in p) smart.touch("deadline");
                      if (
                        "reminder_at" in p ||
                        "reminder_mode" in p ||
                        "reminder_time" in p
                      )
                        smart.touch("reminder");
                      setQuickOptions((v) => ({ ...v, ...p }));
                    }}
                  />
                  <label className="check">
                    <input
                      type="checkbox"
                      checked={important || quickOptions.important}
                      disabled={important}
                      data-tip={
                        important
                          ? "重要で絞り込み中は、重要タスクとして追加します"
                          : undefined
                      }
                      onChange={(e) => {
                        smart.touch("important");
                        setQuickOptions({
                          ...quickOptions,
                          important: e.target.checked,
                        });
                      }}
                    />
                    重要
                  </label>
                </div>
              )}
            </div>
          }
          <CategoryTabs
            categories={visibleCategories}
            selected={category}
            all={view === "board" && category < 0}
            onSelect={(id) => setTab("board", id)}
            onManage={manageCategory}
            onContextMenu={(id, e) => showContext(e, { categoryId: id })}
          />
          <div className="task-filters" aria-label="タスクの絞り込み">
            <button
              aria-pressed={recurringOnly}
              onClick={() => setRecurringOnly(!recurringOnly)}
            >
              <span aria-hidden="true">↻</span>定期
            </button>
            <button
              disabled={view !== "board"}
              aria-pressed={todayOnly}
              onClick={() =>
                void navigate(() => {
                  setTodayOnly(!todayOnly);
                  setPanel("");
                })
              }
            >
              <Icon name="sun" />
              今日やる
            </button>

            <button
              aria-label="期限あり"
              aria-pressed={dated}
              data-tip="期限があるタスクだけ表示"
              onClick={() => setDated(!dated)}
            >
              <Icon name="clock" />
              期限あり
            </button>
            <button
              aria-label="重要"
              aria-pressed={important}
              data-tip="重要なタスクだけ表示"
              onClick={() => setImportant(!important)}
            >
              <Icon name="star" filled={important} />
              重要
            </button>
            <div className={"inline-search" + (query ? " active" : "")}>
              <button
                aria-label="検索"
                aria-expanded={search}
                data-tip="検索 Ctrl+F"
                onClick={() => {
                  setSearch(true);
                  requestAnimationFrame(() =>
                    document.getElementById("search")?.focus(),
                  );
                }}
              >
                <Icon name="search" />
              </button>
              {search && (
                <div className="search-popover">
                  <input
                    id="search"
                    aria-label="検索語"
                    placeholder="タスク・メモを検索"
                    value={query}
                    onChange={(e) => setQuery(e.target.value)}
                    onCompositionEnd={() => {
                      searchCompositionEnd.current = Date.now();
                    }}
                    onKeyDown={(e) => {
                      if (
                        e.key === "Enter" &&
                        !e.nativeEvent.isComposing &&
                        e.keyCode !== 229 &&
                        Date.now() - searchCompositionEnd.current > 80
                      )
                        setSearch(false);
                    }}
                  />
                  <button
                    aria-label="検索語を消去"
                    disabled={!query}
                    onClick={() => {
                      setQuery("");
                      document.getElementById("search")?.focus();
                    }}
                  >
                    ×
                  </button>
                </div>
              )}
              {!search && query && (
                <button
                  aria-label="検索を解除"
                  data-tip={"検索中：" + query}
                  onClick={() => setQuery("")}
                >
                  ×
                </button>
              )}
            </div>
          </div>
          {!todayOnly && (view === "history" || view === "trash") && (
            <div className="section-head">
              <h2>{view === "history" ? "完了済み" : "ごみ箱"}</h2>
              <div className="section-actions">
                <button
                  className="bulk-action"
                  disabled={
                    bulkBusy ||
                    !data.tasks.some((t) =>
                      view === "trash"
                        ? !!t.deleted_at
                        : !t.deleted_at && t.status === "done",
                    )
                  }
                  onClick={() => void bulkDelete(view === "trash")}
                >
                  {view === "trash"
                    ? "ごみ箱を空にする"
                    : "完了済みをすべてごみ箱へ"}
                </button>
                <button onClick={() => setTab("board")}>一覧に戻る</button>
              </div>
            </div>
          )}
          <DndContext
            sensors={sensors}
            collisionDetection={(args) => {
              const hits = pointerWithin(args);
              return hits.length
                ? hits.filter((h) => typeof h.id === "number").length
                  ? hits.filter((h) => typeof h.id === "number")
                  : hits
                : closestCenter(args);
            }}
            onDragEnd={(e) => void dropped(e)}
          >
            {groups.map((c) => {
              const items = tasks.filter((t) => t.category_id === c.id);
              return (
                <Group
                  c={c}
                  onContextMenu={(e) => showContext(e, { categoryId: c.id })}
                  collapsed={
                    selected?.category_id === c.id
                      ? false
                      : (temporaryCollapsed[c.id] ??
                        (!autoExpand &&
                          (data.settings.collapsed || []).includes(c.id)))
                  }
                  count={items.length}
                  toggle={() => {
                    const current =
                      temporaryCollapsed[c.id] ??
                      (!autoExpand &&
                        (data.settings.collapsed || []).includes(c.id));
                    if (autoExpand)
                      setTemporaryCollapsed((v) => ({
                        ...v,
                        [c.id]: !current,
                      }));
                    else
                      void api("SetCategoryCollapsed", c.id, !current)
                        .then(reload)
                        .catch(report);
                  }}
                  key={c.id}
                >
                  <SortableContext
                    items={items.map((t) => t.id)}
                    strategy={verticalListSortingStrategy}
                  >
                    {items.map((t) => (
                      <Row
                        key={t.id}
                        t={t}
                        onContextMenu={(e) => showContext(e, { task: t })}
                        u={urgency(t, data.settings)}
                        drag={drag}
                        open={() => void open(t)}
                        complete={() => void state(t, "done")}
                        reopen={() => void state(t, "pending")}
                        toggleToday={() =>
                          void api("ToggleToday", t.id)
                            .then(reload)
                            .catch(report)
                        }
                        toggleImportant={() =>
                          void api("ToggleImportant", t.id)
                            .then(reload)
                            .catch(report)
                        }
                      />
                    ))}
                  </SortableContext>
                  {!items.length && <div className="empty-category" />}
                </Group>
              );
            })}
          </DndContext>
          {!groups.length && <p className="empty-small">タスクはありません</p>}
        </main>
        {selected && (
          <>
            <button
              className="detail-dismiss"
              aria-label="詳細の外側を閉じる"
              onClick={() => void navigate(() => {})}
            />
            <TaskDetail
              key={selected.id}
              defaultTime={data.settings.reminder_default_time}
              task={selected}
              categories={categories}
              onClose={() => setSelected(null)}
              onSaved={() => void reload()}
              onError={report}
              handle={handle}
            />
          </>
        )}
        {panel === "categories" && (
          <CategoryManager
            data={data}
            initialCategoryId={categoryFocus}
            onClose={() => setPanel("")}
            onSaved={() => reload()}
            onError={report}
          />
        )}
        {panel === "series" && (
          <aside className="detail series-manager">
            <header>
              <h2>定期設定</h2>
              <button
                aria-label="定期設定を閉じる"
                onClick={() => setPanel("")}
              >
                ×
              </button>
            </header>
            <div className="detail-body">
              <button
                className="primary"
                onClick={() => setSeries(emptySeries())}
              >
                定期タスクを追加
              </button>
              {data.series
                .filter((s) => !s.deleted)
                .map((s) => (
                  <section className="series-row" key={s.id}>
                    <button
                      className="series-title"
                      onClick={() => setSeries(s)}
                    >
                      {s.title}
                    </button>
                    <small>次回期限 {s.next_due}</small>
                    <small>
                      未完了{" "}
                      {pending.filter((t) => t.series_id === s.id).length}件
                    </small>
                    <div className="series-auto-add">
                      <span>タスクの自動追加</span>
                      <button
                        className="series-switch"
                        role="switch"
                        aria-label={`${s.title}の自動追加`}
                        aria-checked={s.active}
                        disabled={togglingSeries !== null}
                        data-tip="OFFで今後の追加を一時停止します。追加済みのタスクは残ります。再開時に停止中の過去分は追加しません。"
                        onClick={async () => {
                          setTogglingSeries(s.id);
                          try {
                            await api("SaveSeries", {
                              ...s,
                              active: !s.active,
                            });
                            await reload();
                          } catch (e) {
                            report(e);
                          } finally {
                            setTogglingSeries(null);
                          }
                        }}
                      >
                        <span aria-hidden="true" />
                      </button>
                      <small>{s.active ? "ON" : "OFF"}</small>
                    </div>
                    <div className="actions">
                      <button onClick={() => setSeries(s)}>編集</button>
                      <button
                        className="danger"
                        onClick={() => {
                          if (
                            confirm(
                              "定期設定を削除しますか？作成済みのタスクは残ります。",
                            )
                          )
                            void api("StopSeries", s.id)
                              .then(reload)
                              .catch(report);
                        }}
                      >
                        削除
                      </button>
                    </div>
                  </section>
                ))}
            </div>
          </aside>
        )}
        {panel === "notices" && (
          <aside className="detail">
            <header>
              <h2>未確認の通知</h2>
              <button onClick={() => setPanel("")}>×</button>
            </header>
            <div className="detail-body">
              {data.notifications
                .filter((n) => !n.acknowledged)
                .map((n) => (
                  <section className="series-row" key={n.id}>
                    <button
                      className="series-title"
                      onClick={() =>
                        void api("OpenFromNotice", n.id, n.task_id).catch(
                          report,
                        )
                      }
                    >
                      {n.title}
                    </button>
                    <div className="actions">
                      {n.task_id > 0 && (
                        <>
                          <button
                            onClick={() =>
                              void api("Snooze", n.id, 10)
                                .then(reload)
                                .catch(report)
                            }
                          >
                            10分後
                          </button>
                          <button
                            onClick={() =>
                              void api("SetState", n.task_id, "done")
                                .then(reload)
                                .catch(report)
                            }
                          >
                            完了
                          </button>
                        </>
                      )}
                      <button
                        onClick={() =>
                          void api("Acknowledge", n.id)
                            .then(reload)
                            .catch(report)
                        }
                      >
                        閉じる
                      </button>
                    </div>
                  </section>
                ))}
            </div>
          </aside>
        )}
      </div>
      {series && (
        <SeriesForm
          initial={series}
          categories={categories}
          defaultShowDays={data.settings.series_show_days ?? 7}
          onClose={() => setSeries(null)}
          onSaved={() => void reload()}
          onError={report}
        />
      )}
      {context && (
        <ContextMenu
          key={`${context.x}:${context.y}:${context.task?.id ?? context.categoryId}`}
          x={context.x}
          y={context.y}
          label={context.task ? "タスクの操作" : "カテゴリの操作"}
          onClose={() => setContext(null)}
          items={
            context.task
              ? [
                  {
                    label: "詳細を開く",
                    action: () => void open(context.task!),
                  },
                  ...(context.task.deleted_at
                    ? [
                        {
                          label: "ごみ箱から戻す",
                          action: () => void state(context.task!, "restore"),
                        },
                      ]
                    : [
                        context.task.status === "pending"
                          ? {
                              label: "完了にする",
                              action: () => void state(context.task!, "done"),
                            }
                          : {
                              label: "未完了に戻す",
                              action: () =>
                                void state(context.task!, "pending"),
                            },
                        {
                          label: "ごみ箱へ送る",
                          action: () => void state(context.task!, "trash"),
                        },
                      ]),
                ]
              : [
                  {
                    label: "カテゴリ編集",
                    action: () => manageCategory(context.categoryId!),
                  },
                ]
          }
        />
      )}
      <UndoToast onError={report} />
      {settings && (
        <SettingsForm
          initial={data.settings}
          onClose={() => setSettings(false)}
          onSaved={() => void reload()}
          onError={report}
        />
      )}
    </div>
  );
}
export function Notifications() {
  const [data, setData] = useState<Snapshot | null>(null),
    [error, setError] = useState("");
  useTheme(data?.settings.theme);
  const id = Number(new URLSearchParams(location.search).get("notice") || 0);
  const reload = () =>
    void api<Snapshot>("GetSnapshot")
      .then(setData)
      .catch((e) => setError(String(e)));
  useEffect(() => {
    reload();
    void api("Ready", id ? "notifications:" + id : "notifications").catch((e) =>
      setError(String(e)),
    );
    const off = on("board:changed", reload);
    const timer = setInterval(reload, 15000);
    return () => {
      off();
      clearInterval(timer);
    };
  }, []);
  const call = (method: string, ...args: unknown[]) =>
    void api(method, ...args)
      .then(reload)
      .catch((e) => setError(String(e)));
  if (!data) return <p>読み込み中…</p>;
  const pending = data.notifications.filter((n) => !n.acknowledged),
    n = id ? pending.find((n) => n.id === id) : pending[0];
  if (!n) return <div className="notice-window" />;
  const task = data.tasks.find((t) => t.id === n.task_id),
    summary = n.kind === "summary",
    urgent = data.tasks
      .filter((t) => urgency(t, data.settings))
      .sort((a, b) => a.deadline.localeCompare(b.deadline));
  const open = () => call("OpenFromNotice", n.id, n.task_id);
  return (
    <main
      className={
        "notice-window " + (summary ? "summary-notice" : "task-notice")
      }
    >
      <header>
        <strong>{summary ? "期限の確認" : "Kotorima"}</strong>
        {pending.length > 1 && (
          <button
            data-tip="未確認の通知をすべて表示"
            onClick={() => call("OpenNotifications")}
          >
            未確認 {pending.length}件
          </button>
        )}
        <button
          aria-label="通知を閉じる"
          data-tip="通知を閉じる。タスクは未完了のまま残ります。"
          onClick={() => call("Acknowledge", n.id)}
        >
          ×
        </button>
      </header>
      {error && (
        <p className="error" role="alert">
          {error}
        </p>
      )}
      {summary ? (
        <>
          <div className="summary-list">
            {!data.settings.private ? (
              urgent.map((t) => (
                <div className="summary-item" key={t.id}>
                  <button onClick={() => call("OpenFromNotice", n.id, t.id)}>
                    {t.title}
                    <small>
                      {t.deadline} · {urgencyLabel[urgency(t, data.settings)]}
                    </small>
                  </button>
                  <button
                    className="summary-complete"
                    onClick={() => call("SetState", t.id, "done")}
                  >
                    完了
                  </button>
                  {t.show_memo_in_notice && t.memo && (
                    <MemoPreview
                      value={t.memo}
                      onError={(e) => setError(String(e))}
                    />
                  )}
                </div>
              ))
            ) : (
              <p>期限が近いタスク {urgent.length}件</p>
            )}
          </div>
          <footer>
            <button
              className="primary"
              onClick={() => call("OpenFromNotice", n.id, 0)}
            >
              ボードを開く
            </button>
            <button onClick={() => call("Acknowledge", n.id)}>閉じる</button>
          </footer>
        </>
      ) : (
        <>
          <div className="notice-body">
            <button
              className="notice-content"
              data-tip="タスクを開いて、この通知を閉じる"
              onClick={open}
            >
              <h2>
                {data.settings.private
                  ? "タスクの通知"
                  : (task?.title ?? n.title)}
              </h2>
              {!data.settings.private && task?.deadline && (
                <small>期限 {task.deadline}</small>
              )}
            </button>
            {!data.settings.private &&
              task?.show_memo_in_notice &&
              task.memo && (
                <MemoPreview
                  value={task.memo}
                  onError={(e) => setError(String(e))}
                />
              )}
          </div>
          <footer>
            {n.task_id > 0 && (
              <>
                <button
                  className="primary"
                  onClick={() => call("SetState", n.task_id, "done")}
                >
                  完了
                </button>
                <button
                  data-tip="10分後に再通知"
                  onClick={() => call("Snooze", n.id, 10)}
                >
                  10分後
                </button>
                <select
                  aria-label="あとで通知"
                  value=""
                  data-tip="選んだ時間後に再通知"
                  onChange={(e) => {
                    if (e.target.value) call("Snooze", n.id, +e.target.value);
                  }}
                >
                  <option value="" disabled>
                    あとで…
                  </option>
                  {[
                    [5, "5分後"],
                    [15, "15分後"],
                    [30, "30分後"],
                    [60, "1時間後"],
                    [360, "6時間後"],
                    [1440, "1日後"],
                    [2880, "2日後"],
                    [10080, "1週間後"],
                  ].map(([value, label]) => (
                    <option key={value} value={value}>
                      {label}
                    </option>
                  ))}
                </select>
              </>
            )}
          </footer>
        </>
      )}
    </main>
  );
}
