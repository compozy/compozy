import { fileURLToPath } from "node:url";
import path from "node:path";

import { defineConfig, devices } from "@playwright/test";

const webRoot = path.dirname(fileURLToPath(import.meta.url));
const repoRoot = path.resolve(webRoot, "..");
const sharedTmpDir = path.join(repoRoot, ".tmp", "playwright");

process.env.COMPOZY_PLAYWRIGHT_REPO_ROOT ??= repoRoot;
process.env.COMPOZY_PLAYWRIGHT_ENV_FILE ??= path.join(sharedTmpDir, "daemon-ui-env.json");

export default defineConfig({
  testDir: "./e2e",
  testMatch: ["**/*.spec.ts"],
  fullyParallel: false,
  forbidOnly: Boolean(process.env.CI),
  retries: process.env.CI ? 1 : 0,
  workers: 1,
  timeout: 120_000,
  expect: {
    timeout: 15_000,
  },
  outputDir: path.join(sharedTmpDir, "test-results"),
  globalSetup: path.join(webRoot, "e2e", "global.setup.ts"),
  globalTeardown: path.join(webRoot, "e2e", "global.teardown.ts"),
  reporter: [
    ["list"],
    ["html", { open: "never", outputFolder: path.join(sharedTmpDir, "report") }],
  ],
  use: {
    ...devices["Desktop Chrome"],
    headless: process.env.PLAYWRIGHT_HEADFUL !== "1",
    trace: "on-first-retry",
    screenshot: "only-on-failure",
    video: "retain-on-failure",
  },
});
