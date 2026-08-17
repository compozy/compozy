import { defineConfig } from "@playwright/test";
import { join } from "node:path";

export default defineConfig({
  testDir: "./e2e/_electron",
  fullyParallel: false,
  forbidOnly: Boolean(process.env.CI),
  retries: 0,
  workers: 1,
  timeout: 90_000,
  expect: { timeout: 20_000 },
  outputDir: join("..", ".tmp", "playwright", "desktop-results"),
  reporter: [
    ["list"],
    ["html", { open: "never", outputFolder: join("..", ".tmp", "playwright", "desktop-report") }],
  ],
  use: { trace: "off", screenshot: "only-on-failure", video: "off" },
});
