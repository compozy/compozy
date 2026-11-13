import path from "node:path";
import { fileURLToPath } from "node:url";

import type { Page } from "@playwright/test";

import { sessionWindow, windowFrame } from "../fixtures/os-navigation";
import type { BrowserRuntime, WorkspacePayload } from "../fixtures/runtime";
import { seedSessionPullRequest } from "../fixtures/session-pull-request";
import { expect, test } from "../fixtures/test";
import { ensureProjectWorkspace } from "../fixtures/workspace";

// Invariant: a session's window topbar shows the PR the daemon linked to its branch in place
// of the agent/provider/time chip, and the chip menu's primary action follows the PR state —
// failing checks send one normal prompt naming the PR to this session (E2E-001, E2E-003).
// Owner: public session window + daemon session reads. PR facts come from the store seeder
// (the display cache, observed now); mapping rules are owned by the Vitest suites.

const agentName = "browser-lifecycle-agent";
const fixturePath = path.resolve(
  path.dirname(fileURLToPath(import.meta.url)),
  "../../../internal/testutil/acpmock/testdata/browser_session_lifecycle_fixture.json"
);

test.use({ runtimeOptions: { seed: { mockAgents: [{ fixturePath, fixtureAgent: agentName }] } } });

async function resolveWorkspace(runtime: BrowserRuntime): Promise<WorkspacePayload> {
  const workspace =
    runtime.seeded.workspace ??
    (runtime.paths?.workspaceDir
      ? await runtime.resolveWorkspace(runtime.paths.workspaceDir)
      : null);
  if (!workspace) throw new Error("PR chip walk requires a seeded workspace");
  return workspace;
}

/** A real session the daemon can read by id; the walk then seeds its branch and PR. */
async function createSession(page: Page, runtime: BrowserRuntime, workspaceId: string) {
  await page.goto(runtime.url("/"), { waitUntil: "domcontentloaded" });
  await ensureProjectWorkspace(page, runtime);
  const created = await runtime.requestJSON<{ session: { id: string } }>("/api/sessions", {
    method: "POST",
    body: JSON.stringify({ agent_name: agentName, workspace: workspaceId }),
  });
  return created.session.id;
}

async function openSession(page: Page, runtime: BrowserRuntime, sessionId: string) {
  await page.goto(runtime.url(`/agents/${agentName}/sessions/${sessionId}`));
  const win = sessionWindow(page, sessionId);
  await expect(win).toBeVisible();
  return windowFrame(win);
}

test("E2E-001: the topbar shows the linked PR instead of the agent chip", async ({
  appPage,
  browserArtifacts,
  runtime,
}) => {
  const workspace = await resolveWorkspace(runtime);
  const sessionId = await createSession(appPage, runtime, workspace.id);
  await seedSessionPullRequest(runtime, {
    workspaceId: workspace.id,
    sessionId,
    branch: "feat/kanban",
    number: 931,
    checks: "passing",
    review: "approved",
  });
  const frame = await openSession(appPage, runtime, sessionId);

  const chip = frame.getByTestId("session-pull-request-chip");
  await expect(chip).toHaveAccessibleName("Pull request #931, Open");
  await expect(chip).toContainText("#931");
  await expect(chip).toContainText("Open");
  // Every Business Rule 9 fact is known and fresh, so one Ready signal replaces both.
  await expect(chip.locator('[data-signal="ready"]')).toHaveText("Ready to merge");
  await expect(frame.getByTestId("session-status-agent-chip")).toHaveCount(0);
  await expect(frame.getByTestId("session-status-provider")).toHaveCount(0);

  // The breakdown lives in the tooltip.
  await chip.hover();
  const tooltip = appPage.getByTestId("session-pull-request-tooltip");
  await expect(tooltip).toContainText("All 14 checks passed");
  await expect(tooltip).toContainText("Approved");
  await browserArtifacts.captureScreenshot("session-pr-chip-ready", appPage);
});

test("E2E-003: failing checks lead the chip menu and send one prompt naming the PR", async ({
  appPage,
  browserArtifacts,
  runtime,
}) => {
  const workspace = await resolveWorkspace(runtime);
  const sessionId = await createSession(appPage, runtime, workspace.id);
  await seedSessionPullRequest(runtime, {
    workspaceId: workspace.id,
    sessionId,
    branch: "feat/forge-gql",
    number: 932,
    checks: "failing",
    review: "review_required",
  });
  const frame = await openSession(appPage, runtime, sessionId);

  const chip = frame.getByTestId("session-pull-request-chip");
  await expect(chip.locator('[data-signal="checks"]')).toHaveText("2 failing");
  await chip.click();
  const menu = appPage.getByTestId("session-pull-request-menu");
  const items = menu.getByRole("menuitem");
  await expect(items.first()).toHaveAttribute("data-testid", "session-pull-request-action-fix");
  await expect(items.first()).toHaveText("Fix failing checks");
  await browserArtifacts.captureScreenshot("session-pr-chip-fix-menu", appPage);

  await items.first().click();
  const message = frame.getByTestId("user-message-bubble").filter({
    hasText: /The checks on pull request #932/,
  });
  await expect(message).toHaveCount(1);
  await expect(message).toBeVisible({ timeout: 30_000 });
  await expect(message).toContainText("lint");
  await expect(frame.getByText("I will inspect the failing PR checks.")).toBeVisible();
});

// Invariant: a session shows its linked work in the head: several items as one count, the list one
// step away at every width, and an unlink that touches nothing else and can be undone.
// Owner: public session window + daemon link resources. The second item is added through the same
// route the link dialog uses; list ordering and copy are owned by the Vitest suites.
test("E2E-006: the head lists linked work and an unlink can be undone", async ({
  appPage,
  browserArtifacts,
  runtime,
}) => {
  const workspace = await resolveWorkspace(runtime);
  const sessionId = await createSession(appPage, runtime, workspace.id);
  await seedSessionPullRequest(runtime, {
    workspaceId: workspace.id,
    sessionId,
    branch: "feat/linked-work",
    number: 940,
    checks: "failing",
    review: "review_required",
  });
  await runtime.requestJSON(`/api/workspaces/${workspace.id}/sessions/${sessionId}/links`, {
    method: "POST",
    body: JSON.stringify({ branch: "spike/second-item" }),
  });
  const frame = await openSession(appPage, runtime, sessionId);

  const summary = frame.getByTestId("session-links-summary");
  await expect(summary).toHaveText("2 linked");
  await expect(frame.getByTestId("session-pull-request-chip")).toHaveCount(0);
  await summary.click();
  const rows = appPage.locator('[data-slot="session-link-row"]');
  await expect(rows).toHaveCount(2);
  await expect(rows.first()).toContainText("Checks failing");
  await expect(appPage.locator('[data-slot="session-runs-in"]')).toContainText(
    "Linking never moves it."
  );
  await browserArtifacts.captureScreenshot("session-linked-work-list", appPage);

  const branchRow = rows.filter({ hasText: "spike/second-item" });
  await branchRow.hover();
  await branchRow.getByRole("button", { name: "Actions for spike/second-item" }).click();
  await appPage.getByRole("menuitem", { name: "Unlink from this session" }).click();
  await expect(rows).toHaveCount(1);
  const toast = appPage.getByText(
    "Its files are untouched. Only the link to this session is gone."
  );
  await expect(toast).toBeVisible();
  await appPage.getByRole("button", { name: "Undo" }).click();
  await expect(rows).toHaveCount(2);

  await appPage.keyboard.press("Escape");
  await appPage.setViewportSize({ width: 480, height: 800 });
  await expect(frame.getByRole("button", { name: "2 linked to this session" })).toBeVisible();
  await browserArtifacts.captureScreenshot("session-linked-work-narrow", appPage);
});
