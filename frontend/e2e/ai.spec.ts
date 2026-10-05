import { expect, test } from "@playwright/test";
import { createServer } from "node:http";
import { AddressInfo } from "node:net";

test("local AI without credentials, settings placement and suggestion", async ({
  page,
}) => {
  let calls = 0;
  const server = createServer((req, res) => {
    let body = "";
    req.on("data", (chunk) => (body += chunk));
    req.on("end", () => {
      expect(req.url).toBe("/v1/chat/completions");
      expect(req.headers.authorization).toBeUndefined();
      const payload = JSON.parse(body);
      expect(payload.model).toBe("test-local");
      expect(payload.messages[0].role).toBe("system");
      calls++;
      res.setHeader("Content-Type", "application/json");
      res.end(
        JSON.stringify({
          choices: [
            {
              message: {
                content: JSON.stringify({
                  important: true,
                  deadline: "tomorrow",
                  reminder: "off",
                  ok: true,
                }),
              },
              finish_reason: "stop",
            },
          ],
        }),
      );
    });
  });
  await new Promise<void>((resolve) => server.listen(0, "127.0.0.1", resolve));
  try {
    await page.goto("/");
    await page.getByRole("button", { name: "メニュー", exact: true }).click();
    await page.getByRole("button", { name: "設定", exact: true }).click();
    const headings = await page.locator(".modal h3").allTextContents();
    expect(headings.indexOf("AI対応")).toBeGreaterThan(
      headings.indexOf("データとバックアップ"),
    );
    await page.getByLabel("AIの接続方式").selectOption("local");
    await page
      .getByLabel("ベースURL")
      .fill(`http://127.0.0.1:${(server.address() as AddressInfo).port}/v1`);
    await page.getByLabel("モデル名").fill("test-local");
    await page.getByRole("button", { name: "接続確認", exact: true }).click();
    await expect(page.locator(".jev-key [role=status]")).toContainText(
      "接続できました",
    );
    await page.getByLabel("AIの待ち時間（秒）").fill("30");
    await page.getByLabel("AIでタスク入力を補助する").check();
    await page.getByLabel("AIの接続方式").scrollIntoViewIfNeeded();
    await page.screenshot({ path: "/tmp/kotorima-ai-settings.png" });
    await page.getByRole("button", { name: "今すぐ保存", exact: true }).click();
    await page.getByLabel("新しい付箋").fill("明日までに提出するAI確認");
    await expect(page.getByLabel("推定した初期値")).toContainText("重要");
    await expect(page.getByLabel("推定した初期値")).toContainText("期限 明日");
    expect(calls).toBeGreaterThanOrEqual(2);
    await page.getByLabel("新しい付箋").fill("");
    await page.getByRole("button", { name: "メニュー", exact: true }).click();
    await page.getByRole("button", { name: "設定", exact: true }).click();
    await page.getByLabel("AIでタスク入力を補助する").uncheck();
    await page.getByLabel("AIの接続方式").selectOption("typesafe");
    await page.getByRole("button", { name: "今すぐ保存", exact: true }).click();
  } finally {
    server.closeAllConnections();
    await new Promise<void>((resolve) => server.close(() => resolve()));
  }
});
