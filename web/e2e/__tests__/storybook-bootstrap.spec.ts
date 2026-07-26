import { spawn, type ChildProcessWithoutNullStreams } from "node:child_process";
import { once } from "node:events";
import { setTimeout as delay } from "node:timers/promises";
import path from "node:path";
import { fileURLToPath } from "node:url";

import { expect, test } from "@playwright/test";

const storybookHost = "127.0.0.1";
const storybookPort = 6106;
const storybookBaseURL = `http://${storybookHost}:${storybookPort}`;
const storyURL = `${storybookBaseURL}/iframe.html?id=systems-design-system-components-designsystemshowcase--default&viewMode=story`;
const storyModulePath =
  "/src/systems/design-system/components/stories/design-system-showcase.stories.tsx";
const currentDir = path.dirname(fileURLToPath(import.meta.url));

test.setTimeout(180_000);

test("registers the MSW worker and fails unknown local API requests in web Storybook", async ({
  page,
}) => {
  const cwd = path.resolve(currentDir, "../..");
  const output: string[] = [];
  const browserConsole: string[] = [];
  const storybook = spawn(
    "bunx",
    ["storybook", "dev", "--host", storybookHost, "--port", String(storybookPort), "--ci"],
    {
      cwd,
      env: process.env,
      stdio: "pipe",
    }
  );

  const captureOutput = (chunk: string) => {
    output.push(chunk);
  };

  storybook.stdout.on("data", chunk => {
    captureOutput(chunk.toString());
  });
  storybook.stderr.on("data", chunk => {
    captureOutput(chunk.toString());
  });

  try {
    await waitForStorybook(storybookBaseURL, storybook);
    await waitForStoryModule(storybookBaseURL, storybook);

    page.on("console", message => {
      browserConsole.push(message.text());
    });

    await page.goto(storyURL, { waitUntil: "domcontentloaded" });
    await expect(page.getByTestId("design-system-showcase")).toBeVisible();
    await expect(page.getByText("AGH design system")).toBeVisible();

    await expect
      .poll(() => browserConsole.find(entry => entry.includes("[MSW]")), {
        timeout: 15_000,
      })
      .toBeTruthy();

    const unknownRequest = await page.evaluate(async () => {
      try {
        const response = await fetch("/api/storybook-unhandled-request");
        return {
          ok: response.ok,
          rejected: false,
          status: response.status,
        };
      } catch (error) {
        return {
          message: error instanceof Error ? error.message : String(error),
          rejected: true,
        };
      }
    });

    expect(unknownRequest).toMatchObject({
      message: expect.stringMatching(/fetch|network/i),
      rejected: true,
    });
    expect(browserConsole.some(entry => entry.includes("without a matching request handler"))).toBe(
      true
    );
  } finally {
    await stopStorybook(storybook);
  }
});

async function waitForStorybook(
  baseURL: string,
  storybook: ChildProcessWithoutNullStreams
): Promise<void> {
  const deadline = Date.now() + 60_000;

  while (Date.now() < deadline) {
    if (storybook.exitCode !== null) {
      throw new Error(`Storybook exited early with code ${storybook.exitCode}.`);
    }

    try {
      const response = await fetch(`${baseURL}/iframe.html`);
      if (response.ok) {
        return;
      }
    } catch {
      // Storybook is still starting.
    }

    await delay(500);
  }

  throw new Error("Timed out waiting for Storybook to start.");
}

async function waitForStoryModule(
  baseURL: string,
  storybook: ChildProcessWithoutNullStreams
): Promise<void> {
  const deadline = Date.now() + 60_000;

  while (Date.now() < deadline) {
    if (storybook.exitCode !== null) {
      throw new Error(`Storybook exited early with code ${storybook.exitCode}.`);
    }

    try {
      const response = await fetch(`${baseURL}${storyModulePath}`);
      const body = await response.text();
      if (response.ok && body.includes("DesignSystemShowcase")) {
        return;
      }
    } catch {
      // Vite is still transforming the story module.
    }

    await delay(500);
  }

  throw new Error("Timed out waiting for the Storybook story module.");
}

async function stopStorybook(storybook: ChildProcessWithoutNullStreams): Promise<void> {
  if (storybook.exitCode !== null) {
    return;
  }

  storybook.kill("SIGTERM");
  const exited = Promise.race([once(storybook, "exit"), delay(5_000).then(() => "timeout")]);

  if ((await exited) === "timeout" && storybook.exitCode === null) {
    storybook.kill("SIGKILL");
    await once(storybook, "exit");
  }
}
