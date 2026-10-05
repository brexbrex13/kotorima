import { describe, expect, it } from "vitest";
import {
  applySuggestion,
  blankOptions,
  chipLabels,
  deadlineDate,
  dropChip,
  Field,
  Suggestion,
} from "./smartAdd";
const none: Suggestion = {
  category_id: 0,
  important: false,
  deadline: "",
  reminder: "",
  duplicate: null,
  invalid: false,
};
// 2026-10-05 is a Monday.
const at = (d: number, h = 10) => new Date(2026, 9, d, h, 0, 0);
describe("deadlineDate", () => {
  it.each([
    ["today", 5, "2026-10-05"],
    ["tomorrow", 5, "2026-10-06"],
    ["this_week", 5, "2026-10-09"],
    ["this_week", 9, "2026-10-09"],
    ["this_week", 10, "2026-10-11"],
    ["this_week", 11, "2026-10-11"],
    ["next_week", 5, "2026-10-16"],
    ["next_week", 11, "2026-10-16"],
    ["none", 5, ""],
    ["toString", 5, ""],
  ])("%s on day %i", (kind, day, want) => {
    expect(deadlineDate(kind as string, at(day as number))).toBe(want);
  });
});
describe("applySuggestion", () => {
  const full: Suggestion = {
    ...none,
    category_id: 2,
    important: true,
    deadline: "tomorrow",
    reminder: "1h",
  };
  it("fills untouched fields and records them as automatic", () => {
    const r = applySuggestion(
      blankOptions(),
      new Set(),
      {},
      full,
      "09:00",
      at(5),
    );
    expect(r.options).toEqual({
      category_id: 2,
      important: true,
      deadline: "2026-10-06",
      reminder_at: "2026-10-05T11:00:00",
      reminder_mode: "",
      reminder_time: "",
    });
    expect(r.auto).toEqual({
      category: "2",
      important: "1",
      deadline: "tomorrow",
      reminder: "1h",
    });
  });
  it("never overwrites touched fields", () => {
    const touched = new Set<Field>(["important", "deadline"]);
    const start = { ...blankOptions(), deadline: "2026-12-24" };
    const r = applySuggestion(start, touched, {}, full, "09:00", at(5));
    expect(r.options.important).toBe(false);
    expect(r.options.deadline).toBe("2026-12-24");
    expect(r.auto.important).toBeUndefined();
    expect(r.auto.deadline).toBeUndefined();
    expect(r.options.category_id).toBe(2);
  });
  it("clears automatic values the new suggestion no longer has, but keeps manual ones", () => {
    const first = applySuggestion(
      blankOptions(),
      new Set(),
      {},
      full,
      "09:00",
      at(5),
    );
    const manual = { ...first.options, important: true };
    const r = applySuggestion(
      manual,
      new Set<Field>(["important"]),
      first.auto,
      none,
      "09:00",
      at(5),
    );
    expect(r.options).toEqual({ ...blankOptions(), important: true });
    expect(r.auto).toEqual({});
  });
  it("clears everything automatic when the title is emptied", () => {
    const first = applySuggestion(
      blankOptions(),
      new Set(),
      {},
      full,
      "09:00",
      at(5),
    );
    expect(
      applySuggestion(first.options, new Set(), first.auto, null).options,
    ).toEqual(blankOptions());
  });
  it("uses the deadline reminder only when a deadline exists", () => {
    const s = { ...none, deadline: "today", reminder: "deadline" };
    const r = applySuggestion(blankOptions(), new Set(), {}, s, "08:30", at(5));
    expect(r.options).toMatchObject({
      deadline: "2026-10-05",
      reminder_mode: "deadline",
      reminder_time: "08:30",
      reminder_at: "",
    });
    const orphan = applySuggestion(
      blankOptions(),
      new Set(),
      {},
      { ...none, reminder: "deadline" },
      "08:30",
      at(5),
    );
    expect(orphan.options.reminder_mode).toBe("");
    expect(orphan.auto.reminder).toBeUndefined();
  });
  it("drops a stale deadline reminder when the deadline goes away", () => {
    const start = {
      ...blankOptions(),
      reminder_mode: "deadline",
      reminder_time: "09:00",
    };
    const r = applySuggestion(
      start,
      new Set<Field>(["reminder"]),
      { deadline: "today" },
      none,
    );
    expect(r.options.reminder_mode).toBe("");
  });
});
describe("dropChip", () => {
  it("removes a deadline together with its deadline reminder", () => {
    const s = { ...none, deadline: "today", reminder: "deadline" };
    const first = applySuggestion(
      blankOptions(),
      new Set(),
      {},
      s,
      "09:00",
      at(5),
    );
    const r = dropChip(first.options, first.auto, "deadline");
    expect(r.options).toEqual(blankOptions());
    expect(r.auto).toEqual({});
  });
  it("removes only the chosen field", () => {
    const r = dropChip(
      { ...blankOptions(), category_id: 3, important: true },
      { category: "3", important: "1" },
      "category",
    );
    expect(r.options).toEqual({ ...blankOptions(), important: true });
    expect(r.auto).toEqual({ important: "1" });
  });
});
describe("chipLabels", () => {
  it("labels automatic fields in a fixed order", () => {
    const options = {
      ...blankOptions(),
      category_id: 2,
      important: true,
      deadline: "2026-10-06",
      reminder_at: "2026-10-06T09:00:00",
    };
    const labels = chipLabels(
      {
        reminder: "tomorrow",
        deadline: "tomorrow",
        important: "1",
        category: "2",
      },
      options,
      [{ id: 2, name: "経理" }],
    );
    expect(labels.map((l) => l.label)).toEqual([
      "経理",
      "重要",
      "期限 明日",
      "通知 10/6 09:00",
    ]);
    expect(
      chipLabels({ category: "9" }, { ...options, category_id: 9 }, []),
    ).toEqual([]);
    expect(chipLabels({ reminder: "deadline" }, options, [])[0].label).toBe(
      "期限日に通知",
    );
  });
});
