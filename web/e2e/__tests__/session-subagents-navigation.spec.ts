import { randomUUID } from "node:crypto";
import path from "node:path";
import { fileURLToPath } from "node:url";

import { sessionWindow } from "../fixtures/os-navigation";
import { type BrowserRuntime, waitForSeedSessionActive } from "../fixtures/runtime";
import { expect, test } from "../fixtures/test";
import { ensureProjectWorkspace } from "../fixtures/workspace";

/**
 * E2E-009 (ADR-005, `_uiux.md` S9–S10): a parent that delegated ten subagents
 * keeps one sidebar row with a folded subagent branch, its inspector lists the roster, opening a
 * subagent reveals only that row under the parent, and plain spawned children
 * keep their nesting. Runtime truth comes from acpmock agents: the parent turn
 * calls `compozy__subagent_delegate` ten times through its hosted MCP server
 * (`native_tool_call`, like a real agent), and each child works for 30 s and
 * then completes.
 */

const fixturePath = path.resolve(
  path.dirname(fileURLToPath(import.meta.url)),
  "../../../internal/testutil/acpmock/testdata/browser_subagents_navigation_fixture.json"
);
const parentAgent = "subagent-nav-parent";
const childAgent = "subagent-nav-child";

interface SessionPayload {
  id: string;
  workspace_id: string;
}

interface SubagentRow {
  subagent_id: string;
  child_session_id: string | null;
  title: string;
  status: string;
}

test.use({
  runtimeOptions: {
    seed: {
      mockAgents: [
        { fixturePath, fixtureAgent: parentAgent },
        { fixturePath, fixtureAgent: childAgent },
      ],
    },
  },
});

function sessionAPIPath(workspaceID: string, sessionID: string, suffix = ""): string {
  return `/api/workspaces/${encodeURIComponent(workspaceID)}/sessions/${encodeURIComponent(
    sessionID
  )}${suffix}`;
}

async function createSession(
  runtime: BrowserRuntime,
  workspaceID: string,
  agentName: string,
  parentSessionID?: string
): Promise<SessionPayload> {
  const { session } = await runtime.requestJSON<{ session: SessionPayload }>("/api/sessions", {
    method: "POST",
    body: JSON.stringify({
      agent_name: agentName,
      workspace: workspaceID,
      ...(parentSessionID ? { parent_session_id: parentSessionID } : {}),
    }),
  });
  await waitForSeedSessionActive(runtime, session.id);
  return session;
}

async function promptSession(
  runtime: BrowserRuntime,
  session: SessionPayload,
  message: string
): Promise<void> {
  const response = await fetch(
    runtime.url(sessionAPIPath(session.workspace_id, session.id, "/prompt")),
    {
      headers: { "content-type": "application/json" },
      method: "POST",
      body: JSON.stringify({ idempotency_key: randomUUID(), message, message_id: randomUUID() }),
    }
  );
  if (!response.ok) {
    throw new Error(`prompt failed with ${response.status}: ${await response.text()}`);
  }
  await response.text();
}

async function listSubagents(runtime: BrowserRuntime, parent: SessionPayload) {
  const { subagents } = await runtime.requestJSON<{ subagents: SubagentRow[] }>(
    `${sessionAPIPath(parent.workspace_id, parent.id, "/subagents")}?limit=200`
  );
  return subagents;
}

test("E2E-009: ten subagents stay one sidebar row with a bounded branch and an inspector roster", async ({
  appPage,
  browserArtifacts,
  runtime,
}) => {
  test.setTimeout(180_000);
  await appPage.setViewportSize({ width: 1600, height: 1000 });
  await ensureProjectWorkspace(appPage, runtime);
  const workspace =
    runtime.seeded.workspace ??
    (runtime.paths?.workspaceDir
      ? await runtime.resolveWorkspace(runtime.paths.workspaceDir)
      : null);
  if (!workspace) throw new Error("Subagent navigation walk requires a seeded workspace");

  const other = await createSession(runtime, workspace.id, childAgent);
  const plainChild = await createSession(runtime, workspace.id, childAgent, other.id);
  const parent = await createSession(runtime, workspace.id, parentAgent);
  await promptSession(runtime, parent, "delegate ten subagents");

  const subagents = await listSubagents(runtime, parent);
  expect(subagents).toHaveLength(10);
  const childSessionIDs = subagents.map(row => row.child_session_id).filter(id => id !== null);
  expect(childSessionIDs).toHaveLength(10);

  await appPage.goto(runtime.url(`/agents/${parentAgent}/sessions/${parent.id}`), {
    waitUntil: "domcontentloaded",
  });
  const parentWin = sessionWindow(appPage, parent.id);
  const sidebar = parentWin.getByTestId("session-sidebar");
  await expect(sidebar).toBeVisible();

  await test.step("one parent row carries a folded 10/10 indicator and no subagent rows", async () => {
    const parentRow = sidebar.getByTestId(`session-sidebar-session-${parent.id}`);
    await expect(parentRow).toBeVisible();
    // The parent turn ended while its subagents work: the daemon badges it waiting on
    // sessions, not done (Sessions Kanban Business Rule 10, ADR-003).
    await expect(
      parentRow.locator('[data-badge="waiting"][data-waiting-on="sessions"]')
    ).toBeVisible();
    const indicator = sidebar.getByRole("button", { name: "10 of 10 subagents running" });
    await expect(indicator).toHaveText("10/10");
    await expect(indicator).toHaveAttribute("aria-expanded", "false");
    for (const id of childSessionIDs) {
      await expect(sidebar.getByTestId(`session-sidebar-session-${id}`)).toHaveCount(0);
    }
    // Plain spawned children keep their nesting.
    await expect(
      sidebar
        .getByTestId(`session-sidebar-thread-${other.id}`)
        .getByTestId(`session-sidebar-session-${plainChild.id}`)
    ).toBeVisible();
    await browserArtifacts.captureScreenshot("subagents-sidebar-indicator", appPage);
  });

  await test.step("unfolding the branch shows eight subagents, then Show 2 more", async () => {
    await sidebar.getByRole("button", { name: "10 of 10 subagents running" }).click();
    const branch = sidebar.getByTestId(`session-sidebar-subagents-${parent.id}`);
    await expect(branch.getByRole("button", { name: /^Open Nav subagent/ })).toHaveCount(8);
    await branch.getByRole("button", { name: "Show 2 more" }).click();
    await expect(branch.getByRole("button", { name: /^Open Nav subagent/ })).toHaveCount(10);
    await browserArtifacts.captureScreenshot("subagents-sidebar-branch", appPage);
    // Invariant: a listed child opens its live drawer while preserving the parent.
    // Owner: browser/daemon navigation; canonical suite: E2E-009.
    const opened = branch.getByRole("button", { name: /^Open Nav subagent/ }).first();
    const title = ((await opened.getAttribute("aria-label")) ?? "").replace(/^Open /, "");
    const target = subagents.find(row => row.title === title);
    expect(target?.child_session_id).toBeTruthy();
    await opened.click();
    const drawer = appPage.locator('[data-slot="resizable-drawer"]');
    await expect(drawer).toBeVisible();
    await expect(drawer).toBeInViewport({ ratio: 1 });
    await expect(drawer).toHaveCSS("opacity", "1");
    await expect(drawer).toContainText(title);
    await expect(parentWin).toBeVisible();
    await browserArtifacts.captureScreenshot("subagents-sidebar-open-drawer", appPage);
    const otherRow = branch
      .getByRole("button", { name: /^Open Nav subagent/, includeHidden: true })
      .nth(1);
    const otherTitle = ((await otherRow.getAttribute("aria-label")) ?? "").replace(/^Open /, "");
    await otherRow.click();
    await expect(drawer).toHaveCount(1);
    await expect(drawer.getByRole("heading")).toContainText(otherTitle);
    await appPage.keyboard.press("Escape");
    await expect(drawer).toHaveCount(0);
    await appPage.getByRole("button", { name: "Switch to light mode" }).click();
    await expect
      .poll(() =>
        opened.evaluate(
          element =>
            element.getAnimations().filter(animation => animation.playState === "running").length
        )
      )
      .toBe(0);
    await opened.click();
    await expect(drawer).toBeInViewport({ ratio: 1 });
    await browserArtifacts.captureScreenshot("subagents-sidebar-open-drawer-light", appPage);
    await appPage.keyboard.press("Escape");
    await expect(drawer).toHaveCount(0);
    await appPage.getByRole("button", { name: "Switch to dark mode" }).click();
  });

  const roster = parentWin.getByTestId("session-inspector-subagents");

  await test.step("the inspector roster lists 6 rows, then Show 4 more", async () => {
    const inspectorToggle = parentWin.getByTestId("session-inspector-toggle");
    if ((await inspectorToggle.getAttribute("aria-pressed")) !== "true")
      await inspectorToggle.click();
    await expect(roster).toBeVisible();
    await expect(roster.getByText("Subagents · 10 running")).toBeVisible();
    await expect(roster.getByRole("button", { name: /^Open Nav subagent/ })).toHaveCount(6);
    await expect(roster.getByRole("button", { name: "Show 4 more" })).toBeVisible();
    await browserArtifacts.captureScreenshot("subagents-inspector-roster", appPage);
  });

  await test.step("the inspector opens the drawer beside the unchanged parent", async () => {
    const opened = roster.getByRole("button", { name: /^Open Nav subagent/ }).first();
    await opened.click();
    const drawer = appPage.locator('[data-slot="resizable-drawer"]');
    await expect(drawer).toBeVisible();
    await expect(drawer).toBeInViewport({ ratio: 1 });
    await expect(drawer).toHaveCSS("opacity", "1");
    await expect(parentWin).toBeVisible();
    await browserArtifacts.captureScreenshot("subagents-inspector-drawer", appPage);
    await appPage.keyboard.press("Escape");
    await expect(drawer).toHaveCount(0);
    await appPage.getByRole("button", { name: "Switch to light mode" }).click();
    await expect
      .poll(() =>
        opened.evaluate(
          element =>
            element.getAnimations().filter(animation => animation.playState === "running").length
        )
      )
      .toBe(0);
    await opened.click();
    await expect(drawer).toBeInViewport({ ratio: 1 });
    await browserArtifacts.captureScreenshot("subagents-inspector-drawer-light", appPage);
    await drawer.getByRole("button", { name: "Open as session" }).click();
    const target = subagents[0]!;
    const childWin = sessionWindow(appPage, target.child_session_id!);
    await expect(childWin).toBeVisible();
    await childWin.getByTestId(`session-sidebar-session-${parent.id}`).click();
    await expect(parentWin).toBeVisible();
  });

  await test.step("the indicator settles live to a quiet total once every subagent completed", async () => {
    // No reload: the parent's catalog upserts carry the fresh summary to the open sidebar.
    const liveSidebar = parentWin.getByTestId("session-sidebar");
    const parentRow = liveSidebar.getByTestId(`session-sidebar-session-${parent.id}`);
    await expect(parentRow).toBeVisible();
    await expect(liveSidebar.getByRole("button", { name: "10 subagents" })).toHaveText("10", {
      timeout: 90_000,
    });
    expect((await listSubagents(runtime, parent)).map(row => row.status)).toEqual(
      Array.from({ length: 10 }, () => "completed")
    );
    await browserArtifacts.captureScreenshot("subagents-indicator-settled-live", appPage);
  });
});
