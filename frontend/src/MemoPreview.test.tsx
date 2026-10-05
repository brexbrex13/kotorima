import { afterEach, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { MemoPreview } from "./MemoPreview";
import { api } from "./api";

vi.mock("./api", () => ({ api: vi.fn(() => Promise.resolve()) }));
afterEach(() => {
  cleanup();
  vi.clearAllMocks();
});

it("opens file links through the app and images separately without changing the memo", () => {
  const error = vi.fn();
  render(
    <MemoPreview
      value={
        '<p><strong>作業</strong></p><a href="file:///C:/Work%20Files">作業場所</a><img src="/images/test.png"><input type="checkbox" checked>'
      }
      onError={error}
    />,
  );
  expect(screen.getByText("作業").tagName).toBe("STRONG");
  fireEvent.click(screen.getByRole("link", { name: "作業場所" }));
  expect(api).toHaveBeenCalledWith("OpenURL", "file:///C:/Work%20Files");
  expect(screen.getByRole("checkbox")).toHaveProperty("disabled", true);
  fireEvent.keyDown(screen.getByRole("button", { name: "添付画像を拡大" }), {
    key: "Enter",
  });
  expect(api).toHaveBeenCalledWith("OpenImageViewer", "/images/test.png");
  expect(api).toHaveBeenCalledTimes(2);
});

it("removes executable HTML and prevents editable or form controls in the preview", () => {
  const { container } = render(
    <MemoPreview
      value={
        '<script>alert(1)</script><a href="javascript:alert(1)">bad</a><img src="/images/test.png" onerror="alert(1)"><p contenteditable="true">text</p><form><button>submit</button></form>'
      }
      onError={vi.fn()}
    />,
  );
  expect(
    container.querySelector("script,form,[onerror],[contenteditable]"),
  ).toBeNull();
  expect(screen.getByText("bad").getAttribute("href")).toBeNull();
});
