import { fileURLToPath } from "node:url";
import path from "node:path";

import type { AutomationJob, AutomationSuggestion } from "@/systems/automation";
import { automationOperatorSelectors, sessionWindowSelectors } from "../fixtures/selectors";
import {
  browserAutomationOperatorFlowScenario,
  seedBrowserAutomationOperatorFlow,
} from "../fixtures/runtime";
import { openAppWindow, sessionWindow, windowTitle } from "../fixtures/os-navigation";
import { expect, test } from "../fixtures/test";
import { useGlobalWorkspaceIfPrompted } from "../fixtures/workspace";

const automationTaskFixture = path.resolve(
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

const automationAgentName = "browser-automation-runner";

function automationSessionPath(sessionId: string): string {
  return `/agents/${automationAgentName}/sessions/${sessionId}`;
}

test.use({
  runtimeOptions: {
    seed: {
      mockAgents: [
        {
          fixturePath: automationTaskFixture,
          fixtureAgent: "automation-runner",
          agentName: automationAgentName,
        },
      ],
    },
  },
});

test("operator can inspect automation, trigger a real run, and inspect the linked session transcript", async ({
  appPage,
  browserArtifacts,
  runtime,
}) => {
  const automationUI = automationOperatorSelectors(appPage);
  const seeded = await seedBrowserAutomationOperatorFlow(runtime, {
    agentName: automationAgentName,
  });

  await useGlobalWorkspaceIfPrompted(automationUI);

  await expect(automationUI.osDesktop).toBeVisible();
  const jobsWin = await openAppWindow(appPage, "Jobs", "jobs");
  const jobsUI = automationOperatorSelectors(jobsWin, appPage);

  await expect(appPage).toHaveURL(/\/jobs$/);
  await expect(jobsUI.jobsShell).toBeVisible();
  await expect(jobsUI.jobsListRows).toBeVisible();
  await expect(jobsUI.item(seeded.job.id)).toBeVisible();
  await jobsUI.itemLink(seeded.job.id).click();

  await expect(appPage).toHaveURL(new RegExp(`/jobs/${seeded.job.id}$`));
  await expect(jobsUI.detailPanel).toBeVisible();
  await expect(windowTitle(jobsWin)).toContainText(seeded.job.name);
  await expect(jobsUI.detailPanel).toContainText(browserAutomationOperatorFlowScenario.job.prompt);
  await expect(jobsUI.runHistory).toBeVisible();
  await expect(jobsUI.run(seeded.baselineRun.id)).toBeVisible();
  await expect(jobsUI.run(seeded.baselineRun.id)).toContainText(/completed/i);
  await expect(jobsUI.runSessionLink(seeded.baselineRun.id)).toBeVisible();
  await expect(jobsUI.runSessionLink(seeded.baselineRun.id)).toHaveAttribute(
    "href",
    `/session/${seeded.baselineRun.session_id}`
  );

  const triggersWin = await openAppWindow(appPage, "Triggers", "triggers");
  const triggersUI = automationOperatorSelectors(triggersWin, appPage);
  await expect(appPage).toHaveURL(/\/triggers$/);
  await expect(triggersUI.triggersShell).toBeVisible();
  await expect(triggersUI.triggersListRows).toBeVisible();
  await expect(triggersUI.item(seeded.trigger.id)).toBeVisible();
  await triggersUI.itemLink(seeded.trigger.id).click();

  await expect(appPage).toHaveURL(new RegExp(`/triggers/${seeded.trigger.id}$`));
  await expect(windowTitle(triggersWin)).toContainText(seeded.trigger.name);
  await expect(triggersUI.detailPanel).toContainText(
    browserAutomationOperatorFlowScenario.trigger.webhookID
  );

  await triggersUI.detailOverflow.click();
  const editTrigger = appPage.getByTestId("edit-automation-btn");
  await expect(editTrigger).toBeEnabled();
  await editTrigger.click();
  await expect(triggersUI.triggerNameInput).toHaveValue(seeded.trigger.name);
  const triggerDialog = triggersUI.editorDialog;
  await expect(triggerDialog).toHaveAttribute("data-frame", "unframed");
  await expect(triggerDialog.locator('[data-slot="dialog-header"]')).toHaveAttribute(
    "data-variant",
    "ruled"
  );
  await expect(triggerDialog.locator('[data-slot="dialog-footer"]')).toHaveAttribute(
    "data-variant",
    "ruled"
  );
  await expect(triggersUI.triggerRetryMax).toBeVisible();
  await triggersWin.getByTestId("trigger-governance-toggle").click();
  await expect(triggersUI.triggerRetryMax).toBeHidden();
  await triggersWin.getByTestId("trigger-governance-toggle").click();
  await expect(triggersUI.triggerRetryMax).toBeVisible();
  await appPage.keyboard.press("Escape");
  await expect(triggerDialog).toBeHidden();

  await appPage.goto(runtime.url("/jobs"), { waitUntil: "domcontentloaded" });
  await expect(appPage).toHaveURL(/\/jobs$/);
  await expect(jobsUI.jobsShell).toBeVisible();
  await jobsUI.itemLink(seeded.job.id).click();
  await expect(appPage).toHaveURL(new RegExp(`/jobs/${seeded.job.id}$`));

  await jobsUI.detailOverflow.click();
  const editJob = appPage.getByTestId("edit-automation-btn");
  await expect(editJob).toBeEnabled();
  await editJob.click();
  await expect(jobsUI.jobForm).toBeVisible();
  const jobDialog = jobsUI.editorDialog;
  await expect(jobDialog).toHaveAttribute("data-frame", "unframed");
  await expect(jobDialog.locator('[data-slot="dialog-header"]')).toHaveAttribute(
    "data-variant",
    "ruled"
  );
  await expect(jobDialog.locator('[data-slot="dialog-footer"]')).toHaveAttribute(
    "data-variant",
    "ruled"
  );
  await expect(jobsUI.jobNameInput).toHaveValue(seeded.job.name);
  await expect(jobsUI.jobScheduleExpr).toHaveValue(
    browserAutomationOperatorFlowScenario.job.scheduleExpr
  );
  await appPage.keyboard.press("Escape");
  await expect(jobsUI.jobForm).toBeHidden();

  await jobsUI.triggerJobButton.click();

  await expect
    .poll(async () => {
      const payload = await runtime.requestJSON<{
        runs: Array<{ id: string }>;
      }>(`/api/automation/jobs/${encodeURIComponent(seeded.job.id)}/runs?limit=10`);
      return payload.runs.length;
    })
    .toBe(2);

  let uiTriggeredRun:
    | {
        id: string;
        session_id?: string | null;
      }
    | undefined;
  await expect
    .poll(
      async () => {
        const runsPayload = await runtime.requestJSON<{
          runs: Array<{ id: string; session_id?: string | null }>;
        }>(`/api/automation/jobs/${encodeURIComponent(seeded.job.id)}/runs?limit=10`);
        uiTriggeredRun = runsPayload.runs.find(
          run => run.id !== seeded.baselineRun.id && run.session_id
        );
        return uiTriggeredRun?.session_id ?? "";
      },
      {
        timeout: 20_000,
      }
    )
    .not.toBe("");

  if (!uiTriggeredRun?.session_id) {
    throw new Error("Expected the UI-triggered automation run to include a linked session.");
  }

  await expect(jobsUI.run(uiTriggeredRun.id)).toBeVisible();
  await browserArtifacts.captureScreenshot("automation-operator-history", appPage);

  await jobsUI.runSessionLink(uiTriggeredRun.id).click();

  await expect
    .poll(() => new URL(appPage.url()).pathname)
    .toBe(automationSessionPath(uiTriggeredRun.session_id));
  const sessionUI = sessionWindowSelectors(sessionWindow(appPage, uiTriggeredRun.session_id));
  await expect(sessionUI.chatView).toBeVisible();
  await expect(sessionUI.chatView).toContainText(browserAutomationOperatorFlowScenario.job.prompt);
  await expect(sessionUI.chatView).toContainText(
    browserAutomationOperatorFlowScenario.transcript.assistant
  );

  await browserArtifacts.captureScreenshot("automation-linked-session", appPage);
});

test("operator can accept and dismiss workspace suggestions through the real daemon", async ({
  appPage,
  runtime,
}) => {
  const automationUI = automationOperatorSelectors(appPage);

  await useGlobalWorkspaceIfPrompted(automationUI);
  const workspaces = await runtime.requestJSON<{
    workspaces: Array<{ id: string }>;
  }>("/api/workspaces");
  const workspaceID = workspaces.workspaces[0]?.id;
  if (!workspaceID) {
    throw new Error("Expected the browser Automation scenario to have an active workspace.");
  }

  const suggestionPath = `/api/workspaces/${encodeURIComponent(workspaceID)}/automation/suggestions`;
  const initial = await runtime.requestJSON<{ suggestions: AutomationSuggestion[] }>(
    `${suggestionPath}?status=pending`
  );
  expect(initial.suggestions).toHaveLength(4);

  const acceptTarget = initial.suggestions.find(
    suggestion => suggestion.payload.name === "Daily workspace briefing"
  );
  const dismissTarget = initial.suggestions.find(
    suggestion => suggestion.payload.name === "Weekday standup draft"
  );
  if (!acceptTarget || !dismissTarget) {
    throw new Error("Expected the deterministic starter suggestion catalog.");
  }

  const jobsWin = await openAppWindow(appPage, "Jobs", "jobs");
  const jobsUI = automationOperatorSelectors(jobsWin, appPage);
  await expect(appPage).toHaveURL(/\/jobs$/);
  await expect(jobsUI.automationSuggestionsCard).toBeVisible();

  const acceptedRow = jobsUI.suggestion(acceptTarget.id);
  await expect(acceptedRow).toContainText(acceptTarget.payload.prompt);
  await acceptedRow.getByRole("button", { name: "Create job" }).click();

  await expect(acceptedRow).toBeHidden();
  await expect(jobsUI.item(acceptTarget.payload.id)).toBeVisible();
  const acceptedJob = await runtime.requestJSON<{ job: AutomationJob }>(
    `/api/automation/jobs/${encodeURIComponent(acceptTarget.payload.id)}`
  );
  expect(acceptedJob.job).toMatchObject({
    enabled: true,
    id: acceptTarget.payload.id,
    workspace_id: workspaceID,
  });

  const dismissedRow = jobsUI.suggestion(dismissTarget.id);
  await dismissedRow.getByRole("button", { name: "Dismiss" }).click();
  await expect(dismissedRow).toBeHidden();

  await appPage.reload({ waitUntil: "domcontentloaded" });
  await expect(jobsUI.jobsShell).toBeVisible();
  await expect(jobsUI.suggestion(acceptTarget.id)).toBeHidden();
  await expect(jobsUI.suggestion(dismissTarget.id)).toBeHidden();

  const accepted = await runtime.requestJSON<{ suggestions: AutomationSuggestion[] }>(
    `${suggestionPath}?status=accepted`
  );
  const dismissed = await runtime.requestJSON<{ suggestions: AutomationSuggestion[] }>(
    `${suggestionPath}?status=dismissed`
  );
  expect(accepted.suggestions.map(suggestion => suggestion.id)).toContain(acceptTarget.id);
  expect(dismissed.suggestions.map(suggestion => suggestion.id)).toContain(dismissTarget.id);
});
