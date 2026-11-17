import { fileURLToPath } from "node:url";
import path from "node:path";

import { defineConfig, devices } from "@playwright/test";

const rootDir = path.dirname(fileURLToPath(import.meta.url));
const perfCensus = process.env.COMPOZY_E2E_PERF === "1";
const sharedTmpDir = path.resolve(rootDir, "..", ".tmp", "playwright");

// CI fans the suite out across jobs via COMPOZY_E2E_WEB_SHARD ("2/4"). A
// malformed value must fail loudly instead of silently running the full suite
// on every shard job.
function parseShard(raw: string | undefined): { current: number; total: number } | null {
  if (!raw?.trim()) {
    return null;
  }
  const match = /^([1-9]\d*)\/([1-9]\d*)$/.exec(raw.trim());
  if (!match) {
    throw new Error(`COMPOZY_E2E_WEB_SHARD must look like "1/4", received ${JSON.stringify(raw)}`);
  }
  const current = Number(match[1]);
  const total = Number(match[2]);
  if (current > total) {
    throw new Error(`COMPOZY_E2E_WEB_SHARD shard ${current} exceeds total ${total}`);
  }
  return { current, total };
}

export default defineConfig({
  testDir: "./e2e",
  testMatch: ["**/*.spec.ts"],
  fullyParallel: false,
  forbidOnly: Boolean(process.env.CI),
  retries: perfCensus ? 0 : process.env.CI ? 1 : 0,
  repeatEach: perfCensus ? 5 : 1,
  grep: perfCensus ? /@perf/ : undefined,
  grepInvert: perfCensus
    ? undefined
    : process.env.COMPOZY_E2E_NIGHTLY_REPORT === "1"
      ? /@perf/
      : /@nightly|@perf/,
  workers: 1,
  shard: parseShard(process.env.COMPOZY_E2E_WEB_SHARD),
  timeout: 90_000,
  expect: {
    timeout: 20_000,
  },
  outputDir: path.join(sharedTmpDir, "test-results"),
  reporter: [
    ["list"],
    [
      "json",
      {
        outputFile: path.join(
          sharedTmpDir,
          process.env.COMPOZY_E2E_NIGHTLY_REPORT === "1"
            ? "playwright-nightly.json"
            : "playwright-web.json"
        ),
      },
    ],
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
