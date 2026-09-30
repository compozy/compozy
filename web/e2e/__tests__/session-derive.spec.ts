import { randomUUID } from "node:crypto";
import path from "node:path";
import { fileURLToPath } from "node:url";

import { sessionWindow, switchWorkspace, windowFrame } from "../fixtures/os-navigation";
import { sessionLifecycleSelectors, sessionWindowSelectors } from "../fixtures/selectors";
import type { BrowserRuntime, WorkspacePayload } from "../fixtures/runtime";
import { expect, test } from "../fixtures/test";
import { completeOnboardingIfPrompted } from "../fixtures/workspace";

// session-continue-fork: E2E-001 (continue via the web) and E2E-004 (a failed turn offers
// Continue), written by task_04; E2E-002 (fork from here via the web), written by task_06.
// Executed by task_08 against the e2e daemon fixture.
const acpmockTestdata = path.resolve(
  path.dirname(fileURLToPath(import.meta.url)),
  "../../../internal/testutil/acpmock/testdata"
);
const fixturePath = path.join(acpmockTestdata, "multi_agent_fixture.json");
const forkFixturePath = path.join(acpmockTestdata, "browser_session_fork_fixture.json");
const providerErrorFixturePath = path.join(acpmockTestdata, "provider_error_fixture.json");
const sourceAgent = "alpha";
const targetAgent = "beta";
const forkAgent = "fork-web-agent";
const handoffAgent = "handoff-agent";
// The virtualized row; assistant-ui also stamps data-message-id on the message root inside it.
const messageRow = '[data-testid="thread-message-row"]';

test.use({
  runtimeOptions: {
    seed: {
      mockAgents: [
        { fixturePath, fixtureAgent: sourceAgent },
        { fixturePath, fixtureAgent: targetAgent },
        { fixturePath: forkFixturePath, fixtureAgent: forkAgent },
        { fixturePath: providerErrorFixturePath, fixtureAgent: handoffAgent },
      ],
    },
  },
});

interface SessionPayload {
  id: string;
  name?: string;
  agent_name: string;
  workspace_id: string;
  state: string;
  lineage?: { kind?: string; parent_session_id?: string; origin_agent_name?: string } | null;
}

interface TranscriptPayload {
  max_sequence: number;
  entries: Array<{ message: { id: string; role?: string } }>;
}

function sessionAPIPath(workspaceID: string, sessionID: string, suffix = ""): string {
  return `/api/workspaces/${encodeURIComponent(workspaceID)}/sessions/${encodeURIComponent(
    sessionID
  )}${suffix}`;
}

// The prompt route answers with an event stream; drain it so the turn has settled.
async function promptSession(
  runtime: BrowserRuntime,
  workspaceID: string,
  sessionID: string,
  message: string
): Promise<void> {
  const response = await fetch(runtime.url(sessionAPIPath(workspaceID, sessionID, "/prompt")), {
    headers: { "content-type": "application/json" },
    method: "POST",
    body: JSON.stringify({ idempotency_key: randomUUID(), message, message_id: randomUUID() }),
  });
  if (!response.ok) {
    throw new Error(`prompt failed with ${response.status}: ${await response.text()}`);
  }
  expect(response.headers.get("content-type")).toContain("text/event-stream");
  await response.text();
}

async function prepareWorkspace(
  runtime: BrowserRuntime,
  page: import("@playwright/test").Page
): Promise<WorkspacePayload> {
  if (!runtime.paths?.workspaceDir) {
    throw new Error("session derive E2E requires launch-mode runtime paths.");
  }
  const workspace = await runtime.resolveWorkspace(runtime.paths.workspaceDir);
  await page.goto(runtime.url("/"), { waitUntil: "domcontentloaded" });
  await completeOnboardingIfPrompted(sessionLifecycleSelectors(page));
  await switchWorkspace(page, workspace.id, workspace.name);
  return workspace;
}

async function createPromptedSource(
  runtime: BrowserRuntime,
  workspace: WorkspacePayload,
  agentName = sourceAgent,
  message = "hello alpha"
): Promise<SessionPayload> {
  const { session } = await runtime.requestJSON<{ session: SessionPayload }>("/api/sessions", {
    method: "POST",
    body: JSON.stringify({ agent_name: agentName, workspace: workspace.id }),
  });
  await promptSession(runtime, workspace.id, session.id, message);
  await expect
    .poll(async () => {
      const transcript = await runtime.requestJSON<TranscriptPayload>(
        sessionAPIPath(workspace.id, session.id, "/transcript")
      );
      return transcript.entries.length;
    })
    .toBeGreaterThanOrEqual(2);
  return session;
}

test("E2E-001: operator continues a session with another agent in a new window", async ({
  appPage,
  browserArtifacts,
  runtime,
}) => {
  const workspace = await prepareWorkspace(runtime, appPage);
  const source = await createPromptedSource(runtime, workspace);
  const before = await runtime.requestJSON<TranscriptPayload>(
    sessionAPIPath(workspace.id, source.id, "/transcript")
  );

  await appPage.goto(runtime.url(`/agents/${sourceAgent}/sessions/${source.id}`), {
    waitUntil: "domcontentloaded",
  });
  const sourceWin = sessionWindow(appPage, source.id);
  await expect(sessionWindowSelectors(sourceWin, appPage).chatView).toBeVisible();

  await windowFrame(sourceWin).getByTestId("session-topbar-overflow").click();
  await appPage.getByTestId("continue-menu-item").click();
  const dialog = appPage.getByTestId("session-continue-dialog");
  await expect(dialog).toBeVisible();
  await expect(dialog.getByTestId("session-derive-preview")).toContainText("Carries over");
  await expect(dialog.getByTestId("session-continue-agent-select")).toContainText(targetAgent);
  await expect(dialog.getByTestId("session-derive-placement-new-window")).toBeChecked();
  await browserArtifacts.captureScreenshot("continue-dialog", appPage);

  await dialog.getByTestId("session-continue-submit").click();
  await expect(dialog).toBeHidden();

  let child: SessionPayload | undefined;
  await expect
    .poll(async () => {
      const { sessions } = await runtime.requestJSON<{ sessions: SessionPayload[] }>(
        `/api/sessions?workspace_id=${encodeURIComponent(workspace.id)}`
      );
      child = sessions.find(
        candidate =>
          candidate.lineage?.kind === "continue" &&
          candidate.lineage.parent_session_id === source.id
      );
      return child?.agent_name ?? null;
    })
    .toBe(targetAgent);

  const childWin = sessionWindow(appPage, child!.id);
  await expect(childWin).toBeVisible();
  await expect(windowFrame(childWin).getByTestId("session-origin-pill")).toHaveText(
    `Continued from ${sourceAgent}`
  );
  await expect(childWin.getByTestId("session-origin-divider")).toContainText("Continued from");
  await browserArtifacts.captureScreenshot("continue-child-window", appPage);

  // The source window is still there, unchanged.
  await expect(sourceWin).toBeVisible();
  const after = await runtime.requestJSON<TranscriptPayload>(
    sessionAPIPath(workspace.id, source.id, "/transcript")
  );
  expect(after.max_sequence).toBe(before.max_sequence);
  for (const entry of before.entries) {
    await expect(
      sourceWin.locator(`${messageRow}[data-message-id="${entry.message.id}"]`)
    ).toHaveCount(1);
  }
});

async function promptAndSettle(
  runtime: BrowserRuntime,
  workspace: WorkspacePayload,
  sessionID: string,
  message: string,
  entriesAfter: number
): Promise<void> {
  await promptSession(runtime, workspace.id, sessionID, message);
  await expect
    .poll(async () => {
      const transcript = await runtime.requestJSON<TranscriptPayload>(
        sessionAPIPath(workspace.id, sessionID, "/transcript")
      );
      return transcript.entries.length;
    })
    .toBeGreaterThanOrEqual(entriesAfter);
}

test("E2E-002: operator forks a session from a message and the source keeps every turn", async ({
  appPage,
  browserArtifacts,
  runtime,
}) => {
  const workspace = await prepareWorkspace(runtime, appPage);
  const { session: source } = await runtime.requestJSON<{ session: SessionPayload }>(
    "/api/sessions",
    { method: "POST", body: JSON.stringify({ agent_name: forkAgent, workspace: workspace.id }) }
  );
  await promptAndSettle(runtime, workspace, source.id, "First step", 2);
  await promptAndSettle(runtime, workspace, source.id, "Second step", 4);
  await promptAndSettle(runtime, workspace, source.id, "Third step", 6);
  const before = await runtime.requestJSON<TranscriptPayload>(
    sessionAPIPath(workspace.id, source.id, "/transcript")
  );
  const userMessages = before.entries.filter(entry => entry.message.role === "user");
  expect(userMessages).toHaveLength(3);
  const secondUser = userMessages[1]!.message.id;

  await appPage.goto(runtime.url(`/agents/${forkAgent}/sessions/${source.id}`), {
    waitUntil: "domcontentloaded",
  });
  const sourceWin = sessionWindow(appPage, source.id);
  await expect(sessionWindowSelectors(sourceWin, appPage).chatView).toBeVisible();

  const row = sourceWin.locator(`${messageRow}[data-message-id="${secondUser}"]`);
  await row.hover();
  const forkFromHere = row.getByTestId("user-message-fork");
  await expect(forkFromHere).toBeEnabled();
  await expect(row.getByTestId("user-message-rewind")).toBeVisible();
  await forkFromHere.click();

  const dialog = appPage.getByTestId("session-fork-dialog");
  await expect(dialog).toBeVisible();
  await expect(dialog.getByTestId("session-fork-agent")).toContainText(forkAgent);
  await expect(dialog.getByTestId("session-fork-point")).toContainText("Through");
  await expect(dialog.getByTestId("session-fork-point")).toContainText("Second step");
  await expect(dialog.getByTestId("session-derive-preview")).toContainText("Carries over");
  await expect(dialog.getByTestId("session-derive-placement-new-window")).toBeChecked();
  await browserArtifacts.captureScreenshot("fork-dialog-from-message", appPage);

  await dialog.getByTestId("session-fork-submit").click();
  await expect(dialog).toBeHidden();

  let child: (SessionPayload & { lineage?: { origin_message_id?: string } | null }) | undefined;
  await expect
    .poll(async () => {
      const { sessions } = await runtime.requestJSON<{ sessions: SessionPayload[] }>(
        `/api/sessions?workspace_id=${encodeURIComponent(workspace.id)}`
      );
      child = sessions.find(
        candidate =>
          candidate.lineage?.kind === "fork" && candidate.lineage.parent_session_id === source.id
      );
      return child?.agent_name ?? null;
    })
    .toBe(forkAgent);
  expect(child!.lineage?.origin_message_id).toBe(secondUser);

  const childWin = sessionWindow(appPage, child!.id);
  await expect(childWin).toBeVisible();
  await expect(windowFrame(childWin).getByTestId("session-origin-pill")).toContainText(
    "Forked from"
  );
  await browserArtifacts.captureScreenshot("fork-child-window", appPage);

  // The source keeps all three turns and its fences.
  await expect(sourceWin).toBeVisible();
  const after = await runtime.requestJSON<TranscriptPayload>(
    sessionAPIPath(workspace.id, source.id, "/transcript")
  );
  expect(after.max_sequence).toBe(before.max_sequence);
  expect(after.entries.map(entry => entry.message.id)).toEqual(
    before.entries.map(entry => entry.message.id)
  );
  for (const entry of before.entries) {
    await expect(
      sourceWin.locator(`${messageRow}[data-message-id="${entry.message.id}"]`)
    ).toHaveCount(1);
  }
});

// E2E-004: `handoff-agent` (provider_error_fixture.json) answers its second prompt with a
// `fail_prompt` driver_control error ("429 rate limit exceeded"), which the daemon classifies
// as rate limited and decorates with `next_action: "handoff"` on a user session.
test("E2E-004: a rate-limited turn offers Continue with this session as the source", async ({
  appPage,
  runtime,
}) => {
  const workspace = await prepareWorkspace(runtime, appPage);
  const source = await createPromptedSource(runtime, workspace, handoffAgent, "hello handoff");
  // Second prompt: scripted by the fixture to fail as a provider rate limit.
  await promptSession(runtime, workspace.id, source.id, "rate limit this turn");

  await appPage.goto(runtime.url(`/agents/${handoffAgent}/sessions/${source.id}`), {
    waitUntil: "domcontentloaded",
  });
  const sourceWin = sessionWindow(appPage, source.id);
  const marker = sourceWin.getByTestId("session-error-notice");
  await expect(marker).toHaveAttribute("data-provider-next-action", "handoff");
  await expect(marker).toContainText("is rate limited");

  const { sessions: beforeSessions } = await runtime.requestJSON<{ sessions: SessionPayload[] }>(
    `/api/sessions?workspace_id=${encodeURIComponent(workspace.id)}`
  );
  await marker.getByTestId("provider-error-continue").click();
  const dialog = appPage.getByTestId("session-continue-dialog");
  await expect(dialog).toBeVisible();
  await expect(dialog.getByTestId("session-continue-source-note")).toContainText(handoffAgent);

  // Nothing is created until Continue.
  const { sessions: afterSessions } = await runtime.requestJSON<{ sessions: SessionPayload[] }>(
    `/api/sessions?workspace_id=${encodeURIComponent(workspace.id)}`
  );
  expect(afterSessions).toHaveLength(beforeSessions.length);
});
