import { expect, test } from "@playwright/test";
import { createServer } from "node:http";
import { AddressInfo } from "node:net";

const service = async (page: any, method: string, ...args: unknown[]) => {
  const response = await page.request.get(
    "/wails/runtime?object=0&method=0&args=" +
      encodeURIComponent(
        JSON.stringify({
          methodName: "main.App." + method,
          args,
          "call-id": "review-" + Math.random(),
        }),
      ),
  );
  const body = await response.text();
  let value: any;
  try {
    value = JSON.parse(body);
  } catch {
    return body;
  }
  if (value.error) throw new Error(JSON.stringify(value.error));
  return value.result ?? value;
};

test("category tabs wrap at minimum width and right click focuses the selected category", async ({
  page,
}) => {
  await page.goto("/");
  await page.getByLabel("カテゴリを追加", { exact: true }).click();
  await expect(page.getByLabel("新しいカテゴリ名")).toBeFocused();
  await page.getByLabel("カテゴリ管理を閉じる").click();
  for (let i = 0; i < 9; i++)
    await service(page, "SaveCategory", {
      id: 0,
      name: `確認用カテゴリ${i}`,
      color: "#fffdf8",
      text_color: "#302d25",
      sort_order: 0,
      dormant: false,
    });
  await page.setViewportSize({ width: 360, height: 760 });
  await expect(
    page
      .locator(".category-tabs")
      .getByRole("button", { name: "確認用カテゴリ8", exact: true }),
  ).toBeVisible();
  expect(
    await page.evaluate(() => document.documentElement.scrollWidth),
  ).toBeLessThanOrEqual(360);
  await page
    .locator(".category-tabs")
    .getByRole("button", { name: "確認用カテゴリ8", exact: true })
    .click({ button: "right" });
  await page
    .getByRole("menuitem", { name: "カテゴリ編集", exact: true })
    .click();
  await expect(page.getByLabel("確認用カテゴリ8の名前")).toBeFocused();
  await page.getByLabel("カテゴリ管理を閉じる").click();
  await page.screenshot({ path: "/tmp/kotorima-review-narrow.png" });
});

test("bulk trash is independent of search and empty trash permanently removes only trashed tasks", async ({
  page,
}) => {
  await page.goto("/");
  const create = (title: string, status: string) =>
    service(page, "SaveTask", {
      id: 0,
      version: 0,
      title,
      status,
      memo: "",
      deadline: "",
      reminder_at: "",
      category_id: 0,
      important: false,
    });
  const a = await create("一括削除確認A", "done");
  const b = await create("一括削除確認B", "done");
  const keep = await create("一括削除で残す未完了", "pending");
  await page.getByRole("button", { name: "完了済み", exact: true }).click();
  await page.getByLabel("検索", { exact: true }).click();
  await page.getByLabel("検索語", { exact: true }).fill("一括削除確認A");
  await page.getByLabel("検索語", { exact: true }).press("Enter");
  page.once("dialog", (dialog) => dialog.accept());
  await page
    .getByRole("button", { name: "完了済みをすべてごみ箱へ", exact: true })
    .click();
  await expect
    .poll(
      async () =>
        (await service(page, "GetSnapshot")).tasks.find(
          (t: any) => t.id === a.id,
        ).deleted_at,
    )
    .not.toBe("");
  const moved = await service(page, "GetSnapshot");
  expect(moved.tasks.find((t: any) => t.id === a.id).deleted_at).toBeTruthy();
  expect(moved.tasks.find((t: any) => t.id === b.id).deleted_at).toBeTruthy();
  await page.getByRole("button", { name: "メニュー", exact: true }).click();
  await page.getByRole("button", { name: "ごみ箱", exact: true }).click();
  page.once("dialog", (dialog) => dialog.accept());
  await page
    .getByRole("button", { name: "ごみ箱を空にする", exact: true })
    .click();
  await expect
    .poll(async () =>
      (await service(page, "GetSnapshot")).tasks.some(
        (t: any) => t.id === a.id || t.id === b.id,
      ),
    )
    .toBe(false);
  await expect(
    page.getByRole("button", { name: "ごみ箱を空にする", exact: true }),
  ).toBeDisabled();
  const purged = await service(page, "GetSnapshot");
  expect(purged.tasks.some((t: any) => t.id === a.id || t.id === b.id)).toBe(
    false,
  );
  expect(purged.tasks.some((t: any) => t.id === keep.id && !t.deleted_at)).toBe(
    true,
  );
  await page.screenshot({ path: "/tmp/kotorima-review-trash.png" });
});

test("AI failure stops requests, ordinary registration works and connection check resumes AI", async ({
  page,
}) => {
  let healthy = false,
    calls = 0;
  const server = createServer((req, res) => {
    req.resume();
    req.on("end", () => {
      calls++;
      if (!healthy) {
        res.writeHead(503);
        res.end();
        return;
      }
      res.setHeader("Content-Type", "application/json");
      res.end(
        JSON.stringify({
          choices: [
            {
              message: {
                content: JSON.stringify({
                  ok: true,
                  important: true,
                  deadline: "tomorrow",
                  reminder: "off",
                }),
              },
              finish_reason: "stop",
            },
          ],
        }),
      );
    });
  });
  await new Promise<void>((r) => server.listen(0, "127.0.0.1", r));
  try {
    await page.goto("/");
    const snapshot = await service(page, "GetSnapshot");
    await service(page, "SaveSettings", {
      ...snapshot.settings,
      smart_add: true,
      jev_provider: "local",
      jev_custom_url: `http://127.0.0.1:${(server.address() as AddressInfo).port}/v1`,
      jev_custom_model: "review-local",
      ai_timeout_seconds: 2,
    });
    await page.reload();
    const input = page.getByLabel("新しい付箋");
    await input.fill("接続失敗を確認する入力");
    await expect(page.locator(".smart-hints")).toContainText(
      "通常登録に切り替え",
    );
    expect(calls).toBe(1);
    await input.fill("通信せず通常登録する入力");
    await page.waitForTimeout(600);
    expect(calls).toBe(1);
    await input.press("Enter");
    await expect(
      page.locator(".task-row").filter({ hasText: "通信せず通常登録する入力" }),
    ).toBeVisible();
    healthy = true;
    await page.getByRole("button", { name: "メニュー", exact: true }).click();
    await page.getByRole("button", { name: "設定", exact: true }).click();
    await page.getByRole("button", { name: "接続確認", exact: true }).click();
    await expect(page.locator(".jev-key")).toContainText("接続できました");
    await page.getByRole("button", { name: "今すぐ保存", exact: true }).click();
    await input.fill("再接続後の入力");
    await expect(page.getByLabel("推定した初期値")).toContainText("重要");
    const latest = await service(page, "GetSnapshot");
    await service(page, "SaveSettings", {
      ...latest.settings,
      smart_add: false,
    });
  } finally {
    server.closeAllConnections();
    await new Promise<void>((r) => server.close(() => r()));
  }
});

test("list context menus edit categories and follow task state without opening the category menu", async ({
  page,
}) => {
  await page.goto("/");
  const category = await service(page, "SaveCategory", {
    id: 0,
    name: "右クリック確認",
    color: "#fffdf8",
    text_color: "#302d25",
    sort_order: 0,
    dormant: false,
  });
  const task = await service(page, "SaveTask", {
    id: 0,
    version: 0,
    title: "右クリック用タスク",
    status: "pending",
    memo: "",
    deadline: "",
    reminder_at: "",
    category_id: category.id,
    important: false,
  });
  await page.reload();
  const group = page.getByRole("region", {
    name: "右クリック確認",
    exact: true,
  });
  await group.locator(".category-caption").click({ button: "right" });
  await page
    .getByRole("menuitem", { name: "カテゴリ編集", exact: true })
    .click();
  await expect(page.getByLabel("右クリック確認の名前")).toBeFocused();
  await page.getByLabel("カテゴリ管理を閉じる").click();
  const row = page
    .locator(".task-row")
    .filter({ hasText: "右クリック用タスク" });
  await row.click({ button: "right" });
  await expect(
    page.getByRole("menuitem", { name: "カテゴリ編集", exact: true }),
  ).toHaveCount(0);
  await page.getByRole("menuitem", { name: "詳細を開く", exact: true }).click();
  await expect(
    page.getByRole("complementary", { name: "タスクの詳細" }),
  ).toBeVisible();
  await page.getByLabel("詳細の外側を閉じる").click();
  await row.click({ button: "right" });
  await page.getByRole("menuitem", { name: "完了にする", exact: true }).click();
  await page.getByRole("button", { name: "完了済み", exact: true }).click();
  await expect(row).toBeVisible();
  await row.click({ button: "right" });
  await expect(
    page.getByRole("menuitem", { name: "未完了に戻す", exact: true }),
  ).toBeVisible();
  await page
    .getByRole("menuitem", { name: "ごみ箱へ送る", exact: true })
    .click();
  await expect
    .poll(
      async () =>
        (await service(page, "GetSnapshot")).tasks.find(
          (t: any) => t.id === task.id,
        ).deleted_at,
    )
    .not.toBe("");
  await page.getByRole("button", { name: "メニュー", exact: true }).click();
  await page.getByRole("button", { name: "ごみ箱", exact: true }).click();
  await row.click({ button: "right" });
  await expect(
    page.getByRole("menuitem", { name: "完了にする", exact: true }),
  ).toHaveCount(0);
  await page
    .getByRole("menuitem", { name: "ごみ箱から戻す", exact: true })
    .click();
  await expect
    .poll(
      async () =>
        (await service(page, "GetSnapshot")).tasks.find(
          (t: any) => t.id === task.id,
        ).deleted_at,
    )
    .toBe("");
  await page.getByRole("button", { name: "完了済み", exact: true }).click();
  await row.click({ button: "right" });
  await page
    .getByRole("menuitem", { name: "未完了に戻す", exact: true })
    .click();
  await page.getByRole("button", { name: "一覧に戻る", exact: true }).click();
  await expect(row).toBeVisible();
  await page.setViewportSize({ width: 360, height: 760 });
  await row.click({ button: "right", position: { x: 280, y: 20 } });
  const menu = page.getByRole("menu", { name: "タスクの操作" });
  const box = await menu.boundingBox();
  expect(box!.x + box!.width).toBeLessThanOrEqual(360);
  expect(box!.y + box!.height).toBeLessThanOrEqual(760);
  await page.keyboard.press("Escape");
  await expect(menu).toHaveCount(0);
});

test("notification memo is opt-in, images expand without acknowledging, and private mode hides it", async ({
  page,
  context,
}) => {
  await page.goto("/");
  const settings = (await service(page, "GetSnapshot")).settings;
  await service(page, "SaveSettings", {
    ...settings,
    private: false,
    smart_add: false,
  });
  const image = await service(
    page,
    "SaveImage",
    "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAHgAAAA8CAIAAAAiz+n/AAAAo0lEQVR4nO3QQRHAIADAMMC/qVnAAFamYuWxREGv89ln8L11O+AvjI4YHTE6YnTE6IjREaMjRkeMjhgdMTpidMToiNERoyNGR4yOGB0xOmJ0xOiI0RGjI0ZHjI4YHTE6YnTE6IjREaMjRkeMjhgdMTpidMToiNERoyNGR4yOGB0xOmJ0xOiI0RGjI0ZHjI4YHTE6YnTE6IjREaMjRkeMjhgdeQEW7wLqdTHsdAAAAABJRU5ErkJggg==",
  );
  const task = await service(page, "SaveTask", {
    id: 0,
    version: 0,
    title: "メモ付き作業通知",
    memo: `<p><strong>確認して保存</strong></p><a href="file:///C:/Work%20Files">作業場所</a><img src="${image}"><p>${"長い手順\n".repeat(30)}</p>`,
    status: "pending",
    reminder_at: "2000-01-01T00:00:00",
    category_id: 0,
  });
  await page.reload();
  const n = (await service(page, "GetSnapshot")).notifications.find(
    (n: any) => n.task_id === task.id && !n.acknowledged,
  );
  const notice = await context.newPage();
  await notice.setViewportSize({ width: 360, height: 330 });
  await notice.goto("/?window=notifications&notice=" + n.id);
  await expect(notice.getByLabel("通知のメモ")).toHaveCount(0);
  await page
    .locator(".task-row")
    .filter({ hasText: task.title })
    .locator(".card-content")
    .click();
  const detail = page.getByLabel("タスクの詳細", { exact: true });
  await expect(detail.getByLabel("通知にメモを表示する")).not.toBeChecked();
  await detail.getByLabel("通知にメモを表示する").check();
  await detail.getByRole("button", { name: "今すぐ保存", exact: true }).click();
  await expect(notice.getByLabel("通知のメモ")).toBeVisible();
  await expect(notice.getByRole("link", { name: "作業場所" })).toHaveAttribute(
    "href",
    "file:///C:/Work%20Files",
  );
  const img = notice.getByRole("button", {
    name: "添付画像を拡大",
    exact: true,
  });
  await expect
    .poll(() =>
      img.evaluate(
        (el: HTMLImageElement) => el.complete && el.naturalWidth > 0,
      ),
    )
    .toBe(true);
  await img.click();
  const viewer = await context.newPage();
  await viewer.setViewportSize({ width: 900, height: 650 });
  await viewer.goto("/?window=image-viewer&src=" + encodeURIComponent(image));
  await expect(viewer.getByAltText("メモの添付画像")).toBeVisible();
  await viewer.getByLabel("画像ビューアを閉じる").click();
  await viewer.close();
  expect(
    (await service(page, "GetSnapshot")).notifications.find(
      (x: any) => x.id === n.id,
    ).acknowledged,
  ).toBe(false);
  await expect(
    notice.getByRole("button", { name: "完了", exact: true }),
  ).toBeInViewport();
  expect(
    await notice.evaluate(() => document.documentElement.scrollWidth),
  ).toBeLessThanOrEqual(360);
  await notice.screenshot({ path: "/tmp/kotorima-notice-memo.png" });
  await service(page, "SaveSettings", {
    ...settings,
    private: true,
    smart_add: false,
  });
  await notice.reload();
  await expect(notice.getByLabel("通知のメモ")).toHaveCount(0);
  await expect(notice.locator(".notice-content")).toContainText("タスクの通知");
  await service(page, "SaveSettings", {
    ...settings,
    private: false,
    smart_add: false,
  });
  await notice.reload();
  await notice.getByRole("button", { name: "完了", exact: true }).click();
  await expect
    .poll(
      async () =>
        (await service(page, "GetSnapshot")).tasks.find(
          (x: any) => x.id === task.id,
        ).status,
    )
    .toBe("done");
  await notice.close();
});

test("deadline and reminder drafts do not execute before explicit save and can be discarded", async ({
  page,
}) => {
  await page.goto("/");
  const task = await service(page, "SaveTask", {
    id: 0,
    version: 0,
    title: "保存前通知の回帰確認",
    status: "pending",
    memo: "",
    deadline: "",
    reminder_at: "",
    category_id: 0,
  });
  await page.reload();
  const row = page.locator(".task-row").filter({ hasText: task.title });
  const detail = page.getByLabel("タスクの詳細", { exact: true });
  const edit = async () => {
    await row.locator(".card-content").click();
    await detail.getByLabel("期限日", { exact: true }).fill("2000-01-01");
    await detail.getByLabel("通知方法").selectOption("custom");
    await detail
      .getByLabel("通知時刻", { exact: true })
      .fill("2000-01-01T10:00");
  };
  await edit();
  await page.waitForTimeout(1100);
  let snapshot = await service(page, "GetSnapshot");
  expect(snapshot.tasks.find((t: any) => t.id === task.id)).toMatchObject({
    deadline: "",
    reminder_at: "",
  });
  expect(snapshot.notifications.some((n: any) => n.task_id === task.id)).toBe(
    false,
  );
  await detail.getByLabel("詳細を閉じる").click();
  await page
    .getByRole("button", { name: "破棄して続ける", exact: true })
    .click();
  await expect(detail).toHaveCount(0);
  await row.locator(".card-content").click();
  await expect(detail.getByLabel("期限日", { exact: true })).toHaveValue("");
  await expect(detail.getByLabel("通知方法")).toHaveValue("off");
  await detail.getByLabel("詳細を閉じる").click();
  await edit();
  await detail.getByLabel("詳細を閉じる").click();
  await page
    .getByRole("button", { name: "保存して続ける", exact: true })
    .click();
  await expect(detail).toHaveCount(0);
  snapshot = await service(page, "GetSnapshot");
  expect(snapshot.tasks.find((t: any) => t.id === task.id)).toMatchObject({
    deadline: "2000-01-01",
    reminder_at: "2000-01-01T10:00",
  });
  await expect
    .poll(async () =>
      (await service(page, "GetSnapshot")).notifications.some(
        (n: any) => n.task_id === task.id && !n.acknowledged,
      ),
    )
    .toBe(true);
  await service(page, "SetState", task.id, "done");
});

test("completed and trash category groups only show categories with matching tasks", async ({
  page,
}) => {
  await page.goto("/");
  const categories = [];
  for (const name of ["履歴表示対象", "履歴表示空欄"])
    categories.push(
      await service(page, "SaveCategory", {
        id: 0,
        name,
        color: "#fffdf8",
        text_color: "#302d25",
        sort_order: 0,
        dormant: false,
      }),
    );
  const task = await service(page, "SaveTask", {
    id: 0,
    version: 0,
    title: "カテゴリ絞り込み確認",
    status: "done",
    category_id: categories[0].id,
    memo: "",
    deadline: "",
    reminder_at: "",
  });
  await page.reload();
  await page.getByRole("button", { name: "完了済み", exact: true }).click();
  await expect(
    page.getByRole("region", { name: categories[0].name, exact: true }),
  ).toBeVisible();
  await expect(
    page.getByRole("region", { name: categories[1].name, exact: true }),
  ).toHaveCount(0);
  await service(page, "SetState", task.id, "trash");
  await expect(
    page.getByRole("region", { name: categories[0].name, exact: true }),
  ).toHaveCount(0);
  await page.getByRole("button", { name: "メニュー", exact: true }).click();
  await page.getByRole("button", { name: "ごみ箱", exact: true }).click();
  await expect(
    page.getByRole("region", { name: categories[0].name, exact: true }),
  ).toBeVisible();
  await expect(
    page.getByRole("region", { name: categories[1].name, exact: true }),
  ).toHaveCount(0);
});
