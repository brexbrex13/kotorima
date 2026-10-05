import { expect, test } from "@playwright/test";
const menu = async (page: any, label: string) => {
  await page.getByRole("button", { name: "メニュー", exact: true }).click();
  await page.getByRole("button", { name: label, exact: true }).click();
};
test("task, rich memo, save-close, history and reopen", async ({ page }) => {
  await page.goto("/");
  const quick = page.getByLabel("新しい付箋");
  await expect(page.getByLabel("期限の確認")).toHaveCount(0);
  await quick.fill("業務メモをすばやく登録");
  await quick.press("Enter");
  const row = page
    .locator("article")
    .filter({ hasText: "業務メモをすばやく登録" });
  await row.locator(".card-content").click();
  await page.getByLabel("期限日", { exact: true }).fill("2000-01-01");
  const memo = page.locator(".ProseMirror");
  await memo.fill("太字の業務メモ");
  await memo.press("Control+a");
  await page.getByRole("button", { name: "太字", exact: true }).click();
  await expect(memo.locator("strong")).toHaveText("太字の業務メモ");
  await page.getByRole("button", { name: "文字色：赤", exact: true }).click();
  await expect(memo.locator("span")).toHaveAttribute("style", /color/);
  await page.locator(".memo input[type=file]").setInputFiles({
    name: "example.png",
    mimeType: "image/png",
    buffer: Buffer.from(
      "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+aZXkAAAAASUVORK5CYII=",
      "base64",
    ),
  });
  await expect(memo.locator("img")).toBeVisible();
  await page.getByRole("button", { name: "今すぐ保存", exact: true }).click();
  await expect(page.getByLabel("詳細を閉じる")).toHaveCount(0);
  await expect(page.getByLabel("期限の確認")).toContainText(
    "業務メモをすばやく登録",
  );
  await page.reload();
  await row.locator(".card-content").click();
  await expect(memo.locator("strong")).toHaveText("太字の業務メモ");
  await expect(memo.locator("img")).toBeVisible();
  await page.getByRole("button", { name: "完了", exact: true }).click();
  await expect(row).toHaveCount(0);
  await page.getByRole("button", { name: "完了済み", exact: true }).click();
  await row.locator(".card-content").click();
  await page.getByRole("button", { name: "再開する", exact: true }).click();
  await page.getByRole("button", { name: "全て", exact: true }).click();
  await expect(row).toBeVisible();
});
test("category context, live colors, custom palette, dormancy and reordering", async ({
  page,
}) => {
  await page.goto("/");
  await menu(page, "カテゴリ管理");
  const name = page.getByLabel("新しいカテゴリ名");
  await name.fill("明日");
  await page.getByRole("button", { name: "追加", exact: true }).click();
  await name.fill("今期");
  await page.getByRole("button", { name: "追加", exact: true }).click();
  await page.getByLabel("明日の配色").click();
  await page.getByRole("button", { name: "ミント", exact: true }).click();
  await page.getByLabel("今期を上へ").click();
  await page.getByLabel("カテゴリ管理を閉じる").click();
  await page.getByRole("button", { name: "明日", exact: true }).click();
  await page.getByLabel("新しい付箋").fill("カテゴリに自動登録");
  await page.getByLabel("新しい付箋").press("Enter");
  await expect(page.locator(".category")).toContainText("カテゴリに自動登録");
  await menu(page, "カテゴリ管理");
  await page.getByLabel("明日の配色").click();
  await page.getByRole("button", { name: "カスタム配色", exact: true }).click();
  await page.getByLabel("名前", { exact: true }).fill("業務用");
  await page.getByRole("button", { name: "保存", exact: true }).click();
  await expect(
    page.getByRole("button", { name: "業務用", exact: true }),
  ).toBeVisible();
  const entry = page
    .locator(".manage-category")
    .filter({ has: page.getByLabel("明日の名前") });
  await entry.getByRole("checkbox", { name: "表示" }).uncheck();
  await expect(
    page.locator(".filters").getByRole("button", { name: "明日", exact: true }),
  ).toHaveCount(0);
  await entry.getByRole("checkbox", { name: "表示" }).check();
  await page.getByLabel("カテゴリ管理を閉じる").click();
});
test("recurring rules without first date, shared lead and separate task list", async ({
  page,
}) => {
  await page.goto("/");
  await menu(page, "定期設定");
  await page
    .getByRole("button", { name: "定期タスクを追加", exact: true })
    .click();
  await page.getByLabel("タスク名", { exact: true }).fill("毎週の報告");
  await expect(page.getByText("初回期限日", { exact: true })).toHaveCount(0);
  await expect(page.getByText("周期間隔", { exact: true })).toHaveCount(0);
  await expect(page.getByText("新しい付箋を作る", { exact: true })).toHaveCount(
    0,
  );
  await expect(
    page.getByLabel("タスクリストへの追加", { exact: true }),
  ).toHaveValue("7");
  await page.getByRole("button", { name: "今すぐ保存", exact: true }).click();
  await expect(
    page.getByRole("region", { name: "定期タスク設定" }),
  ).toHaveCount(0);
  const autoAdd = page.getByRole("switch", { name: "毎週の報告の自動追加" });
  await expect(autoAdd).toHaveAttribute("aria-checked", "true");
  await autoAdd.click();
  await expect(autoAdd).toHaveAttribute("aria-checked", "false");
  await page
    .locator(".series-row")
    .filter({ hasText: "毎週の報告" })
    .getByRole("button", { name: "編集", exact: true })
    .click();
  await page.getByRole("button", { name: "今すぐ保存", exact: true }).click();
  await expect(autoAdd).toHaveAttribute("aria-checked", "false");
  await page.reload();
  await menu(page, "定期設定");
  await expect(autoAdd).toHaveAttribute("aria-checked", "false");
  await autoAdd.click();
  await expect(autoAdd).toHaveAttribute("aria-checked", "true");
  await page.screenshot({ path: "../docs/screenshots/recurring-list.png" });
  await page.getByLabel("定期設定を閉じる").click();
  await expect(
    page.locator("article").filter({ hasText: "毎週の報告" }).first(),
  ).toBeVisible();
  await page.getByRole("button", { name: "定期", exact: true }).click();
  await expect(
    page.locator("article").filter({ hasText: "毎週の報告" }).first(),
  ).toBeVisible();
  await expect(page.getByText("定期タスクを除く", { exact: true })).toHaveCount(
    0,
  );
});
test("notification close does not complete and summary open closes only summary", async ({
  page,
  context,
}) => {
  await page.goto("/");
  await menu(page, "設定");
  await page.getByRole("button", { name: "通知をテスト", exact: true }).click();
  await page.getByRole("button", { name: "今すぐ保存", exact: true }).click();
  await expect(
    page.getByRole("heading", { name: "設定", exact: true }),
  ).toHaveCount(0);
  await expect(
    page
      .getByRole("button", { name: "メニュー", exact: true })
      .locator(".notification-badge"),
  ).toBeVisible();
  await page.getByRole("button", { name: "メニュー", exact: true }).click();
  await expect(
    page
      .locator(".app-menu button")
      .filter({ hasText: "未確認の通知" })
      .locator(".notification-badge"),
  ).toBeVisible();
  await page.getByRole("button", { name: "メニュー", exact: true }).click();
  const notice = await context.newPage();
  await notice.setViewportSize({ width: 360, height: 170 });
  const result = await page.request.get(
    "/wails/runtime?object=0&method=0&args=" +
      encodeURIComponent(
        JSON.stringify({
          methodName: "main.App.GetSnapshot",
          args: [],
          "call-id": "notice-test",
        }),
      ),
  );
  const payload = await result.json();
  const snapshot = payload.result ?? payload;
  const testNotice = snapshot.notifications
    .filter((n: any) => n.kind === "test" && !n.acknowledged)
    .at(-1);
  await notice.goto("/?window=notifications&notice=" + testNotice.id);
  await expect(
    notice.getByRole("button", { name: "通知を閉じる" }),
  ).toBeVisible();
  await notice.screenshot({ path: "test-results/notification.png" });
  await notice
    .getByRole("button", { name: "通知を閉じる", exact: true })
    .click();
  await expect(
    notice.getByRole("button", { name: "通知を閉じる" }),
  ).toHaveCount(0);
});
test("DnD across category boxes including empty categories", async ({
  page,
}) => {
  await page.goto("/");
  await page.getByRole("button", { name: "全て", exact: true }).click();
  const source = page
    .locator("article")
    .filter({ hasText: "カテゴリに自動登録" });
  const dest = page.locator('.category[aria-label="今期"]');
  const from = await source.locator(".grip").boundingBox(),
    to = await dest.boundingBox();
  if (!from || !to) throw Error("Missing drag bounds");
  await page.mouse.move(from.x + from.width / 2, from.y + from.height / 2);
  await page.mouse.down();
  await page.mouse.move(
    from.x + from.width / 2 + 10,
    from.y + from.height / 2,
    { steps: 3 },
  );
  await page.mouse.move(to.x + to.width / 2, to.y + to.height / 2, {
    steps: 15,
  });
  await page.mouse.up();
  await expect(dest).toContainText("カテゴリに自動登録");
  await page.reload();
  await expect(dest).toContainText("カテゴリに自動登録");
  await page.screenshot({ path: "test-results/board.png" });
});
test("memo links, pasted images and editor height survive save and reopen", async ({
  page,
}) => {
  await page.goto("/");
  await page
    .locator("article")
    .filter({ hasText: "業務メモをすばやく登録" })
    .locator(".card-content")
    .click();
  const memo = page.locator(".ProseMirror");
  await expect(
    page.getByRole("button", { name: "箇条書き", exact: true }),
  ).toHaveCount(0);
  await expect(
    page.getByRole("button", { name: "見出し", exact: true }),
  ).toHaveCount(0);
  await expect(
    page.getByRole("button", { name: "チェック", exact: true }),
  ).toHaveCount(0);
  await memo.fill("資料を開く");
  await memo.press("Control+Alt+2");
  await memo.press("Control+Shift+8");
  await expect(memo.locator("h2,ul")).toHaveCount(0);
  await memo.press("Control+a");
  page.once("dialog", (d) => d.accept('"C:\\Work\\資料 #1.pdf"'));
  await page.getByRole("button", { name: "リンク", exact: true }).click();
  await expect(memo.locator("a")).toHaveAttribute(
    "href",
    "file:///C:/Work/%E8%B3%87%E6%96%99%20%231.pdf",
  );
  await page
    .locator(".memo-editor")
    .evaluate((el) => ((el as HTMLElement).style.height = "310px"));
  await expect
    .poll(() => page.evaluate(() => localStorage.getItem("memo-height-small")))
    .toBe("310");
  const frameBox = await page.locator(".memo-editor").boundingBox();
  const editingBox = await memo.boundingBox();
  expect(editingBox!.height).toBeGreaterThanOrEqual(frameBox!.height - 3);
  await memo.click({ position: { x: 15, y: editingBox!.height - 15 } });
  await expect(memo).toBeFocused();
  await memo.press("ArrowRight");
  await memo.evaluate((el) => {
    const dt = new DataTransfer();
    const bytes = Uint8Array.from(
      atob(
        "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+aZXkAAAAASUVORK5CYII=",
      ),
      (c) => c.charCodeAt(0),
    );
    dt.items.add(new File([bytes], "clipboard.png", { type: "image/png" }));
    el.dispatchEvent(
      new ClipboardEvent("paste", {
        clipboardData: dt,
        bubbles: true,
        cancelable: true,
      }),
    );
  });
  await expect(memo.locator("img")).toBeVisible();
  await page.getByRole("button", { name: "今すぐ保存", exact: true }).click();
  await page.reload();
  await page
    .locator("article")
    .filter({ hasText: "業務メモをすばやく登録" })
    .locator(".card-content")
    .click();
  await expect(memo.locator("a")).toHaveAttribute("href", /file:\/\/\/C:/);
  await expect(page.locator(".memo-editor")).toHaveCSS("height", "310px");
  await expect(memo.locator("img")).toBeVisible();
  await page.screenshot({ path: "test-results/task-detail.png" });
});
test("deadline and menu popovers close on outside click; save footer remains visible", async ({
  page,
}) => {
  await page.goto("/");
  await page.getByRole("button", { name: "期限順の一覧", exact: true }).click();
  await expect(page.getByLabel("期限タスク一覧")).toBeVisible();
  await page.getByLabel("新しい付箋").click();
  await expect(page.getByLabel("期限タスク一覧")).toHaveCount(0);
  await menu(page, "設定");
  const button = page.getByRole("button", { name: "今すぐ保存", exact: true });
  const box = await button.boundingBox();
  expect(box!.y + box!.height).toBeLessThanOrEqual(page.viewportSize()!.height);
  await page.screenshot({ path: "test-results/settings.png" });
  await button.click();
  await expect(
    page.getByRole("heading", { name: "設定", exact: true }),
  ).toHaveCount(0);
  await menu(page, "定期設定");
  await page
    .getByRole("button", { name: "定期タスクを追加", exact: true })
    .click();
  await page.getByLabel("タスク名", { exact: true }).fill("水曜までの確認");
  await page.screenshot({ path: "test-results/recurring-form.png" });
});
