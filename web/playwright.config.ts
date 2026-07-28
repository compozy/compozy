import { fileURLToPath } from "node:url";
import path from "node:path";

import { defineConfig, devices } from "@playwright/test";

const rootDir = path.dirname(fileURLToPath(import.meta.url));
const sharedTmpDir = path.resolve(rootDir, "..", ".tmp", "playwright");

export default defineConfig({
  testDir: "./e2e",
  testMatch: ["**/*.spec.ts"],
  fullyParallel: false,
  forbidOnly: Boolean(process.env.CI),
  retries: 0,
  workers: 1,
  timeout: 90_000,
  expect: {
    timeout: 20_000,
  },
  outputDir: path.join(sharedTmpDir, "test-results"),
  reporter: [
    ["list"],
    ["html", { open: "never", outputFolder: path.join(sharedTmpDir, "report") }],
  ],
  use: {
    ...devices["Desktop Chrome"],
    headless: process.env.PLAYWRIGHT_HEADFUL !== "1",
    // BrowserArtifactSession owns trace capture for scenario manifests.
    trace: "off",
    screenshot: "only-on-failure",
    video: "retain-on-failure",
  },
});
