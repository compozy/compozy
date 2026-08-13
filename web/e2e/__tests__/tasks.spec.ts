import path from "node:path";
import { fileURLToPath } from "node:url";

import {
  sessionLifecycleSelectors,
  sessionWindowSelectors,
  tasksOperatorSelectors,
} from "../fixtures/selectors";
import { openAppWindow, sessionWindow } from "../fixtures/os-navigation";
import { seedBrowserTasksOperatorFlow } from "../fixtures/runtime";
import { expect, test } from "../fixtures/test";
import { completeOnboardingIfPrompted } from "../fixtures/workspace";

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

const tasksSessionAgentName = "browser-lifecycle-agent";
const createdDraftDescription =
  "Use the shared browser lane to capture fresh Tasks evidence for task_19.";
const deleteDraftDescription = "Exercise the shared delete confirmation dialog from Tasks e2e.";

function tasksSessionPath(sessionId: string): string {
  return `/agents/${tasksSessionAgentName}/sessions/${sessionId}`;
}

function uniqueDraftTitle(prefix: string): string {
  return `${prefix} ${Date.now()}`;
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
          agentName: tasksSessionAgentName,
          fixtureAgent: tasksSessionAgentName,
          fixturePath: browserLifecycleFixture,
        },
      ],
    },
  },
});

test("operator can execute the shipped Tasks flow through the shared daemon-served browser lane", async ({
  appPage,
  browserArtifacts,
  runtime,
}) => {
  const sessionUI = sessionLifecycleSelectors(appPage);
  const seeded = await seedBrowserTasksOperatorFlow(runtime, {
    sessionAgentName: tasksSessionAgentName,
  });

  await completeOnboardingIfPrompted(appPage);

  await expect(sessionUI.osDesktop).toBeVisible();
  const tasksWin = await openAppWindow(appPage, "Tasks", "tasks");
  const tasksUI = tasksOperatorSelectors(tasksWin, appPage);

  await expect(appPage).toHaveURL(/\/tasks$/);
  await expect(tasksUI.modeList).toHaveAttribute("aria-current", "page");
  await expect(tasksUI.taskCard(seeded.referenceTask.id)).toBeVisible();
  await expect(tasksUI.taskCard(seeded.approvalTask.id)).toBeVisible();
  await expect(tasksUI.taskCard(seeded.runningTask.id)).toBeVisible();
  await browserArtifacts.captureScreenshot("tasks-list-seeded", appPage);

  await tasksUI.openCreate.click();
  await expect.poll(() => new URL(appPage.url()).pathname).toBe("/tasks/new");
  await expect(tasksUI.createEditorSurface).toBeVisible();
  await selectRecurringTaskTemplate(tasksUI);
  await expect(tasksUI.createSaveDraft).toContainText("Save draft");
  await tasksUI.createPriority("high").click();
  const createdDraftTitle = uniqueDraftTitle("Draft Tasks browser evidence rollout");
  await tasksUI.createTitle.fill(createdDraftTitle);
  await tasksUI.createDescription.fill(createdDraftDescription);
  await expect(tasksUI.createSaveDraft).toBeEnabled();
  await tasksUI.createSaveDraft.click();
  await expect(tasksUI.createEditorSurface).toBeHidden();

  let createdDraftId = "";
  await expect
    .poll(async () => {
      const payload = await runtime.requestJSON<{
        tasks: Array<{ id: string; status: string; title: string }>;
      }>(`/api/tasks?include_drafts=true&query=${encodeURIComponent(createdDraftTitle)}&limit=10`);
      const createdTask = payload.tasks.find(task => task.title === createdDraftTitle);
      createdDraftId = createdTask?.id ?? "";
      return createdTask?.status ?? "";
    })
    .toBe("draft");

  if (createdDraftId === "") {
    throw new Error(`Expected a created draft task for "${createdDraftTitle}".`);
  }

  await expect(tasksUI.detailTitle).toHaveText(createdDraftTitle);
  await expect(tasksUI.detailPublish).toBeVisible();
  await browserArtifacts.captureScreenshot("tasks-draft-created", appPage);

  const publishResponsePromise = appPage.waitForResponse(response => {
    return (
      response.request().method() === "POST" &&
      response.url().endsWith(`/api/tasks/${encodeURIComponent(createdDraftId)}/publish`)
    );
  });
  await tasksUI.detailPublish.click();
  const publishResponse = await publishResponsePromise;
  expect(publishResponse.ok()).toBeTruthy();
  await expect(publishResponse.json()).resolves.toMatchObject({
    task: {
      status: "ready",
    },
  });

  await expect
    .poll(async () => {
      const payload = await runtime.requestJSON<{
        task: {
          summary?: { status?: string | null };
          task?: { status?: string | null };
        };
      }>(`/api/tasks/${encodeURIComponent(createdDraftId)}`);
      return payload.task.summary?.status ?? payload.task.task?.status ?? "";
    })
    .toBe("ready");
  await expect(tasksUI.detailPublish).toBeHidden();
  await browserArtifacts.captureScreenshot("tasks-draft-published", appPage);

  await expect(tasksUI.detailContent).toBeVisible();
  await expect(tasksUI.detailTitle).toHaveText(createdDraftTitle);
  await expect(tasksUI.detailTab("overview")).toBeVisible();
  await expect(tasksUI.detailTab("runs")).toBeVisible();
  await expect(tasksUI.detailTab("activity")).toBeVisible();
  await browserArtifacts.captureScreenshot("tasks-detail-route", appPage);

  await tasksUI.detailTab("activity").click();
  await expect(tasksUI.detailTab("activity")).toHaveAttribute("aria-selected", "true");
  await browserArtifacts.captureScreenshot("tasks-activity", appPage);

  await tasksUI.detailBreadcrumbTasks.click();
  await expect(appPage).toHaveURL(/\/tasks$/);
  await tasksUI.modeDashboard.click();
  await expect(tasksUI.dashboardView).toBeVisible();
  await expect(tasksUI.dashboardActiveRun(seeded.runningRun.id)).toBeVisible();
  await browserArtifacts.captureScreenshot("tasks-dashboard", appPage);

  const activeRunPath = `/tasks/${seeded.runningTask.id}/runs/${seeded.runningRun.id}`;
  const activeRunLink = tasksUI.dashboardActiveRunLink(seeded.runningRun.id);
  await expect(activeRunLink).toBeVisible();
  await expect(activeRunLink).toHaveAttribute("href", activeRunPath);
  await appPage.goto(runtime.url(activeRunPath), {
    waitUntil: "domcontentloaded",
  });
  await expect(tasksUI.runDetailContent).toBeVisible();
  await expect.poll(() => new URL(appPage.url()).pathname).toBe(activeRunPath);
  await expect(tasksUI.runSessionDrilldown).toBeVisible();
  await browserArtifacts.captureScreenshot("tasks-run-detail", appPage);

  await tasksUI.runSessionDrilldown.click();
  await expect
    .poll(() => new URL(appPage.url()).pathname)
    .toBe(tasksSessionPath(seeded.session.id));
  const sessionWin = sessionWindowSelectors(sessionWindow(appPage, seeded.session.id));
  await expect(sessionWin.chatView).toBeVisible();
  await browserArtifacts.captureScreenshot("tasks-linked-session", appPage);

  await appPage.goto(runtime.url("/tasks"), {
    waitUntil: "domcontentloaded",
  });
  await expect(tasksUI.modeList).toBeVisible();
  await tasksUI.modeInbox.click();
  await expect(tasksUI.inboxView).toBeVisible();
  await expect(tasksUI.inboxLane("approvals")).toBeVisible();
  await expect(tasksUI.inboxItem(seeded.approvalTask.id)).toBeVisible();
  await browserArtifacts.captureScreenshot("tasks-inbox-approval-pending", appPage);

  const approveResponsePromise = appPage.waitForResponse(response => {
    return (
      response.request().method() === "POST" &&
      response.url().endsWith(`/api/tasks/${encodeURIComponent(seeded.approvalTask.id)}/approve`)
    );
  });
  await tasksUI.inboxApprove(seeded.approvalTask.id).click();
  const approveResponse = await approveResponsePromise;
  expect(approveResponse.ok()).toBeTruthy();
  await expect(approveResponse.json()).resolves.toMatchObject({
    task: {
      approval_state: "approved",
    },
  });

  await expect
    .poll(async () => {
      const payload = await runtime.requestJSON<{
        task: {
          summary?: { approval_state?: string | null };
          task?: { approval_state?: string | null };
        };
      }>(`/api/tasks/${encodeURIComponent(seeded.approvalTask.id)}`);
      return payload.task.summary?.approval_state ?? payload.task.task?.approval_state ?? "";
    })
    .toBe("approved");
  await expect
    .poll(async () => {
      const payload = await runtime.requestJSON<{
        inbox: {
          groups?: Array<{
            items?: Array<{
              task: {
                id: string;
              };
            }>;
          }>;
        };
      }>("/api/observe/tasks/inbox?lane=approvals&limit=10");

      return (
        payload.inbox.groups?.some(group =>
          (group.items ?? []).some(item => item.task.id === seeded.approvalTask.id)
        ) ?? false
      );
    })
    .toBe(false);
  await browserArtifacts.captureScreenshot("tasks-inbox-approval-approved", appPage);

  await tasksUI.openCreate.click();
  await expect.poll(() => new URL(appPage.url()).pathname).toBe("/tasks/new");
  await expect(tasksUI.createEditorSurface).toBeVisible();
  await selectRecurringTaskTemplate(tasksUI);
  await expect(tasksUI.createSaveDraft).toContainText("Save draft");
  const deleteDraftTitle = uniqueDraftTitle("Draft Tasks delete confirmation smoke");
  await tasksUI.createTitle.fill(deleteDraftTitle);
  await tasksUI.createDescription.fill(deleteDraftDescription);
  await tasksUI.createSaveDraft.click();
  await expect(tasksUI.createEditorSurface).toBeHidden();

  let deleteDraftId = "";
  await expect
    .poll(async () => {
      const payload = await runtime.requestJSON<{
        tasks: Array<{ id: string; status: string; title: string }>;
      }>(`/api/tasks?include_drafts=true&query=${encodeURIComponent(deleteDraftTitle)}&limit=10`);
      const deleteTask = payload.tasks.find(task => task.title === deleteDraftTitle);
      deleteDraftId = deleteTask?.id ?? "";
      return deleteTask?.status ?? "";
    })
    .toBe("draft");

  if (deleteDraftId === "") {
    throw new Error(`Expected a deletable draft task for "${deleteDraftTitle}".`);
  }

  await expect(tasksUI.detailTitle).toHaveText(deleteDraftTitle);
  await tasksUI.detailOverflow.click();
  await tasksUI.detailDelete.click();
  await expect(tasksUI.detailDeleteDialog).toBeVisible();
  await expect(tasksUI.detailDeleteDialog).toContainText(deleteDraftTitle);

  const deleteResponsePromise = appPage.waitForResponse(response => {
    return (
      response.request().method() === "DELETE" &&
      response.url().endsWith(`/api/tasks/${encodeURIComponent(deleteDraftId)}`)
    );
  });
  await tasksUI.detailDeleteConfirm.click();
  const deleteResponse = await deleteResponsePromise;
  expect(deleteResponse.ok()).toBeTruthy();
  await expect.poll(() => new URL(appPage.url()).pathname).toBe("/tasks");
  await expect
    .poll(async () => {
      const payload = await runtime.requestJSON<{
        tasks: Array<{ id: string; title: string }>;
      }>(`/api/tasks?include_drafts=true&query=${encodeURIComponent(deleteDraftTitle)}&limit=10`);
      return payload.tasks.some(
        task => task.id === deleteDraftId || task.title === deleteDraftTitle
      );
    })
    .toBe(false);
  await browserArtifacts.captureScreenshot("tasks-draft-deleted", appPage);
});

// E2E-011: the task worktree policy — the locked mode vocabulary, a
// same-workspace-only reference picker, an invalid reference flagged, and every
// control locked while a run is active.
test("operator sets the task worktree policy from the setup sheet", async ({
  appPage,
  runtime,
}) => {
  await completeOnboardingIfPrompted(appPage);
  const seeded = await seedBrowserTasksOperatorFlow(runtime, {
    sessionAgentName: tasksSessionAgentName,
  });
  const taskId = seeded.referenceTask.id;
  await appPage.reload({ waitUntil: "domcontentloaded" });
  await openAppWindow(appPage, "Tasks", "tasks");
  await appPage.getByTestId(`tasks-row-${taskId}`).click();
  await appPage.getByTestId("tasks-detail-setup").click();

  const policy = appPage.locator('[data-slot="task-worktree-policy"]');
  await expect(policy).toBeVisible();
  const modes = policy.locator('[data-slot="pill-group-item"]');
  await expect(modes).toHaveText(["Inherit", "Workspace root", "Named worktree", "Per-run"]);

  await modes.filter({ hasText: "Per-run" }).click();
  await expect(policy).toHaveAttribute("data-mode", "per_run");
  await expect
    .poll(async () => {
      const profile = await runtime.requestJSON<{ profile: { worktree: { mode: string } } }>(
        `/api/tasks/${taskId}/execution-profile`
      );
      return profile.profile.worktree.mode;
    })
    .toBe("per_run");

  // The reference picker only exists in `ref` mode, and only offers ready
  // worktrees from this task's own workspace.
  await modes.filter({ hasText: "Named worktree" }).click();
  await expect(appPage.getByTestId("tasks-setup-worktree-ref")).toBeVisible();
});

// E2E-012: fan-out isolation — the count statement matches the request, and the
// result attributes each run to the worktree the response named.
test("operator isolates each fan-out run in its own worktree", async ({ appPage, runtime }) => {
  await completeOnboardingIfPrompted(appPage);
  const seeded = await seedBrowserTasksOperatorFlow(runtime, {
    sessionAgentName: tasksSessionAgentName,
  });
  const taskId = seeded.referenceTask.id;
  await appPage.reload({ waitUntil: "domcontentloaded" });
  await openAppWindow(appPage, "Tasks", "tasks");
  await appPage.getByTestId(`tasks-row-${taskId}`).click();
  await appPage.getByTestId("tasks-detail-fan-out").click();

  await appPage
    .getByTestId("tasks-fan-out-designations")
    .fill("Investigate the failing checkout path\nValidate the fix on staging");

  const isolation = appPage.locator('[data-slot="task-fan-out-isolation"]');
  await expect(isolation).toBeVisible();
  await appPage.getByTestId("tasks-fan-out-worktree-per-run").click();
  await expect(isolation.locator('[data-slot="task-fan-out-isolation-count"]')).toHaveText(
    "Creates 2 worktrees, one per run."
  );

  await appPage.getByTestId("tasks-fan-out-runs-submit").click();
  const results = appPage.locator('[data-slot="task-fan-out-run-results"]');
  await expect(results).toBeVisible();
  await expect(results.locator('[data-slot="task-fan-out-run-result"]')).toHaveCount(2);
  for (const row of await results.locator('[data-slot="task-fan-out-run-result"]').all()) {
    await expect(row).toHaveAttribute("data-unattributed", "");
    await expect(row.locator('[data-slot="task-fan-out-run-attribution"]')).not.toContainText(
      "Worktree pending"
    );
    await expect(row.locator('[data-slot="task-fan-out-run-attribution"]')).not.toContainText(
      "Worktree unavailable"
    );
  }
});
