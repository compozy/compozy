import { randomUUID } from "node:crypto";
import path from "node:path";
import { fileURLToPath } from "node:url";

import { sessionWindow, switchWorkspace, windowID } from "../fixtures/os-navigation";
import { sessionLifecycleSelectors, sessionWindowSelectors } from "../fixtures/selectors";
import type { BrowserRuntime, WorkspacePayload } from "../fixtures/runtime";
import { expect, test } from "../fixtures/test";
import { completeOnboardingIfPrompted } from "../fixtures/workspace";
import { humanizeAgentSlug } from "@/systems/agent/lib/agent-identity";

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
  entries: Array<{
    message: {
      id: string;
      role?: string;
      parts?: Array<{ type: string; data?: { type?: string } }>;
    };
  }>;
}

function sessionAPIPath(workspaceID: string, sessionID: string, suffix = ""): string {
  return `/api/workspaces/${encodeURIComponent(workspaceID)}/sessions/${encodeURIComponent(
    sessionID
  )}${suffix}`;
}

async function settleInitialMCPNotification(
  runtime: BrowserRuntime,
  workspaceID: string,
  sessionID: string
): Promise<void> {
  // The watcher records its baseline independently of prompt completion and derivation.
  await expect
    .poll(async () => {
      const transcript = await runtime.requestJSON<TranscriptPayload>(
        sessionAPIPath(workspaceID, sessionID, "/transcript")
      );
      return transcript.entries.some(entry =>
        entry.message.parts?.some(
          part => part.type === "data-compozy-event" && part.data?.type === "session.mcp_changed"
        )
      );
    })
    .toBe(true);
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
  await settleInitialMCPNotification(runtime, workspace.id, session.id);
  return session;
}

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

// Invariant: fork/continue preserve source transcripts and provider failures offer the source.
// Owner: this browser suite; fork runs first to preserve its cold half-width layout.
test("E2E-002 / E2E-001 / E2E-004: operator forks, continues, and recovers a rate-limited session", async ({
  appPage,
  browserArtifacts,
  runtime,
}) => {
  await test.step("E2E-002: operator forks a session from a message and the source keeps every turn", async () => {
    const workspace = await prepareWorkspace(runtime, appPage);
    const { session: source } = await runtime.requestJSON<{ session: SessionPayload }>(
      "/api/sessions",
      { method: "POST", body: JSON.stringify({ agent_name: forkAgent, workspace: workspace.id }) }
    );
    await promptAndSettle(runtime, workspace, source.id, "First step", 2);
    await promptAndSettle(runtime, workspace, source.id, "Second step", 4);
    await promptAndSettle(runtime, workspace, source.id, "Third step", 6);
    await settleInitialMCPNotification(runtime, workspace.id, source.id);
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
    const sourceWindowID = await windowID(sourceWin);

    const row = sourceWin.locator(`${messageRow}[data-message-id="${secondUser}"]`);
    await row.hover();
    const forkFromHere = row.getByTestId("user-message-fork");
    await expect(forkFromHere).toBeEnabled();
    await expect(row.getByTestId("user-message-rewind")).toBeVisible();
    await forkFromHere.click();

    const dialog = appPage.getByTestId("session-fork-dialog");
    await expect(dialog).toBeVisible();
    await expect(dialog.getByTestId("session-fork-agent")).toContainText(
      humanizeAgentSlug(forkAgent)
    );
    await expect(dialog.getByTestId("session-derive-preview")).toContainText("Carries over");
    // Nothing to choose: no fork-point field and no Open in choice.
    await expect(dialog.getByText("Fork point")).toHaveCount(0);
    await expect(dialog.getByRole("radiogroup")).toHaveCount(0);
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

    // The child replaces its source in the same window.
    const childWin = sessionWindow(appPage, child!.id);
    await expect(childWin).toBeVisible();
    expect(await windowID(childWin)).toBe(sourceWindowID);
    await expect(sourceWin).toHaveCount(0);
    await expect(childWin.getByTestId("session-origin-pill")).toContainText("Forked from");
    // The head's meta (agent, origin pill, time) yields before the window title ever truncates.
    const childTitle = childWin.locator('[data-slot="topbar-title"] button');
    await expect(childTitle).toHaveText("New session");
    await expect
      .poll(() => childTitle.evaluate(element => element.scrollWidth <= element.clientWidth))
      .toBe(true);
    await browserArtifacts.captureScreenshot("fork-child-window", appPage);

    // The source keeps all three turns and its fences.
    const after = await runtime.requestJSON<TranscriptPayload>(
      sessionAPIPath(workspace.id, source.id, "/transcript")
    );
    expect(after.max_sequence).toBe(before.max_sequence);
    expect(after.entries.map(entry => entry.message.id)).toEqual(
      before.entries.map(entry => entry.message.id)
    );
    // The origin pill reopens the source, which still renders every turn.
    await childWin.getByTestId("session-origin-pill").click();
    await expect(sessionWindowSelectors(sourceWin, appPage).chatView).toBeVisible();
    for (const entry of before.entries) {
      await expect(
        sourceWin.locator(`${messageRow}[data-message-id="${entry.message.id}"]`)
      ).toHaveCount(1);
    }
  });

  await test.step("E2E-001: operator continues a session with another agent in a new window", async () => {
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

    // A frame can retain sibling tabs from the fork step; the session's own row menu owns Continue.
    const sourceUi = sessionWindowSelectors(sourceWin, appPage);
    await sourceUi.sessionRowActions(source.id).click();
    // Continue lives under the row menu's "New from this ▸" submenu (`derive`), mounted on open.
    await sourceUi.sessionRowMenuItem("derive", source.id).click();
    await sourceUi.sessionRowMenuItem("continue", source.id).click();
    const dialog = appPage.getByTestId("session-continue-dialog");
    await expect(dialog).toBeVisible();
    await expect(dialog.getByTestId("session-derive-preview")).toContainText("Carries over");
    await expect(dialog.getByTestId("session-continue-agent-select")).toContainText(
      humanizeAgentSlug(targetAgent)
    );
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
    await expect(childWin.getByTestId("session-origin-pill")).toHaveText("Continued from Alpha");
    await expect(childWin.getByTestId("session-origin-divider")).toContainText("Continued from");
    await browserArtifacts.captureScreenshot("continue-child-window", appPage);

    // The source window is still there, unchanged.
    await expect(sourceWin).toBeVisible();
    const after = await runtime.requestJSON<TranscriptPayload>(
      sessionAPIPath(workspace.id, source.id, "/transcript")
    );
    expect(after.max_sequence).toBe(before.max_sequence);
  });

  // The handoff fixture fails its second prompt with a 429 rate limit error.
  await test.step("E2E-004: a rate-limited turn offers Continue with this session as the source", async () => {
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
    await expect(dialog.getByTestId("session-continue-source-note")).toContainText(
      humanizeAgentSlug(handoffAgent)
    );

    // Nothing is created until Continue.
    const { sessions: afterSessions } = await runtime.requestJSON<{ sessions: SessionPayload[] }>(
      `/api/sessions?workspace_id=${encodeURIComponent(workspace.id)}`
    );
    expect(afterSessions).toHaveLength(beforeSessions.length);
  });
});

// side-chat: E2E-001 (`/side` while the main turn runs), E2E-002 (the head toggle, then the
// shortcut), E2E-003 (an idle `/side` adds nothing to the main thread). The acpmock fixture is the
// backend slice's `browser_side_chat_fixture.json`: agent `side-web-agent` answers "hello side",
// holds "keep working until stopped" open until the turn is cancelled, and answers side first
// prompts containing "what now?", "what is this repo?" and "quick side question".
const sideFixturePath = path.join(acpmockTestdata, "browser_side_chat_fixture.json");
const sideAgent = "side-web-agent";

test.describe("side chat", () => {
  test.use({
    runtimeOptions: {
      seed: { mockAgents: [{ fixturePath: sideFixturePath, fixtureAgent: sideAgent }] },
    },
  });

  async function openSideSource(runtime: BrowserRuntime, page: import("@playwright/test").Page) {
    const workspace = await prepareWorkspace(runtime, page);
    const source = await createPromptedSource(runtime, workspace, sideAgent, "hello side");
    await page.goto(runtime.url(`/agents/${sideAgent}/sessions/${source.id}`), {
      waitUntil: "domcontentloaded",
    });
    const win = sessionWindow(page, source.id);
    await expect(sessionWindowSelectors(win, page).chatView.first()).toBeVisible();
    // The side panel mounts its own composer after the main one: the main one is the first.
    const mainComposer = win.getByRole("textbox", { name: "Session prompt" }).first();
    const before = await runtime.requestJSON<TranscriptPayload>(
      sessionAPIPath(workspace.id, source.id, "/transcript")
    );
    return { workspace, source, win, mainComposer, before };
  }

  async function sideChats(runtime: BrowserRuntime, workspaceID: string, sessionID: string) {
    const { sides } = await runtime.requestJSON<{ sides: Array<{ id: string }> }>(
      sessionAPIPath(workspaceID, sessionID, "/sides")
    );
    return sides;
  }

  // Invariant: `/side` opens a side chat that answers while the main turn keeps streaming, and the
  // panel's status chip follows the main session; closing stops and archives the side chat.
  test("E2E-001: operator asks a side question while the main session works", async ({
    appPage,
    browserArtifacts,
    runtime,
  }) => {
    const { workspace, source, win, mainComposer } = await openSideSource(runtime, appPage);
    await mainComposer.click();
    // The fixture holds this turn until it is stopped, so no timer races the side chat (N-004).
    await mainComposer.pressSequentially("keep working until stopped");
    await mainComposer.press("Enter");
    // The side panel mounts its own composer after the main one: the main stop control is first.
    const mainStop = win.getByTestId("composer-stop-button").first();
    await expect(mainStop).toBeVisible();

    await mainComposer.pressSequentially("/side what now?");
    await mainComposer.press("Enter");

    const panel = appPage.getByTestId("side-chat-thread");
    await expect(panel).toBeVisible();
    await expect(panel).toContainText("what now?");
    await expect(panel).toContainText("side: main is still working");
    await expect(panel).not.toContainText("hello side");
    await expect(appPage.getByTestId("side-chat-main-status")).toHaveText("Main working");
    await browserArtifacts.captureScreenshot("side-chat-while-main-works", appPage);

    // Releasing the held main turn: the chip follows the main session, the side chat stays.
    await mainStop.click();
    await expect(appPage.getByTestId("side-chat-main-status")).toHaveText("Main finished");
    await expect(panel).toContainText("side: main is still working");

    const [side] = await sideChats(runtime, workspace.id, source.id);
    expect(side).toBeDefined();
    await appPage.getByTestId("side-chat-close").click();
    await expect(panel).toBeHidden();
    await expect
      .poll(async () => (await sideChats(runtime, workspace.id, source.id)).length)
      .toBe(0);
  });

  // Invariant: the head toggle opens an empty side chat whose first question creates it, and the
  // `session.side.toggle` shortcut brings the same side chat back after Escape in the panel hides it.
  test("E2E-002: operator opens a side chat from the head toggle and reopens it with the shortcut", async ({
    appPage,
    runtime,
  }) => {
    const { win } = await openSideSource(runtime, appPage);
    await win.getByTestId("session-side-toggle").click();
    const question = appPage.getByTestId("side-chat-question-input");
    await expect(question).toBeFocused();
    await question.fill("quick side question");
    await question.press("Enter");

    const panel = appPage.getByTestId("side-chat-thread");
    await expect(panel).toContainText("side: quick answer");
    await expect(win.getByTestId("session-side-toggle-count")).toHaveText("1");

    // The question settles into its thread and focus lands in the side composer, so Escape there
    // hides the non-modal panel; Escape from the main pane leaves it open.
    const sideComposer = panel.getByRole("textbox", { name: "Session prompt" });
    await expect(sideComposer).toBeFocused();
    await sideComposer.press("Escape");
    await expect(panel).toBeHidden();
    await appPage.keyboard.press(
      process.platform === "darwin" ? "Meta+Alt+KeyS" : "Control+Alt+KeyS"
    );
    await expect(panel).toContainText("side: quick answer");
  });

  // Invariant: an idle `/side <question>` reaches the side chat only; the main transcript is unchanged.
  test("E2E-003: an idle `/side` question adds nothing to the main session", async ({
    appPage,
    runtime,
  }) => {
    const { workspace, source, win, mainComposer, before } = await openSideSource(runtime, appPage);
    await mainComposer.click();
    await mainComposer.pressSequentially("/side what is this repo?");
    await mainComposer.press("Enter");

    const panel = appPage.getByTestId("side-chat-thread");
    await expect(panel).toContainText("side: this repo is compozy");
    await expect(mainComposer).toHaveText("");
    const after = await runtime.requestJSON<TranscriptPayload>(
      sessionAPIPath(workspace.id, source.id, "/transcript")
    );
    expect(after.max_sequence).toBe(before.max_sequence);
    await expect(win.locator(messageRow).filter({ hasText: "what is this repo?" })).toHaveCount(1);
  });
});
