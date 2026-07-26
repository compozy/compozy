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
import { useGlobalWorkspaceIfPrompted } from "../fixtures/workspace";

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

  await useGlobalWorkspaceIfPrompted(appPage);

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
  await expect(appPage).toHaveURL(/\/tasks\/new$/);
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
  await expect(appPage).toHaveURL(/\/tasks\/new$/);
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
