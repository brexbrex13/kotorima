import { expect, test } from "@playwright/test";
const menu = async (page: any, label: string) => {
  await page.getByRole("button", { name: "メニュー", exact: true }).click();
  await page.getByRole("button", { name: label, exact: true }).click();
};
const service = async (page: any, method: string, ...args: unknown[]) => {
  const response = await page.request.get(
    "/wails/runtime?object=0&method=0&args=" +
      encodeURIComponent(
        JSON.stringify({
          methodName: "main.App." + method,
          args,
          "call-id": "usability-" + Math.random(),
        }),
      ),
  );
  const payload = await response.json();
  if (payload.error) throw new Error(JSON.stringify(payload.error));
  return payload.result ?? payload;
};
test("compact layout, inline importance, quick deadline, dark menu and outside detail close", async ({
  page,
}) => {
  await page.setViewportSize({ width: 360, height: 700 });
  await page.goto("/");
  const quick = page.getByLabel("新しい付箋");
  await quick.fill("すぐ期限付きで登録");
  await page.getByLabel("登録時の期限・通知").click();
  await page
    .getByLabel("登録時の設定")
    .getByLabel("期限日", { exact: true })
    .fill("2000-01-01");
  await quick.press("Enter");
  const row = page.locator("article").filter({ hasText: "すぐ期限付きで登録" });
  await expect(row).toBeVisible();
  const star = row.getByRole("button", {
    name: "すぐ期限付きで登録の重要マーク",
  });
  await star.click();
  await expect(star).toHaveAttribute("aria-pressed", "true");
  const quickBox = await quick.boundingBox();
  const categoryBox = await page.locator(".filters").boundingBox();
  const filterBox = await page.getByLabel("タスクの絞り込み").boundingBox();
  expect(quickBox!.y).toBeLessThan(categoryBox!.y);
  expect(categoryBox!.y).toBeLessThan(filterBox!.y);
  await expect(page.getByLabel("並び順")).toHaveCount(0);
  await page.screenshot({ path: "../docs/screenshots/compact-light.png" });
  await menu(page, "ダークモード");
  await expect(page.locator("html")).toHaveAttribute("data-theme", "dark");
  await page.reload();
  await expect(page.locator("html")).toHaveAttribute("data-theme", "dark");
  await expect
    .poll(() =>
      page.evaluate(() => document.documentElement.scrollWidth <= innerWidth),
    )
    .toBe(true);
  await page.screenshot({ path: "../docs/screenshots/compact-dark.png" });
  await row.locator(".card-content").click();
  const detail = page.getByLabel("タスクの詳細", { exact: true });
  const detailBox = await detail.boundingBox();
  const bandBox = await page.getByLabel("期限の確認").boundingBox();
  expect(detailBox!.y).toBeGreaterThanOrEqual(bandBox!.y + bandBox!.height - 1);
  await expect(
    detail.getByRole("button", { name: "完了", exact: true }),
  ).toBeInViewport();
  await page.screenshot({ path: "../docs/screenshots/compact-detail.png" });
  await page
    .getByLabel("詳細の外側を閉じる")
    .click({ position: { x: 12, y: 100 } });
  await expect(detail).toHaveCount(0);
  await menu(page, "ダークモード");
});
test("notification click acknowledges itself and later selector persists snooze", async ({
  page,
  context,
}) => {
  await page.goto("/");
  const make = async (title: string) =>
    service(page, "SaveTask", {
      id: 0,
      version: 0,
      title,
      memo: "",
      status: "pending",
      deadline: "",
      reminder_at: "2000-01-01T00:00:00",
      category_id: 0,
      important: false,
    });
  const first = await make("通知から開く一件目");
  const second = await make("あとでやる二件目");
  let snapshot = await service(page, "GetSnapshot");
  const n1 = snapshot.notifications.find(
    (n: any) => n.task_id === first.id && !n.acknowledged,
  );
  const n2 = snapshot.notifications.find(
    (n: any) => n.task_id === second.id && !n.acknowledged,
  );
  const notice = await context.newPage();
  await notice.setViewportSize({ width: 360, height: 170 });
  await notice.goto("/?window=notifications&notice=" + n1.id);
  await expect(
    notice.getByRole("button", { name: "タスクを開く", exact: true }),
  ).toHaveCount(0);
  await expect(
    notice.getByRole("button", { name: "閉じる", exact: true }),
  ).toHaveCount(0);
  await expect(
    notice.getByRole("button", { name: "完了", exact: true }),
  ).toBeVisible();
  await notice.screenshot({
    path: "../docs/screenshots/notification-actions.png",
  });
  await notice.locator(".notice-content").click();
  await expect
    .poll(
      async () =>
        (await service(page, "GetSnapshot")).notifications.find(
          (n: any) => n.id === n1.id,
        ).acknowledged,
    )
    .toBe(true);
  snapshot = await service(page, "GetSnapshot");
  expect(
    snapshot.notifications.find((n: any) => n.id === n2.id).acknowledged,
  ).toBe(false);
  await notice.goto("/?window=notifications&notice=" + n2.id);
  await notice.getByLabel("あとで通知").selectOption("360");
  await expect
    .poll(
      async () =>
        (await service(page, "GetSnapshot")).notifications.find(
          (n: any) => n.id === n2.id,
        ).acknowledged,
    )
    .toBe(true);
  snapshot = await service(page, "GetSnapshot");
  expect(
    snapshot.tasks.find((t: any) => t.id === second.id).reminder_at,
  ).not.toBe("2000-01-01T00:00:00");
  await notice.close();
});
test("new recurring default is editable; tray input registers independently", async ({
  page,
  context,
}) => {
  await page.goto("/");
  await menu(page, "設定");
  await page
    .getByLabel("タスクリストへ追加する日数（期限の何日前）", { exact: true })
    .fill("12");
  await page.getByRole("button", { name: "今すぐ保存", exact: true }).click();
  await menu(page, "定期設定");
  await page
    .getByRole("button", { name: "定期タスクを追加", exact: true })
    .click();
  await expect(
    page.getByText("共通の追加日数を使う", { exact: true }),
  ).toHaveCount(0);
  await expect(
    page.getByLabel("タスクリストへの追加", { exact: true }),
  ).toHaveValue("12");
  await page.getByLabel("タスクリストへの追加", { exact: true }).fill("3");
  await expect(
    page.getByLabel("タスクリストへの追加", { exact: true }),
  ).toHaveValue("3");
  await page
    .getByRole("region", { name: "定期タスク設定" })
    .getByLabel("定期設定を閉じる")
    .click();
  const mini = await context.newPage();
  await mini.setViewportSize({ width: 360, height: 105 });
  await mini.goto("/?window=quick-add");
  const input = mini.getByLabel("トレイからタスク追加");
  await input.fill("トレイからの要件");
  await mini.screenshot({ path: "../docs/screenshots/tray-input.png" });
  await input.press("Enter");
  await expect(input).toHaveValue("");
  await expect
    .poll(async () =>
      (await service(page, "GetSnapshot")).tasks.some(
        (t: any) => t.title === "トレイからの要件",
      ),
    )
    .toBe(true);
  await mini.close();
});

test("detail groups content with memo and top icon actions save importance", async ({
  page,
}) => {
  await page.goto("/");
  const task = await service(page, "SaveTask", {
    id: 0,
    version: 0,
    title: "詳細のアイコン操作",
    memo: "",
    status: "pending",
    deadline: "",
    reminder_at: "",
    category_id: 0,
    important: false,
  });
  await page.reload();
  await page
    .locator("article")
    .filter({ hasText: task.title })
    .locator(".card-content")
    .click();
  const detail = page.getByLabel("タスクの詳細", { exact: true });
  await expect(detail.locator("legend")).toHaveText([
    "内容",
    "期限・通知",
    "カテゴリ",
  ]);
  await expect(
    detail
      .locator("fieldset")
      .first()
      .getByRole("textbox", { name: "メモ", exact: true }),
  ).toBeVisible();
  const important = detail
    .locator("header")
    .getByRole("button", { name: "重要", exact: true });
  await important.click();
  await expect(important).toHaveAttribute("aria-pressed", "true");
  expect(
    (await service(page, "GetSnapshot")).tasks.find(
      (t: any) => t.id === task.id,
    ).important,
  ).toBe(false);
  await detail.getByRole("button", { name: "今すぐ保存", exact: true }).click();
  await page
    .locator("article")
    .filter({ hasText: task.title })
    .locator(".card-content")
    .click();
  await detail
    .locator("header")
    .getByRole("button", { name: "完了", exact: true })
    .click();
  await expect(detail).toHaveCount(0);
  expect(
    (await service(page, "GetSnapshot")).tasks.find(
      (t: any) => t.id === task.id,
    ).status,
  ).toBe("done");
});

test("deadline groups share a row when they fit and wrap as groups on compact windows", async ({
  page,
}) => {
  await page.setViewportSize({ width: 980, height: 740 });
  await page.goto("/");
  const today = new Date();
  const date = (d: Date) =>
    `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, "0")}-${String(d.getDate()).padStart(2, "0")}`;
  const tomorrow = new Date(today);
  tomorrow.setDate(today.getDate() + 1);
  for (const [title, deadline] of [
    ["本日の書類確認", date(today)],
    ["請求書の確認", date(tomorrow)],
  ]) {
    await service(page, "SaveTask", {
      id: 0,
      version: 0,
      title,
      memo: "",
      status: "pending",
      deadline,
      reminder_at: "",
      category_id: 0,
      important: false,
    });
  }
  await page.reload();
  const band = page.getByLabel("期限の確認");
  await expect(band.locator(".band-group")).toHaveCount(3);
  const wide = await band
    .locator(".band-group")
    .evaluateAll((els) => els.map((el) => el.getBoundingClientRect().top));
  expect(Math.max(...wide) - Math.min(...wide)).toBeLessThan(2);
  await page.screenshot({ path: "../docs/screenshots/polished-board.png" });
  await band
    .getByRole("button", { name: "本日の書類確認", exact: true })
    .click();
  await expect(page.getByLabel("タスクの詳細", { exact: true })).toBeVisible();
  await page.screenshot({ path: "../docs/screenshots/polished-detail.png" });
  await page.getByLabel("詳細を閉じる").click();
  await page.setViewportSize({ width: 360, height: 700 });
  const compact = await band
    .locator(".band-group")
    .evaluateAll((els) => els.map((el) => el.getBoundingClientRect().top));
  expect(Math.max(...compact) - Math.min(...compact)).toBeGreaterThan(10);
  expect(
    await page.evaluate(() => document.documentElement.scrollWidth),
  ).toBeLessThanOrEqual(360);
});
