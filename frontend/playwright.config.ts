import { defineConfig } from "@playwright/test";
import { mkdtempSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
const dir = mkdtempSync(join(tmpdir(), "kotorima-e2e-"));
export default defineConfig({
  testDir: "e2e",
  workers: 1,
  timeout: 30000,
  use: {
    baseURL: "http://127.0.0.1:18991",
    headless: true,
    launchOptions: process.env.KOTORIMA_CHROMIUM
      ? {
          executablePath: process.env.KOTORIMA_CHROMIUM,
          args: ["--no-sandbox", "--no-zygote", "--disable-dev-shm-usage"],
        }
      : undefined,
    channel: process.env.KOTORIMA_CHROME ? "chrome" : undefined,
    screenshot: "only-on-failure",
    trace: "retain-on-failure",
  },
  webServer: {
    command: process.env.KOTORIMA_SERVER ?? "/tmp/kotorima-server",
    url: "http://127.0.0.1:18991",
    reuseExistingServer: false,
    env: {
      KOTORIMA_DATA_DIR: dir,
      WAILS_SERVER_HOST: "127.0.0.1",
      WAILS_SERVER_PORT: "18991",
    },
  },
});
