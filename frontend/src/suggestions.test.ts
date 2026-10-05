import { describe, it, expect } from "vitest";
import { suggestions } from "./Suggestions";
import { emptyTask } from "./types";
describe("input suggestions", () => {
  it("shows frequent titles on empty focus and matches kana across scripts", () => {
    const tasks = ["カタカナ業務", "かたかな業務", "日報作成", "日報作成"].map(
      (title) => ({ ...emptyTask(), title }),
    );
    expect(suggestions(tasks, "かた", 2)).toEqual(["カタカナ業務"]);
    expect(suggestions(tasks, "ｶﾀ", 2)).toEqual(["カタカナ業務"]);
    expect(suggestions(tasks, "", 2)).toContain("日報作成");
  });
  it("counts manual registrations, excludes recurring and trash, and respects threshold", () => {
    const task = {
      ...emptyTask(),
      title: "請求書確認",
      created_at: "2026-10-01T09:00",
    };
    const tasks = [
      task,
      { ...task, id: 2, status: "done" },
      { ...task, id: 3 },
      { ...task, title: "請求書送付", series_id: 1 },
      { ...task, title: "請求書作成", deleted_at: "2026-10-01" },
    ];
    expect(suggestions(tasks, "請", 3)).toEqual(["請求書確認"]);
    expect(suggestions(tasks, "請", 4)).toEqual([]);
    expect(suggestions(tasks, "請", 0)).toEqual([]);
    expect(suggestions(tasks, "請求書確認", 3)).toEqual([]);
  });
  it("prefers selected category and normalizes width and case", () => {
    const task = { ...emptyTask(), created_at: "2026-10-01T09:00" };
    const tasks = [
      { ...task, title: "ＡＢＣ作業", category_id: 2 },
      { ...task, title: "abc作業", category_id: 2 },
      { ...task, title: "ABC別件", category_id: 3 },
      { ...task, title: "ABC別件", category_id: 3 },
      { ...task, title: "ABC別件", category_id: 3 },
    ];
    expect(suggestions(tasks, "ab", 2, 2)).toEqual(["ＡＢＣ作業", "ABC別件"]);
  });
});
