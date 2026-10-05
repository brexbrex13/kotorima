import { afterEach, beforeEach, expect, it, vi } from "vitest";
import {
  act,
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
} from "@testing-library/react";
import { JevKeyField } from "./JevKeyField";
import { SettingsForm } from "./SettingsForm";
import { JevEndpoint } from "./smartAdd";
import { Settings } from "./types";
import { api } from "./api";
vi.mock("./api", () => ({ api: vi.fn(), on: vi.fn(() => () => {}) }));
const endpoint = (p: Partial<JevEndpoint> = {}): JevEndpoint => ({
  provider: "typesafe",
  account: "",
  base_url: "",
  model: "",
  ...p,
});
let keys: Record<string, string> = {};
beforeEach(() => {
  keys = {};
  vi.mocked(api).mockReset();
  vi.mocked(api).mockImplementation(async (method, ...args) => {
    if (method === "GetJevStatus") {
      const key = keys[String(args[0])] ?? "";
      return {
        supported: true,
        configured: !!key,
        hint: key.slice(-4),
        invalid: false,
      };
    }
    if (method === "SetJevKey") {
      keys[String(args[0])] = String(args[1]);
      return undefined;
    }
    if (method === "ClearJevKey") {
      delete keys[String(args[0])];
      return undefined;
    }
    if (method === "TestJevKey") return "invalid";
    if (method === "GetShortcutStatus") return "";
    return undefined;
  });
});
afterEach(cleanup);
it("saves a key for the selected provider without keeping it in the field", async () => {
  render(<JevKeyField endpoint={endpoint()} />);
  const field = (await screen.findByLabelText(
    "TypeSafe APIキー",
  )) as HTMLInputElement;
  expect(field.type).toBe("password");
  await waitFor(() => expect(field.placeholder).toBe("未設定"));
  fireEvent.change(field, { target: { value: "tsk-secret-1234" } });
  fireEvent.click(screen.getByText("保存"));
  await waitFor(() =>
    expect(field.placeholder).toBe("設定済み（末尾 ••••1234）"),
  );
  expect(field.value).toBe("");
  expect(keys).toEqual({ typesafe: "tsk-secret-1234" });
  fireEvent.click(screen.getByText("接続確認"));
  await screen.findByText("キーが無効です");
  expect(
    vi.mocked(api).mock.calls.find((c) => c[0] === "TestJevKey")?.[1],
  ).toMatchObject(endpoint());
  fireEvent.click(screen.getByText("削除"));
  await waitFor(() => expect(field.placeholder).toBe("未設定"));
  expect(keys).toEqual({});
});
it("switches the key label, status and destination with the provider", async () => {
  keys = { cloudflare: "cf-token-9876" };
  const { rerender } = render(<JevKeyField endpoint={endpoint()} />);
  await screen.findByLabelText("TypeSafe APIキー");
  expect(screen.getByText(/TypeSafe AI に送信します/)).toBeTruthy();
  const cf = endpoint({
    provider: "cloudflare",
    account: "0123456789abcdef0123456789abcdef",
  });
  rerender(<JevKeyField endpoint={cf} />);
  const field = (await screen.findByLabelText(
    "Cloudflare APIトークン",
  )) as HTMLInputElement;
  await waitFor(() =>
    expect(field.placeholder).toBe("設定済み（末尾 ••••9876）"),
  );
  expect(screen.getByText(/Cloudflare に送信します/)).toBeTruthy();
  fireEvent.click(screen.getByText("接続確認"));
  await screen.findByText("キーが無効です");
  expect(
    vi.mocked(api).mock.calls.find((c) => c[0] === "TestJevKey")?.[1],
  ).toMatchObject(cf);
});
it("shows save errors and the unsupported state", async () => {
  vi.mocked(api).mockImplementation(async (method) => {
    if (method === "GetJevStatus")
      return { supported: true, configured: false, hint: "", invalid: false };
    if (method === "SetJevKey") throw "APIキーを入力してください";
    return undefined;
  });
  render(<JevKeyField endpoint={endpoint()} />);
  const field = await screen.findByLabelText("TypeSafe APIキー");
  fireEvent.change(field, { target: { value: "x" } });
  fireEvent.click(screen.getByText("保存"));
  await screen.findByText("APIキーを入力してください");
  cleanup();
  vi.mocked(api).mockImplementation(async (method) =>
    method === "GetJevStatus"
      ? { supported: false, configured: false, hint: "", invalid: false }
      : undefined,
  );
  render(<JevKeyField endpoint={endpoint()} />);
  await screen.findByText("この環境では使えません");
  await act(async () => {});
  expect(screen.queryByLabelText("TypeSafe APIキー")).toBeNull();
});
it("asks for provider details in settings and saves them", async () => {
  const initial = {
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
  } as Settings;
  render(
    <SettingsForm
      initial={initial}
      onClose={() => {}}
      onSaved={() => {}}
      onError={() => {}}
    />,
  );
  const provider = await screen.findByLabelText("AIの接続方式");
  expect(screen.queryByLabelText("CloudflareのアカウントID")).toBeNull();
  fireEvent.change(provider, { target: { value: "custom" } });
  expect(screen.getByLabelText("ベースURL")).toBeTruthy();
  await screen.findByLabelText("APIキー");
  fireEvent.change(provider, { target: { value: "cloudflare" } });
  expect(screen.queryByLabelText("ベースURL")).toBeNull();
  fireEvent.change(screen.getByLabelText("CloudflareのアカウントID"), {
    target: { value: " 0123456789abcdef0123456789abcdef " },
  });
  await screen.findByLabelText("Cloudflare APIトークン");
  fireEvent.click(screen.getByText("今すぐ保存"));
  await waitFor(() =>
    expect(
      vi.mocked(api).mock.calls.find((c) => c[0] === "SaveSettings")?.[1],
    ).toMatchObject({
      jev_provider: "cloudflare",
      jev_cloudflare_account: "0123456789abcdef0123456789abcdef",
    }),
  );
});
it("does not label a keyless local connection as a saved API key", async () => {
  vi.mocked(api).mockImplementation(async (method) =>
    method === "GetJevStatus"
      ? {
          supported: true,
          configured: true,
          key_configured: false,
          hint: "",
          invalid: false,
        }
      : undefined,
  );
  render(
    <JevKeyField
      endpoint={endpoint({
        provider: "local",
        base_url: "http://localhost:1234/v1",
        model: "m",
      })}
    />,
  );
  const field = (await screen.findByLabelText(
    "APIキー（認証ありの場合のみ）",
  )) as HTMLInputElement;
  await waitFor(() => expect(field.placeholder).toBe("未設定"));
  expect((screen.getByText("削除") as HTMLButtonElement).disabled).toBe(true);
  expect((screen.getByText("接続確認") as HTMLButtonElement).disabled).toBe(
    false,
  );
});
