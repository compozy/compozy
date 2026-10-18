import path from "node:path";
import { fileURLToPath } from "node:url";

import type { AutomationJob, AutomationRun, AutomationTrigger } from "@/systems/automation";
import { appWindow } from "../fixtures/os-navigation";
import { automationOperatorSelectors } from "../fixtures/selectors";
import type { BrowserRuntime } from "../fixtures/runtime";
import { expect, test } from "../fixtures/test";
import { completeOnboardingIfPrompted } from "../fixtures/workspace";

// Detail half of E2E-002 (Run now, the failed run's drawer, "Set up retries", the switch) and
// E2E-004 (an event automation has no Run now; delete by typing the name). The listing and
// editor halves of those journeys live in their own specs.

const testdata = path.resolve(
  path.dirname(fileURLToPath(import.meta.url)),
  "..",
  "..",
  "..",
  "internal",
  "testutil",
  "acpmock",
  "testdata"
);
const faultAgentName = "browser-automations-fault";

test.use({
  runtimeOptions: {
    seed: {
      mockAgents: [
        {
          agentName: faultAgentName,
          fixtureAgent: "faulty",
          fixturePath: path.join(testdata, "driver_fault_fixture.json"),
        },
      ],
    },
  },
});

function uniqueName(prefix: string): string {
  return `${prefix}-${Date.now().toString(36)}`;
}

async function createFailingJob(runtime: BrowserRuntime, name: string): Promise<AutomationJob> {
  const response = await runtime.requestJSON<{ job: AutomationJob }>("/api/automation/jobs", {
    method: "POST",
    body: JSON.stringify({
      agent_name: faultAgentName,
      enabled: true,
      fire_limit: { max: 24, window: "1h" },
      name,
      prompt: "trigger crash mid-stream",
      retry: { strategy: "none", max_retries: 0, base_delay: "" },
      schedule: { mode: "cron", expr: "0 9 * * 1-5" },
      scope: "global",
    }),
  });
  return response.job;
}

async function createEventTrigger(
  runtime: BrowserRuntime,
  name: string
): Promise<AutomationTrigger> {
  const response = await runtime.requestJSON<{ trigger: AutomationTrigger }>(
    "/api/automation/triggers",
    {
      method: "POST",
      body: JSON.stringify({
        agent_name: faultAgentName,
        enabled: true,
        event: "session.stopped",
        filter: { "data.stop_reason": "error" },
        fire_limit: { max: 24, window: "1h" },
        name,
        prompt: "Summarize {{ .Data.session_id }}.",
        retry: { strategy: "none", max_retries: 0, base_delay: "" },
        scope: "global",
      }),
    }
  );
  return response.trigger;
}

async function latestFailedRun(runtime: BrowserRuntime, jobID: string): Promise<AutomationRun> {
  let failed: AutomationRun | undefined;
  await expect
    .poll(
      async () => {
        const { runs } = await runtime.requestJSON<{ runs: AutomationRun[] }>(
          `/api/automation/jobs/${encodeURIComponent(jobID)}/runs?limit=10`
        );
        failed = runs.find(run => run.status === "failed");
        return failed?.id ?? "";
      },
      { timeout: 30_000 }
    )
    .not.toBe("");
  if (!failed) throw new Error(`Expected a failed run for ${jobID}.`);
  return failed;
}

test("a schedule's detail runs it now, explains the failure and offers the retries fix", async ({
  appPage,
  runtime,
}) => {
  const job = await createFailingJob(runtime, uniqueName("nightly-delivery"));
  await completeOnboardingIfPrompted(appPage);
  await appPage.goto(runtime.url(`/automations/jobs/${encodeURIComponent(job.id)}`), {
    waitUntil: "domcontentloaded",
  });
  const ui = automationOperatorSelectors(appWindow(appPage, "automations"), appPage);
  await expect(ui.detailPanel).toBeVisible({ timeout: 20_000 });
  await expect(ui.ruleStarts).toContainText("Every weekday at 09:00");

  await test.step("Run now starts a run that shows up in the list", async () => {
    await ui.detailRunNow.click();
    const failed = await latestFailedRun(runtime, job.id);
    await expect(ui.run(failed.id)).toBeVisible({ timeout: 20_000 });
    await expect(ui.run(failed.id)).toContainText("Failed");

    await ui.run(failed.id).click();
    const drawer = ui.runDrawer(failed.id);
    await expect(drawer.locator('[data-tone="danger"]').first()).toBeVisible();
    await drawer.getByRole("button", { name: "Set up retries" }).click();
    await expect(ui.editorDialog).toBeVisible();
    await appPage.keyboard.press("Escape");
  });

  await test.step("the switch waits for the daemon, then reads Off with the pause line", async () => {
    await ui.enableSwitch.click();
    await expect(ui.enableLabel).toHaveText("Off");
    await expect(
      ui.detailPanel.getByText("Off. It won't run on its schedule until you turn it on.")
    ).toBeVisible();
    await expect(ui.detailRunNow).toBeEnabled();
  });
});

test("an event automation has no Run now and is deleted by typing its name", async ({
  appPage,
  runtime,
}) => {
  const trigger = await createEventTrigger(runtime, uniqueName("summarize-failures"));
  await completeOnboardingIfPrompted(appPage);
  await appPage.goto(runtime.url(`/automations/triggers/${encodeURIComponent(trigger.id)}`), {
    waitUntil: "domcontentloaded",
  });
  const ui = automationOperatorSelectors(appWindow(appPage, "automations"), appPage);
  await expect(ui.detailPanel).toBeVisible({ timeout: 20_000 });
  await expect(ui.ruleStarts).toContainText("A session stops");
  await expect(ui.ruleOnlyIf).toContainText("Stop reason is");
  await expect(ui.detailRunNow).toHaveCount(0);

  await ui.detailOverflow.click();
  await ui.deleteAutomationButton.click();
  await ui.automationDeleteConfirmTyping.fill(trigger.name);
  await ui.confirmDeleteAutomationButton.click();
  await expect(appPage.getByText(`Deleted ${trigger.name}.`)).toBeVisible();
  await expect(appPage).toHaveURL(/\/automations$/);
});
