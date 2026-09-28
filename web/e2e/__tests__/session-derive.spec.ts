import { randomUUID } from "node:crypto";
import path from "node:path";
import { fileURLToPath } from "node:url";

import { sessionWindow, switchWorkspace, windowFrame } from "../fixtures/os-navigation";
import { sessionLifecycleSelectors, sessionWindowSelectors } from "../fixtures/selectors";
import type { BrowserRuntime, WorkspacePayload } from "../fixtures/runtime";
import { expect, test } from "../fixtures/test";
import { completeOnboardingIfPrompted } from "../fixtures/workspace";

// session-continue-fork task_04: E2E-001 (continue via the web) and E2E-004 (a failed turn offers
// Continue). Written by task_04; executed by task_08 against the e2e daemon fixture.
const fixturePath = path.resolve(
  path.dirname(fileURLToPath(import.meta.url)),
  "../../../internal/testutil/acpmock/testdata/multi_agent_fixture.json"
);
const sourceAgent = "alpha";
const targetAgent = "beta";

test.use({
  runtimeOptions: {
    seed: {
      mockAgents: [
        { fixturePath, fixtureAgent: sourceAgent },
        { fixturePath, fixtureAgent: targetAgent },
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
  entries: Array<{ message: { id: string } }>;
}

function sessionAPIPath(workspaceID: string, sessionID: string, suffix = ""): string {
  return `/api/workspaces/${encodeURIComponent(workspaceID)}/sessions/${encodeURIComponent(
    sessionID
  )}${suffix}`;
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
  workspace: WorkspacePayload
): Promise<SessionPayload> {
  const { session } = await runtime.requestJSON<{ session: SessionPayload }>("/api/sessions", {
    method: "POST",
    body: JSON.stringify({ agent_name: sourceAgent, workspace: workspace.id }),
  });
  await runtime.requestJSON(sessionAPIPath(workspace.id, session.id, "/prompt"), {
    method: "POST",
    body: JSON.stringify({
      idempotency_key: randomUUID(),
      message: "hello alpha",
      message_id: randomUUID(),
    }),
  });
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
    await expect(sourceWin.locator(`[data-message-id="${entry.message.id}"]`)).toHaveCount(1);
  }
});

// E2E-004 needs an acpmock turn that fails `session/prompt` as a provider rate limit (or auth
// lapse) so the daemon decorates the error with `next_action: "handoff"`. acpmock has no such
// step today (internal/testutil/acpmock StepKind has no prompt-error kind); task_08 adds that
// capability or walks it with a real rate-limited provider (RT-provider-error-handoff).
test.fixme("E2E-004: a rate-limited turn offers Continue with this session as the source", async ({
  appPage,
  runtime,
}) => {
  const workspace = await prepareWorkspace(runtime, appPage);
  const source = await createPromptedSource(runtime, workspace);
  // Second prompt: scripted by the (future) fixture to rate-limit the turn.
  await runtime.requestJSON(sessionAPIPath(workspace.id, source.id, "/prompt"), {
    method: "POST",
    body: JSON.stringify({
      idempotency_key: randomUUID(),
      message: "rate limit this turn",
      message_id: randomUUID(),
    }),
  });

  await appPage.goto(runtime.url(`/agents/${sourceAgent}/sessions/${source.id}`), {
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
  await expect(dialog.getByTestId("session-continue-source-note")).toContainText(sourceAgent);

  // Nothing is created until Continue.
  const { sessions: afterSessions } = await runtime.requestJSON<{ sessions: SessionPayload[] }>(
    `/api/sessions?workspace_id=${encodeURIComponent(workspace.id)}`
  );
  expect(afterSessions).toHaveLength(beforeSessions.length);
});
