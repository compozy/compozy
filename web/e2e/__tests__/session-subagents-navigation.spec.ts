import { randomUUID } from "node:crypto";
import path from "node:path";
import { fileURLToPath } from "node:url";

import { sessionWindow } from "../fixtures/os-navigation";
import { type BrowserRuntime, waitForSeedSessionActive } from "../fixtures/runtime";
import { expect, test } from "../fixtures/test";
import { ensureProjectWorkspace } from "../fixtures/workspace";

/**
 * E2E-009 (ADR-005, `_uiux.md` S9–S10): a parent that delegated ten subagents
 * keeps one sidebar row with a chip, its inspector lists the roster, opening a
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

test("E2E-009: ten subagents stay one sidebar row with a chip and an inspector roster", async ({
  appPage,
  browserArtifacts,
  runtime,
}) => {
  test.setTimeout(180_000);
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

  await test.step("one parent row carries a 10/10 chip and no subagent rows", async () => {
    const parentRow = sidebar.getByTestId(`session-sidebar-session-${parent.id}`);
    await expect(parentRow).toBeVisible();
    // The parent turn ended while its subagents work: it reads delegated, not done (UT-W14).
    await expect(parentRow.locator('[data-badge="delegated"]')).toBeVisible();
    const chip = sidebar.getByRole("button", { name: "10 of 10 subagents running" });
    await expect(chip).toHaveText("10/10");
    for (const id of childSessionIDs) {
      await expect(sidebar.getByTestId(`session-sidebar-session-${id}`)).toHaveCount(0);
    }
    // Plain spawned children keep their nesting.
    await expect(
      sidebar
        .getByTestId(`session-sidebar-thread-${other.id}`)
        .getByTestId(`session-sidebar-session-${plainChild.id}`)
    ).toBeVisible();
    await browserArtifacts.captureScreenshot("subagents-sidebar-chip", appPage);
  });

  await test.step("hovering the chip previews five subagents and +5 more", async () => {
    await sidebar.getByRole("button", { name: "10 of 10 subagents running" }).hover();
    const preview = appPage.locator('[data-slot="hover-card-content"]');
    await expect(preview.getByText("+5 more")).toBeVisible();
    await expect(preview.getByText(/^Nav subagent \d+$/)).toHaveCount(5);
    await browserArtifacts.captureScreenshot("subagents-chip-preview", appPage);
    await appPage.mouse.move(0, 0);
  });

  const roster = parentWin.getByTestId("session-inspector-subagents");
  await test.step("clicking the chip opens the inspector roster: 6 rows, then Show 4 more", async () => {
    await sidebar.getByRole("button", { name: "10 of 10 subagents running" }).click();
    await expect(roster).toBeVisible();
    await expect(roster.getByText("Subagents · 10 running")).toBeVisible();
    await expect(roster.getByRole("button", { name: /^Open Nav subagent/ })).toHaveCount(6);
    await expect(roster.getByRole("button", { name: "Show 4 more" })).toBeVisible();
    await browserArtifacts.captureScreenshot("subagents-inspector-roster", appPage);
  });

  await test.step("opening a subagent reveals only it under the parent", async () => {
    const opened = roster.getByRole("button", { name: /^Open Nav subagent/ }).first();
    const title = ((await opened.getAttribute("aria-label")) ?? "").replace(/^Open /, "");
    const target = subagents.find(row => row.title === title);
    expect(target?.child_session_id).toBeTruthy();
    await opened.click();
    const childWin = sessionWindow(appPage, target!.child_session_id!);
    await expect(childWin).toBeVisible();
    const childSidebar = childWin.getByTestId("session-sidebar");
    await expect(
      childSidebar
        .getByTestId(`session-sidebar-thread-${parent.id}`)
        .getByTestId(`session-sidebar-session-${target!.child_session_id}`)
    ).toBeVisible();
    for (const id of childSessionIDs.filter(id => id !== target!.child_session_id)) {
      await expect(childSidebar.getByTestId(`session-sidebar-session-${id}`)).toHaveCount(0);
    }
    await browserArtifacts.captureScreenshot("subagents-revealed-child", appPage);
  });

  await test.step("the chip hides once every subagent settled cleanly", async () => {
    await expect
      .poll(async () => (await listSubagents(runtime, parent)).map(row => row.status), {
        timeout: 90_000,
      })
      .toEqual(Array.from({ length: 10 }, () => "completed"));
    await appPage.goto(runtime.url(`/agents/${parentAgent}/sessions/${parent.id}`), {
      waitUntil: "domcontentloaded",
    });
    const settledSidebar = sessionWindow(appPage, parent.id).getByTestId("session-sidebar");
    await expect(settledSidebar.getByTestId(`session-sidebar-session-${parent.id}`)).toBeVisible();
    await expect(settledSidebar.locator('[data-slot="subagent-chip"]')).toHaveCount(0);
  });
});
