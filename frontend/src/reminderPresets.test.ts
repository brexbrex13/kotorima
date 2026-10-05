import { it, expect } from "vitest";
import { presetTime } from "./reminderPresets";
import { shortcutWarning } from "./ShortcutField";
it("uses the configured wall-clock time across week/month/year boundaries", () => {
  const monday = new Date(2026, 11, 28, 13, 15);
  expect(presetTime("next-week", "08:30", monday)).toBe("2027-01-04T08:30:00");
  expect(presetTime("next-month", "08:30", monday)).toBe("2027-01-01T08:30:00");
  expect(presetTime("tomorrow", "08:30", monday)).toBe("2026-12-29T08:30:00");
  expect(
    new Date(presetTime("4h", "08:30", monday)).getTime() - monday.getTime(),
  ).toBe(4 * 3600000);
});
it("warns about conventional shortcuts without calling them actual registration conflicts", () => {
  expect(shortcutWarning("Ctrl+V")).toContain("標準操作");
  expect(shortcutWarning("Alt+Tab")).toContain("Windows");
  expect(shortcutWarning("Alt+Space")).toContain("Windows");
  expect(shortcutWarning("Ctrl+Alt+Shift+Space")).toBe("");
  expect(shortcutWarning("Ctrl+Alt+N")).toContain("Office");
  expect(shortcutWarning("Ctrl+1")).toContain("Office");
  expect(shortcutWarning("Ctrl+Alt+Shift+F11")).toBe("");
});
