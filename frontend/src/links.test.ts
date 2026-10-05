import { expect, it } from "vitest";
import { memoLink } from "./links";
it("accepts quoted Windows Copy as path values without storing quotation marks", () => {
  expect(memoLink('"C:\\Work Files\\report.pdf"')).toBe(
    "file:///C:/Work%20Files/report.pdf",
  );
  expect(memoLink(' "\\\\server\\share\\report.pdf" ')).toBe(
    "file://server/share/report.pdf",
  );
  expect(memoLink('"https://example.com/report"')).toBe(
    "https://example.com/report",
  );
  expect(() => memoLink('"not a path"')).toThrow();
});
