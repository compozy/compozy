import { fileURLToPath } from "node:url";
import path from "node:path";

import type { Locator, Page } from "@playwright/test";

import type { AutomationJob, AutomationTrigger } from "@/systems/automation";
import type { LoopDefinition } from "@/systems/loops";
import { openAppWindow } from "../fixtures/os-navigation";
import { automationOperatorSelectors } from "../fixtures/selectors";
import { expect, test } from "../fixtures/test";
import { completeOnboardingIfPrompted, ensureProjectWorkspace } from "../fixtures/workspace";

// Suite: Automations editor (S3) — the one dialog creates a job or a trigger.
// Journeys: E2E-001 create steps (schedule), E2E-004 create steps (event) and E2E-005 (Loop entry).

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

const agentName = "browser-automation-runner";

test.use({
  runtimeOptions: {
    seed: {
      mockAgents: [
        {
          fixturePath: automationTaskFixture,
          fixtureAgent: "automation-runner",
          agentName,
        },
      ],
    },
  },
});

/** Opens the editor from the first-run empty state, or from New automation when rows exist. */
async function openEditor(win: Locator, start: "schedule" | "event"): Promise<void> {
  const emptyStart = win.getByTestId(`automations-empty-start-${start}`);
  if (await emptyStart.isVisible()) {
    await emptyStart.click();
    return;
  }
  await win.getByTestId("automations-create").click();
}

function editor(page: Page): Locator {
  return page.getByTestId("automation-editor-dialog");
}

async function pickAgent(page: Page, dialog: Locator): Promise<void> {
  await dialog.getByTestId("automation-agent-input").click();
  await page.getByTestId(`agent-command-item-${agentName}`).click();
}

test("operator creates a scheduled automation from the one editor", async ({
  appPage,
  runtime,
}) => {
  test.setTimeout(120_000);
  await ensureProjectWorkspace(appPage, runtime);
  await completeOnboardingIfPrompted(automationOperatorSelectors(appPage));

  const win = await openAppWindow(appPage, "Automations", "automations");
  await openEditor(win, "schedule");

  const dialog = editor(appPage);
  await expect(dialog).toBeVisible();
  await expect(dialog.getByText("New automation")).toBeVisible();
  await expect(dialog.getByTestId("automation-start-schedule")).toHaveAttribute(
    "aria-checked",
    "true"
  );

  await dialog.getByTestId("automation-name-input").fill("morning-digest");
  await dialog.getByRole("button", { name: "Weekdays 9am" }).click();
  await expect(dialog.getByTestId("automation-schedule-readout")).toContainText(
    "Every weekday at 09:00 UTC"
  );
  await expect(dialog.getByTestId("automation-editor-status")).toHaveText("Needs a fix");

  await pickAgent(appPage, dialog);
  await dialog.getByTestId("automation-prompt-input").fill("Summarize yesterday.");

  const sentence = dialog.getByTestId("automation-editor-sentence");
  await expect(sentence).toContainText(`Every weekday at 09:00 UTC, ask ${agentName}`);
  await expect(dialog.getByTestId("automation-editor-status")).toHaveText("Ready");
  await expect(dialog.getByTestId("automation-destination")).toContainText("Creates in");

  await dialog.getByTestId("automation-form-submit").click();

  await expect(appPage.getByText("Created morning-digest.")).toBeVisible();
  await expect(dialog).toBeHidden();

  const jobs = await runtime.requestJSON<{ jobs: AutomationJob[] }>("/api/automation/jobs");
  const job = jobs.jobs.find(item => item.name === "morning-digest");
  expect(job).toMatchObject({
    agent_name: agentName,
    enabled: true,
    schedule: expect.objectContaining({ mode: "cron", expr: "0 9 * * 1-5" }),
    scope: "workspace",
  });
  await expect(appPage).toHaveURL(new RegExp(`/automations/jobs/${job?.id ?? "missing"}`));
});

test("operator creates an event automation with a condition from the one editor", async ({
  appPage,
  runtime,
}) => {
  test.setTimeout(120_000);
  await ensureProjectWorkspace(appPage, runtime);
  await completeOnboardingIfPrompted(automationOperatorSelectors(appPage));

  const win = await openAppWindow(appPage, "Automations", "automations");
  await openEditor(win, "event");

  const dialog = editor(appPage);
  await expect(dialog).toBeVisible();
  await dialog.getByTestId("automation-start-event").click();
  await dialog.getByTestId("automation-event-session.stopped").click();
  await expect(dialog.getByTestId("automation-does-task")).toBeDisabled();

  await dialog.getByTestId("automation-name-input").fill("summarize-failures");
  await dialog.getByTestId("automation-condition-add").click();
  await dialog.getByTestId("automation-condition-field-0").selectOption("data.stop_reason");
  await expect(dialog.getByTestId("automation-editor-status")).toHaveText("Needs a fix");
  await dialog.getByTestId("automation-condition-value-0").fill("error");

  await pickAgent(appPage, dialog);
  await dialog.getByTestId("automation-prompt-input").fill("Explain why the session failed.");
  await expect(dialog.getByTestId("automation-editor-sentence")).toContainText(
    "When a session stops in"
  );
  await expect(dialog.getByTestId("automation-editor-status")).toHaveText("Ready");

  await dialog.getByTestId("automation-form-submit").click();

  await expect(appPage.getByText("Created summarize-failures.")).toBeVisible();
  await expect(dialog).toBeHidden();

  const triggers = await runtime.requestJSON<{ triggers: AutomationTrigger[] }>(
    "/api/automation/triggers"
  );
  const trigger = triggers.triggers.find(item => item.name === "summarize-failures");
  expect(trigger).toMatchObject({
    agent_name: agentName,
    event: "session.stopped",
    filter: { "data.stop_reason": "error" },
    scope: "workspace",
  });
  await expect(appPage).toHaveURL(new RegExp(`/automations/triggers/${trigger?.id ?? "missing"}`));
});

const eventLoopName = "automate-on-event-e2e";

/** A one-node Loop whose start allowlist permits event automations. */
const eventLoopDefinition: LoopDefinition = {
  apiVersion: "compozy.loop/v1",
  kind: "Loop",
  meta: {
    name: eventLoopName,
    description: "A Loop an event automation can start.",
    catalog: { category: "Testing" },
  },
  concurrency: "allow",
  contract: {
    goal: "Record that an event started the Loop.",
    definition_of_done: "The transform completes.",
    stop_when: "nodes.finish.status == 'succeeded'",
    iteration_cap: 1,
    no_progress: { window: 2 },
    budget: { tokens: 0, wall_clock_sec: 0, on_exceeded: "halt" },
    terminal_states: ["done", "failed", "blocked", "exhausted", "stalled"],
  },
  graph: {
    nodes: [
      {
        id: "finish",
        class: "action",
        kind: "transform",
        params: { map: { done: { value: true } } },
      },
    ],
    edges: [],
  } as LoopDefinition["graph"],
  start: [{ kind: "http" }, { kind: "trigger" }],
};

test("E2E-005 operator automates a Loop from its page and finds it through the Start panel", async ({
  appPage,
  runtime,
}) => {
  test.setTimeout(120_000);
  await ensureProjectWorkspace(appPage, runtime);
  await completeOnboardingIfPrompted(automationOperatorSelectors(appPage));
  const workspaces = await runtime.requestJSON<{ workspaces: Array<{ id: string }> }>(
    "/api/workspaces"
  );
  const workspaceID = workspaces.workspaces[0]?.id;
  if (!workspaceID) throw new Error("Expected an active workspace for the Loop entry journey.");
  await runtime.requestJSON(`/api/workspaces/${encodeURIComponent(workspaceID)}/loops`, {
    method: "POST",
    body: JSON.stringify({ definition: eventLoopDefinition }),
  });

  const loopPath = `/loops/${encodeURIComponent(eventLoopName)}`;
  await appPage.goto(runtime.url(loopPath), { waitUntil: "domcontentloaded" });
  await expect(appPage.getByTestId("loop-start-bindings")).toContainText("Manual only");

  await appPage.getByTestId("loop-automate-action").click();
  await appPage.getByTestId("loop-automate-event").click();

  const dialog = editor(appPage);
  await expect(dialog).toBeVisible();
  await expect(dialog.getByTestId("automation-start-event")).toHaveAttribute(
    "aria-checked",
    "true"
  );
  await expect(dialog.getByTestId("automation-does-loop")).toHaveAttribute("aria-checked", "true");
  await expect(dialog.getByTestId("automation-does-agent")).toBeDisabled();
  await expect(dialog.getByTestId("loop-target-select")).toContainText(eventLoopName);

  await dialog.getByTestId("automation-name-input").fill("loop-on-stop");
  await expect(dialog.getByTestId("automation-editor-sentence")).toContainText(
    `start the Loop ${eventLoopName}`
  );
  await expect(dialog.getByTestId("automation-editor-status")).toHaveText("Ready");
  await dialog.getByTestId("automation-form-submit").click();

  await expect(appPage.getByText("Created loop-on-stop.")).toBeVisible();
  await expect(dialog).toBeHidden();
  const triggers = await runtime.requestJSON<{ triggers: AutomationTrigger[] }>(
    "/api/automation/triggers"
  );
  const trigger = triggers.triggers.find(item => item.name === "loop-on-stop");
  expect(trigger).toMatchObject({
    target_kind: "loop",
    loop_target: expect.objectContaining({ loop_name: eventLoopName }),
  });

  await appPage.goto(runtime.url(loopPath), { waitUntil: "domcontentloaded" });
  const bindings = appPage.getByTestId("loop-start-bindings");
  await expect(bindings).toContainText("1 automation");
  await bindings.getByTestId("loop-bindings-open-automations").click();

  await expect(appPage).toHaveURL(new RegExp(`/automations\\?loop=${eventLoopName}`));
  const rows = appPage.locator('[data-testid^="automation-row-"]');
  await expect(rows).toHaveCount(1);
  await expect(
    appPage.getByTestId(`automation-row-trigger-${trigger?.id ?? "missing"}`)
  ).toBeVisible();
});

// F1 (QA walk): a full page load of a create link opens the editor, keeps `create=1`
// unquoted, and the params leave the URL only when the dialog closes.
test("a cold-loaded create link opens the editor and clears on close", async ({
  appPage,
  runtime,
}) => {
  test.setTimeout(120_000);
  await ensureProjectWorkspace(appPage, runtime);
  await completeOnboardingIfPrompted(automationOperatorSelectors(appPage));

  await appPage.goto(runtime.url("/automations?create=1&start=event"), {
    waitUntil: "domcontentloaded",
  });

  const dialog = editor(appPage);
  await expect(dialog).toBeVisible();
  await expect(dialog.getByTestId("automation-start-event")).toHaveAttribute(
    "aria-checked",
    "true"
  );
  expect(new URL(appPage.url()).search).not.toContain("%22");

  await dialog.getByRole("button", { name: "Cancel" }).click();
  await expect(dialog).toBeHidden();
  await expect(appPage).toHaveURL(/\/automations(?:\?(?!.*create=).*)?$/);
});
