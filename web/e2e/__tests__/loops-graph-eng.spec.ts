import { expect, test, type Page } from "@playwright/test";

import {
  spawnStorybook,
  stopStorybook,
  storyURL,
  waitForStorybook,
  waitForStoryModule,
  type StorybookServer,
} from "../fixtures/storybook-server";

const STORYBOOK_PORT = 6108;
const STORY_MODULE_PATH = "/src/systems/loops/components/stories/loop-run-page.stories.tsx";
const RUN_PAGE = "systems-loops-components-looprunpage";
const RUN_ROUTES = "systems-loops-routes-loopruns";
const LOOP_EDITOR = "systems-loops-components-loopeditor";

let storybook: StorybookServer;

test.beforeAll(async () => {
  test.setTimeout(240_000);
  storybook = spawnStorybook(STORYBOOK_PORT);
  await waitForStorybook(storybook);
  await waitForStoryModule(storybook, STORY_MODULE_PATH, "PendingRequests");
});

test.afterAll(async () => {
  if (storybook) await stopStorybook(storybook);
});

test.setTimeout(120_000);

async function openStory(page: Page, storyId: string): Promise<void> {
  await page.goto(storyURL(storybook.baseURL, storyId), { waitUntil: "domcontentloaded" });
}

test.describe("Human requests on the run page", () => {
  test("E2E-020: an ask renders its schema form and reports the daemon's field errors", async ({
    page,
  }) => {
    test.setTimeout(180_000);
    await openStory(page, `${RUN_PAGE}--pending-requests`);

    const cards = page.getByTestId("loop-request-card");
    await expect(cards).toHaveCount(1);
    await expect(page.getByTestId("loop-request-progress")).toHaveText("Question 1 of 2");
    await expect(page.getByTestId("loop-request-prompt")).toContainText(
      "Which regions ship first?"
    );
    await page.getByTestId("loop-request-details").click();
    await expect(page.getByTestId("loop-request-context")).toBeVisible();
    await expect(page.getByTestId("loop-request-context-fetch")).toBeVisible();

    const askCard = cards.first();
    await expect(askCard.getByTestId("loop-request-submit")).toBeEnabled();

    await openStory(page, `${RUN_PAGE}--request-validation-failure`);
    await expect(page.getByTestId("loop-request-field-error-regions")).toContainText(
      "At least one region is required."
    );
    await expect(page.getByTestId("loop-request-submit").first()).toBeEnabled();
  });

  test("E2E-021: a review shows proposed args and only the persisted decisions", async ({
    page,
  }) => {
    await openStory(page, `${RUN_PAGE}--pending-requests`);

    await page.getByTestId("loop-request-next").click();
    await expect(page.getByTestId("loop-request-progress")).toHaveText("Question 2 of 2");
    const reviewCard = page.getByTestId("loop-request-card");
    await expect(reviewCard.getByTestId("loop-review-proposed-args")).toBeVisible();

    for (const decision of ["approve", "edit", "reject", "respond"]) {
      await expect(reviewCard.getByTestId(`loop-request-decision-${decision}`)).toBeVisible();
    }
    await expect(reviewCard.getByTestId("loop-request-decision-escalate")).toHaveCount(0);

    await reviewCard.getByTestId("loop-request-decision-edit").click();
    await expect(reviewCard.getByTestId("loop-review-proposed-args")).toBeVisible();
  });

  test("E2E-022: resolved and refused requests render outcomes, never a form", async ({ page }) => {
    await openStory(page, `${RUN_PAGE}--resolved-requests`);

    const resolutions = page.getByTestId("loop-request-resolution");
    await expect(resolutions.first()).toBeVisible();
    await expect(page.getByTestId("loop-request-submit")).toHaveCount(0);

    await openStory(page, `${RUN_PAGE}--request-already-answered`);
    await expect(page.getByTestId("loop-request-refusal")).toContainText("already answered");

    await openStory(page, `${RUN_PAGE}--request-answer-pending`);
    await expect(page.getByTestId("loop-request-submit").first()).toBeDisabled();

    await openStory(page, `${RUN_ROUTES}--run-requests`);
    const card = page.getByTestId("loop-request-card").first();
    await expect(card).toBeVisible();
    await card.getByTestId("loop-request-field-regions").fill('["us-east"]');
    await card.getByTestId("loop-request-option-canary-true").click();
    const submit = card.getByTestId("loop-request-submit");
    await expect(submit).toBeEnabled();
    const responsePromise = page.waitForResponse(
      response =>
        response.request().method() === "POST" &&
        new URL(response.url()).pathname.endsWith("/respond")
    );
    await submit.click();
    const response = await responsePromise;
    expect(response.status()).toBe(200);
    expect(await response.json()).toMatchObject({ state: "answered" });
    await expect(card.getByTestId("loop-request-resolution")).toHaveCount(0);
    await expect(submit).toBeEnabled();
  });
});

test("E2E-024: the timeline renders the new graph-completion row families", async ({ page }) => {
  await openStory(page, `${RUN_PAGE}--pending-requests`);

  const story = page.getByTestId("loop-run-story");
  await expect(story).toBeVisible();
  const rows = story.getByTestId(/^loop-run-beat-/);
  await expect(rows.first()).toBeVisible();

  const timeline = await story.innerText();
  for (const fragment of ["standard", "Which regions ship first?", "render-notes"]) {
    expect(timeline).toContain(fragment);
  }
});

test("E2E-029: editor grammar round-trips and reports a missing route default", async ({
  page,
}) => {
  await openStory(page, `${LOOP_EDITOR}--chrome-calm-default`);
  await expect(page.getByTestId("loop-editor")).toBeVisible();
  await page.getByTestId("loop-editor-palette-toggle").click();

  await page.getByTestId("loop-palette-item-route").click();
  await page.getByTestId("loop-field-id").fill("decision_route");

  await page.getByTestId("loop-palette-item-ask").click();
  await page.getByTestId("loop-field-id").fill("release_approval");
  await page.getByTestId("loop-field-ask_prompt").fill("Approve the release regions?");
  await page
    .getByTestId("loop-field-ask_expect")
    .fill('{"type":"object","required":["approved"],"properties":{"approved":{"type":"boolean"}}}');

  const route = page.locator('[data-testid="loop-editor-node"][data-node-id="decision_route"]');
  await route.click();
  await page.getByTestId("loop-route-default").selectOption("release_approval");

  await page.locator('[data-testid="loop-editor-node"][data-node-id="rollout"]').click();
  await page.getByTestId("loop-field-bind_as").fill("artifact");
  await page.getByTestId("loop-strategy-kind-race").click();
  await page.getByTestId("loop-strategy-kind-best_effort").click();
  await page.getByTestId("loop-strategy-threshold").fill("75%");

  await page.getByTestId("loop-editor-view-dsl").click();
  const dsl = page.getByTestId("loop-editor-dsl");
  await expect(dsl).toBeVisible();
  const text = await dsl.innerText();
  for (const fragment of [
    "decision_route",
    "release_approval",
    "Approve the release regions?",
    "best_effort",
    "75%",
    "bind_as: artifact",
  ]) {
    expect(text).toContain(fragment);
  }

  await page.getByTestId("loop-editor-view-graph").click();
  const graph = page.getByTestId("loop-editor-canvas");
  await expect(graph).toBeVisible();
  const graphRoute = graph.locator(
    '[data-testid="loop-editor-node"][data-node-id="decision_route"]'
  );
  await expect(graphRoute).toBeVisible();
  await graphRoute.click();
  await expect(page.getByTestId("loop-route-default")).toBeVisible();
  await expect(page.getByTestId("loop-linter-error-count")).toHaveCount(0);
  await page.getByTestId("loop-route-default").selectOption("");
  await expect(page.getByTestId("loop-linter-error-count")).toBeVisible();
  await page.getByTestId("loop-linter-toggle").click();
  await expect(
    page.getByTestId("loop-linter-issue").getByTitle("route_default_missing")
  ).toBeVisible();
});

test("E2E-030: loop requests compose into the bell count and jump to their form", async ({
  page,
}) => {
  await openStory(page, `${RUN_ROUTES}--attention-requests`);

  const bell = page.getByRole("button", { name: "Attention, 4 waiting" });
  await expect(bell).toBeVisible();
  await bell.click();

  const requestRow = page.getByTestId(
    "os-attention-loop-request-ws_launch_hq:looprun_release_train:confirm-rollout:0"
  );
  await expect(requestRow).toContainText("launch-hq");
  await expect(requestRow).toContainText("release-train — ask");
  await requestRow.click();
  await expect(page.getByTestId("os-bell-popover")).toHaveCount(0);

  await openStory(page, `${RUN_ROUTES}--attention-request-target`);
  await expect(page.getByTestId("loop-run-detail-content")).toBeVisible();
  await expect(page.getByTestId("loop-request-field-regions")).toBeFocused();
});
