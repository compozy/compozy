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
  appWindow,
  openAppWindow,
  setGlobalScope,
  sessionWindow,
  switchWorkspace,
} from "../fixtures/os-navigation";
import { seedBrowserTasksOperatorFlow } from "../fixtures/runtime";
import { expect, test } from "../fixtures/test";
import { ensureProjectWorkspace, completeOnboardingIfPrompted } from "../fixtures/workspace";

/**
 * Manual operator control and coordinator-handoff bookends for the Tasks UI. These cases verify:
 *
 *   1. Creating a task is saved intent only, no run is queued, the runs panel
 *      explains that boundary, and the Publish CTA names coordinator handoff.
 *   2. Publishing/starting exposes the authoritative active run without retired coordination channels.
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

test("draft publishing and agent approval preserve coordinator-handoff boundaries", async ({
  appPage,
  browserArtifacts,
  runtime,
}) => {
  // Preserve the combined budget of the two original journeys.
  test.setTimeout(180_000);
  await test.step("publishing a draft hands off to the coordinator without retired coordination channels", async () => {
    const tasksWin = appWindow(appPage, "tasks");
    const tasksUI = tasksOperatorSelectors(tasksWin, appPage);
    const workspaceRoot = await mkdtemp(path.join(os.tmpdir(), "compozy-tasks-handoff-workspace-"));
    const workspace = await runtime.resolveWorkspace(workspaceRoot);

    await ensureProjectWorkspace(appPage, runtime);
    await switchWorkspace(appPage, workspace.id, workspace.name);
    await appPage.goto(runtime.url("/tasks"), { waitUntil: "domcontentloaded" });
    await completeOnboardingIfPrompted(appPage);
    await expect.poll(() => new URL(appPage.url()).pathname).toBe("/tasks");
    await expect(tasksWin).toBeVisible();

    await tasksUI.openCreate.click();
    await selectRecurringTaskTemplate(tasksUI);
    await expect(tasksWin.getByTestId("workspace-scope-statement")).toContainText(
      `Creates in ${workspace.name}`
    );
    await expect(tasksUI.createSaveDraft).toContainText("Save draft");
    await tasksUI.createPriority("high").click();
    const publishedTitle = `Coordinator handoff publish ${Date.now()}`;
    await tasksUI.createTitle.fill(publishedTitle);
    await tasksUI.createDescription.fill("Coordinator handoff without a channel.");
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

    await expect(tasksUI.detailTitle).toHaveText(publishedTitle);
    await expect(tasksUI.detailStatus).toHaveText(/draft/i);
    await expect(tasksUI.detailPublish).toBeVisible();
    await expect(tasksUI.detailPublish).toHaveAttribute("title", /coordinator handoff/i);
    await expect(tasksUI.detailEnqueue).toBeHidden();
    await expect(tasksUI.detailCoordination).toBeHidden();
    await tasksUI.detailTab("runs").click();
    await expect(tasksUI.detailRunsEmpty).toContainText(/saved intent only/i);
    await expect(tasksUI.detailRunsEmpty).toContainText(/publish, start, or approve/i);
    const runsPayload = await runtime.requestJSON<{
      runs: Array<{ id: string; status: string }>;
    }>(`/api/tasks/${encodeURIComponent(draftId)}/runs?limit=10`);
    expect(runsPayload.runs).toHaveLength(0);
    await tasksUI.detailTab("overview").click();

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

    await expect(tasksUI.detailNowRun).toBeVisible();
    await expect(tasksUI.detailCoordination).toHaveCount(0);

    await tasksUI.detailTab("runs").click();
    await expect(tasksUI.detailRunsEmpty).toBeHidden();
  });
  await test.step("approving an agent-created approval task is the coordinator-handoff boundary, not creation", async () => {
    const seeded = await seedBrowserTasksOperatorFlow(runtime, {
      sessionAgentName: handoffAgentName,
    });

    await completeOnboardingIfPrompted(appPage);
    await switchWorkspace(appPage, seeded.workspace.id, seeded.workspace.name);
    await setGlobalScope(appPage, true);

    await appPage.goto(runtime.url("/tasks"), { waitUntil: "domcontentloaded" });
    // Wait for route hydration instead of toggling its already-focused Dock entry.
    const tasksWin = appWindow(appPage, "tasks");
    await expect(tasksWin).toBeVisible();
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
    await expect(tasksUI.detailNowApproval).toContainText(/won't start until someone approves it/i);
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
});

test("starting a manual session is unaffected by task autonomy labels", async ({
  appPage,
  runtime,
}) => {
  const sessionUI = sessionLifecycleSelectors(appPage);

  await ensureProjectWorkspace(appPage, runtime);
  if (!runtime.paths?.workspaceDir) {
    throw new Error("manual session browser test requires launch-mode workspace paths");
  }
  const workspace = await runtime.resolveWorkspace(runtime.paths.workspaceDir);
  await appPage.goto(runtime.url("/"), { waitUntil: "domcontentloaded" });
  await completeOnboardingIfPrompted(appPage);

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
    return (
      response.request().method() === "POST" && new URL(response.url()).pathname === "/api/sessions"
    );
  });
  await appPage.getByTestId("session-create-submit").click();
  const createResponse = await createResponsePromise;
  expect(createResponse.ok()).toBeTruthy();
  const created = (await createResponse.json()) as { session?: { id?: string } };

  let sessionId = created.session?.id ?? "";
  expect(sessionId).not.toBe("");
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
  await sessionWin.composerTextarea.fill(SESSION_CREATE_FIRST_MESSAGE);
  await sessionWin.composerTextarea.press("Enter");
  await expect(sessionWin.chatView).toContainText(SESSION_CREATE_FIRST_MESSAGE);

  const sessions = await runtime.requestJSON<{
    sessions: Array<{ id: string; agent_name: string; state?: string }>;
  }>(`/api/sessions?workspace_id=${encodeURIComponent(workspace.id)}`);
  expect(
    sessions.sessions.some(
      session => session.id === sessionId && session.agent_name === handoffAgentName
    )
  ).toBe(true);
});
