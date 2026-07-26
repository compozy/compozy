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
        artifactRootDir: testInfo.outputPath("agh-artifacts"),
        ...runtimeOptions,
      });
      try {
        await provide(runtime);
      } finally {
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
        qaOutputRootDir: process.env.AGH_E2E_QA_OUTPUT_DIR,
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
