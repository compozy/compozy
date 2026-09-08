import { copyFile } from "node:fs/promises";
import process from "node:process";

import { expect, test as base } from "@playwright/test";

import { BrowserArtifactSession } from "./browser-artifact-session";
import { createBrowserRuntime, type BrowserRuntime, type BrowserRuntimeOptions } from "./runtime";

type E2EFixtures = {
  appPage: import("@playwright/test").Page;
  browserArtifacts: BrowserArtifactSession;
  runtime: BrowserRuntime;
  runtimeOptions: Omit<BrowserRuntimeOptions, "artifactRootDir">;
};

export const test = base.extend<E2EFixtures>({
  runtimeOptions: [{}, { option: true }],
  runtime: [
    async ({ browserName: _browserName, runtimeOptions }, provide, testInfo) => {
      const runtime = await createBrowserRuntime({
        artifactRootDir: testInfo.outputPath("compozy-artifacts"),
        ...runtimeOptions,
      });
      try {
        await provide(runtime);
      } finally {
        // A failed test keeps the daemon's own log next to its screenshots, so a
        // CI artifact explains a daemon-side stall and not only the browser's view.
        if (testInfo.status !== testInfo.expectedStatus && runtime.paths?.daemonLog) {
          await copyFile(runtime.paths.daemonLog, testInfo.outputPath("daemon-process.log")).catch(
            () => undefined
          );
        }
        await runtime.dispose();
      }
    },
    { timeout: 180_000 },
  ],
  browserArtifacts: [
    async ({ context, runtime }, provide) => {
      const session = await BrowserArtifactSession.start({
        collector: runtime.artifactCollector,
        context,
        qaOutputRootDir: process.env.COMPOZY_E2E_QA_OUTPUT_DIR,
      });
      await provide(session);
      await session.persist();
    },
    { auto: true, timeout: 60_000 },
  ],
  appPage: async ({ page, runtime }, provide) => {
    await page.goto(runtime.url("/"), { waitUntil: "domcontentloaded" });
    await provide(page);
  },
});

export { expect };
