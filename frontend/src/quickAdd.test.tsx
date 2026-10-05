import { afterEach, beforeEach, expect, it, vi } from "vitest";
import {
  act,
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from "@testing-library/react";
import { QuickAdd } from "./QuickAdd";
import { api, on } from "./api";
vi.mock("./api", () => ({ api: vi.fn(), on: vi.fn() }));
const listeners = new Map<string, (value: unknown) => void>();
beforeEach(() => {
  localStorage.clear();
  listeners.clear();
  vi.mocked(api).mockReset();
  vi.mocked(on).mockImplementation((name, callback) => {
    listeners.set(name, callback);
    return () => {
      listeners.delete(name);
    };
  });
  vi.mocked(api).mockImplementation(async (method) =>
    method === "GetSnapshot"
      ? { tasks: [], settings: { suggest_min_count: 3 } }
      : method === "SetQuickSuggestions"
        ? false
        : undefined,
  );
});
afterEach(cleanup);
it("keeps a focus-loss draft but cancels with Escape and never restores legacy persisted input", async () => {
  localStorage.setItem("tray-quick-draft", "古い入力");
  localStorage.setItem(
    "tray-quick-options",
    JSON.stringify({ deadline: "2000-01-01" }),
  );
  render(<QuickAdd />);
  const input = screen.getByLabelText("トレイからタスク追加");
  await act(async () => {});
  expect((input as HTMLInputElement).value).toBe("");
  expect(localStorage.getItem("tray-quick-draft")).toBeNull();
  fireEvent.change(input, { target: { value: "途中の入力" } });
  fireEvent.blur(input);
  expect((input as HTMLInputElement).value).toBe("途中の入力");
  fireEvent.keyDown(input, { key: "Escape" });
  expect((input as HTMLInputElement).value).toBe("");
  await waitFor(() =>
    expect(vi.mocked(api).mock.calls.some((c) => c[0] === "HideQuickAdd")).toBe(
      true,
    ),
  );
});
it("ignores an old cancel event and old save completion after a newer input session", async () => {
  let finishSave!: (value: unknown) => void;
  const original = vi.mocked(api).getMockImplementation()!;
  vi.mocked(api).mockImplementation((method, ...args) =>
    method === "SaveSuggestedTask"
      ? new Promise((resolve) => {
          finishSave = resolve;
        })
      : original(method, ...args),
  );
  render(<QuickAdd />);
  const input = screen.getByLabelText("トレイからタスク追加");
  await act(async () => {});
  act(() => listeners.get("board:mini-focus")?.(1));
  fireEvent.change(input, { target: { value: "登録中" } });
  fireEvent.keyDown(input, { key: "Enter", keyCode: 13 });
  await waitFor(() => expect(finishSave).toBeDefined());
  act(() => listeners.get("board:mini-reset")?.(2));
  act(() => listeners.get("board:mini-focus")?.(3));
  fireEvent.change(input, { target: { value: "次の入力" } });
  act(() => listeners.get("board:mini-reset")?.(2));
  expect((input as HTMLInputElement).value).toBe("次の入力");
  act(() =>
    listeners.get("board:mini-choice")?.({ title: "古い候補", generation: 1 }),
  );
  expect((input as HTMLInputElement).value).toBe("次の入力");
  await act(async () => {
    finishSave({ id: 1 });
  });
  expect((input as HTMLInputElement).value).toBe("次の入力");
});
const sleep = (ms: number) => new Promise((r) => setTimeout(r, ms));
function smartMock(
  suggest: (title: string) => unknown | Promise<unknown>,
  extra: Partial<Record<string, unknown>> = {},
) {
  vi.mocked(api).mockImplementation(async (method, ...args) => {
    if (method === "GetSnapshot")
      return {
        tasks: [],
        categories: [
          { id: 1, name: "未分類", color: "", sort_order: 0 },
          { id: 2, name: "経理", color: "", sort_order: 1 },
        ],
        settings: {
          suggest_min_count: 3,
          smart_add: true,
          reminder_default_time: "09:00",
        },
        ...extra,
      };
    if (method === "GetJevStatus")
      return { supported: true, configured: true, hint: "", invalid: false };
    if (method === "SetQuickSuggestions") return false;
    if (method === "SuggestQuickAdd") return suggest(args[0] as string);
    return undefined;
  });
}
const result = (p: object) => ({
  category_id: 0,
  important: false,
  deadline: "",
  reminder: "",
  duplicate: null,
  invalid: false,
  ...p,
});
const calls = (m: string) =>
  vi.mocked(api).mock.calls.filter((c) => c[0] === m);
it("asks Jev 0.4s after typing stops and shows chips and a duplicate warning", async () => {
  smartMock(() =>
    result({
      category_id: 2,
      important: true,
      duplicate: { task_id: 7, title: "請求書提出" },
    }),
  );
  render(<QuickAdd />);
  const input = screen.getByLabelText("トレイからタスク追加");
  await act(async () => {});
  fireEvent.change(input, { target: { value: "請求書を経理に" } });
  expect(calls("SuggestQuickAdd")).toHaveLength(0);
  await waitFor(() => screen.getByLabelText("推定した初期値"), {
    timeout: 1500,
  });
  expect(calls("SuggestQuickAdd")[0].slice(1)).toEqual(["請求書を経理に", 0]);
  expect(screen.getByText("経理")).toBeTruthy();
  expect(screen.getByText("重要")).toBeTruthy();
  fireEvent.click(screen.getByText(/似たタスクがあります: 請求書提出/));
  await waitFor(() => expect(calls("OpenTask")[0][1]).toBe(7));
  await waitFor(() =>
    expect(calls("SetQuickAddLayout").slice(-1)[0].slice(1)).toEqual([
      false,
      false,
      2,
      0,
    ]),
  );
  fireEvent.keyDown(input, { key: "Enter", keyCode: 13 });
  await waitFor(() => expect(calls("SaveSuggestedTask")).toHaveLength(1));
  expect(calls("SaveSuggestedTask")[0][1]).toMatchObject({
    title: "請求書を経理に",
    category_id: 2,
    important: true,
  });
});
it("does not ask while composing and asks after composition ends", async () => {
  smartMock(() => result({}));
  render(<QuickAdd />);
  const input = screen.getByLabelText("トレイからタスク追加");
  await act(async () => {});
  fireEvent.compositionStart(input);
  fireEvent.change(input, { target: { value: "せいきゅう" } });
  await act(() => sleep(600));
  expect(calls("SuggestQuickAdd")).toHaveLength(0);
  fireEvent.compositionEnd(input);
  await waitFor(() => expect(calls("SuggestQuickAdd")).toHaveLength(1), {
    timeout: 1500,
  });
});
it("ignores stale answers and does not wait for Jev on Enter", async () => {
  const pending: ((v: unknown) => void)[] = [];
  smartMock(() => new Promise((resolve) => pending.push(resolve)));
  render(<QuickAdd />);
  const input = screen.getByLabelText("トレイからタスク追加");
  await act(async () => {});
  fireEvent.change(input, { target: { value: "一つ目" } });
  await waitFor(() => expect(pending).toHaveLength(1), { timeout: 1500 });
  fireEvent.change(input, { target: { value: "二つ目" } });
  await waitFor(() => expect(pending).toHaveLength(2), { timeout: 1500 });
  await act(async () => pending[0](result({ important: true })));
  expect(screen.queryByLabelText("推定した初期値")).toBeNull();
  fireEvent.keyDown(input, { key: "Enter", keyCode: 13 });
  await waitFor(() => expect(calls("SaveSuggestedTask")).toHaveLength(1));
  expect(calls("SaveSuggestedTask")[0][1]).toMatchObject({
    title: "二つ目",
    important: false,
  });
  await act(async () => pending[1](result({ important: true })));
  expect(screen.queryByLabelText("推定した初期値")).toBeNull();
});
it("keeps a field the user changed and lets a chip be removed", async () => {
  smartMock((title) =>
    result({ important: true, category_id: title.includes("経理") ? 2 : 0 }),
  );
  render(<QuickAdd />);
  const input = screen.getByLabelText("トレイからタスク追加");
  await act(async () => {});
  fireEvent.click(screen.getByLabelText("登録時の期限・通知"));
  fireEvent.click(screen.getByLabelText("重要"));
  fireEvent.click(screen.getByLabelText("重要"));
  fireEvent.change(input, { target: { value: "経理に連絡" } });
  await waitFor(() => screen.getByText("経理"), { timeout: 1500 });
  expect(
    within(screen.getByLabelText("推定した初期値")).queryByText(/重要/),
  ).toBeNull();
  expect((screen.getByLabelText("重要") as HTMLInputElement).checked).toBe(
    false,
  );
  fireEvent.click(screen.getByLabelText("経理を外す"));
  expect(screen.queryByLabelText("推定した初期値")).toBeNull();
  fireEvent.change(input, { target: { value: "経理に電話" } });
  await act(() => sleep(700));
  expect(screen.queryByLabelText("推定した初期値")).toBeNull();
});
it("saves into the default category when the suggested one was deleted meanwhile", async () => {
  smartMock(() => result({ category_id: 2 }));
  render(<QuickAdd />);
  const input = screen.getByLabelText("トレイからタスク追加");
  await act(async () => {});
  fireEvent.change(input, { target: { value: "経理に連絡" } });
  await waitFor(() => screen.getByText("経理"), { timeout: 1500 });
  const original = vi.mocked(api).getMockImplementation()!;
  vi.mocked(api).mockImplementation(async (method, ...args) =>
    method === "GetSnapshot"
      ? {
          tasks: [],
          categories: [{ id: 1, name: "未分類", color: "", sort_order: 0 }],
          settings: { smart_add: true },
        }
      : original(method, ...args),
  );
  await act(async () => listeners.get("board:changed")?.(undefined));
  fireEvent.keyDown(input, { key: "Enter", keyCode: 13 });
  await waitFor(() => expect(calls("SaveSuggestedTask")).toHaveLength(1));
  expect(calls("SaveSuggestedTask")[0][1]).toMatchObject({ category_id: 0 });
});
it("shows an invalid-key notice once and stops asking", async () => {
  smartMock(() => result({ invalid: true }));
  render(<QuickAdd />);
  const input = screen.getByLabelText("トレイからタスク追加");
  await act(async () => {});
  fireEvent.change(input, { target: { value: "一つ目" } });
  await waitFor(() => screen.getByText("AIのキーが無効です（設定で確認）"), {
    timeout: 1500,
  });
  fireEvent.change(input, { target: { value: "二つ目" } });
  await act(() => sleep(700));
  expect(calls("SuggestQuickAdd")).toHaveLength(1);
});
it("never asks Jev when smart add is off or no key is set", async () => {
  smartMock(() => result({ important: true }), {
    settings: { smart_add: false },
  });
  render(<QuickAdd />);
  const input = screen.getByLabelText("トレイからタスク追加");
  await act(async () => {});
  fireEvent.change(input, { target: { value: "何かの用事" } });
  await act(() => sleep(700));
  expect(calls("SuggestQuickAdd")).toHaveLength(0);
});
it("drops an answer for a title that was cleared or replaced before it arrived", async () => {
  const pending: ((v: unknown) => void)[] = [];
  smartMock(() => new Promise((resolve) => pending.push(resolve)));
  render(<QuickAdd />);
  const input = screen.getByLabelText("トレイからタスク追加");
  await act(async () => {});
  fireEvent.change(input, { target: { value: "請求書提出" } });
  await waitFor(() => expect(pending).toHaveLength(1), { timeout: 1500 });
  fireEvent.change(input, { target: { value: "" } });
  await act(async () =>
    pending[0](
      result({
        important: true,
        duplicate: { task_id: 7, title: "請求書提出" },
      }),
    ),
  );
  expect(screen.queryByLabelText("推定した初期値")).toBeNull();
  expect(screen.queryByText(/似たタスクがあります/)).toBeNull();
  fireEvent.change(input, { target: { value: "別の用事" } });
  await waitFor(() => expect(pending).toHaveLength(2), { timeout: 1500 });
  fireEvent.change(input, { target: { value: "別の用事2" } });
  await act(async () => pending[1](result({ important: true })));
  fireEvent.keyDown(input, { key: "Enter", keyCode: 13 });
  await waitFor(() => expect(calls("SaveSuggestedTask")).toHaveLength(1));
  expect(calls("SaveSuggestedTask")[0][1]).toMatchObject({
    title: "別の用事2",
    important: false,
  });
});
it("keeps a deadline reminder time the user changed", async () => {
  smartMock(() => result({ deadline: "today", reminder: "deadline" }));
  render(<QuickAdd />);
  const input = screen.getByLabelText("トレイからタスク追加");
  await act(async () => {});
  fireEvent.change(input, { target: { value: "請求書提出です" } });
  await waitFor(() => screen.getByText("期限日に通知"), { timeout: 1500 });
  fireEvent.click(screen.getByLabelText("登録時の期限・通知"));
  fireEvent.change(screen.getByLabelText("期限日の通知時刻"), {
    target: { value: "18:00" },
  });
  fireEvent.change(input, { target: { value: "請求書提出ですね" } });
  await waitFor(() => expect(calls("SuggestQuickAdd")).toHaveLength(2), {
    timeout: 1500,
  });
  await act(async () => {});
  fireEvent.keyDown(input, { key: "Enter", keyCode: 13 });
  await waitFor(() => expect(calls("SaveSuggestedTask")).toHaveLength(1));
  expect(calls("SaveSuggestedTask")[0][1]).toMatchObject({
    reminder_mode: "deadline",
    reminder_time: "18:00",
  });
});
