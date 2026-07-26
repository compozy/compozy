import { execFile } from "node:child_process";
import { randomUUID } from "node:crypto";
import { mkdtemp, readFile } from "node:fs/promises";
import os from "node:os";
import path from "node:path";
import process from "node:process";
import { fileURLToPath } from "node:url";
import { promisify } from "node:util";

import type { Page } from "@playwright/test";

import { reloadDaemonServedPage } from "../fixtures/navigation";
import { automationOperatorSelectors, sessionWindowSelectors } from "../fixtures/selectors";
import { sessionWindow, windowTitle } from "../fixtures/os-navigation";
import type { BrowserRuntime } from "../fixtures/runtime";
import { browserAutomationOperatorFlowScenario } from "../fixtures/runtime";
import { expect, test } from "../fixtures/test";
import { useGlobalWorkspaceIfPrompted } from "../fixtures/workspace";

const execFileAsync = promisify(execFile);
const automationFixture = path.resolve(
  path.dirname(fileURLToPath(import.meta.url)),
  "..",
  "..",
  "..",
  "internal",
  "testutil",
  "acpmock",
  "testdata",
  "automation_task_fixture.json"
);
const faultFixture = path.resolve(
  path.dirname(fileURLToPath(import.meta.url)),
  "..",
  "..",
  "..",
  "internal",
  "testutil",
  "acpmock",
  "testdata",
  "driver_fault_fixture.json"
);
const automationAgentName = "browser-jobs-runner";
const faultAgentName = "browser-jobs-fault";
const sensitivePattern =
  /agh_claim_|["']claim_token["']\s*:|mcp[_-]?auth|telegram-bot-token|pkce|oauth|webhook_secret|provider[_-]?credentials?["'\s]*[:=]|browser-job-secret|shared-secret/i;

async function enableCronExpressionEditing(
  ui: ReturnType<typeof automationOperatorSelectors>
): Promise<void> {
  await ui.jobScheduleCustom.click();
  await expect(ui.jobScheduleExpr).toBeEditable();
}

interface AutomationJob {
  id: string;
  agent_name: string;
  enabled: boolean;
  name: string;
  next_run?: string | null;
  prompt: string;
  schedule?: AutomationSchedule | null;
  scheduler?: AutomationSchedulerState | null;
  scope: "global" | "workspace";
  source: "dynamic" | "config";
  workspace_id?: string | null;
}

interface AutomationRun {
  id: string;
  attempt: number;
  delivery_error?: string | null;
  error?: string | null;
  fire_id?: string | null;
  job_id?: string | null;
  scheduled_at?: string | null;
  session_id?: string | null;
  status: "scheduled" | "running" | "delegated" | "completed" | "failed" | "canceled";
  workspace_id?: string | null;
}

interface AutomationSchedule {
  expr?: string;
  interval?: string;
  mode: "cron" | "every" | "at";
  time?: string;
}

interface AutomationSchedulerState {
  job_id: string;
  last_fire_id?: string;
  last_scheduled_at?: string | null;
  next_run_at?: string | null;
  registered: boolean;
}

interface AutomationHealth {
  automation: {
    scheduled_jobs?: AutomationSchedulerState[];
    scheduler_running: boolean;
  };
}

interface SettingsRestartAction {
  operation_id: string;
  status_url: string;
}

interface SettingsRestartStatus {
  status: string;
}

interface WorkspacePayload {
  id: string;
  name: string;
  root_dir: string;
}

interface JobRequest {
  agent_name: string;
  enabled: boolean;
  fire_limit: { max: number; window: string };
  name: string;
  prompt: string;
  retry: { base_delay: string; max_retries: number; strategy: "none" | "backoff" };
  schedule: AutomationSchedule;
  scope: "global" | "workspace";
  workspace_id?: string;
}

interface JobsResponse {
  jobs: AutomationJob[];
}

interface JobResponse {
  job: AutomationJob;
}

interface RunResponse {
  run: AutomationRun;
}

interface RunsResponse {
  runs: AutomationRun[];
}

test.use({
  runtimeOptions: {
    seed: {
      mockAgents: [
        {
          agentName: automationAgentName,
          fixtureAgent: "automation-runner",
          fixturePath: automationFixture,
        },
        {
          agentName: faultAgentName,
          fixtureAgent: "faulty",
          fixturePath: faultFixture,
        },
      ],
    },
  },
});

test("operator creates edits disables enables triggers and deletes a dynamic job with parity evidence", async ({
  appPage,
  browserArtifacts,
  runtime,
}) => {
  const jobsWin = appPage.getByTestId("os-window-app:jobs");
  const ui = automationOperatorSelectors(jobsWin, appPage);
  const shellUI = automationOperatorSelectors(appPage);
  const jobStatus = jobsWin.locator('[data-slot="topbar-status"]');
  const workspace = await createWorkspace(runtime);
  const workspaceJob = await createJob(runtime, workspaceJobRequest(workspace));
  await useGlobalWorkspaceIfPrompted(shellUI);

  await appPage.goto(runtime.url("/jobs"), { waitUntil: "domcontentloaded" });
  await expect(ui.jobsShell).toBeVisible();
  await expect(ui.jobsListRows).toBeVisible();
  await expect(ui.item(workspaceJob.id)).toBeVisible();
  await ui.itemLink(workspaceJob.id).click();
  await expect(appPage).toHaveURL(new RegExp(`/jobs/${workspaceJob.id}$`));
  await expect(ui.detailPanel).toContainText("Scope: WORKSPACE");

  await jobsWin
    .getByRole("navigation", { name: "Window path" })
    .getByRole("button", { exact: true, name: "Jobs" })
    .click();
  await expect(appPage).toHaveURL(/\/jobs$/);
  await expect(ui.jobsShell).toBeVisible();
  await ui.createJobButton.click();
  await expect(ui.editorDialog).toBeVisible();
  await expect(ui.submitJobForm).toBeDisabled();
  const initialName = uniqueName("jobs-lifecycle");
  await ui.jobNameInput.fill(initialName);
  await ui.jobAgentInput.fill(automationAgentName);
  await ui.jobPromptInput.fill(browserAutomationOperatorFlowScenario.job.prompt);
  await enableCronExpressionEditing(ui);
  await ui.jobScheduleExpr.fill("");
  await expect(ui.submitJobForm).toBeDisabled();
  await ui.jobScheduleExpr.fill(browserAutomationOperatorFlowScenario.job.scheduleExpr);
  await ui.jobGovernanceToggle.click();
  await expect(ui.jobFireLimitMax).toBeVisible();
  await ui.jobFireLimitMax.fill("24");
  await ui.jobFireLimitWindow.fill("1h");

  const createResponse = appPage.waitForResponse(
    response =>
      response.request().method() === "POST" && response.url().endsWith("/api/automation/jobs")
  );
  await ui.submitJobForm.click();
  expect((await createResponse).ok()).toBe(true);
  await expect(ui.editorDialog).toBeHidden();
  const created = await waitForJobByName(runtime, initialName);
  // Saving from the editor navigates straight to the new job's detail route.
  await expect(appPage).toHaveURL(new RegExp(`/jobs/${created.id}$`));
  await expect(windowTitle(jobsWin)).toContainText(initialName);
  await expect(ui.detailPanel).toContainText("REGISTERED");

  await ui.detailOverflow.click();
  await appPage.getByTestId("edit-automation-btn").click();
  await expect(ui.editorDialog).toBeVisible();
  const editedName = `${initialName}-edited`;
  await ui.jobNameInput.fill(editedName);
  await ui.jobPromptInput.fill(browserAutomationOperatorFlowScenario.job.prompt);
  await enableCronExpressionEditing(ui);
  await ui.jobScheduleExpr.fill(browserAutomationOperatorFlowScenario.job.updatedScheduleExpr);
  const updateResponse = appPage.waitForResponse(
    response =>
      response.request().method() === "PATCH" &&
      response.url().endsWith(`/api/automation/jobs/${encodeURIComponent(created.id)}`)
  );
  await ui.submitJobForm.click();
  expect((await updateResponse).ok()).toBe(true);
  await expect(ui.editorDialog).toBeHidden();
  await expect(windowTitle(jobsWin)).toContainText(editedName);

  await ui.detailOverflow.click();
  await ui.toggleAutomationButton.click();
  await expect.poll(async () => (await getJob(runtime, created.id)).job.enabled).toBe(false);
  await expect(jobStatus).toContainText("DISABLED");
  let disabledHealth = await getAutomationHealth(runtime);
  expect(
    disabledHealth.automation.scheduled_jobs?.some(
      state => state.job_id === created.id && state.registered
    )
  ).toBe(false);

  await ui.detailOverflow.click();
  await ui.toggleAutomationButton.click();
  await expect.poll(async () => (await getJob(runtime, created.id)).job.enabled).toBe(true);
  await expect(jobStatus).toContainText("ENABLED");
  await expect
    .poll(async () => schedulerState(runtime, created.id).then(state => state?.registered))
    .toBe(true);

  await ui.triggerJobButton.click();
  const completedRun = await waitForLatestRun(runtime, created.id, "completed");
  await expect(ui.run(completedRun.id)).toBeVisible();
  await expect(ui.runSessionLink(completedRun.id)).toBeVisible();

  const parity = await captureJobParity(runtime, created.id, completedRun.id);
  expect(parity.http.job.name).toBe(editedName);
  expect(parity.http.job.enabled).toBe(true);
  expect(parity.uds.job.name).toBe(editedName);
  expect(parity.cliGet.id).toBe(created.id);
  expect(parity.cliHistory.runs.some(run => run.id === completedRun.id)).toBe(true);
  expect(parity.health.automation.scheduler_running).toBe(true);

  await assertJobsLifecycleViewportMatrix(appPage, browserArtifacts, runtime, created.id);
  await runtime.artifactCollector.captureJSON("browser_api_snapshots", parity);
  await browserArtifacts.captureScreenshot("jobs-lifecycle-history", appPage);
  await browserArtifacts.persist(appPage);
  const routeState = await readRouteState(runtime);
  expect(routeState).toMatchObject({
    automation_active_tab: "jobs",
    automation_detail_overflow_visible: true,
    automation_run_history_visible: true,
    automation_scheduler_visible: true,
    automation_selected_item: editedName,
    automation_session_link_count: expect.any(Number),
    automation_trigger_visible: true,
    automation_view_visible: true,
  });

  await ui.runSessionLink(completedRun.id).click();
  const completedSessionId = completedRun.session_id;
  if (!completedSessionId) {
    throw new Error("Expected the completed job run to expose a linked session.");
  }
  const sessionUI = sessionWindowSelectors(sessionWindow(appPage, completedSessionId));
  await expect(sessionUI.chatView).toBeVisible();
  await expect(sessionUI.chatView).toContainText(browserAutomationOperatorFlowScenario.job.prompt);
  await appPage.goto(runtime.url("/jobs"), { waitUntil: "domcontentloaded" });
  await ui.itemLink(created.id).click();
  await expect(appPage).toHaveURL(new RegExp(`/jobs/${created.id}$`));

  await ui.detailOverflow.click();
  await ui.deleteAutomationButton.click();
  await expect(ui.automationDeleteDialog).toBeVisible();
  await ui.automationDeleteConfirmTyping.fill(editedName);
  const deleteResponsePromise = appPage.waitForResponse(
    response =>
      response.request().method() === "DELETE" &&
      new URL(response.url()).pathname === `/api/automation/jobs/${created.id}`
  );
  await ui.confirmDeleteAutomationButton.click();
  const deleteResponse = await deleteResponsePromise;
  expect(deleteResponse.ok()).toBe(true);
  await expect(ui.automationDeleteDialog).toBeHidden();
  await expect.poll(async () => await getJobStatus(runtime, created.id)).toBe(404);
  const afterDelete = await automationCLI<JobsResponse>(runtime, [
    "automation",
    "jobs",
    "--limit",
    "50",
  ]);
  expect(afterDelete.jobs.some(job => job.id === created.id)).toBe(false);
  disabledHealth = await getAutomationHealth(runtime);
  expect(disabledHealth.automation.scheduled_jobs?.some(state => state.job_id === created.id)).toBe(
    false
  );

  await assertNoJobSensitiveLeak(appPage, runtime, {
    afterDelete,
    parity,
    routeState,
    workspaceJob,
  });
  await deleteSessionIfExists(runtime, completedRun.workspace_id, completedRun.session_id);
  await deleteJobIfExists(runtime, workspaceJob.id);
});

test("scheduled job survives daemon restart and does not duplicate fire ids", async ({
  appPage,
  browserArtifacts,
  runtime,
}) => {
  const jobsWin = appPage.getByTestId("os-window-app:jobs");
  const ui = automationOperatorSelectors(jobsWin, appPage);
  const shellUI = automationOperatorSelectors(appPage);
  const job = await createJob(
    runtime,
    jobRequest({
      fireLimit: { max: 100, window: "2m" },
      name: uniqueName("jobs-restart"),
      schedule: { mode: "every", interval: "1s" },
    })
  );
  await useGlobalWorkspaceIfPrompted(shellUI);
  await appPage.goto(runtime.url("/jobs"), { waitUntil: "domcontentloaded" });
  await expect(ui.jobsListRows).toBeVisible();
  await expect(ui.item(job.id)).toBeVisible({ timeout: 20_000 });
  await ui.itemLink(job.id).click();
  await expect(appPage).toHaveURL(new RegExp(`/jobs/${job.id}$`));
  await expect(ui.detailPanel).toContainText("REGISTERED");

  const beforeRestart = await waitForScheduledRuns(runtime, job.id, 1);
  const beforeFireIDs = uniqueFireIDs(beforeRestart);
  expect(new Set(beforeFireIDs).size).toBe(beforeFireIDs.length);

  const restart = await runtime.requestJSON<SettingsRestartAction>(
    "/api/settings/actions/restart",
    {
      method: "POST",
      body: "{}",
    }
  );
  expect(restart.operation_id).toMatch(
    /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i
  );
  await expect
    .poll(async () => await pollRestartStatus(runtime, restart.status_url), {
      timeout: 45_000,
    })
    .toBe("ready");
  await reloadDaemonServedPage(appPage, runtime, "/jobs", { readyTestId: "jobs-shell" });
  await expect(ui.jobsShell).toBeVisible();
  await expect(ui.item(job.id)).toBeVisible({ timeout: 20_000 });
  await ui.itemLink(job.id).click();

  const afterRestart = await waitForScheduledRuns(runtime, job.id, 2);
  const fireIDs = uniqueFireIDs(afterRestart);
  expect(fireIDs.length).toBeGreaterThanOrEqual(2);
  expect(new Set(fireIDs).size).toBe(fireIDs.length);
  const state = await schedulerState(runtime, job.id);
  expect(state).toMatchObject({ job_id: job.id, registered: true });
  expect(state?.last_fire_id).toBeTruthy();

  const parity = await captureJobParity(runtime, job.id, afterRestart[0].id);
  expect(parity.httpRuns.runs.filter(run => run.scheduled_at).length).toBeGreaterThanOrEqual(2);
  expect(parity.cliHistory.runs.filter(run => run.fire_id).length).toBeGreaterThanOrEqual(2);
  expect(parity.uds.job.scheduler?.registered).toBe(true);

  await runtime.artifactCollector.captureJSON("browser_api_snapshots", parity);
  await browserArtifacts.captureScreenshot("jobs-restart-scheduled-history", appPage);
  await browserArtifacts.persist(appPage);
  await assertNoJobSensitiveLeak(appPage, runtime, parity);
});

test("failed job run is diagnosable from browser and CLI without leaking secrets", async ({
  appPage,
  browserArtifacts,
  runtime,
}) => {
  const jobsWin = appPage.getByTestId("os-window-app:jobs");
  const ui = automationOperatorSelectors(jobsWin, appPage);
  const shellUI = automationOperatorSelectors(appPage);
  const job = await createJob(
    runtime,
    jobRequest({
      agentName: faultAgentName,
      name: uniqueName("jobs-failure"),
      prompt: "trigger crash mid-stream",
    })
  );
  await useGlobalWorkspaceIfPrompted(shellUI);
  await appPage.goto(runtime.url("/jobs"), { waitUntil: "domcontentloaded" });
  await expect(ui.item(job.id)).toBeVisible();
  await ui.itemLink(job.id).click();

  const trigger = await fetch(
    runtime.url(`/api/automation/jobs/${encodeURIComponent(job.id)}/trigger`),
    {
      method: "POST",
    }
  );
  expect(await trigger.text()).not.toMatch(sensitivePattern);
  const failedRun = await waitForLatestRun(runtime, job.id, "failed");
  await appPage.reload({ waitUntil: "domcontentloaded" });
  await expect(windowTitle(jobsWin)).toContainText(job.name, { timeout: 20_000 });
  await expect(ui.run(failedRun.id)).toBeVisible();
  await expect(ui.run(failedRun.id)).toContainText("FAILED");
  await expect(ui.run(failedRun.id)).toContainText(/disconnect|prompt|session|failed/i);

  const parity = await captureJobParity(runtime, job.id, failedRun.id);
  expect(parity.httpRun.run.status).toBe("failed");
  expect(parity.httpRun.run.error || parity.httpRun.run.delivery_error).toBeTruthy();
  expect(parity.cliRun.status).toBe("failed");
  expect(
    parity.cliHistory.runs.some(run => run.id === failedRun.id && run.status === "failed")
  ).toBe(true);

  await assertJobsViewportMatrix(appPage, browserArtifacts, runtime, job.id);
  await runtime.artifactCollector.captureJSON("browser_api_snapshots", parity);
  await browserArtifacts.captureScreenshot("jobs-failure-diagnostics", appPage);
  await browserArtifacts.persist(appPage);
  const routeState = await readRouteState(runtime);
  expect(routeState).toMatchObject({
    automation_run_count: expect.any(Number),
    automation_run_history_visible: true,
    automation_selected_item: job.name,
    automation_view_visible: true,
  });
  await assertNoJobSensitiveLeak(appPage, runtime, {
    parity,
    routeState,
    triggerStatus: trigger.status,
  });
});

async function createWorkspace(runtime: BrowserRuntime): Promise<WorkspacePayload> {
  const rootDir = await mkdtemp(path.join(os.tmpdir(), "agh-browser-jobs-workspace-"));
  return await runtime.resolveWorkspace(rootDir);
}

function workspaceJobRequest(workspace: WorkspacePayload): JobRequest {
  return jobRequest({
    name: uniqueName("jobs-workspace-scope"),
    scope: "workspace",
    workspaceID: workspace.id,
  });
}

function jobRequest(input: {
  agentName?: string;
  fireLimit?: { max: number; window: string };
  name: string;
  prompt?: string;
  schedule?: AutomationSchedule;
  scope?: "global" | "workspace";
  workspaceID?: string;
}): JobRequest {
  return {
    agent_name: input.agentName ?? automationAgentName,
    enabled: true,
    fire_limit: input.fireLimit ?? { max: 24, window: "1h" },
    name: input.name,
    prompt: input.prompt ?? browserAutomationOperatorFlowScenario.job.prompt,
    retry: { strategy: "none", max_retries: 0, base_delay: "" },
    schedule: input.schedule ?? {
      mode: "cron",
      expr: browserAutomationOperatorFlowScenario.job.scheduleExpr,
    },
    scope: input.scope ?? "global",
    workspace_id: input.workspaceID,
  };
}

async function createJob(runtime: BrowserRuntime, request: JobRequest): Promise<AutomationJob> {
  return (
    await runtime.requestJSON<JobResponse>("/api/automation/jobs", {
      method: "POST",
      body: JSON.stringify(request),
    })
  ).job;
}

async function getJob(runtime: BrowserRuntime, id: string): Promise<JobResponse> {
  return await runtime.requestJSON<JobResponse>(`/api/automation/jobs/${encodeURIComponent(id)}`);
}

async function getJobStatus(runtime: BrowserRuntime, id: string): Promise<number> {
  const response = await fetch(runtime.url(`/api/automation/jobs/${encodeURIComponent(id)}`));
  return response.status;
}

async function deleteJobIfExists(runtime: BrowserRuntime, id: string): Promise<void> {
  const response = await fetch(runtime.url(`/api/automation/jobs/${encodeURIComponent(id)}`), {
    method: "DELETE",
  });
  expect([204, 404]).toContain(response.status);
}

async function deleteSessionIfExists(
  runtime: BrowserRuntime,
  workspaceID: string | null | undefined,
  id: string | null | undefined
): Promise<void> {
  if (!id) {
    return;
  }
  const workspace = await resolveSessionWorkspaceID(runtime, workspaceID, id);
  const response = await fetch(runtime.url(sessionAPIPath(workspace, id)), {
    method: "DELETE",
  });
  expect([204, 404]).toContain(response.status);
}

async function resolveSessionWorkspaceID(
  runtime: BrowserRuntime,
  workspaceID: string | null | undefined,
  sessionID: string
): Promise<string> {
  const workspace = workspaceID?.trim();
  if (workspace) {
    return workspace;
  }

  const seededWorkspace = runtime.seeded.workspace?.id?.trim();
  if (seededWorkspace) {
    return seededWorkspace;
  }

  if (runtime.paths?.homeDir) {
    return (await runtime.resolveWorkspace(runtime.paths.homeDir)).id;
  }

  throw new Error(`delete session ${sessionID} requires workspace_id`);
}

function sessionAPIPath(workspaceID: string, sessionID: string): string {
  return `/api/workspaces/${encodeURIComponent(workspaceID)}/sessions/${encodeURIComponent(sessionID)}`;
}

async function getAutomationHealth(runtime: BrowserRuntime): Promise<AutomationHealth> {
  return await runtime.requestJSON<AutomationHealth>("/api/status");
}

async function schedulerState(
  runtime: BrowserRuntime,
  jobID: string
): Promise<AutomationSchedulerState | undefined> {
  const health = await getAutomationHealth(runtime);
  return health.automation.scheduled_jobs?.find(state => state.job_id === jobID);
}

async function waitForJobByName(runtime: BrowserRuntime, name: string): Promise<AutomationJob> {
  let matched: AutomationJob | undefined;
  await expect
    .poll(async () => {
      const payload = await runtime.requestJSON<JobsResponse>("/api/automation/jobs?limit=50");
      matched = payload.jobs.find(job => job.name === name);
      return matched?.id ?? "";
    })
    .not.toBe("");
  if (!matched) {
    throw new Error(`Expected automation job ${name} to be visible.`);
  }
  return matched;
}

async function waitForLatestRun(
  runtime: BrowserRuntime,
  jobID: string,
  status: AutomationRun["status"]
): Promise<AutomationRun> {
  let matched: AutomationRun | undefined;
  await expect
    .poll(
      async () => {
        const runs = await listJobRuns(runtime, jobID);
        matched = runs.find(run => run.status === status);
        return matched?.id ?? "";
      },
      { timeout: 30_000 }
    )
    .not.toBe("");
  if (!matched) {
    throw new Error(`Expected ${status} automation run for job ${jobID}.`);
  }
  return matched;
}

async function waitForScheduledRuns(
  runtime: BrowserRuntime,
  jobID: string,
  minimumCount: number
): Promise<AutomationRun[]> {
  let matched: AutomationRun[] = [];
  await expect
    .poll(
      async () => {
        matched = (await listJobRuns(runtime, jobID)).filter(run => Boolean(run.fire_id));
        return matched.length;
      },
      { timeout: 45_000 }
    )
    .toBeGreaterThanOrEqual(minimumCount);
  return matched;
}

async function listJobRuns(runtime: BrowserRuntime, jobID: string): Promise<AutomationRun[]> {
  return (
    await runtime.requestJSON<RunsResponse>(
      `/api/automation/jobs/${encodeURIComponent(jobID)}/runs?limit=50`
    )
  ).runs;
}

async function captureJobParity(runtime: BrowserRuntime, jobID: string, runID: string) {
  const http = await getJob(runtime, jobID);
  const uds = await requestOperatorJSONOrThrow<JobResponse>(
    runtime,
    `/api/automation/jobs/${encodeURIComponent(jobID)}`
  );
  const httpRuns = await runtime.requestJSON<RunsResponse>(
    `/api/automation/jobs/${encodeURIComponent(jobID)}/runs?limit=50`
  );
  const httpRun = await runtime.requestJSON<RunResponse>(
    `/api/automation/runs/${encodeURIComponent(runID)}`
  );
  const cliGet = await automationCLI<AutomationJob>(runtime, ["automation", "jobs", "get", jobID]);
  const cliHistory = await automationCLI<RunsResponse>(runtime, [
    "automation",
    "jobs",
    "history",
    jobID,
    "--last",
    "50",
  ]);
  const cliRun = await automationCLI<AutomationRun>(runtime, ["automation", "runs", "get", runID]);
  const health = await getAutomationHealth(runtime);
  return {
    cliGet,
    cliHistory,
    cliRun,
    health,
    http,
    httpRun,
    httpRuns,
    uds,
  };
}

async function automationCLI<T>(runtime: BrowserRuntime, args: string[]): Promise<T> {
  if (!runtime.paths) {
    throw new Error("automation CLI checks require launch-mode runtime paths.");
  }
  const { stdout } = await execFileAsync(runtime.paths.cliShim, [...args, "-o", "json"], {
    env: cliEnv(runtime.paths),
  });
  expect(stdout).not.toMatch(sensitivePattern);
  return JSON.parse(stdout) as T;
}

async function requestOperatorJSONOrThrow<T>(
  runtime: BrowserRuntime,
  pathname: string,
  init?: RequestInit
): Promise<T> {
  if (!runtime.requestOperatorJSON) {
    throw new Error("operator UDS parity checks require requestOperatorJSON.");
  }
  return await runtime.requestOperatorJSON<T>(pathname, init);
}

async function pollRestartStatus(runtime: BrowserRuntime, statusURL: string): Promise<string> {
  try {
    return (await runtime.requestJSON<SettingsRestartStatus>(statusURL)).status;
  } catch {
    return "restarting";
  }
}

async function assertJobsLifecycleViewportMatrix(
  appPage: Page,
  browserArtifacts: { captureScreenshot: (name: string, page?: Page) => Promise<unknown> },
  runtime: BrowserRuntime,
  jobID: string
): Promise<void> {
  const jobsWin = appPage.getByTestId("os-window-app:jobs");
  const ui = automationOperatorSelectors(jobsWin, appPage);
  for (const width of [375, 768, 1280]) {
    await appPage.setViewportSize({ width, height: 820 });
    await appPage.goto(runtime.url("/jobs"), { waitUntil: "domcontentloaded" });
    await expect(ui.jobsShell).toBeVisible();
    await expect(ui.item(jobID)).toBeVisible();
    await ui.itemLink(jobID).click();
    await expect(ui.runHistory).toBeVisible();
    await browserArtifacts.captureScreenshot(`jobs-lifecycle-history-viewport-${width}`, appPage);
    await ui.detailOverflow.click();
    await appPage.getByTestId("edit-automation-btn").click();
    await expect(ui.editorDialog).toBeVisible();
    await expect(ui.jobForm).toBeVisible();
    await expect(ui.jobScheduleExpr).toBeVisible();
    await expect(ui.submitJobForm).toBeEnabled();
    await browserArtifacts.captureScreenshot(`jobs-lifecycle-editor-viewport-${width}`, appPage);
    await appPage.keyboard.press("Escape");
    await expect(ui.editorDialog).toBeHidden();
  }
}

async function assertJobsViewportMatrix(
  appPage: Page,
  browserArtifacts: { captureScreenshot: (name: string, page?: Page) => Promise<unknown> },
  runtime: BrowserRuntime,
  jobID: string
): Promise<void> {
  const jobsWin = appPage.getByTestId("os-window-app:jobs");
  const ui = automationOperatorSelectors(jobsWin, appPage);
  for (const width of [375, 768, 1280]) {
    await appPage.setViewportSize({ width, height: 820 });
    await appPage.goto(runtime.url("/jobs"), { waitUntil: "domcontentloaded" });
    await expect(ui.jobsShell).toBeVisible();
    await expect(ui.item(jobID)).toBeVisible();
    await ui.itemLink(jobID).click();
    await expect(ui.detailPanel).toBeVisible();
    await expect(ui.runHistory).toBeVisible();
    await browserArtifacts.captureScreenshot(`jobs-failure-viewport-${width}`, appPage);
  }
}

async function assertNoJobSensitiveLeak(
  appPage: Page,
  runtime: BrowserRuntime,
  snapshot: unknown
): Promise<void> {
  expect(JSON.stringify(snapshot)).not.toMatch(sensitivePattern);
  expect((await appPage.textContent("body")) ?? "").not.toMatch(sensitivePattern);
  const consolePath = runtime.artifactCollector.artifactPath("browser_console");
  const networkPath = runtime.artifactCollector.artifactPath("browser_network");
  const routeStatePath = runtime.artifactCollector.artifactPath("browser_route_state");
  const apiSnapshotPath = runtime.artifactCollector.artifactPath("browser_api_snapshots");
  await expect(readFileIfExists(consolePath)).resolves.not.toMatch(sensitivePattern);
  await expect(readFileIfExists(networkPath)).resolves.not.toMatch(sensitivePattern);
  await expect(readFileIfExists(routeStatePath)).resolves.not.toMatch(sensitivePattern);
  await expect(readFileIfExists(apiSnapshotPath)).resolves.not.toMatch(sensitivePattern);
  if (runtime.paths?.daemonLog) {
    await expect(readFileIfExists(runtime.paths.daemonLog)).resolves.not.toMatch(sensitivePattern);
  }
}

async function readFileIfExists(filePath: string): Promise<string> {
  try {
    return await readFile(filePath, "utf8");
  } catch (error) {
    const maybeNodeError = error as NodeJS.ErrnoException;
    if (maybeNodeError.code === "ENOENT") {
      return "";
    }
    throw error;
  }
}

function cliEnv(paths: { cliShim: string; homeDir: string }): NodeJS.ProcessEnv {
  return {
    ...process.env,
    AGH_HOME: paths.homeDir,
    HOME: paths.homeDir,
    PATH: [path.dirname(paths.cliShim), process.env.PATH ?? ""]
      .filter(Boolean)
      .join(path.delimiter),
  };
}

function uniqueFireIDs(runs: AutomationRun[]): string[] {
  return runs.map(run => run.fire_id).filter((fireID): fireID is string => Boolean(fireID));
}

function uniqueName(prefix: string): string {
  return `${prefix}-${randomUUID().slice(0, 8)}`;
}

async function readRouteState(runtime: BrowserRuntime): Promise<Record<string, unknown>> {
  return JSON.parse(
    await readFile(runtime.artifactCollector.artifactPath("browser_route_state"), "utf8")
  ) as Record<string, unknown>;
}
