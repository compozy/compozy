import { test, expect } from "@playwright/test";

import {
  loadDaemonUIEnvironment,
  PLAYWRIGHT_ARCHIVE_WORKFLOW_SLUG,
  PLAYWRIGHT_START_WORKFLOW_SLUG,
} from "./support/daemon-fixture";

test.describe.serial("daemon-served web UI smoke flows", () => {
  test("loads the embedded dashboard and drills into workflows and tasks", async ({ page }) => {
    const env = await loadDaemonUIEnvironment();

    await page.goto(`${env.baseUrl}/`);

    await expect(page.getByTestId("dashboard-view")).toBeVisible();
    await expect(page.getByTestId("app-shell-active-workspace-name")).toHaveText(env.workspaceName);

    await page.getByTestId("dashboard-view-all-workflows").click();
    await expect(page.getByTestId("workflow-inventory-view")).toBeVisible();

    await page.getByTestId("workflow-sync-daemon-web-ui").click();
    await expect(page.getByTestId("workflow-inventory-action-success")).toContainText(
      "Synced daemon-web-ui"
    );

    await page.getByTestId("workflow-view-board-daemon-web-ui").click();
    await expect(page.getByTestId("task-board-view")).toBeVisible();

    await page.locator("[data-testid^='task-board-link-']").first().click();
    await expect(page.getByTestId("task-detail-view")).toBeVisible();
  });

  test("serves deep-linked spec and memory routes through the daemon HTTP listener", async ({
    page,
  }) => {
    const env = await loadDaemonUIEnvironment();

    await page.goto(`${env.baseUrl}/workflows/daemon-web-ui/spec`);
    await expect(page.getByTestId("workflow-spec-view")).toBeVisible();
    await page.getByTestId("workflow-spec-tab-techspec").click();
    await expect(page.getByTestId("workflow-spec-techspec-body")).toContainText("Testing Approach");

    await page.goto(`${env.baseUrl}/memory/daemon-web-ui`);
    await expect(page.getByTestId("workflow-memory-view")).toBeVisible();
    await expect(page.getByTestId("workflow-memory-document-body")).toContainText(
      "Workflow Memory"
    );
  });

  test("renders reviews and runs from daemon-seeded data", async ({ page }) => {
    const env = await loadDaemonUIEnvironment();

    await page.goto(`${env.baseUrl}/reviews`);
    await expect(page.getByTestId("reviews-index-view")).toBeVisible();

    await page.getByTestId("reviews-index-round-link-daemon").click();
    await expect(page.getByTestId("review-round-detail-view")).toBeVisible();

    await page.locator("[data-testid^='review-round-issue-link-daemon-']").first().click();
    await expect(page.getByTestId("review-detail-view")).toBeVisible();

    await page.getByTestId(`review-detail-run-link-${env.seededReviewRunId}`).click();
    await expect(page.getByTestId("run-detail-view")).toBeVisible();
    await expect(page.getByTestId("run-detail-stream-status")).toContainText("stream");

    await page.goto(`${env.baseUrl}/runs`);
    await expect(page.getByTestId("runs-list-view")).toBeVisible();
    await expect(page.getByTestId(`runs-list-link-${env.seededTaskRunId}`)).toBeVisible();
  });

  test("archives a workflow through the daemon API surface", async ({ page }) => {
    const env = await loadDaemonUIEnvironment();

    await page.goto(`${env.baseUrl}/workflows`);
    await expect(page.getByTestId("workflow-inventory-view")).toBeVisible();

    await page.getByTestId(`workflow-sync-${PLAYWRIGHT_ARCHIVE_WORKFLOW_SLUG}`).click();
    await expect(page.getByTestId("workflow-inventory-action-success")).toContainText(
      `Synced ${PLAYWRIGHT_ARCHIVE_WORKFLOW_SLUG}`
    );

    await page.getByTestId(`workflow-archive-${PLAYWRIGHT_ARCHIVE_WORKFLOW_SLUG}`).click();
    await expect(page.getByTestId("workflow-inventory-action-success")).toContainText(
      `Archived ${PLAYWRIGHT_ARCHIVE_WORKFLOW_SLUG}`
    );
    await expect(page.getByTestId("workflow-inventory-archived")).toBeVisible();
    await expect(
      page.getByTestId(`workflow-archived-${PLAYWRIGHT_ARCHIVE_WORKFLOW_SLUG}`)
    ).toBeVisible();
  });

  test("starts a workflow run from the workflow inventory", async ({ page }) => {
    const env = await loadDaemonUIEnvironment();

    await page.goto(`${env.baseUrl}/workflows`);
    await expect(page.getByTestId("workflow-inventory-view")).toBeVisible();

    const startResponse = page.waitForResponse(
      response =>
        response.request().method() === "POST" &&
        response.url().endsWith(`/api/tasks/${PLAYWRIGHT_START_WORKFLOW_SLUG}/runs`)
    );

    await page.getByTestId(`workflow-start-${PLAYWRIGHT_START_WORKFLOW_SLUG}`).click();
    await expect((await startResponse).status()).toBe(201);
    await expect(page.getByTestId("workflow-inventory-start-success")).toContainText("Started run");
    await expect(page.getByTestId("workflow-inventory-start-success-link")).toBeVisible();
  });
});
