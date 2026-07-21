import { test, expect } from "@playwright/test";

import {
  loadDaemonUIEnvironment,
  PLAYWRIGHT_ARCHIVE_WORKFLOW_SLUG,
  PLAYWRIGHT_NESTED_WORKFLOW_SLUG,
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

  test("navigates an initiative hierarchy without exposing task group slugs as routes", async ({
    page,
  }) => {
    // CONTRACT: E2E-004.
    const env = await loadDaemonUIEnvironment();

    await page.goto(`${env.baseUrl}/workflows`);
    await expect(page.getByTestId(`workflow-row-${PLAYWRIGHT_NESTED_WORKFLOW_SLUG}`)).toBeVisible();
    await expect(
      page.getByTestId(`workflow-task-groups-${PLAYWRIGHT_NESTED_WORKFLOW_SLUG}`)
    ).toBeVisible();
    await expect(page.getByTestId("workflow-row-nested-fixture/TG-001")).toHaveCount(0);

    await page
      .getByTestId(`workflow-task-group-tasks-${PLAYWRIGHT_NESTED_WORKFLOW_SLUG}-TG-002`)
      .click();
    await expect(page).toHaveURL(
      new RegExp(`/workflows/${PLAYWRIGHT_NESTED_WORKFLOW_SLUG}/tasks\\?task_group_id=TG-002`)
    );
    await expect(page.getByTestId("task-board-view")).toContainText(
      `${PLAYWRIGHT_NESTED_WORKFLOW_SLUG}/TG-002`
    );

    await page.locator("[data-testid^='task-board-link-']").first().click();
    await expect(page).toHaveURL(/\/tasks\/task_001\?task_group_id=TG-002/);

    await page.goto(
      `${env.baseUrl}/workflows/${PLAYWRIGHT_NESTED_WORKFLOW_SLUG}/spec?task_group_id=TG-002`
    );
    await expect(page.getByTestId("workflow-spec-tab-task-group")).toBeVisible();
    await expect(page.getByTestId("workflow-spec-task-group-body")).toContainText(
      "TG-002 — Task Group 002"
    );
    await expect(page.getByTestId("workflow-spec-task-group-body")).not.toContainText(
      "TG-001 — Task Group 001"
    );
  });

  test("filters a large task group collection and selects it from the keyboard", async ({
    page,
  }) => {
    // CONTRACT: E2E-006.
    const env = await loadDaemonUIEnvironment();
    await page.goto(`${env.baseUrl}/workflows`);

    const filter = page.getByTestId(
      `workflow-task-groups-filter-${PLAYWRIGHT_NESTED_WORKFLOW_SLUG}`
    );
    await filter.focus();
    await page.keyboard.type("TG-100");
    await expect(
      page.getByTestId(`workflow-task-group-${PLAYWRIGHT_NESTED_WORKFLOW_SLUG}-TG-100`)
    ).toBeVisible();
    await expect(
      page.getByTestId(`workflow-task-group-${PLAYWRIGHT_NESTED_WORKFLOW_SLUG}-TG-001`)
    ).toHaveCount(0);

    const selection = page.getByRole("link", {
      name: /TG-100, Task Group 100.*lifecycle incomplete.*0 unmet dependencies/i,
    });
    await selection.focus();
    await page.keyboard.press("Enter");
    await expect(page).toHaveURL(
      new RegExp(`/workflows/${PLAYWRIGHT_NESTED_WORKFLOW_SLUG}/tasks\\?task_group_id=TG-100`)
    );
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
