import { expect, test } from "@playwright/test";
const service = async (page: any, method: string, ...args: unknown[]) => {
  const response = await page.request.get(
    "/wails/runtime?object=0&method=0&args=" +
      encodeURIComponent(
        JSON.stringify({
          methodName: "main.App." + method,
          args,
          "call-id": "productivity-" + Math.random(),
        }),
      ),
  );
  const value = await response.json();
  if (value.error) throw new Error(JSON.stringify(value.error));
  return value.result ?? value;
};
const create = (page: any, title: string, extra: any = {}) =>
  service(page, "SaveTask", {
    id: 0,
    version: 0,
    title,
    memo: "",
    status: "pending",
    deadline: "",
    reminder_at: "",
    category_id: 0,
    important: false,
    ...extra,
  });
test("today selection combines normal and recurring without changing deadline; completion can be undone", async ({
  page,
}) => {
  await page.goto("/");
  await expect(page.getByLabel("新しい付箋")).toBeVisible();
  const normal = await create(page, "今日着手する来週締切", {
    deadline: "2099-10-08",
  });
  // Seed a regular rule then use its generated occurrence.
  const rule = await service(page, "SaveSeries", {
    id: 0,
    version: 0,
    title: "今日の定例",
    memo: "",
    category_id: 0,
    important: false,
    period: "daily",
    interval: 1,
    weekdays: [],
    month_day: 1,
    month: 1,
    first_due: "",
    next_due: "",
    due_time: "",
    show_days: 7,
    near_days: 3,
    notify_mode: "off",
    notify_days: 0,
    notify_time: "09:00",
    notify_hours: 0,
    active: true,
    deleted: false,
    end_date: "",
  });
  let snapshot = await service(page, "GetSnapshot");
  const recurring = snapshot.tasks.find((t: any) => t.series_id === rule.id);
  expect(recurring).toBeTruthy();
  await service(page, "ToggleToday", normal.id);
  await service(page, "ToggleToday", recurring.id);
  await page.getByRole("button", { name: "今日やる", exact: true }).click();
  const rows = page.locator("article");
  await expect(rows.filter({ hasText: normal.title })).toBeVisible();
  await expect(rows.filter({ hasText: recurring.title })).toBeVisible();
  snapshot = await service(page, "GetSnapshot");
  expect(snapshot.tasks.find((t: any) => t.id === normal.id).deadline).toBe(
    "2099-10-08",
  );
  await rows
    .filter({ hasText: normal.title })
    .getByLabel(normal.title + "を完了")
    .click();
  await expect(rows.filter({ hasText: normal.title })).toHaveCount(0);
  await page.getByRole("button", { name: "元に戻す", exact: true }).click();
  await expect(rows.filter({ hasText: normal.title })).toBeVisible();
  await service(page, "StopSeries", rule.id);
  snapshot = await service(page, "GetSnapshot");
  for (const task of snapshot.tasks.filter((t: any) => t.series_id === rule.id))
    await service(page, "SetState", task.id, "done");
});
test("deadline alone has no reminder; linked notification follows date changes", async ({
  page,
}) => {
  await page.goto("/");
  await expect(page.getByLabel("新しい付箋")).toBeVisible();
  const task = await create(page, "期限日の通知を設定", {
    deadline: "2099-10-08",
  });
  await page
    .locator("article")
    .filter({ hasText: task.title })
    .locator(".card-content")
    .click();
  const detail = page.getByLabel("タスクの詳細", { exact: true });
  await expect(
    detail.getByRole("status").filter({ hasText: "通知なし" }),
  ).toBeVisible();
  await detail.getByLabel("通知方法").selectOption("deadline");
  await detail.getByLabel("期限日の通知時刻").fill("10:30");
  await detail.getByLabel("期限日", { exact: true }).fill("2099-10-09");
  await detail.getByRole("button", { name: "今すぐ保存", exact: true }).click();
  const snapshot = await service(page, "GetSnapshot");
  expect(snapshot.tasks.find((t: any) => t.id === task.id)).toMatchObject({
    deadline: "2099-10-09",
    reminder_at: "2099-10-09T10:30:00",
    reminder_mode: "deadline",
  });
});
test("suggestions complete only title; Enter still adds directly and IME is safe", async ({
  page,
}) => {
  await page.goto("/");
  await expect(page.getByLabel("新しい付箋")).toBeVisible();
  for (let i = 0; i < 3; i++)
    await create(page, "請求書の確認", {
      deadline: "2099-01-01",
      reminder_at: "",
    });
  const input = page.getByLabel("新しい付箋");
  await input.fill("請");
  await expect(
    page.getByRole("option", { name: "請求書の確認", exact: true }),
  ).toBeVisible();
  await input.press("Tab");
  await expect(input).toHaveValue("請求書の確認");
  await input.press("Enter");
  let snapshot = await service(page, "GetSnapshot");
  const added = snapshot.tasks
    .filter((t: any) => t.title === "請求書の確認")
    .at(-1);
  expect(added.deadline).toBe("");
  expect(added.reminder_at).toBe("");
  await input.fill("請");
  await input.press("Enter");
  snapshot = await service(page, "GetSnapshot");
  expect(snapshot.tasks.some((t: any) => t.title === "請")).toBe(true);
});

test("stable tabs, reversible today filter, continuous registration and uncommitted IME suggestions", async ({
  page,
}) => {
  await page.goto("/");
  const input = page.getByLabel("新しい付箋");
  await expect(input).toBeVisible();
  const category = await service(page, "SaveCategory", {
    id: 0,
    name: "復帰確認カテゴリ",
    color: "#ffffff",
    text_color: "#222222",
    sort_order: 0,
    dormant: false,
  });
  const task = await create(page, "カテゴリ復帰確認", {
    category_id: category.id,
  });
  const outside = await create(page, "カテゴリ外の今日", {});
  await service(page, "ToggleToday", outside.id);
  const tab = page.getByRole("button", {
    name: "復帰確認カテゴリ",
    exact: true,
  });
  await tab.click();
  await expect(
    page.locator("article").filter({ hasText: task.title }),
  ).toBeVisible();
  await page.getByRole("button", { name: "今日やる", exact: true }).click();
  await expect(
    page.locator("article").filter({ hasText: outside.title }),
  ).toBeVisible();
  await page.getByRole("button", { name: "今日やる", exact: true }).click();
  await expect(tab).toHaveClass(/active/);
  await expect(
    page.locator("article").filter({ hasText: task.title }),
  ).toBeVisible();
  await expect(
    page.locator("article").filter({ hasText: outside.title }),
  ).toHaveCount(0);
  const regular = page.getByRole("button", { name: "定期", exact: true });
  const before = await regular.boundingBox();
  await regular.click();
  await expect(input).toHaveAttribute("readonly", "");
  const after = await regular.boundingBox();
  expect(after?.y).toBe(before?.y);
  await regular.click();
  await page.getByRole("button", { name: "全て", exact: true }).click();
  await input.fill("連続登録一件目");
  await input.press("Enter");
  await expect(input).toHaveValue("");
  await expect(input).toBeFocused();
  await input.pressSequentially("second continuous task");
  await input.press("Enter");
  await expect(input).toHaveValue("");
  await expect(input).toBeFocused();
  for (let i = 0; i < 3; i++) await create(page, "日報作成");
  await input.evaluate((el: HTMLTextAreaElement) => el.blur());
  await input.click();
  await expect(
    page.getByRole("option", { name: "日報作成", exact: true }),
  ).toBeVisible();
  await input.dispatchEvent("compositionstart", { data: "" });
  await input.fill("にっ");
  await input.dispatchEvent("compositionupdate", { data: "にっ" });
  await expect(
    page.getByRole("option", { name: "日報作成", exact: true }),
  ).toBeVisible();
  await input.press("Enter");
  await expect(input).toHaveValue(/^にっ/);
  const snapshot = await service(page, "GetSnapshot");
  expect(snapshot.tasks.some((t: any) => t.title === "にっ")).toBe(false);
  await input.dispatchEvent("compositionend", { data: "日報" });
  await input.dispatchEvent("keyup", { key: "Enter" });
});

test("notification pause accepts relative periods and can be cleared; shortcut offers Space", async ({
  page,
}) => {
  await page.goto("/");
  await expect(page.getByLabel("新しい付箋")).toBeVisible();
  await page.getByRole("button", { name: "メニュー", exact: true }).click();
  await page.getByRole("button", { name: "設定", exact: true }).click();
  const panel = page
    .locator(".modal")
    .filter({ has: page.getByRole("heading", { name: "設定", exact: true }) });
  await expect(
    panel
      .getByLabel("ショートカットのキー")
      .locator('option[value="Space"], option')
      .filter({ hasText: /^Space$/ }),
  ).toHaveCount(1);
  await panel.getByRole("button", { name: "今すぐ保存", exact: true }).click();
  const pauseOpen = async () => {
    await page.getByRole("button", { name: "メニュー", exact: true }).click();
    await page
      .getByRole("button", { name: "通知を一時停止", exact: true })
      .click();
  };
  await pauseOpen();
  await page.getByRole("button", { name: "30分", exact: true }).click();
  await expect(page.getByLabel("通知を一時停止", { exact: true })).toHaveCount(
    0,
  );
  const snapshot = await service(page, "GetSnapshot");
  expect(
    new Date(snapshot.settings.pause_until).getTime() - Date.now(),
  ).toBeGreaterThan(28 * 60000);
  await pauseOpen();
  await page
    .getByRole("button", { name: "一時停止を解除", exact: true })
    .click();
  await expect
    .poll(async () => (await service(page, "GetSnapshot")).settings.pause_until)
    .toBe("");
});

test("status filters seed new tasks, history reopens inline, search opens an anchored popup", async ({
  page,
}) => {
  await page.goto("/");
  const input = page.getByLabel("新しい付箋");
  await expect(input).toBeVisible();
  await page.getByRole("button", { name: "今日やる", exact: true }).click();
  await page.getByRole("button", { name: "重要", exact: true }).click();
  await expect(input).toHaveAttribute(
    "placeholder",
    /今日やる・重要のタスクとして登録/,
  );
  await input.fill("今日と重要の新規登録");
  await input.press("Enter");
  const row = page
    .locator("article")
    .filter({ hasText: "今日と重要の新規登録" });
  await expect(row).toBeVisible();
  const snapshot = await service(page, "GetSnapshot");
  expect(
    snapshot.tasks.find((t: any) => t.title === "今日と重要の新規登録")
      .important,
  ).toBe(true);
  await row.getByLabel("今日と重要の新規登録を完了").click();
  const history = page.getByRole("button", { name: "完了済み", exact: true });
  await history.click();
  await expect(history).toHaveAttribute("aria-pressed", "true");
  await expect(
    page.getByRole("button", { name: "今日やる", exact: true }),
  ).toBeDisabled();
  await expect(row).toBeVisible();
  await row.getByLabel("今日と重要の新規登録を再開").click();
  await expect(row).toHaveCount(0);
  await history.click();
  await expect(row).toBeVisible();
  await page.getByRole("button", { name: "検索", exact: true }).click();
  const search = page.getByRole("textbox", { name: "検索語", exact: true });
  await search.fill("今日と重要");
  const controls = page.getByLabel("タスクの絞り込み");
  expect((await search.boundingBox())!.y).toBeGreaterThan(
    (await controls.boundingBox())!.y,
  );
  await input.click();
  await expect(search).toHaveCount(0);
  await expect(row).toBeVisible();
  await page.getByLabel("検索を解除").click();
});

test("mini input clock preserves custom selection and saves deadline and notification", async ({
  page,
  context,
}) => {
  await page.goto("/");
  await expect(page.getByLabel("新しい付箋")).toBeVisible();
  const mini = await context.newPage();
  await mini.setViewportSize({ width: 360, height: 430 });
  await mini.goto("/?window=quick-add");
  const input = mini.getByLabel("トレイからタスク追加");
  await input.fill("ミニ期限通知確認");
  await mini.getByLabel("登録時の期限・通知").click();
  await mini.getByLabel("期限日", { exact: true }).fill("2099-10-02");
  await mini.getByLabel("通知方法").selectOption("deadline");
  await mini.getByLabel("通知方法").selectOption("custom");
  await expect(mini.getByLabel("通知時刻", { exact: true })).toBeVisible();
  await mini.getByLabel("通知時刻", { exact: true }).fill("2099-10-02T10:30");
  await mini.getByLabel("重要", { exact: true }).check();
  await mini.screenshot({ path: "test-results/mini-options.png" });
  await mini.getByRole("button", { name: "登録", exact: true }).click();
  await expect(input).toHaveValue("");
  await expect(input).toBeFocused();
  const snapshot = await service(page, "GetSnapshot");
  expect(
    snapshot.tasks.find((t: any) => t.title === "ミニ期限通知確認"),
  ).toMatchObject({
    deadline: "2099-10-02",
    important: true,
    reminder_at: "2099-10-02T10:30",
  });
  await mini.close();
});

test("category collapse persists and notification opening expands it; closing resets draft and filters", async ({
  page,
}) => {
  await page.goto("/");
  const input = page.getByLabel("新しい付箋");
  await expect(input).toBeVisible();
  const task = await create(page, "折り畳み検証");
  const snap = await service(page, "GetSnapshot");
  const category = snap.categories.find((c: any) => c.id === task.category_id);
  const group = page.getByRole("region", { name: category.name, exact: true });
  await expect(group.getByText("折り畳み検証", { exact: true })).toBeVisible();
  await group
    .getByRole("button", { name: category.name + "を折り畳む", exact: true })
    .click();
  await expect(group.getByText("折り畳み検証", { exact: true })).toHaveCount(0);
  await page.reload();
  await expect(
    page.locator("section.category button.category-caption").first(),
  ).toHaveAttribute("aria-expanded", "false");
  await service(page, "OpenTask", task.id);
  await expect(page.getByLabel("タスク名")).toBeVisible();
  await expect(
    page.locator("section.category button.category-caption").first(),
  ).toHaveAttribute("aria-expanded", "true");
  expect((await service(page, "GetSnapshot")).settings.collapsed).toContain(
    category.id,
  );
  await page.getByRole("button", { name: "詳細を閉じる", exact: true }).click();
  await page.getByRole("button", { name: "今日やる", exact: true }).click();
  await page.getByRole("button", { name: "重要", exact: true }).click();
  await input.fill("破棄する入力");
  await page.getByRole("button", { name: "検索", exact: true }).click();
  const search = page.getByLabel("検索語", { exact: true });
  await search.fill("検索中");
  await search.dispatchEvent("keydown", {
    key: "Enter",
    keyCode: 229,
    isComposing: true,
  });
  await expect(search).toBeVisible();
  await page.getByLabel("トレイに格納").click();
  await expect(input).toHaveValue("");
  await expect(
    page.getByRole("button", { name: "全て", exact: true }),
  ).toHaveClass(/active/);
  await expect(
    page.getByRole("button", { name: "今日やる", exact: true }),
  ).toHaveAttribute("aria-pressed", "false");
  await expect(
    page.getByRole("button", { name: "重要", exact: true }),
  ).toHaveAttribute("aria-pressed", "false");
  await expect(page.getByLabel("検索を解除")).toHaveCount(0);
  await service(page, "SetCategoryCollapsed", category.id, false);
});

test("memo inserts unselected links and recognizes quoted local paths; notification wheel requires focus", async ({
  page,
}) => {
  await page.goto("/");
  await expect(page.getByLabel("新しい付箋")).toBeVisible();
  const task = await create(page, "リンクとホイール検証");
  await page.getByText(task.title, { exact: true }).click();
  const memo = page.getByRole("textbox", { name: "メモ", exact: true });
  await memo.click();
  page.once("dialog", (dialog) => dialog.accept("https://example.com/manual"));
  await page.getByRole("button", { name: "リンク", exact: true }).click();
  await expect(memo.locator("a")).toHaveText("https://example.com/manual");
  await memo.press("End");
  await memo.press("Enter");
  await memo.evaluate((el) => {
    const data = new DataTransfer();
    data.setData("text/plain", String.raw`"C:\業務\資料.txt"`);
    el.dispatchEvent(
      new ClipboardEvent("paste", {
        clipboardData: data,
        bubbles: true,
        cancelable: true,
      }),
    );
  });
  await expect(memo.locator('a[href^="file:///"]')).toHaveText(
    String.raw`C:\業務\資料.txt`,
  );
  const notify = page.getByLabel("通知方法");
  await notify.dispatchEvent("wheel", { deltaY: 120 });
  await expect(notify).toHaveValue("off");
  await notify.focus();
  await notify.dispatchEvent("wheel", { deltaY: 120 });
  await expect(notify).toHaveValue("tomorrow");
  await notify.dispatchEvent("wheel", { deltaY: 120 });
  await expect(notify).toHaveValue("30m");
  await page.getByRole("button", { name: "今すぐ保存", exact: true }).click();
  await service(page, "SetState", task.id, "done");
});

test("category/search/status selection expands temporarily and restores saved collapse", async ({
  page,
}) => {
  await page.goto("/");
  await expect(page.getByLabel("新しい付箋")).toBeVisible();
  const task = await create(page, "一時展開確認", {
    important: true,
    deadline: "2099-10-03",
  });
  const snap = await service(page, "GetSnapshot");
  const c = snap.categories.find((c: any) => c.id === task.category_id);
  await service(page, "SetCategoryCollapsed", c.id, true);
  const group = page.getByRole("region", { name: c.name, exact: true });
  const header = group.locator(".category-caption");
  await expect(header).toHaveAttribute("aria-expanded", "false");
  await page
    .locator(".filters")
    .getByRole("button", { name: c.name, exact: true })
    .click();
  await expect(header).toHaveAttribute("aria-expanded", "true");
  await expect(group.getByText(task.title, { exact: true })).toBeVisible();
  await header.click();
  await expect(header).toHaveAttribute("aria-expanded", "false");
  await page.getByRole("button", { name: "全て", exact: true }).click();
  await expect(header).toHaveAttribute("aria-expanded", "false");
  await page.getByRole("button", { name: "検索", exact: true }).click();
  await page.getByLabel("検索語", { exact: true }).fill(task.title);
  await expect(group.getByText(task.title, { exact: true })).toBeVisible();
  await page.getByLabel("検索語を消去").click();
  await expect(header).toHaveAttribute("aria-expanded", "false");
  await page.getByLabel("新しい付箋").click();
  await page.getByRole("button", { name: "重要", exact: true }).click();
  await expect(header).toHaveAttribute("aria-expanded", "true");
  await page.getByRole("button", { name: "重要", exact: true }).click();
  await expect(header).toHaveAttribute("aria-expanded", "false");
  expect((await service(page, "GetSnapshot")).settings.collapsed).toContain(
    c.id,
  );
  await service(page, "SetCategoryCollapsed", c.id, false);
});

test("mini suggestions remain available with details and preserve input geometry; explicit close discards", async ({
  page,
  context,
}) => {
  await page.goto("/");
  await expect(page.getByLabel("新しい付箋")).toBeVisible();
  for (let i = 0; i < 3; i++) await create(page, "簡易候補の業務");
  const mini = await context.newPage();
  await mini.setViewportSize({ width: 360, height: 430 });
  await mini.goto("/?window=quick-add");
  const input = mini.getByLabel("トレイからタスク追加");
  await input.fill("候補のない入力");
  const before = await input.boundingBox();
  await input.fill("簡易候補");
  await expect(
    mini.getByRole("option", { name: "簡易候補の業務", exact: true }),
  ).toBeVisible();
  expect(await input.boundingBox()).toEqual(before);
  await mini.getByLabel("登録時の期限・通知").click();
  await input.focus();
  await expect(
    mini.getByRole("option", { name: "簡易候補の業務", exact: true }),
  ).toBeVisible();
  await mini
    .getByRole("option", { name: "簡易候補の業務", exact: true })
    .click();
  await expect(input).toHaveValue("簡易候補の業務");
  await mini.getByLabel("期限日", { exact: true }).fill("2099-10-03");
  await mini.getByLabel("入力欄を閉じる").click();
  await expect(input).toHaveValue("");
  await mini.getByLabel("登録時の期限・通知").click();
  await expect(mini.getByLabel("期限日", { exact: true })).toHaveValue("");
  await input.fill("Escで破棄");
  await input.press("Escape");
  await expect(input).toHaveValue("");
  await mini.close();
});

test("main suggestions require explicit input and never cover the task list after restore", async ({
  page,
}) => {
  await page.goto("/");
  for (let n = 1; n <= 5; n++)
    for (let i = 0; i < 3; i++) await create(page, `復帰候補の作業${n}`);
  await page.reload();
  const input = page.getByLabel("新しい付箋");
  await expect(input).toBeVisible();
  await input.focus();
  await expect(page.locator(".quick .task-suggestions")).toHaveCount(0);
  await input.click();
  const list = page.locator(".quick .task-suggestions");
  await expect(list).toBeVisible();
  await expect(list.getByRole("option")).toHaveCount(5);
  await input.dispatchEvent("pointerdown", { button: 2 });
  await input.dispatchEvent("pointerup", { button: 2 });
  await input.press("Shift+Tab");
  await expect(list).toHaveCount(0);
  await input.click();
  await expect(list).toBeVisible();
  const listBox = await list.boundingBox();
  const filtersBox = await page.locator(".filters").boundingBox();
  expect(listBox!.y + listBox!.height).toBeLessThanOrEqual(filtersBox!.y);
  expect(listBox!.height).toBeLessThanOrEqual(120);
  for (let i = 0; i < 5; i++) await input.press("ArrowDown");
  const active = list.locator(".active");
  const activeBox = await active.boundingBox();
  const currentListBox = await list.boundingBox();
  expect(activeBox!.y).toBeGreaterThanOrEqual(currentListBox!.y);
  expect(activeBox!.y + activeBox!.height).toBeLessThanOrEqual(
    currentListBox!.y + currentListBox!.height,
  );
  await page.evaluate(() => window.dispatchEvent(new Event("blur")));
  await page.evaluate(() => window.dispatchEvent(new Event("focus")));
  await input.focus();
  await expect(list).toHaveCount(0);
  await input.fill("復帰候補");
  await expect(list).toBeVisible();
  await input.press("Tab");
  await expect(input).toHaveValue(/復帰候補の作業/);
  await expect(list).toHaveCount(0);
  await input.press("Enter");
  await expect(input).toHaveValue("");
  await expect(input).toBeFocused();
  await expect(list).toHaveCount(0);
  await page.setViewportSize({ width: 360, height: 420 });
  await input.click();
  await expect(list).toBeVisible();
  const smallList = await list.boundingBox();
  const smallFilters = await page.locator(".filters").boundingBox();
  expect(smallList!.y + smallList!.height).toBeLessThanOrEqual(smallFilters!.y);
});
