import { spawn, type ChildProcessWithoutNullStreams } from "node:child_process";
import { setTimeout as delay } from "node:timers/promises";
import path from "node:path";
import { fileURLToPath } from "node:url";

import { expect, test, type Page } from "@playwright/test";

const storybookHost = "127.0.0.1";
const storybookPort = 6207;
const storybookBaseURL = `http://${storybookHost}:${storybookPort}`;
const currentDir = path.dirname(fileURLToPath(import.meta.url));
const repoRoot = path.resolve(currentDir, "../../..");
const packagesUiRoot = path.resolve(repoRoot, "packages/ui");

test.setTimeout(180_000);

test("packages/ui Storybook overlays open visibly", async ({ page }) => {
  const storybook = spawn(
    "bunx",
    ["storybook", "dev", "--host", storybookHost, "--port", String(storybookPort), "--ci"],
    {
      cwd: packagesUiRoot,
      env: process.env,
      stdio: "pipe",
    }
  );

  const output: string[] = [];
  storybook.stdout.on("data", chunk => output.push(chunk.toString()));
  storybook.stderr.on("data", chunk => output.push(chunk.toString()));

  try {
    await waitForStorybook(storybookBaseURL, storybook);

    await expectOverlayVisible(
      page,
      "components-custom-commandselect--single-select",
      async storyPage => {
        await storyPage.getByRole("button", { name: /select model/i }).click();
      },
      "[data-slot='command-select-shell']"
    );

    await expectOverlayVisible(
      page,
      "components-ui-popover--default",
      async storyPage => {
        await storyPage.getByRole("button", { name: /open popover/i }).click();
      },
      "[data-slot='popover-content']"
    );

    await expectOverlayVisible(
      page,
      "components-ui-dialog--default",
      async storyPage => {
        await storyPage.getByRole("button", { name: /rename task/i }).click();
      },
      "[data-slot='dialog-content']"
    );

    await expectOverlayVisible(
      page,
      "components-ui-tooltip--default",
      async storyPage => {
        await storyPage.getByRole("button", { name: /hover me/i }).hover();
      },
      "[data-slot='tooltip-content']"
    );
  } finally {
    await stopStorybook(storybook, output);
  }
});

async function expectOverlayVisible(
  page: Page,
  storyId: string,
  openOverlay: (storyPage: Page) => Promise<void>,
  selector: string
): Promise<void> {
  await page.goto(`${storybookBaseURL}/iframe.html?id=${storyId}&viewMode=story`, {
    waitUntil: "domcontentloaded",
  });
  await expect(page.locator("#storybook-root")).toBeVisible();

  await openOverlay(page);

  const overlay = page.locator(selector);
  await expect(overlay).toBeVisible();
  await expect.poll(() => overlay.evaluate(element => getComputedStyle(element).opacity)).toBe("1");
}

async function waitForStorybook(
  baseURL: string,
  storybook: ChildProcessWithoutNullStreams
): Promise<void> {
  const deadline = Date.now() + 60_000;
  let lastError = "Storybook did not respond.";

  while (Date.now() < deadline) {
    if (storybook.exitCode !== null) {
      throw new Error(`Storybook exited early with code ${storybook.exitCode}.`);
    }

    try {
      const response = await fetch(`${baseURL}/iframe.html`);
      if (response.ok) {
        return;
      }
      lastError = `Unexpected HTTP status ${response.status}.`;
    } catch (error) {
      lastError = error instanceof Error ? error.message : String(error);
    }

    await delay(500);
  }

  throw new Error(`Timed out waiting for Storybook to start: ${lastError}`);
}

async function stopStorybook(
  storybook: ChildProcessWithoutNullStreams,
  output: string[]
): Promise<void> {
  if (storybook.exitCode !== null) return;

  storybook.kill("SIGTERM");
  const deadline = Date.now() + 10_000;
  while (Date.now() < deadline) {
    if (storybook.exitCode !== null) return;
    await delay(100);
  }

  storybook.kill("SIGKILL");
  throw new Error(`Storybook did not stop cleanly. Output:\n${output.join("")}`);
}
