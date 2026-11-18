import { defineConfig } from "@playwright/test";
import { join } from "node:path";

export default defineConfig({
  testDir: "./e2e/_electron",
  fullyParallel: false,
  forbidOnly: Boolean(process.env.CI),
  retries: process.env.CI ? 1 : 0,
  workers: 1,
  timeout: 90_000,
  expect: { timeout: 20_000 },
  outputDir: join("..", ".tmp", "playwright", "desktop-results"),
  reporter: [
    ["list"],
    ["json", { outputFile: join("..", ".tmp", "playwright", "playwright-desktop.json") }],
    ["html", { open: "never", outputFolder: join("..", ".tmp", "playwright", "desktop-report") }],
  ],
  use: { trace: "off", screenshot: "only-on-failure", video: "off" },
});
