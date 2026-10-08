import { fileURLToPath } from "node:url";
import path from "node:path";

import type { AutomationJob, AutomationSuggestion } from "@/systems/automation";
import { automationOperatorSelectors, sessionWindowSelectors } from "../fixtures/selectors";
import {
  browserAutomationOperatorFlowScenario,
  seedBrowserAutomationOperatorFlow,
} from "../fixtures/runtime";
import {
  appWindow,
  focusWindowThroughPalette,
  openAppWindow,
  sessionWindow,
  windowTitle,
} from "../fixtures/os-navigation";
import { expect, test } from "../fixtures/test";
import { completeOnboardingIfPrompted, ensureProjectWorkspace } from "../fixtures/workspace";

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
  return `/session/${sessionId}`;
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

test("operator manages workspace suggestions and inspects a real automation run and linked session", async ({
  appPage,
  browserArtifacts,
  runtime,
}) => {
  // Keep the starter catalog untouched until suggestion decisions are verified.
  test.setTimeout(180_000);

  await test.step("operator can accept and dismiss workspace suggestions through the real daemon", async () => {
    const automationUI = automationOperatorSelectors(appPage);

    await ensureProjectWorkspace(appPage, runtime);
    await completeOnboardingIfPrompted(automationUI);
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

    const jobsWin = await openAppWindow(appPage, "Automations", "automations");
    const jobsUI = automationOperatorSelectors(jobsWin, appPage);
    await expect(appPage).toHaveURL(/\/automations$/);
    await expect(jobsUI.automationSuggestionsCard).toBeVisible();

    const dismissedRow = jobsUI.suggestion(dismissTarget.id);
    await dismissedRow.getByRole("button", { name: "Dismiss" }).click();
    await expect(dismissedRow).toBeHidden();

    const acceptedRow = jobsUI.suggestion(acceptTarget.id);
    await acceptedRow.locator('[data-slot="collapsible-trigger"]').click();
    await expect(acceptedRow.locator('[data-slot="collapsible-content"]')).toContainText(
      acceptTarget.payload.prompt
    );
    await acceptedRow.getByRole("button", { name: "Create automation" }).click();

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

    await appPage.reload({ waitUntil: "domcontentloaded" });
    await expect(jobsUI.automationsShell).toBeVisible();
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

  await test.step("operator can inspect automation, trigger a real run, and inspect the linked session transcript", async () => {
    const automationUI = automationOperatorSelectors(appPage);
    const seeded = await seedBrowserAutomationOperatorFlow(runtime, {
      agentName: automationAgentName,
    });

    await ensureProjectWorkspace(appPage, runtime);
    await completeOnboardingIfPrompted(automationUI);

    await expect(automationUI.osDesktop).toBeVisible();
    await appPage.goto(runtime.url("/automations?start=schedule"), {
      waitUntil: "domcontentloaded",
    });
    // The route opens/focuses Automations after hydration; another Dock click can minimize it.
    const jobsWin = appWindow(appPage, "automations");
    await expect(jobsWin).toBeVisible();
    const jobsUI = automationOperatorSelectors(jobsWin, appPage);

    await expect(appPage).toHaveURL(/\/automations\?start=schedule$/);
    await expect(jobsUI.automationsShell).toBeVisible();
    await expect(jobsUI.automationsListRows).toBeVisible();
    await expect(jobsUI.item(seeded.job.id)).toBeVisible();
    await jobsUI.itemLink(seeded.job.id).click();

    await expect(appPage).toHaveURL(new RegExp(`/automations/jobs/${seeded.job.id}(?:\\?.*)?$`));
    await expect(jobsUI.detailPanel).toBeVisible();
    await expect(windowTitle(jobsWin)).toContainText(seeded.job.name);
    await expect(jobsUI.detailPanel).toContainText(
      browserAutomationOperatorFlowScenario.job.prompt
    );
    await expect(jobsUI.runList).toBeVisible();
    await expect(jobsUI.run(seeded.baselineRun.id)).toBeVisible();
    await expect(jobsUI.run(seeded.baselineRun.id)).toContainText(/completed/i);
    await jobsUI.run(seeded.baselineRun.id).click();
    await expect(jobsUI.runOpenLink(seeded.baselineRun.id)).toBeVisible();
    await expect(jobsUI.runOpenLink(seeded.baselineRun.id)).toHaveAttribute(
      "href",
      `/session/${seeded.baselineRun.session_id}`
    );

    // Jobs and triggers share the Automations window: the job detail's back crumb returns to
    // the listing, whose On events view lists the trigger.
    await jobsWin
      .getByRole("navigation", { name: "Window path" })
      .getByRole("button", { exact: true, name: "Automations" })
      .click();
    const triggersWin = appWindow(appPage, "automations");
    const triggersUI = automationOperatorSelectors(triggersWin, appPage);
    await expect(triggersUI.automationsShell).toBeVisible();
    await triggersUI.automationStartView("event").click();
    await expect(appPage).toHaveURL(/\/automations\?start=event$/);
    await expect(triggersUI.automationsListRows).toBeVisible();
    await expect(triggersUI.item(seeded.trigger.id)).toBeVisible();
    await triggersUI.itemLink(seeded.trigger.id).click();

    await expect(appPage).toHaveURL(
      new RegExp(`/automations/triggers/${seeded.trigger.id}(?:\\?.*)?$`)
    );
    await expect(windowTitle(triggersWin)).toContainText(seeded.trigger.name);
    await expect(triggersUI.detailPanel).toContainText(
      browserAutomationOperatorFlowScenario.trigger.webhookID
    );

    // Edit is a route-chrome action for an automation the operator owns.
    await expect(triggersUI.editAutomationButton).toBeEnabled();
    await triggersUI.editAutomationButton.click();
    await expect(triggersUI.nameInput).toHaveValue(seeded.trigger.name);
    const triggerDialog = triggersUI.editorDialog;
    await expect(triggersUI.retryMax).toBeVisible();
    await triggersWin.getByTestId("automation-options-toggle").click();
    await expect(triggersUI.retryMax).toBeHidden();
    await triggersWin.getByTestId("automation-options-toggle").click();
    await expect(triggersUI.retryMax).toBeVisible();
    await appPage.keyboard.press("Escape");
    await expect(triggerDialog).toBeHidden();

    await appPage.goto(runtime.url("/automations?start=schedule"), {
      waitUntil: "domcontentloaded",
    });
    await expect(appPage).toHaveURL(/\/automations\?start=schedule$/);
    await expect(jobsUI.automationsShell).toBeVisible();
    await focusWindowThroughPalette(appPage, jobsWin);
    await jobsUI.itemLink(seeded.job.id).click();
    await expect(appPage).toHaveURL(new RegExp(`/automations/jobs/${seeded.job.id}(?:\\?.*)?$`));
    await expect(jobsUI.detailPanel).toBeVisible();

    const editJob = jobsUI.editAutomationButton;
    await expect(editJob).toBeEnabled();
    await editJob.click();
    await expect(jobsUI.form).toBeVisible();
    await expect(jobsUI.nameInput).toHaveValue(seeded.job.name);
    await expect(jobsUI.scheduleExpr).toHaveValue(
      browserAutomationOperatorFlowScenario.job.scheduleExpr
    );
    await appPage.keyboard.press("Escape");
    await expect(jobsUI.form).toBeHidden();

    await jobsUI.detailRunNow.click();

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

    await jobsUI.run(uiTriggeredRun.id).click();
    await jobsUI.runOpenLink(uiTriggeredRun.id).click();

    await expect
      .poll(() => new URL(appPage.url()).pathname)
      .toBe(automationSessionPath(uiTriggeredRun.session_id));
    const workspaceSwitchDialog = appPage.getByRole("dialog", { name: "Switch project?" });
    await expect(workspaceSwitchDialog).toBeVisible();
    await workspaceSwitchDialog.getByRole("button", { name: "Switch project" }).click();
    const sessionUI = sessionWindowSelectors(sessionWindow(appPage, uiTriggeredRun.session_id));
    await expect(sessionUI.chatView).toBeVisible();
    await expect(sessionUI.chatView).toContainText(
      browserAutomationOperatorFlowScenario.job.prompt
    );
    await expect(sessionUI.chatView).toContainText(
      browserAutomationOperatorFlowScenario.transcript.assistant
    );

    await browserArtifacts.captureScreenshot("automation-linked-session", appPage);
  });
});
