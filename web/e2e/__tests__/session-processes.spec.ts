import { spawn } from "node:child_process";
import path from "node:path";
import process from "node:process";
import { fileURLToPath } from "node:url";

import type { Page } from "@playwright/test";

import { sessionWindow, windowFrame } from "../fixtures/os-navigation";
import { sessionWindowSelectors } from "../fixtures/selectors";
import type { BrowserRuntime, WorkspacePayload } from "../fixtures/runtime";
import { expect, test } from "../fixtures/test";
import { ensureProjectWorkspace } from "../fixtures/workspace";

// Invariant: a process carrying a session's COMPOZY_SESSION_ID that escaped the agent's process
// group shows on that session's composer; stopping, archiving, or deleting the session asks what
// happens to it, and the answer reaches the daemon: terminate ends the process, keep leaves it
// running (keep/terminate choice, D4/D5; RT-session-processes).
// Owner: public session window + daemon process scan and stop. Copy and read states are owned by
// the Vitest suites.

const agentName = "browser-lifecycle-agent";
const fixturePath = path.resolve(
  path.dirname(fileURLToPath(import.meta.url)),
  "../../../internal/testutil/acpmock/testdata/browser_session_lifecycle_fixture.json"
);
// The daemon scans on a cycle; the indicator follows the next scan, not the spawn.
const SCAN_TIMEOUT_MS = 45_000;

test.use({ runtimeOptions: { seed: { mockAgents: [{ fixturePath, fixtureAgent: agentName }] } } });

async function resolveWorkspace(runtime: BrowserRuntime): Promise<WorkspacePayload> {
  const workspace =
    runtime.seeded.workspace ??
    (runtime.paths?.workspaceDir
      ? await runtime.resolveWorkspace(runtime.paths.workspaceDir)
      : null);
  if (!workspace) throw new Error("Session processes walk requires a seeded workspace");
  return workspace;
}

async function createSession(page: Page, runtime: BrowserRuntime, workspaceId: string) {
  await page.goto(runtime.url("/"), { waitUntil: "domcontentloaded" });
  await ensureProjectWorkspace(page, runtime);
  const created = await runtime.requestJSON<{ session: { id: string } }>("/api/sessions", {
    method: "POST",
    body: JSON.stringify({ agent_name: agentName, workspace: workspaceId }),
  });
  return created.session.id;
}

/**
 * What an agent's `cmd &` leaves behind: a detached process in its own group that inherited the
 * session marker. Node stands in for the dev server; a protected system binary would hide its
 * environment from the scan on macOS.
 */
function spawnLeftBehindProcess(sessionId: string): number {
  const child = spawn(process.execPath, ["-e", "setInterval(() => {}, 1_000)"], {
    detached: true,
    stdio: "ignore",
    env: { ...process.env, COMPOZY_SESSION_ID: sessionId },
  });
  child.unref();
  if (child.pid === undefined) throw new Error("left-behind process did not start");
  return child.pid;
}

function isAlive(pid: number): boolean {
  try {
    process.kill(pid, 0);
    return true;
  } catch {
    return false;
  }
}

test("E2E-PROC-001: stopping a session with its processes ends them and clears the indicator", async ({
  appPage,
  browserArtifacts,
  runtime,
}) => {
  const workspace = await resolveWorkspace(runtime);
  const sessionId = await createSession(appPage, runtime, workspace.id);
  const pid = spawnLeftBehindProcess(sessionId);
  try {
    await appPage.goto(runtime.url(`/agents/${agentName}/sessions/${sessionId}`));
    const win = sessionWindow(appPage, sessionId);
    await expect(win).toBeVisible();
    const frame = windowFrame(win);

    const indicator = win.getByTestId("composer-processes-button");
    await expect(indicator).toHaveAccessibleName("Processes, 1 running", {
      timeout: SCAN_TIMEOUT_MS,
    });
    await indicator.click();
    await expect(appPage.getByTestId(`session-process-${pid}`)).toBeVisible();
    await browserArtifacts.captureScreenshot("session-processes-popover", appPage);
    await appPage.keyboard.press("Escape");

    await frame.getByTestId("stop-button").click();
    const dialog = appPage.getByRole("dialog", { name: "Stop this session?" });
    await expect(dialog).toContainText("This session started 1 process that is still running.");
    await browserArtifacts.captureScreenshot("session-processes-stop-dialog", appPage);
    await dialog.getByRole("button", { name: "Stop session and 1 process" }).click();

    await expect(indicator).toHaveCount(0, { timeout: SCAN_TIMEOUT_MS });
    await expect.poll(() => isAlive(pid), { timeout: SCAN_TIMEOUT_MS }).toBe(false);
  } finally {
    if (isAlive(pid)) process.kill(pid, "SIGKILL");
  }
});

type ProcessChoice = "keep" | "terminate";

/** The list verbs that ask about running processes, with their request and choice controls. */
const LIFECYCLE_CHOICES = {
  archive: {
    method: "POST",
    path: (workspaceId: string, sessionId: string) =>
      `/api/workspaces/${workspaceId}/sessions/${sessionId}/archive`,
    dialog: "session-process-choice-dialog",
    keep: "session-process-choice-keep",
    terminate: "session-process-choice-terminate",
  },
  delete: {
    method: "DELETE",
    path: (workspaceId: string, sessionId: string) =>
      `/api/workspaces/${workspaceId}/sessions/${sessionId}`,
    dialog: "delete-dialog",
    keep: "delete-dialog-keep-processes",
    terminate: "delete-dialog-confirm",
  },
} as const;

/**
 * Opens the session once the daemon's scan reports its process, then runs `verb` from the
 * session list row and answers the process choice; resolves once the daemon answered the request,
 * which must carry the choice.
 */
async function runListVerbWithChoice(
  page: Page,
  runtime: BrowserRuntime,
  workspaceId: string,
  sessionId: string,
  verb: keyof typeof LIFECYCLE_CHOICES,
  choice: ProcessChoice
) {
  const flow = LIFECYCLE_CHOICES[verb];
  await page.goto(runtime.url(`/agents/${agentName}/sessions/${sessionId}`));
  const win = sessionWindow(page, sessionId);
  await expect(win).toBeVisible();
  // The row's choice reads the same process summary the composer indicator shows.
  await expect(win.getByTestId("composer-processes-button")).toHaveAccessibleName(
    "Processes, 1 running",
    { timeout: SCAN_TIMEOUT_MS }
  );
  const ui = sessionWindowSelectors(win, page);
  await ui.sessionRowActions(sessionId).click();
  await ui.sessionRowMenuItem(verb, sessionId).click();
  await expect(page.getByTestId(flow.dialog)).toBeVisible();
  const response = page.waitForResponse(
    candidate =>
      candidate.request().method() === flow.method &&
      new URL(candidate.url()).pathname === flow.path(workspaceId, sessionId)
  );
  await page.getByTestId(flow[choice]).click();
  const answered = await response;
  expect(answered.ok(), `${answered.status()} ${answered.statusText()} ${answered.url()}`).toBe(
    true
  );
  expect(new URL(answered.url()).searchParams.get("processes")).toBe(choice);
}

for (const verb of ["archive", "delete"] as const) {
  test(`E2E-PROC-002 (${verb}): the list ${verb} keeps or ends the processes as answered`, async ({
    appPage,
    browserArtifacts,
    runtime,
  }) => {
    const workspace = await resolveWorkspace(runtime);
    const keptSession = await createSession(appPage, runtime, workspace.id);
    const endedSession = await createSession(appPage, runtime, workspace.id);
    const kept = spawnLeftBehindProcess(keptSession);
    const ended = spawnLeftBehindProcess(endedSession);
    try {
      await runListVerbWithChoice(appPage, runtime, workspace.id, keptSession, verb, "keep");
      // Keep leaves the process running after the daemon finished the verb.
      expect(isAlive(kept)).toBe(true);

      await runListVerbWithChoice(appPage, runtime, workspace.id, endedSession, verb, "terminate");
      await expect.poll(() => isAlive(ended), { timeout: SCAN_TIMEOUT_MS }).toBe(false);
      expect(isAlive(kept)).toBe(true);
      await browserArtifacts.captureScreenshot(`session-processes-${verb}-settled`, appPage);
    } finally {
      for (const pid of [kept, ended]) if (isAlive(pid)) process.kill(pid, "SIGKILL");
    }
  });
}
