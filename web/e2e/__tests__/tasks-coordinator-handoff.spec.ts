import { mkdtemp } from "node:fs/promises";
import os from "node:os";
import path from "node:path";
import { fileURLToPath } from "node:url";

import {
  SESSION_CREATE_FIRST_MESSAGE,
  sessionLifecycleSelectors,
  sessionWindowSelectors,
  tasksOperatorSelectors,
} from "../fixtures/selectors";
import {
  ensureAppWindow,
  openAppWindow,
  sessionWindow,
  switchWorkspace,
} from "../fixtures/os-navigation";
import { seedBrowserTasksOperatorFlow } from "../fixtures/runtime";
import { expect, test } from "../fixtures/test";
import { ensureGlobalWorkspace, useGlobalWorkspaceIfPrompted } from "../fixtures/workspace";

/**
 * (manual operator control) and (task-run coordination
 * channels) bookends for the Tasks UI. These cases verify:
 *
 *   1. Creating a task is saved intent only, no run is queued, the runs panel
 *      explains that boundary, and the Publish CTA names coordinator handoff.
 *   2. Publishing/starting exposes the authoritative active run with its bound
 *      coordination channel.
 *   3. Approving an agent-created approval-pending task enqueues a
 *      coordinator-handoff run, never auto-starting on creation.
 *   4. Manual session start UI is unaffected by task autonomy labels.
 */

const browserLifecycleFixture = path.resolve(
  path.dirname(fileURLToPath(import.meta.url)),
  "..",
  "..",
  "..",
  "internal",
  "testutil",
  "acpmock",
  "testdata",
  "browser_session_lifecycle_fixture.json"
);

const handoffAgentName = "browser-lifecycle-agent";
const draftTitle = "Draft handoff smoke task";
const draftDescription =
  "Saved intent for bookend coverage. No run should be queued until publish.";

function handoffAgentSessionPath(sessionId: string): string {
  return `/agents/${handoffAgentName}/sessions/${sessionId}`;
}

async function selectRecurringTaskTemplate(
  tasksUI: ReturnType<typeof tasksOperatorSelectors>
): Promise<void> {
  await tasksUI.createModeAdvanced.click();
  await expect(tasksUI.createModeAdvanced).toHaveAttribute("aria-pressed", "true");
  await tasksUI.createTemplate("recurring").click();
}

test.use({
  runtimeOptions: {
    seed: {
      mockAgents: [
        {
          agentName: handoffAgentName,
          fixtureAgent: handoffAgentName,
          fixturePath: browserLifecycleFixture,
        },
      ],
    },
  },
});

test("creating a task is saved intent, no run is enqueued and labels never imply autonomy", async ({
  appPage,
  runtime,
}) => {
  await ensureGlobalWorkspace(runtime);
  await appPage.goto(runtime.url("/tasks"), { waitUntil: "domcontentloaded" });
  await useGlobalWorkspaceIfPrompted(appPage);

  const tasksWin = appPage.getByTestId("os-window-app:tasks");
  const tasksUI = tasksOperatorSelectors(tasksWin, appPage);
  await expect(tasksWin).toBeVisible();
  await expect(appPage).toHaveURL(/\/tasks$/);

  await tasksUI.openCreate.click();
  await expect(appPage).toHaveURL(/\/tasks\/new$/);
  await selectRecurringTaskTemplate(tasksUI);
  await expect(tasksUI.createSaveDraft).toContainText("Save draft");
  await tasksUI.createPriority("medium").click();
  await tasksUI.createTitle.fill(draftTitle);
  await tasksUI.createDescription.fill(draftDescription);
  await tasksUI.createSaveDraft.click();
  await expect(tasksUI.createEditorSurface).toBeHidden();

  let draftId = "";
  await expect
    .poll(async () => {
      const payload = await runtime.requestJSON<{
        tasks: Array<{ id: string; status: string; title: string }>;
      }>(`/api/tasks?include_drafts=true&query=${encodeURIComponent(draftTitle)}&limit=10`);
      const created = payload.tasks.find(task => task.title === draftTitle);
      draftId = created?.id ?? "";
      return created?.status ?? "";
    })
    .toBe("draft");

  if (draftId === "") {
    throw new Error(`Expected a created draft task for "${draftTitle}".`);
  }

  await expect(tasksUI.detailTitle).toHaveText(draftTitle);
  await expect(tasksUI.detailStatus).toHaveText(/draft/i);

  const publishButton = tasksUI.detailPublish;
  await expect(publishButton).toBeVisible();
  await expect(publishButton).toHaveAttribute("title", /coordinator handoff/i);
  await expect(tasksUI.detailEnqueue).toBeHidden();
  await expect(tasksUI.detailCoordination).toBeHidden();

  await tasksUI.detailTab("runs").click();
  await expect(tasksUI.detailRunsEmpty).toContainText(/saved intent only/i);
  await expect(tasksUI.detailRunsEmpty).toContainText(/publish, start, or approve/i);

  const runsPayload = await runtime.requestJSON<{
    runs: Array<{ id: string; status: string }>;
  }>(`/api/tasks/${encodeURIComponent(draftId)}/runs?limit=10`);
  expect(runsPayload.runs).toHaveLength(0);
});

test("publishing a draft hands off to the coordinator and binds a coordination channel", async ({
  appPage,
  runtime,
}) => {
  const tasksWin = appPage.getByTestId("os-window-app:tasks");
  const tasksUI = tasksOperatorSelectors(tasksWin, appPage);
  const workspaceRoot = await mkdtemp(path.join(os.tmpdir(), "agh-tasks-handoff-workspace-"));
  const workspace = await runtime.resolveWorkspace(workspaceRoot);

  await ensureGlobalWorkspace(runtime);
  await appPage.goto(runtime.url("/tasks"), { waitUntil: "domcontentloaded" });
  await useGlobalWorkspaceIfPrompted(appPage);
  await expect.poll(() => new URL(appPage.url()).pathname).toBe("/tasks");
  await switchWorkspace(appPage, workspace.id, workspace.name);
  await ensureAppWindow(appPage, "Tasks", "tasks");

  await tasksUI.openCreate.click();
  await selectRecurringTaskTemplate(tasksUI);
  await expect(tasksUI.createScopeWorkspace).toHaveAttribute("aria-pressed", "true");
  await expect(tasksUI.createWorkspaceSelect).toHaveAttribute(
    "aria-label",
    `Workspace: ${workspace.name}`
  );
  await expect(tasksUI.createSaveDraft).toContainText("Save draft");
  await tasksUI.createPriority("high").click();
  const publishedTitle = `Coordinator handoff publish ${Date.now()}`;
  await tasksUI.createTitle.fill(publishedTitle);
  await tasksUI.createDescription.fill(" channel binding bookend.");
  await tasksUI.createSaveDraft.click();
  await expect(tasksUI.createEditorSurface).toBeHidden();

  let draftId = "";
  await expect
    .poll(async () => {
      const payload = await runtime.requestJSON<{
        tasks: Array<{
          id: string;
          scope?: string;
          status: string;
          title: string;
          workspace_id?: string | null;
        }>;
      }>(`/api/tasks?include_drafts=true&query=${encodeURIComponent(publishedTitle)}&limit=10`);
      const created = payload.tasks.find(
        task => task.title === publishedTitle && task.workspace_id === workspace.id
      );
      draftId = created?.id ?? "";
      return created?.status ?? "";
    })
    .toBe("draft");

  // Persist Live/run participation on the draft profile so publish binds a derived channel.
  await runtime.requestJSON(`/api/tasks/${encodeURIComponent(draftId)}`, {
    method: "PATCH",
    body: JSON.stringify({
      network_participation: {
        mode: "live",
        channel_strategy: "run",
      },
    }),
  });

  const publishResponsePromise = appPage.waitForResponse(response => {
    return (
      response.request().method() === "POST" &&
      response.url().endsWith(`/api/tasks/${encodeURIComponent(draftId)}/publish`)
    );
  });
  await tasksUI.detailPublish.click();
  const publishResponse = await publishResponsePromise;
  expect(publishResponse.ok()).toBeTruthy();

  await expect(tasksUI.detailPublish).toBeHidden();

  await expect
    .poll(async () => {
      const payload = await runtime.requestJSON<{
        runs: Array<{
          id: string;
          status: string;
          resolved_network_participation?: { channel_id?: string | null } | null;
        }>;
      }>(`/api/tasks/${encodeURIComponent(draftId)}/runs?limit=10`);
      return payload.runs[0]?.resolved_network_participation?.channel_id ?? "";
    })
    .not.toBe("");

  await expect(tasksUI.detailNowRun).toBeVisible();
  await expect(tasksUI.detailCoordination).toBeVisible();
  await expect(tasksUI.detailCoordination).toContainText(/channel/i);
  await expect(tasksUI.detailCoordination).toHaveAttribute(
    "title",
    /channel messages support coordination only/i
  );

  await tasksUI.detailTab("runs").click();
  await expect(tasksUI.detailRunsEmpty).toBeHidden();
});

test("approving an agent-created approval task is the coordinator-handoff boundary, not creation", async ({
  appPage,
  browserArtifacts,
  runtime,
}) => {
  const seeded = await seedBrowserTasksOperatorFlow(runtime, {
    sessionAgentName: handoffAgentName,
  });

  await useGlobalWorkspaceIfPrompted(appPage);

  const tasksWin = await openAppWindow(appPage, "Tasks", "tasks");
  const tasksUI = tasksOperatorSelectors(tasksWin, appPage);
  await expect(appPage).toHaveURL(/\/tasks$/);
  await tasksUI.modeList.click();
  await expect(tasksUI.modeList).toHaveAttribute("aria-current", "page");

  const approvalTaskRunsBefore = await runtime.requestJSON<{
    runs: Array<{ id: string }>;
  }>(`/api/tasks/${encodeURIComponent(seeded.approvalTask.id)}/runs?limit=10`);
  expect(approvalTaskRunsBefore.runs).toHaveLength(0);

  await tasksUI.taskCard(seeded.approvalTask.id).click();
  await expect(tasksUI.detailApprovalPill).toContainText(/approval pending/i);
  await expect(tasksUI.detailNowApproval).toContainText(/waiting for your approval/i);
  await expect(tasksUI.detailNowApproval).toContainText(/manual check is required/i);
  await tasksUI.detailTabRuns.click();
  await expect(tasksUI.detailRunsEmpty).toBeVisible();

  await tasksUI.detailBreadcrumbTasks.click();
  await tasksUI.modeInbox.click();
  await expect(tasksUI.inboxView).toBeVisible();
  await expect(tasksUI.inboxLane("approvals")).toBeVisible();

  const approveResponsePromise = appPage.waitForResponse(response => {
    return (
      response.request().method() === "POST" &&
      response.url().endsWith(`/api/tasks/${encodeURIComponent(seeded.approvalTask.id)}/approve`)
    );
  });
  await tasksUI.inboxApprove(seeded.approvalTask.id).click();
  const approveResponse = await approveResponsePromise;
  expect(approveResponse.ok()).toBeTruthy();

  await expect
    .poll(async () => {
      const payload = await runtime.requestJSON<{
        runs: Array<{ id: string; status: string }>;
      }>(`/api/tasks/${encodeURIComponent(seeded.approvalTask.id)}/runs?limit=10`);
      return payload.runs.length;
    })
    .toBeGreaterThan(0);

  await browserArtifacts.captureScreenshot("tasks-approval-handoff-enqueued", appPage);
});

test("starting a manual session is unaffected by task autonomy labels", async ({
  appPage,
  runtime,
}) => {
  const sessionUI = sessionLifecycleSelectors(appPage);

  await ensureGlobalWorkspace(runtime);
  await appPage.goto(runtime.url("/"), { waitUntil: "domcontentloaded" });
  await useGlobalWorkspaceIfPrompted(appPage);

  await expect(sessionUI.osDesktop).toBeVisible();

  const agentsWin = await openAppWindow(appPage, "Agents", "agents");
  const agentsUI = sessionLifecycleSelectors(agentsWin);
  await expect.poll(() => new URL(appPage.url()).pathname).toBe("/agents");
  await expect(agentsUI.agentRow(handoffAgentName)).toBeVisible();
  await agentsUI.agentRow(handoffAgentName).click();
  await expect.poll(() => new URL(appPage.url()).pathname).toBe(`/agents/${handoffAgentName}`);
  await expect(agentsUI.agentPageNewSession).toBeVisible();
  await agentsUI.agentPageNewSession.click();

  await expect(appPage.getByTestId("session-create-dialog")).toBeVisible();
  await expect(appPage.getByTestId("session-create-agent-select")).toContainText(handoffAgentName);

  const createResponsePromise = appPage.waitForResponse(response => {
    return response.request().method() === "POST" && response.url().endsWith("/api/sessions");
  });
  await appPage.getByTestId("session-create-prompt").fill(SESSION_CREATE_FIRST_MESSAGE);
  await appPage.getByTestId("session-create-send").click();
  const createResponse = await createResponsePromise;
  expect(createResponse.ok()).toBeTruthy();

  let sessionId = "";
  await expect
    .poll(() => {
      const pathname = new URL(appPage.url()).pathname;
      const prefix = `/agents/${handoffAgentName}/sessions/`;
      sessionId = pathname.startsWith(prefix) ? pathname.slice(prefix.length) : "";
      return sessionId;
    })
    .not.toBe("");
  await expect.poll(() => new URL(appPage.url()).pathname).toBe(handoffAgentSessionPath(sessionId));

  const sessionWin = sessionWindowSelectors(sessionWindow(appPage, sessionId));
  await expect(sessionWin.chatView).toBeVisible();

  const sessions = await runtime.requestJSON<{
    sessions: Array<{ id: string; agent_name: string; state?: string }>;
  }>("/api/sessions");
  expect(
    sessions.sessions.some(
      session => session.id === sessionId && session.agent_name === handoffAgentName
    )
  ).toBe(true);
});
