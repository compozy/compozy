import { randomUUID } from "node:crypto";
import { mkdtempSync } from "node:fs";
import { rm } from "node:fs/promises";
import os from "node:os";
import path from "node:path";
import { fileURLToPath } from "node:url";

import type { LoopDefinition, RunLoopResult } from "@/systems/loops";
import {
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
import { seedRetainedLoopTask } from "../fixtures/retained-loop-task";
import type { BrowserRuntime } from "../fixtures/runtime";
import { seedBrowserTasksOperatorFlow } from "../fixtures/runtime";
import { expect, test } from "../fixtures/test";
import { createWorktreeRepo, type WorktreeRepoFixture } from "../fixtures/worktree-repo";
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
const tasksLoopWorkspaceRoot = mkdtempSync(path.join(os.tmpdir(), "compozy-tasks-loop-"));
const tasksSessionAgentName = "browser-lifecycle-agent";
const createdDraftDescription =
  "Use the shared browser lane to capture fresh Tasks evidence for task_19.";
const deleteDraftDescription = "Exercise the shared delete confirmation dialog from Tasks e2e.";

let tasksWorktreeRepo: WorktreeRepoFixture | null = null;

test.afterEach(async () => {
  await tasksWorktreeRepo?.cleanup();
  tasksWorktreeRepo = null;
});

test.afterAll(async () => {
  await rm(tasksLoopWorkspaceRoot, { force: true, recursive: true });
});

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
  await switchWorkspace(appPage, seeded.workspace.id, seeded.workspace.name);
  await setGlobalScope(appPage, true);

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
  tasksWorktreeRepo = await createWorktreeRepo();
  const seeded = await seedBrowserTasksOperatorFlow(runtime, {
    referenceTaskScope: "workspace",
    sessionAgentName: tasksSessionAgentName,
    workspaceRootDir: tasksWorktreeRepo.rootDir,
  });
  const taskId = seeded.referenceTask.id;
  await appPage.reload({ waitUntil: "domcontentloaded" });
  await switchWorkspace(appPage, seeded.workspace.id, seeded.workspace.name);
  const tasksWin = await openAppWindow(appPage, "Tasks", "tasks");
  const tasksUI = tasksOperatorSelectors(tasksWin, appPage);
  await tasksUI.taskCard(taskId).click();
  await appPage.getByTestId("tasks-rail-edit-setup").click();
  await appPage.getByTestId("tasks-setup-edit").click();

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
  tasksWorktreeRepo = await createWorktreeRepo();
  const seeded = await seedBrowserTasksOperatorFlow(runtime, {
    referenceTaskScope: "workspace",
    sessionAgentName: tasksSessionAgentName,
    workspaceRootDir: tasksWorktreeRepo.rootDir,
  });
  const taskId = seeded.referenceTask.id;
  await appPage.reload({ waitUntil: "domcontentloaded" });
  await switchWorkspace(appPage, seeded.workspace.id, seeded.workspace.name);
  const tasksWin = await openAppWindow(appPage, "Tasks", "tasks");
  const tasksUI = tasksOperatorSelectors(tasksWin, appPage);
  await tasksUI.taskCard(taskId).click();
  await tasksUI.detailOverflow.click();
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

// ---------------------------------------------------------------------------
// Loop record legibility (task_04): the Tasks surface is for human-facing work
// items, so Loop execution records leave it by default and come back only for
// one explicit, ephemeral act.
// ---------------------------------------------------------------------------

const loopProvenanceFixture = path.resolve(
  path.dirname(fileURLToPath(import.meta.url)),
  "..",
  "..",
  "..",
  "internal",
  "testutil",
  "acpmock",
  "testdata",
  "loop_runtime_provenance_fixture.json"
);
const loopLegibilityAgent = "loop-legibility-agent";
const loopLegibilityName = "revisao-paralela-e2e";
// The canonical terminal run from the design data story — reused rather than
// minting a third run id. Here it is the run retention removed.
const RETAINED_LOOP_RUN_ID = "looprun-77aa01b2c3d4e5f6";
const RETAINED_TASK_ID = "task-retained-loop-record";

const loopLegibilityDefinition: LoopDefinition = {
  apiVersion: "compozy.loop/v1",
  kind: "Loop",
  meta: {
    name: loopLegibilityName,
    description: "Materialize coordinator and cell records for the Tasks calm-default journeys.",
    catalog: { category: "Testing" },
  },
  concurrency: "allow",
  contract: {
    goal: "Fan out over two review items so the run materializes several cell records.",
    definition_of_done: "Every review item completes.",
    stop_when: "nodes.collect.status == 'succeeded'",
    iteration_cap: 1,
    no_progress: { window: 2 },
    budget: { tokens: 0, wall_clock_sec: 0, on_exceeded: "halt" },
    runtime_defaults: {
      worker: { provider: "acpmock", model: "base-model", reasoning: "low" },
      judge: {},
    },
    terminal_states: ["done", "failed", "blocked", "exhausted", "stalled"],
  },
  graph: {
    nodes: [
      {
        id: "revisores",
        class: "action",
        kind: "transform",
        params: {
          map: {
            tasks: {
              value: [
                {
                  id: "revisor_perf",
                  path: ".compozy/tasks/runtime/task_frontend.md",
                  title: "Frontend runtime task",
                  type: "frontend",
                  complexity: "medium",
                  body: "Frontend fixture",
                },
                {
                  id: "revisor_docs",
                  path: ".compozy/tasks/runtime/task_docs.md",
                  title: "Docs runtime task",
                  type: "docs",
                  complexity: "low",
                  body: "Docs fixture",
                },
              ],
            },
          },
        },
      },
      {
        id: "fan_out_revisores",
        class: "control",
        kind: "fan-out",
        collection: "{{ .nodes.revisores.output.tasks }}",
        batch_size: 1,
        max_parallel: 1,
        max_fan_out: 2,
      },
      {
        id: "revisor",
        class: "action",
        kind: "run-agent",
        params: {
          agent: loopLegibilityAgent,
          prompt: "loop event probe",
          output_schema: {
            type: "object",
            required: ["summary", "message"],
            properties: {
              summary: { type: "string" },
              message: { type: "string" },
            },
          },
        },
      },
      { id: "collect", class: "control", kind: "collect" },
    ],
    edges: [
      { from: "revisores", to: "fan_out_revisores" },
      { from: "fan_out_revisores", to: "revisor" },
      { from: "revisor", to: "collect" },
    ],
  } as LoopDefinition["graph"],
  start: [{ kind: "http" }],
};

interface SeededLoopRecords {
  runId: string;
  loopName: string;
  coordinatorTaskId: string;
  cellTaskId: string;
  workspaceId: string;
}

/**
 * Publishes the loop, starts one run, and waits until the daemon has actually
 * materialized its coordinator and cell task records. Everything downstream
 * asserts against records the runtime really created — no fabricated rows.
 */
async function prepareLoopWorkspace(
  runtime: BrowserRuntime,
  appPage: import("@playwright/test").Page
): Promise<{ id: string }> {
  if (!runtime.paths) {
    throw new Error("Loop legibility browser test requires launch-mode runtime paths");
  }
  const workspace =
    runtime.seeded.workspace ?? (await runtime.resolveWorkspace(tasksLoopWorkspaceRoot));
  await completeOnboardingIfPrompted(appPage);
  await appPage.reload({ waitUntil: "domcontentloaded" });
  await switchWorkspace(appPage, workspace.id, workspace.name);
  return { id: workspace.id };
}

async function seedLoopRecords(
  runtime: BrowserRuntime,
  appPage: import("@playwright/test").Page,
  prepared?: { id: string }
): Promise<SeededLoopRecords> {
  const workspace = prepared ?? (await prepareLoopWorkspace(runtime, appPage));
  const loopName = `${loopLegibilityName}-${randomUUID()}`;
  const definition: LoopDefinition = {
    ...loopLegibilityDefinition,
    meta: { ...loopLegibilityDefinition.meta, name: loopName },
  };

  const workspacePath = `/api/workspaces/${encodeURIComponent(workspace.id)}`;
  await runtime.requestJSON(`${workspacePath}/loops`, {
    method: "POST",
    body: JSON.stringify({ definition }),
  });
  const started = await runtime.requestJSON<RunLoopResult>(
    `${workspacePath}/loops/${encodeURIComponent(loopName)}/run`,
    { method: "POST", body: JSON.stringify({}) }
  );
  if (!started.run) {
    throw new Error("Loop legibility seed did not create a run");
  }

  const revealedPath = `/api/tasks?workspace=${encodeURIComponent(workspace.id)}&include_loop=true&loop_run_id=${encodeURIComponent(started.run.id)}&limit=200`;
  await expect
    .poll(
      async () => {
        const page = await runtime.requestJSON<{ tasks: Array<{ loop?: { role?: string } }> }>(
          revealedPath
        );
        const roles = page.tasks.map(task => task.loop?.role);
        return roles.includes("coordinator") && roles.includes("cell");
      },
      { timeout: 60_000 }
    )
    .toBe(true);

  const revealed = await runtime.requestJSON<{
    tasks: Array<{ id: string; loop?: { role?: string } }>;
  }>(revealedPath);
  const coordinator = revealed.tasks.find(task => task.loop?.role === "coordinator");
  const cell = revealed.tasks.find(task => task.loop?.role === "cell");
  if (!coordinator || !cell) {
    throw new Error(`Loop legibility seed produced no records: ${JSON.stringify(revealed)}`);
  }
  return {
    runId: started.run.id,
    loopName,
    coordinatorTaskId: coordinator.id,
    cellTaskId: cell.id,
    workspaceId: workspace.id,
  };
}

interface CatalogProbe {
  tasks: Array<{ id: string; loop?: unknown }>;
  page: { total: number };
  facets: { statuses: Array<{ count: number }> };
}

interface DashboardProbe {
  dashboard: { status_breakdown: Array<{ count: number }> };
}

interface InboxProbe {
  inbox: { groups: Array<{ items?: Array<{ task: { id: string } }> }> };
}

interface LoopProvenanceProbe {
  run_id: string;
  role: string;
  loop_name?: string;
}

interface RevealedCatalogProbe {
  tasks: Array<{ id: string; loop?: LoopProvenanceProbe }>;
}

/** Reads the calm catalog exactly as the surface does: no include flag at all. */
function calmCatalogPath(workspaceId: string): string {
  return `/api/tasks?workspace=${encodeURIComponent(workspaceId)}&limit=200`;
}

test.describe("Loop record legibility", () => {
  test.use({
    runtimeOptions: {
      seed: {
        workspace: { rootDir: tasksLoopWorkspaceRoot },
        mockAgents: [
          {
            agentName: loopLegibilityAgent,
            fixtureAgent: "loop_runtime_provenance",
            fixturePath: loopProvenanceFixture,
          },
        ],
      },
    },
  });

  // E2E-010 (US-001, US-001.EC-2, US-003, US-003.EC-1)
  test("Loop records leave the default Tasks list, board, dashboard and inbox", async ({
    appPage,
    browserArtifacts,
    runtime,
  }) => {
    const seeded = await seedLoopRecords(runtime, appPage);
    const tasksWin = await openAppWindow(appPage, "Tasks", "tasks");
    const tasksUI = tasksOperatorSelectors(tasksWin, appPage);

    // This workspace holds nothing but the loop run's records, so the calm read
    // is genuinely empty — no mechanical row leaks in to fill it (US-001.EC-2).
    const loopOnly = await runtime.requestJSON<CatalogProbe>(calmCatalogPath(seeded.workspaceId));
    expect(loopOnly.tasks).toHaveLength(0);
    expect(loopOnly.page.total).toBe(0);

    const trueEmpty = tasksWin.getByTestId("tasks-empty-state");
    await expect(trueEmpty).toBeVisible();
    await expect(trueEmpty).toContainText("No tasks yet");
    await expect(tasksWin.locator('[data-slot="task-loop-row"]')).toHaveCount(0);
    await expect(tasksUI.taskCard(seeded.coordinatorTaskId)).toHaveCount(0);
    await expect(tasksUI.taskCard(seeded.cellTaskId)).toHaveCount(0);
    await browserArtifacts.captureScreenshot("tasks-loop-only-true-empty", appPage);

    // Now give the workspace one real work item and prove the exclusion is a
    // filter, not an emptiness: work shows, loop records still do not.
    const workItem = await runtime.requestJSON<{ task: { id: string } }>("/api/tasks", {
      method: "POST",
      body: JSON.stringify({
        description: "Work item that must survive the Loop exclusion.",
        identifier: "TASK-CALM-1",
        owner: { kind: "human", ref: "qa-operator" },
        priority: "medium",
        scope: "workspace",
        title: "Review the loop legibility pass",
        workspace: seeded.workspaceId,
      }),
    });

    const calm = await runtime.requestJSON<CatalogProbe>(calmCatalogPath(seeded.workspaceId));
    expect(calm.tasks.map(task => task.id)).toEqual([workItem.task.id]);
    expect(calm.tasks.some(task => task.loop)).toBe(false);
    expect(calm.page.total).toBe(1);
    // Facets are computed over the same filtered set the rows come from, so a
    // group header can never claim more than the list can show (US-001.AC-3).
    expect(calm.facets.statuses.reduce((total, facet) => total + facet.count, 0)).toBe(
      calm.page.total
    );

    await appPage.reload({ waitUntil: "domcontentloaded" });
    const listWin = appWindow(appPage, "tasks");
    const listUI = tasksOperatorSelectors(listWin, appPage);
    await expect(listWin.getByTestId("tasks-list-surface")).toBeVisible();
    await expect(listUI.taskCard(workItem.task.id)).toBeVisible();
    await expect(listWin.locator('[data-slot="task-loop-row"]')).toHaveCount(0);
    await expect(listWin.getByTestId(`task-card-${seeded.coordinatorTaskId}`)).toHaveCount(0);
    await expect(listWin.getByTestId(`task-card-${seeded.cellTaskId}`)).toHaveCount(0);
    await browserArtifacts.captureScreenshot("tasks-loop-calm-default", appPage);

    // Second projection, same population: no loop cards on the board.
    await listUI.modeKanban.click();
    await expect(listWin.getByTestId("tasks-kanban-board")).toBeVisible();
    await expect(listWin.locator('[data-slot="task-loop-row"]')).toHaveCount(0);
    await expect(listWin.getByTestId(`task-card-${seeded.cellTaskId}`)).toHaveCount(0);
    await browserArtifacts.captureScreenshot("tasks-loop-kanban-default", appPage);

    // Aggregates answer the same question over the same population (US-003.AC-1).
    const dashboard = await runtime.requestJSON<DashboardProbe>(
      `/api/observe/tasks/dashboard?scope=workspace&workspace=${encodeURIComponent(seeded.workspaceId)}`
    );
    const breakdownTotal = dashboard.dashboard.status_breakdown.reduce(
      (total, entry) => total + entry.count,
      0
    );
    expect(breakdownTotal).toBe(1);

    await listUI.modeDashboard.click();
    await expect(listUI.dashboardView).toBeVisible();
    // Status-agnostic on purpose: what must hold is that the breakdown counts the
    // one work item and none of the loop records, whatever status it settles in.
    await expect(listWin.getByTestId("tasks-dashboard-status-breakdown-total")).toHaveText(
      "total 1"
    );
    await browserArtifacts.captureScreenshot("tasks-loop-dashboard-default", appPage);

    // The loop's escalations route through the loop lane, never the inbox
    // (US-003.AC-2 / EC-1).
    const inbox = await runtime.requestJSON<InboxProbe>(
      `/api/observe/tasks/inbox?workspace=${encodeURIComponent(seeded.workspaceId)}`
    );
    const inboxTaskIds = inbox.inbox.groups.flatMap(group =>
      (group.items ?? []).map(item => item.task.id)
    );
    expect(inboxTaskIds).not.toContain(seeded.coordinatorTaskId);
    expect(inboxTaskIds).not.toContain(seeded.cellTaskId);

    await listUI.modeInbox.click();
    await expect(listUI.inboxView).toBeVisible();
    await expect(listWin.getByTestId(`task-card-${seeded.cellTaskId}`)).toHaveCount(0);
    await expect(listWin.locator('[data-slot="task-loop-row"]')).toHaveCount(0);
  });

  // E2E-011 (US-002, US-002.AC-3, US-002.EC-1)
  test("the reveal filter states its own empty, distinguishes records, links to the run and never persists", async ({
    appPage,
    browserArtifacts,
    runtime,
  }) => {
    const prepared = await prepareLoopWorkspace(runtime, appPage);

    // Reveal-empty is asserted against a workspace that genuinely holds no Loop
    // records yet — before any run exists, not by filtering one away.
    const emptyWin = await openAppWindow(appPage, "Tasks", "tasks");
    await expect(emptyWin.getByTestId("tasks-records-filter-work")).toHaveAttribute(
      "aria-pressed",
      "true"
    );
    await emptyWin.getByTestId("tasks-records-filter-loop").click();
    const revealEmpty = emptyWin.getByTestId("tasks-list-surface-loop-empty");
    await expect(revealEmpty).toBeVisible();
    await expect(revealEmpty).toContainText("No loop records in this workspace");
    await expect(revealEmpty).toContainText(
      "Turn the filter back to work items to see your tasks."
    );
    // The filter-scoped empty replaces the generic one rather than sitting beside it.
    await expect(emptyWin.getByTestId("tasks-list-surface-empty")).toHaveCount(0);
    await expect(emptyWin.getByTestId("tasks-empty-state")).toHaveCount(0);
    await browserArtifacts.captureScreenshot("tasks-loop-reveal-empty", appPage);

    // Its action is the way out of the filter it named.
    await emptyWin.getByRole("button", { name: "Show work items" }).click();
    await expect(emptyWin.getByTestId("tasks-records-filter-work")).toHaveAttribute(
      "aria-pressed",
      "true"
    );
    await expect(emptyWin.getByTestId("tasks-list-surface-loop-empty")).toHaveCount(0);

    const seeded = await seedLoopRecords(runtime, appPage, prepared);
    await appPage.reload({ waitUntil: "domcontentloaded" });
    const tasksWin = appWindow(appPage, "tasks");

    await expect(tasksWin.getByTestId("tasks-records-filter-work")).toHaveAttribute(
      "aria-pressed",
      "true"
    );
    await tasksWin.getByTestId("tasks-records-filter-loop").click();

    const coordinatorRow = tasksWin.getByTestId(`task-loop-row-${seeded.coordinatorTaskId}`);
    await expect(coordinatorRow).toBeVisible();
    await expect(coordinatorRow.locator('[data-slot="task-loop-row-identity"]')).toContainText(
      seeded.loopName
    );
    // Plain words lead; the machine id is never the row's primary text.
    await expect(coordinatorRow.locator('[data-slot="task-loop-row-identity"]')).not.toContainText(
      seeded.coordinatorTaskId
    );
    await expect(coordinatorRow.locator('[data-slot="task-loop-row-role"]')).toHaveText("Loop run");
    const cellRow = tasksWin.getByTestId(`task-loop-row-${seeded.cellTaskId}`);
    await expect(cellRow.locator('[data-slot="task-loop-row-role"]')).toHaveText("Loop step");
    await expect(cellRow.locator('[data-slot="task-loop-row-identity"]')).toContainText("step ");
    await browserArtifacts.captureScreenshot("tasks-loop-revealed", appPage);

    // Activation lands on the run page — the observability home for loop work.
    await cellRow.locator("a").first().click();
    await expect(appPage).toHaveURL(new RegExp(`/loop-runs/${seeded.runId}`));
    await browserArtifacts.captureScreenshot("tasks-loop-revealed-run-page", appPage);

    // Revealing is an explicit act per context: coming back starts calm again.
    const returnedWin = await openAppWindow(appPage, "Tasks", "tasks");
    await expect(returnedWin.getByTestId("tasks-records-filter-work")).toHaveAttribute(
      "aria-pressed",
      "true"
    );
    await expect(returnedWin.getByTestId(`task-loop-row-${seeded.cellTaskId}`)).toHaveCount(0);
  });

  // E2E-020 (US-002.EC-2, US-015.AC-2)
  test("a revealed record links back to its run, and survives its run truthfully when it is gone", async ({
    appPage,
    browserArtifacts,
    runtime,
  }) => {
    // `go run` compiles the retention seeder cold on top of the run poll's own
    // budget, so this journey needs more than the 90s config default.
    test.setTimeout(180_000);

    const seeded = await seedLoopRecords(runtime, appPage);

    // The retention boundary: task and run facts are retained, the owning
    // `loop_runs` row and `loop_name` are not. The task id is deliberately not
    // shaped like a Loop cell id, so nothing here can be recovered by parsing —
    // the record reads as a Loop step purely from projected provenance.
    const retained = await seedRetainedLoopTask(runtime, {
      loopRunId: RETAINED_LOOP_RUN_ID,
      runId: "run-retained-loop-record",
      taskId: RETAINED_TASK_ID,
      workspaceId: seeded.workspaceId,
    });

    // The wire contract both surfaces read from.
    const revealed = await runtime.requestJSON<RevealedCatalogProbe>(
      `/api/tasks?workspace=${encodeURIComponent(seeded.workspaceId)}&include_loop=true&limit=200`
    );
    const retainedItem = revealed.tasks.find(task => task.id === retained.taskId);
    if (!retainedItem?.loop) {
      throw new Error(`retained record carried no Loop provenance: ${JSON.stringify(revealed)}`);
    }
    expect(retainedItem.loop.run_id).toBe(retained.loopRunId);
    expect(retainedItem.loop.role).toBe("cell");
    // Omission is the contract, so assert the key is absent — not merely undefined.
    expect("loop_name" in retainedItem.loop).toBe(false);

    // The deep link must not depend on list data (B-003).
    const detailRead = await runtime.requestJSON<{
      task: { task: { loop?: LoopProvenanceProbe } };
    }>(`/api/tasks/${encodeURIComponent(retained.taskId)}`);
    const detailLoop = detailRead.task.task.loop;
    if (!detailLoop) {
      throw new Error(`single-task read carried no Loop provenance: ${JSON.stringify(detailRead)}`);
    }
    expect(detailLoop.run_id).toBe(retained.loopRunId);
    expect(detailLoop.role).toBe("cell");
    expect("loop_name" in detailLoop).toBe(false);

    // Retention does not resurrect the record into the default population.
    const calm = await runtime.requestJSON<RevealedCatalogProbe>(
      calmCatalogPath(seeded.workspaceId)
    );
    expect(calm.tasks.map(task => task.id)).not.toContain(retained.taskId);

    // Revealed rows: one links, one says why it cannot.
    await appPage.reload({ waitUntil: "domcontentloaded" });
    const tasksWin = await openAppWindow(appPage, "Tasks", "tasks");
    await expect(tasksWin.getByTestId("tasks-list-surface")).toBeVisible();
    await tasksWin.getByTestId("tasks-records-filter-loop").click();

    const liveRow = tasksWin.getByTestId(`task-loop-row-${seeded.cellTaskId}`);
    await expect(liveRow).toBeVisible();
    await expect(liveRow.locator("a")).toHaveCount(1);

    const retainedRow = tasksWin.getByTestId(`task-loop-row-${retained.taskId}`);
    await expect(retainedRow).toBeVisible();
    // Without a loop name the row leads with the generic entity name.
    await expect(retainedRow.locator('[data-slot="task-loop-row-identity"]')).toHaveText(
      "Loop step"
    );
    await expect(retainedRow.getByTestId(`task-loop-row-run-gone-${retained.taskId}`)).toHaveText(
      "Run no longer available"
    );
    // The degrade is the absence of a link, never a dead one.
    await expect(retainedRow.locator("a")).toHaveCount(0);
    await expect(retainedRow.locator('[data-slot="task-loop-row-run-id"]')).toContainText(
      retained.loopRunId
    );
    await browserArtifacts.captureScreenshot("tasks-loop-revealed-run-gone", appPage);

    // Detail of the live record: provenance plus a working way back.
    await appPage.goto(runtime.url(`/tasks/${encodeURIComponent(seeded.cellTaskId)}`), {
      waitUntil: "domcontentloaded",
    });
    const liveDetailWin = appWindow(appPage, "tasks");
    const liveProvenance = liveDetailWin.getByTestId("task-loop-provenance");
    await expect(liveProvenance).toBeVisible();
    await expect(liveProvenance).toContainText("Loop step");
    await expect(liveProvenance).toContainText(seeded.loopName);
    await expect(liveProvenance).toContainText(seeded.runId);
    await expect(liveDetailWin.getByTestId("task-loop-provenance-run-gone")).toHaveCount(0);
    await browserArtifacts.captureScreenshot("tasks-loop-detail-provenance", appPage);

    const openRun = liveDetailWin.getByTestId("task-loop-provenance-open-run");
    await expect(openRun).toHaveText(/Open run/);
    await openRun.click();
    await expect(appPage).toHaveURL(new RegExp(`/loop-runs/${seeded.runId}`));

    // Detail of the retained record: the same block, degraded.
    await appPage.goto(runtime.url(`/tasks/${encodeURIComponent(retained.taskId)}`), {
      waitUntil: "domcontentloaded",
    });
    const goneDetailWin = appWindow(appPage, "tasks");
    const goneProvenance = goneDetailWin.getByTestId("task-loop-provenance");
    await expect(goneProvenance).toBeVisible();
    await expect(goneProvenance).toContainText("Loop step");
    await expect(goneDetailWin.getByTestId("task-loop-provenance-run-gone")).toHaveText(
      "Run no longer available"
    );
    await expect(goneDetailWin.getByTestId("task-loop-provenance-open-run")).toHaveCount(0);
    await expect(goneProvenance.locator("a")).toHaveCount(0);
    // Provenance outlives the run: the id still identifies the record.
    await expect(goneProvenance).toContainText(retained.loopRunId);
    // Nothing is fabricated to fill the gaps the deleted run left behind.
    await expect(goneProvenance).not.toContainText("Round");
    await expect(goneProvenance).not.toContainText("Step");
    await expect(goneProvenance).not.toContainText("Item");
    await browserArtifacts.captureScreenshot("tasks-loop-detail-run-gone", appPage);
  });
});
