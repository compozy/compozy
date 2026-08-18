// Suite: Web Storybook browser bootstrap
// Invariant: the real preview worker enforces local API mocks and route stories honor cold deep links
// without publishing cross-component state during render.
// Boundary IN: Storybook dev server, preview iframe, Service Worker, router, and MSW handlers.
// Boundary OUT: individual app rendering contracts owned by their component and route suites.
import { expect, test } from "@playwright/test";

import {
  spawnStorybook,
  stopStorybook,
  storyURL,
  waitForStorybook,
  waitForStoryModule,
  type StorybookServer,
} from "../fixtures/storybook-server";

const STORYBOOK_PORT = 6106;
const STORY_MODULE_PATH =
  "/src/systems/design-system/components/stories/design-system-showcase.stories.tsx";

test.setTimeout(180_000);

test("registers the MSW worker, guards local APIs, and renders route-story deep links", async ({
  page,
}) => {
  const browserConsole: string[] = [];
  let storybook: StorybookServer | null = null;

  try {
    storybook = spawnStorybook(STORYBOOK_PORT);
    await waitForStorybook(storybook);
    await waitForStoryModule(storybook, STORY_MODULE_PATH, "DesignSystemShowcase");

    page.on("console", message => {
      browserConsole.push(message.text());
    });

    await page.goto(
      storyURL(storybook.baseURL, "systems-design-system-components-designsystemshowcase--default"),
      { waitUntil: "domcontentloaded" }
    );
    await expect(page.getByTestId("design-system-showcase")).toBeVisible();
    await expect(page.getByText("CompozyOS design system")).toBeVisible();

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

    const sessionCatalogStreamResponse = page.waitForResponse(response => {
      return new URL(response.url()).pathname === "/api/sessions/catalog-stream";
    });
    await page.goto(storyURL(storybook.baseURL, "systems-loops-routes-loopruns--run-detail"), {
      waitUntil: "domcontentloaded",
    });
    const catalogResponse = await sessionCatalogStreamResponse;
    expect(catalogResponse.status()).toBe(200);
    expect(catalogResponse.headers()["content-type"]).toContain("text/event-stream");

    const loopsWindow = page.getByRole("region", { name: "Loops window" });
    await expect(loopsWindow).toBeVisible();
    await expect(
      loopsWindow.locator("xpath=ancestor::*[@data-slot='os-window-frame'][1]")
    ).toHaveAttribute("data-focused", "");
    await expect(page.getByTestId("loop-run-status-pill")).toBeVisible();
    expect(browserConsole.filter(entry => entry.includes("Cannot update a component"))).toEqual([]);
  } finally {
    if (storybook) await stopStorybook(storybook);
  }
});
