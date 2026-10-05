import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from "@testing-library/react";
import App, { Notifications } from "./App";
import { TaskDetail, DraftHandle } from "./TaskDetail";
import { emptyTask, Snapshot, urgency } from "./types";
import { createRef } from "react";
import { api } from "./api";
vi.mock("./api", () => ({ api: vi.fn(), on: vi.fn(() => () => {}) }));
vi.mock("./Editor", () => ({
  Editor: ({
    value,
    onChange,
  }: {
    value: string;
    onChange: (v: string) => void;
  }) => (
    <textarea
      aria-label="業務メモ"
      value={value}
      onChange={(e) => onChange(e.target.value)}
    />
  ),
}));
const defaults = {
  series_show_days: 7,
  custom_colors: [],
  near_days: 3,
  workdays: true,
  notify_times: [],
  notify_weekdays: [1, 2, 3, 4, 5],
  sound: true,
  native_toast: false,
  private: false,
  pause_until: "",
  font_size: 14,
  compact: false,
  collapsed: [],
  monitor: "active",
};
let snapshot: Snapshot;
beforeEach(() => {
  localStorage.clear();
  snapshot = {
    tasks: [],
    categories: [
      {
        id: 1,
        name: "未分類",
        color: "#fffdf8",
        text_color: "#302d25",
        sort_order: 0,
      },
    ],
    series: [],
    notifications: [],
    settings: defaults,
  };
  vi.mocked(api).mockReset();
  vi.mocked(api).mockImplementation(async (method, ...args) => {
    if (method === "GetSnapshot") return snapshot;
    if (method === "SaveTask" || method === "SaveSuggestedTask") {
      const task = {
        ...(args[0] as object),
        id: (args[0] as { id: number }).id || 1,
        version: 2,
      };
      return task;
    }
    return undefined;
  });
});
afterEach(cleanup);
describe("sticky board and notification semantics", () => {
  it("adds a note with Enter but does not commit an IME composition", async () => {
    render(<App />);
    const input = await screen.findByLabelText("新しい付箋");
    fireEvent.change(input, { target: { value: "ぱぱっと要件" } });
    fireEvent.keyDown(input, { key: "Enter", keyCode: 229, isComposing: true });
    expect(
      vi.mocked(api).mock.calls.filter((c) => c[0] === "SaveSuggestedTask"),
    ).toHaveLength(0);
    fireEvent.keyDown(input, { key: "Enter", keyCode: 13, isComposing: false });
    await waitFor(() =>
      expect(
        vi
          .mocked(api)
          .mock.calls.find((c) => c[0] === "SaveSuggestedTask")?.[1],
      ).toMatchObject({ title: "ぱぱっと要件", deadline: "", reminder_at: "" }),
    );
  });
  it("keeps deadline band separate from manual board order", async () => {
    snapshot.tasks = [
      { ...emptyTask(1), id: 1, title: "期限なしの付箋", sort_order: 0 },
      {
        ...emptyTask(1),
        id: 2,
        title: "期限切れ",
        deadline: "2000-01-01",
        sort_order: 1,
      },
    ];
    render(<App />);
    await screen.findByText("期限なしの付箋");
    const cards = screen.getAllByRole("article");
    expect(cards[0].textContent).toContain("期限なしの付箋");
    expect(cards[1].textContent).toContain("期限切れ");
    expect(screen.getByLabelText("期限の確認").textContent).toContain(
      "期限切れ",
    );
  });
  it("acknowledges notification without completing task", async () => {
    snapshot.tasks = [{ ...emptyTask(1), id: 1, title: "確認と完了は別" }];
    snapshot.notifications = [
      {
        id: 7,
        key: "x",
        task_id: 1,
        kind: "reminder",
        title: "確認と完了は別",
        fired_at: "2026-10-01T09:00",
        acknowledged: false,
      },
    ];
    render(<Notifications />);
    fireEvent.click(
      await screen.findByRole("button", { name: "通知を閉じる" }),
    );
    await waitFor(() => expect(api).toHaveBeenCalledWith("Acknowledge", 7));
    expect(api).not.toHaveBeenCalledWith("SetState", 1, "done");
  });
  it("saves only explicitly and retains the draft after failure", async () => {
    const task = { ...emptyTask(1), id: 3, version: 1, title: "要件" };
    const close = vi.fn(),
      error = vi.fn();
    const handle = createRef<DraftHandle>();
    render(
      <TaskDetail
        task={task}
        categories={[]}
        onClose={close}
        onSaved={() => {}}
        onError={error}
        handle={handle}
      />,
    );
    fireEvent.change(screen.getByLabelText("業務メモ"), {
      target: { value: "保存するメモ" },
    });
    vi.mocked(api).mockRejectedValueOnce(new Error("ディスクエラー"));
    fireEvent.click(
      screen.getByRole("button", { name: "今すぐ保存" }),
    );
    await waitFor(() => expect(error).toHaveBeenCalled());
    expect(close).not.toHaveBeenCalled();
    expect(localStorage.getItem("draft:3")).toContain("保存するメモ");
    fireEvent.click(
      screen.getByRole("button", { name: "今すぐ保存" }),
    );
    await waitFor(() => expect(close).toHaveBeenCalled());
    expect(localStorage.getItem("draft:3")).toBeNull();
  });
  it("uses end of day and workday-based near deadlines", () => {
    const task = { ...emptyTask(1), deadline: "2026-10-05" };
    expect(
      urgency(
        task,
        { ...defaults, near_days: 1 },
        new Date("2026-10-02T12:00"),
      ),
    ).toBe("near");
    expect(
      urgency(
        task,
        { ...defaults, near_days: 1, workdays: false },
        new Date("2026-10-02T12:00"),
      ),
    ).toBe("");
    task.deadline = "2026-10-01";
    expect(urgency(task, defaults, new Date("2026-10-01T23:59"))).toBe("today");
    expect(urgency(task, defaults, new Date("2026-10-02T00:00"))).toBe(
      "overdue",
    );
  });
});

describe("draft conflict resolution", () => {
  it("preserves snooze and completion from notification window while saving memo", async () => {
    const { mergeDraft } = await import("./drafts");
    const base = { ...emptyTask(1), id: 1, version: 1, title: "x" };
    const draft = { ...base, memo: "updated memo" };
    const latest = {
      ...base,
      version: 2,
      reminder_at: "2026-10-01T13:00",
      status: "done",
      done_at: "2026-10-01T10:00",
    };
    expect(mergeDraft(base, draft, latest)).toMatchObject({
      version: 2,
      memo: "updated memo",
      reminder_at: latest.reminder_at,
      status: "done",
    });
  });
  it("rejects concurrent edits of same field", async () => {
    const { mergeDraft } = await import("./drafts");
    const base = { ...emptyTask(1), title: "x" };
    expect(() =>
      mergeDraft(base, { ...base, memo: "a" }, { ...base, memo: "b" }),
    ).toThrow("同じ項目");
  });
});
describe("smart add on the main board", () => {
  const smartBoard = (answer: object) => {
    snapshot.settings = {
      ...defaults,
      smart_add: true,
    } as Snapshot["settings"];
    snapshot.categories.push({
      id: 2,
      name: "経理",
      color: "#fff",
      text_color: "#000",
      sort_order: 1,
    });
    const original = vi.mocked(api).getMockImplementation()!;
    vi.mocked(api).mockImplementation(async (method, ...args) => {
      if (method === "GetJevStatus")
        return { supported: true, configured: true, hint: "", invalid: false };
      if (method === "SuggestTask")
        return {
          category_id: 0,
          important: false,
          deadline: "",
          reminder: "",
          duplicate: null,
          invalid: false,
          ...answer,
        };
      return original(method, ...args);
    });
  };
  const calls = (m: string) =>
    vi.mocked(api).mock.calls.filter((c) => c[0] === m);
  it("adds into the suggested category while all categories are shown", async () => {
    smartBoard({ category_id: 2 });
    render(<App />);
    const input = await screen.findByLabelText("新しい付箋");
    fireEvent.change(input, { target: { value: "請求書を経理に" } });
    const chips = await screen.findByLabelText(
      "推定した初期値",
      {},
      { timeout: 1500 },
    );
    expect(within(chips).getByText("経理")).toBeTruthy();
    expect(calls("SuggestTask")[0].slice(1)).toEqual(["請求書を経理に", true]);
    fireEvent.keyDown(input, { key: "Enter", keyCode: 13 });
    await waitFor(() => expect(calls("SaveSuggestedTask")).toHaveLength(1));
    expect(calls("SaveSuggestedTask")[0][1]).toMatchObject({ category_id: 2 });
  });
  it("keeps the selected category tab and does not ask for a category", async () => {
    smartBoard({ category_id: 2, deadline: "today" });
    render(<App />);
    const input = await screen.findByLabelText("新しい付箋");
    fireEvent.click(
      within(document.querySelector(".filters") as HTMLElement).getByText(
        "未分類",
      ),
    );
    fireEvent.change(input, { target: { value: "請求書を経理に" } });
    const chips = await screen.findByLabelText(
      "推定した初期値",
      {},
      { timeout: 1500 },
    );
    expect(within(chips).queryByText("経理")).toBeNull();
    expect(calls("SuggestTask")[0].slice(1)).toEqual(["請求書を経理に", false]);
    fireEvent.keyDown(input, { key: "Enter", keyCode: 13 });
    await waitFor(() => expect(calls("SaveSuggestedTask")).toHaveLength(1));
    expect(calls("SaveSuggestedTask")[0][1]).toMatchObject({ category_id: 1 });
  });
  it("hides the important chip while filtering by important", async () => {
    smartBoard({ important: true, deadline: "today" });
    render(<App />);
    const input = await screen.findByLabelText("新しい付箋");
    fireEvent.click(
      within(document.querySelector(".task-filters") as HTMLElement).getByText(
        /重要/,
      ),
    );
    fireEvent.change(input, { target: { value: "大事な用事" } });
    const chips = await screen.findByLabelText(
      "推定した初期値",
      {},
      { timeout: 1500 },
    );
    expect(within(chips).queryByText("重要")).toBeNull();
    fireEvent.keyDown(input, { key: "Enter", keyCode: 13 });
    await waitFor(() => expect(calls("SaveSuggestedTask")).toHaveLength(1));
    expect(calls("SaveSuggestedTask")[0][1]).toMatchObject({ important: true });
  });
});

describe("notification memo visibility", () => {
  it.each(["reminder", "summary"])(
    "shows opted-in memos for %s without acknowledging links, and hides them in private mode",
    async (kind) => {
      snapshot.tasks = [
        {
          ...emptyTask(1),
          id: 1,
          title: "メモ通知",
          deadline: "2000-01-01",
          memo: '<p>手順</p><a href="file:///C:/Work">作業場所</a>',
          show_memo_in_notice: true,
        },
      ];
      snapshot.notifications = [
        {
          id: 1,
          key: "memo",
          task_id: kind === "summary" ? 0 : 1,
          kind,
          title: "通知",
          fired_at: "2000-01-01T09:00",
          acknowledged: false,
        },
      ];
      const view = render(<Notifications />);
      const memo = await screen.findByLabelText("通知のメモ");
      fireEvent.click(within(memo).getByRole("link", { name: "作業場所" }));
      await waitFor(() =>
        expect(api).toHaveBeenCalledWith("OpenURL", "file:///C:/Work"),
      );
      expect(api).not.toHaveBeenCalledWith("Acknowledge", 1);
      expect(api).not.toHaveBeenCalledWith("OpenFromNotice", 1, 1);
      view.unmount();
      snapshot.settings = { ...defaults, private: true };
      render(<Notifications />);
      await screen.findByLabelText("通知を閉じる");
      expect(screen.queryByLabelText("通知のメモ")).toBeNull();
    },
  );
  it("defaults off and saves the per-task option with the memo", async () => {
    const task = { ...emptyTask(1), id: 11, version: 1, title: "設定の保存" };
    render(
      <TaskDetail
        task={task}
        categories={[]}
        onClose={vi.fn()}
        onSaved={vi.fn()}
        onError={vi.fn()}
        handle={createRef<DraftHandle>()}
      />,
    );
    expect(screen.getByLabelText("通知にメモを表示する")).toHaveProperty(
      "checked",
      false,
    );
    fireEvent.click(screen.getByLabelText("通知にメモを表示する"));
    fireEvent.change(screen.getByLabelText("業務メモ"), {
      target: { value: "作業場所" },
    });
    fireEvent.click(
      screen.getByRole("button", { name: "今すぐ保存" }),
    );
    await waitFor(() =>
      expect(api).toHaveBeenCalledWith(
        "SaveTask",
        expect.objectContaining({
          memo: "作業場所",
          show_memo_in_notice: true,
        }),
      ),
    );
  });
});

it("keeps edits local until save and offers discard on navigation", async () => {
  const task = { ...emptyTask(1), id: 31, version: 1, title: "正式保存の検証" };
  const handle = createRef<DraftHandle>();
  const close = vi.fn();
  render(
    <TaskDetail
      task={task}
      categories={[]}
      onClose={close}
      onSaved={vi.fn()}
      onError={vi.fn()}
      handle={handle}
    />,
  );
  fireEvent.change(screen.getByLabelText("期限日", { exact: true }), {
    target: { value: "2000-01-01" },
  });
  fireEvent.change(screen.getByLabelText("業務メモ"), {
    target: { value: "未確定" },
  });
  await new Promise((r) => setTimeout(r, 800));
  expect(api).not.toHaveBeenCalledWith("SaveTask", expect.anything());
  expect(localStorage.getItem("draft:31")).toContain("未確定");
  fireEvent.click(screen.getByLabelText("詳細を閉じる"));
  expect(screen.getByRole("dialog", { name: "未保存の変更" })).toBeTruthy();
  fireEvent.click(screen.getByRole("button", { name: "編集に戻る" }));
  expect(close).not.toHaveBeenCalled();
  fireEvent.click(screen.getByRole("button", { name: "変更を破棄" }));
  expect(close).toHaveBeenCalled();
  expect(localStorage.getItem("draft:31")).toBeNull();
  expect(api).not.toHaveBeenCalledWith("SaveTask", expect.anything());
});
