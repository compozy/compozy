import { randomUUID } from "node:crypto";
import path from "node:path";
import { fileURLToPath } from "node:url";

import { appWindow } from "../fixtures/os-navigation";
import type { BrowserRuntime } from "../fixtures/runtime";
import { browserAutomationOperatorFlowScenario } from "../fixtures/runtime";
import { automationOperatorSelectors } from "../fixtures/selectors";
import { expect, test } from "../fixtures/test";
import { completeOnboardingIfPrompted, ensureProjectWorkspace } from "../fixtures/workspace";

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
const automationAgentName = "browser-automations-runner";

interface SeededJob {
  id: string;
  enabled: boolean;
  name: string;
}

interface SeededTrigger {
  id: string;
  enabled: boolean;
  name: string;
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
      ],
    },
  },
});

test("Automations E2E-006: legacy job and trigger URLs replace-redirect into Automations", async ({
  appPage,
  runtime,
}) => {
  const ui = automationOperatorSelectors(appWindow(appPage, "automations"), appPage);
  await ensureProjectWorkspace(appPage, runtime);
  await completeOnboardingIfPrompted(appPage);
  const job = await createJob(runtime, uniqueName("redirect-job"));

  await test.step("a legacy job detail URL lands on the Automations job detail", async () => {
    await appPage.goto(runtime.url("/"), { waitUntil: "domcontentloaded" });
    await expect(appPage.getByTestId("os-desktop")).toBeVisible();
    await appPage.goto(runtime.url(`/jobs/${job.id}`), { waitUntil: "domcontentloaded" });
    await expect(appPage).toHaveURL(new RegExp(`/automations/jobs/${job.id}$`));
    await expect(ui.detailPanel).toBeVisible();

    // The redirect replaces the legacy entry, so Back never returns to `/jobs/...`.
    await appPage.goBack({ waitUntil: "domcontentloaded" });
    await expect.poll(() => new URL(appPage.url()).pathname).not.toMatch(/^\/jobs(?:\/|$)/);
  });

  await test.step("a legacy trigger event filter becomes the On events view with a search", async () => {
    await appPage.goto(runtime.url("/triggers?event=session.stopped"), {
      waitUntil: "domcontentloaded",
    });
    await expect
      .poll(() => {
        const url = new URL(appPage.url());
        return {
          pathname: url.pathname,
          q: url.searchParams.get("q"),
          start: url.searchParams.get("start"),
        };
      })
      .toEqual({ pathname: "/automations", q: "session.stopped", start: "event" });
    await expect(ui.automationsShell).toBeVisible();
  });

  await deleteAutomationIfExists(runtime, "jobs", job.id);
});

test("Automations E2E-001/E2E-002: one listing filters, searches, and toggles jobs and triggers", async ({
  appPage,
  runtime,
}) => {
  const automationsWin = appWindow(appPage, "automations");
  const ui = automationOperatorSelectors(automationsWin, appPage);
  await ensureProjectWorkspace(appPage, runtime);
  await completeOnboardingIfPrompted(appPage);
  const job = await createJob(runtime, uniqueName("listing-job"));
  const trigger = await createTrigger(runtime, uniqueName("listing-trigger"));

  await test.step("the All view lists both kinds with per-start counts", async () => {
    await appPage.goto(runtime.url("/automations"), { waitUntil: "domcontentloaded" });
    await expect(ui.automationsShell).toBeVisible();
    await expect(ui.automationsListRows).toBeVisible();
    await expect(automationsWin.getByTestId(`automation-row-job-${job.id}`)).toBeVisible({
      timeout: 20_000,
    });
    await expect(automationsWin.getByTestId(`automation-row-trigger-${trigger.id}`)).toBeVisible();
    await expect(ui.automationStartView("all")).toHaveText(/^All\s*2$/);
    await expect(ui.automationStartView("schedule")).toHaveText(/^Scheduled\s*1$/);
    await expect(ui.automationStartView("event")).toHaveText(/^On events\s*1$/);
  });

  await test.step("the Scheduled view keeps only the job and records the view in the URL", async () => {
    await ui.automationStartView("schedule").click();
    await expect.poll(() => new URL(appPage.url()).searchParams.get("start")).toBe("schedule");
    await expect(ui.item(job.id)).toBeVisible();
    await expect(ui.item(trigger.id)).toBeHidden();
    await ui.automationStartView("all").click();
    await expect.poll(() => new URL(appPage.url()).searchParams.get("start")).toBeNull();
    await expect(ui.item(trigger.id)).toBeVisible();
  });

  await test.step("search narrows the rows", async () => {
    const search = automationsWin.getByTestId("automation-search-input");
    await search.fill(trigger.name);
    await expect.poll(() => new URL(appPage.url()).searchParams.get("q")).toBe(trigger.name);
    await expect(ui.item(trigger.id)).toBeVisible();
    await expect(ui.item(job.id)).toBeHidden();
    await search.fill("");
    await expect(ui.item(job.id)).toBeVisible();
  });

  await test.step("a filter with no matches shows the filtered empty state until cleared", async () => {
    await appPage.goto(runtime.url("/automations?target=loop"), { waitUntil: "domcontentloaded" });
    const filteredEmpty = automationsWin.getByTestId("automations-list-filtered-empty");
    await expect(filteredEmpty).toBeVisible({ timeout: 20_000 });
    await expect(ui.item(job.id)).toBeHidden();
    await expect(ui.item(trigger.id)).toBeHidden();
    await automationsWin.getByTestId("automations-list-clear-filters").click();
    await expect.poll(() => new URL(appPage.url()).searchParams.get("target")).toBeNull();
    await expect(filteredEmpty).toBeHidden();
    await expect(ui.item(job.id)).toBeVisible();
    await expect(ui.item(trigger.id)).toBeVisible();
  });

  await test.step("the row switch turns the job off through the daemon", async () => {
    const jobSwitch = ui.automationSwitch(job.id);
    await expect(jobSwitch).toHaveAccessibleName(`Turn ${job.name} on or off`);
    await expect(jobSwitch).toHaveAttribute("aria-checked", "true");
    await jobSwitch.click();
    // The switch is not optimistic: it flips once the daemon confirms the PATCH.
    await expect(jobSwitch).toHaveAttribute("aria-checked", "false");
    await expect
      .poll(async () => (await getAutomation<SeededJob>(runtime, "jobs", job.id)).enabled)
      .toBe(false);
    expect((await getAutomation<SeededTrigger>(runtime, "triggers", trigger.id)).enabled).toBe(
      true
    );
  });

  await deleteAutomationIfExists(runtime, "jobs", job.id);
  await deleteAutomationIfExists(runtime, "triggers", trigger.id);
});

async function createJob(runtime: BrowserRuntime, name: string): Promise<SeededJob> {
  return (
    await runtime.requestJSON<{ job: SeededJob }>("/api/automation/jobs", {
      method: "POST",
      body: JSON.stringify({
        agent_name: automationAgentName,
        enabled: true,
        fire_limit: { max: 24, window: "1h" },
        name,
        prompt: browserAutomationOperatorFlowScenario.job.prompt,
        retry: { strategy: "none", max_retries: 0, base_delay: "" },
        schedule: { mode: "cron", expr: browserAutomationOperatorFlowScenario.job.scheduleExpr },
        scope: "global",
      }),
    })
  ).job;
}

async function createTrigger(runtime: BrowserRuntime, name: string): Promise<SeededTrigger> {
  return (
    await runtime.requestJSON<{ trigger: SeededTrigger }>("/api/automation/triggers", {
      method: "POST",
      body: JSON.stringify({
        agent_name: automationAgentName,
        enabled: true,
        endpoint_slug: uniqueName("browser-automations-trigger"),
        event: "webhook",
        filter: { "data.branch": "main" },
        fire_limit: { max: 12, window: "1h" },
        name,
        prompt: browserAutomationOperatorFlowScenario.trigger.prompt,
        retry: { strategy: "none", max_retries: 0, base_delay: "" },
        scope: "global",
        webhook_id: `wbh_${randomUUID().replaceAll("-", "_").slice(0, 18)}`,
        webhook_secret_value: "browser-automations-secret",
      }),
    })
  ).trigger;
}

async function getAutomation<T>(
  runtime: BrowserRuntime,
  kind: "jobs" | "triggers",
  id: string
): Promise<T> {
  const payload = await runtime.requestJSON<Record<"job" | "trigger", T>>(
    `/api/automation/${kind}/${encodeURIComponent(id)}`
  );
  return kind === "jobs" ? payload.job : payload.trigger;
}

async function deleteAutomationIfExists(
  runtime: BrowserRuntime,
  kind: "jobs" | "triggers",
  id: string
): Promise<void> {
  const response = await fetch(runtime.url(`/api/automation/${kind}/${encodeURIComponent(id)}`), {
    method: "DELETE",
  });
  expect([204, 404]).toContain(response.status);
}

function uniqueName(prefix: string): string {
  return `${prefix}-${randomUUID().slice(0, 8)}`;
}
