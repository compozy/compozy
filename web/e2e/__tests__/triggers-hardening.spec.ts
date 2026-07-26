import { execFile } from "node:child_process";
import { createHmac, randomUUID } from "node:crypto";
import { readFile } from "node:fs/promises";
import path from "node:path";
import process from "node:process";
import { fileURLToPath } from "node:url";
import { promisify } from "node:util";

import type { Page } from "@playwright/test";

import { automationOperatorSelectors, sessionWindowSelectors } from "../fixtures/selectors";
import { sessionWindow, windowTitle } from "../fixtures/os-navigation";
import type { BrowserRuntime } from "../fixtures/runtime";
import { browserAutomationOperatorFlowScenario } from "../fixtures/runtime";
import { expect, test } from "../fixtures/test";
import { ensureGlobalWorkspace, useGlobalWorkspaceIfPrompted } from "../fixtures/workspace";

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
const automationAgentName = "browser-triggers-runner";
const faultAgentName = "browser-triggers-fault";
const webhookSecret = "browser-trigger-secret";
const payloadSecret = "browser-trigger-payload-secret";
const sensitivePattern =
  /agh_claim_|["']claim_token["']\s*:|mcp[_-]?auth|telegram-bot-token|pkce|oauth|provider[_-]?credentials?["'\s]*[:=]|browser-trigger-secret|browser-trigger-payload-secret/i;

async function addWebhookBranchFilter(
  ui: ReturnType<typeof automationOperatorSelectors>
): Promise<void> {
  await ui.triggerFilterAdd.click();
  await ui.triggerFilterKey(0).fill("data.branch");
  await ui.triggerFilterValue(0).fill("main");
}

interface AutomationTrigger {
  id: string;
  agent_name: string;
  enabled: boolean;
  endpoint_slug?: string | null;
  event: string;
  fire_limit: { max: number; window: string };
  name: string;
  prompt: string;
  retry: { base_delay: string; max_retries: number; strategy: "none" | "backoff" };
  scope: "global" | "workspace";
  source: "dynamic" | "config";
  webhook_id?: string | null;
  webhook_secret_present?: boolean;
  workspace_id?: string | null;
}

interface AutomationRun {
  id: string;
  attempt: number;
  delivery_error?: string | null;
  error?: string | null;
  session_id?: string | null;
  status: "scheduled" | "running" | "delegated" | "completed" | "failed" | "canceled";
  trigger_id?: string | null;
  workspace_id?: string | null;
}

interface TriggerRequest {
  agent_name: string;
  enabled: boolean;
  endpoint_slug?: string;
  event: string;
  filter?: Record<string, string>;
  fire_limit: { max: number; window: string };
  name: string;
  prompt: string;
  retry: { base_delay: string; max_retries: number; strategy: "none" | "backoff" };
  scope: "global" | "workspace";
  webhook_id?: string;
  webhook_secret_value?: string;
  workspace_id?: string;
}

interface TriggersResponse {
  triggers: AutomationTrigger[];
}

interface TriggerResponse {
  trigger: AutomationTrigger;
}

interface RunResponse {
  run: AutomationRun;
}

interface RunsResponse {
  runs: AutomationRun[];
}

interface WebhookDeliveryResponse {
  result: {
    matched: number;
    runs?: AutomationRun[];
  };
}

interface LogsListResponse {
  events: Array<{
    id: string;
    session_id?: string;
    type: string;
    agent_name?: string;
    summary?: string;
  }>;
}

interface WebhookDelivery {
  body: string;
  json?: WebhookDeliveryResponse;
  status: number;
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

test("operator creates updates fires disables re-enables and deletes a webhook trigger with parity evidence", async ({
  appPage,
  browserArtifacts,
  runtime,
}) => {
  const triggersWin = appPage.getByTestId("os-window-app:triggers");
  const ui = automationOperatorSelectors(triggersWin, appPage);
  const shellUI = automationOperatorSelectors(appPage);
  const triggerStatus = triggersWin.locator('[data-slot="topbar-status"]');
  await ensureGlobalWorkspace(runtime);
  await appPage.reload({ waitUntil: "domcontentloaded" });
  await useGlobalWorkspaceIfPrompted(shellUI);

  await appPage.goto(runtime.url("/triggers"), { waitUntil: "domcontentloaded" });
  await expect(ui.triggersShell).toBeVisible();

  const initialName = uniqueName("triggers-lifecycle");
  const editedName = `${initialName}-edited`;
  const webhookID = `wbh_${randomUUID().replaceAll("-", "_").slice(0, 18)}`;
  const initialEndpointSlug = uniqueName("browser-trigger");
  const editedEndpointSlug = `${initialEndpointSlug}-v2`;
  const prompt = browserAutomationOperatorFlowScenario.trigger.prompt;
  const editedPrompt = `{{ printf "Review payload %s for %s" (index .Data "payload") (index .Data "branch") }}`;

  await ui.createTriggerButton.click();
  await expect(ui.editorDialog).toBeVisible();
  await expect(ui.submitTriggerForm).toBeDisabled();
  await ui.triggerNameInput.fill(initialName);
  await ui.triggerAgentInput.fill(automationAgentName);
  await ui.triggerEventOption("webhook").click();
  await expect(ui.triggerEndpointSlugInput).toBeVisible();
  await ui.triggerPromptInput.fill(prompt);
  await ui.triggerScopeGlobal.click();
  await addWebhookBranchFilter(ui);
  await ui.triggerEndpointSlugInput.fill(initialEndpointSlug);
  await ui.triggerWebhookIDInput.fill(webhookID);
  await ui.triggerWebhookSecretValueInput.fill(webhookSecret);
  await expect(ui.submitTriggerForm).toBeEnabled();

  const createResponse = appPage.waitForResponse(
    response =>
      response.request().method() === "POST" && response.url().endsWith("/api/automation/triggers")
  );
  await ui.submitTriggerForm.click();
  const createBody = await (await createResponse).text();
  expect(createBody).not.toMatch(sensitivePattern);
  await expect(ui.editorDialog).toBeHidden();

  const created = await waitForTriggerByName(runtime, initialName);
  expect(created.webhook_secret_present).toBe(true);
  // Saving from the editor navigates straight to the new trigger's detail route.
  await expect(appPage).toHaveURL(new RegExp(`/triggers/${created.id}$`), { timeout: 20_000 });
  await expect(windowTitle(triggersWin)).toContainText(initialName);
  await expect(ui.detailPanel).toContainText(webhookID);
  await expect(ui.detailPanel).not.toContainText(webhookSecret);

  await ui.detailOverflow.click();
  await appPage.getByTestId("edit-automation-btn").click();
  await expect(ui.editorDialog).toBeVisible();
  await ui.triggerNameInput.fill(editedName);
  await ui.triggerPromptInput.fill(editedPrompt);
  await ui.triggerEndpointSlugInput.fill(editedEndpointSlug);
  const updateResponse = appPage.waitForResponse(
    response =>
      response.request().method() === "PATCH" &&
      response.url().endsWith(`/api/automation/triggers/${encodeURIComponent(created.id)}`)
  );
  await ui.submitTriggerForm.click();
  const updateBody = await (await updateResponse).text();
  expect(updateBody).not.toMatch(sensitivePattern);
  await expect(ui.editorDialog).toBeHidden();

  const updated = await waitForTriggerByName(runtime, editedName);
  expect(updated.endpoint_slug).toBe(editedEndpointSlug);
  expect(updated.prompt).toBe(editedPrompt);
  await expect(windowTitle(triggersWin)).toContainText(editedName);
  await expect(ui.detailPanel).toContainText(editedPrompt);

  const endpoint = endpointFor(updated);
  const invalidSignature = await deliverWebhook(runtime, {
    deliveryID: uniqueName("delivery-invalid"),
    endpoint,
    payload: triggerPayload("deploy", "main"),
    secret: "wrong-secret",
    wantStatus: 401,
  });
  expect(invalidSignature.body).toMatch(/signature/i);
  expect(invalidSignature.body).not.toMatch(sensitivePattern);

  const stale = await deliverWebhook(runtime, {
    deliveryID: uniqueName("delivery-stale"),
    endpoint,
    payload: triggerPayload("deploy", "main"),
    secret: webhookSecret,
    timestamp: new Date(Date.now() - 10 * 60 * 1000),
    wantStatus: 401,
  });
  expect(stale.body).toMatch(/timestamp|freshness|signature/i);
  expect(stale.body).not.toMatch(sensitivePattern);

  const validPayload = triggerPayload("deploy", "main");
  const validDeliveryID = uniqueName("delivery-valid");
  const validDelivery = await deliverWebhook(runtime, {
    deliveryID: validDeliveryID,
    endpoint,
    payload: validPayload,
    secret: webhookSecret,
    wantStatus: 200,
  });
  expect(validDelivery.json?.result.matched).toBe(1);
  const firstRunID = validDelivery.json?.result.runs?.[0]?.id;
  expect(firstRunID).toBeTruthy();
  const firstRun = await waitForTriggerRun(runtime, updated.id, firstRunID ?? "", "completed");
  expect(firstRun.session_id).toBeTruthy();

  const replay = await deliverWebhook(runtime, {
    deliveryID: validDeliveryID,
    endpoint,
    payload: validPayload,
    secret: webhookSecret,
    wantStatus: 409,
  });
  expect(replay.body).toMatch(/processed|replay|delivery/i);
  expect(await triggerRunCount(runtime, updated.id)).toBe(1);

  await appPage.reload({ waitUntil: "domcontentloaded" });
  await expect(windowTitle(triggersWin)).toContainText(editedName, { timeout: 20_000 });
  await expect(ui.run(firstRun.id)).toBeVisible();
  await expect(ui.runSessionLink(firstRun.id)).toBeVisible();

  await ui.detailOverflow.click();
  await ui.toggleAutomationButton.click();
  await expect
    .poll(async () => (await getTrigger(runtime, updated.id)).trigger.enabled)
    .toBe(false);
  await expect(triggerStatus).toContainText("DISABLED");
  const disabledDelivery = await deliverWebhook(runtime, {
    deliveryID: uniqueName("delivery-disabled"),
    endpoint,
    payload: triggerPayload("deploy", "main"),
    secret: webhookSecret,
    wantStatus: 404,
  });
  expect(disabledDelivery.body).toMatch(/not registered|not found/i);
  expect(await triggerRunCount(runtime, updated.id)).toBe(1);

  await ui.detailOverflow.click();
  await ui.toggleAutomationButton.click();
  await expect.poll(async () => (await getTrigger(runtime, updated.id)).trigger.enabled).toBe(true);
  const reenabledDelivery = await deliverWebhook(runtime, {
    deliveryID: uniqueName("delivery-reenabled"),
    endpoint,
    payload: triggerPayload("deploy", "main"),
    secret: webhookSecret,
    wantStatus: 200,
  });
  const reenabledRunID = reenabledDelivery.json?.result.runs?.[0]?.id;
  expect(reenabledRunID).toBeTruthy();
  const reenabledRun = await waitForTriggerRun(
    runtime,
    updated.id,
    reenabledRunID ?? "",
    "completed"
  );
  expect(await triggerRunCount(runtime, updated.id)).toBe(2);

  await appPage.reload({ waitUntil: "domcontentloaded" });
  await expect(windowTitle(triggersWin)).toContainText(editedName, { timeout: 20_000 });
  await expect(ui.run(reenabledRun.id)).toBeVisible();
  await expect(ui.runSessionLink(reenabledRun.id)).toBeVisible();

  const reenabledWorkspaceID = await resolveAutomationWorkspaceID(
    runtime,
    reenabledRun.workspace_id
  );
  const parity = await captureTriggerParity(
    runtime,
    updated.id,
    reenabledRun.id,
    reenabledWorkspaceID
  );
  expect(parity.http.trigger.name).toBe(editedName);
  expect(parity.uds.trigger.webhook_id).toBe(webhookID);
  expect(parity.cliGet.id).toBe(updated.id);
  expect(parity.cliHistory.runs.some(run => run.id === reenabledRun.id)).toBe(true);
  expect(parity.httpRun.run.session_id).toBe(reenabledRun.session_id);
  expect(parity.observe.events.length).toBeGreaterThan(0);

  await assertTriggersViewportMatrix(appPage, browserArtifacts, runtime, updated.id);
  await runtime.artifactCollector.captureJSON("browser_api_snapshots", {
    invalidSignature: { status: invalidSignature.status },
    parity,
    replay: { status: replay.status },
    stale: { status: stale.status },
    trigger_id: updated.id,
    webhook_endpoint: endpoint,
  });
  await browserArtifacts.captureScreenshot("triggers-lifecycle-history", appPage);
  await browserArtifacts.persist(appPage);
  const routeState = await readRouteState(runtime);
  expect(routeState).toMatchObject({
    automation_active_tab: "triggers",
    automation_detail_overflow_visible: true,
    automation_run_count: expect.any(Number),
    automation_run_history_visible: true,
    automation_selected_item: editedName,
    automation_session_link_count: 2,
    automation_view_visible: true,
  });
  expect(Number(routeState.automation_run_count)).toBeGreaterThanOrEqual(2);

  await ui.runSessionLink(reenabledRun.id).click();
  const reenabledSessionId = reenabledRun.session_id;
  if (!reenabledSessionId) {
    throw new Error("Expected the re-enabled trigger run to expose a linked session.");
  }
  const sessionUI = sessionWindowSelectors(sessionWindow(appPage, reenabledSessionId));
  await expect(sessionUI.chatView).toBeVisible();
  await expect(sessionUI.chatView).toContainText("Review payload deploy for main");
  await expect(sessionUI.chatView).toContainText(
    browserAutomationOperatorFlowScenario.transcript.assistant
  );

  await appPage.goto(runtime.url("/triggers"), { waitUntil: "domcontentloaded" });
  await expect(ui.item(updated.id)).toBeVisible({ timeout: 20_000 });
  await ui.itemLink(updated.id).click();
  await ui.detailOverflow.click();
  await ui.deleteAutomationButton.click();
  await expect(ui.automationDeleteDialog).toBeVisible();
  await ui.automationDeleteConfirmTyping.fill(editedName);
  const deleteResponsePromise = appPage.waitForResponse(
    response =>
      response.request().method() === "DELETE" &&
      new URL(response.url()).pathname === `/api/automation/triggers/${updated.id}`
  );
  await ui.confirmDeleteAutomationButton.click();
  const deleteResponse = await deleteResponsePromise;
  expect(deleteResponse.ok()).toBe(true);
  await expect(ui.automationDeleteDialog).toBeHidden();
  await expect.poll(async () => await getTriggerStatus(runtime, updated.id)).toBe(404);
  const afterDelete = await deliverWebhook(runtime, {
    deliveryID: uniqueName("delivery-deleted"),
    endpoint,
    payload: triggerPayload("deploy", "main"),
    secret: webhookSecret,
    wantStatus: 404,
  });
  expect(afterDelete.body).not.toMatch(sensitivePattern);

  await assertNoTriggerSensitiveLeak(appPage, runtime, {
    afterDelete: { status: afterDelete.status },
    parity,
    routeState,
  });
  await deleteSessionIfExists(
    runtime,
    await resolveAutomationWorkspaceID(runtime, firstRun.workspace_id),
    firstRun.session_id
  );
  await deleteSessionIfExists(runtime, reenabledWorkspaceID, reenabledRun.session_id);
});

test("failed webhook trigger run is diagnosable with retry evidence and no secret leakage", async ({
  appPage,
  browserArtifacts,
  runtime,
}) => {
  const triggersWin = appPage.getByTestId("os-window-app:triggers");
  const ui = automationOperatorSelectors(triggersWin, appPage);
  const shellUI = automationOperatorSelectors(appPage);
  const trigger = await createTrigger(
    runtime,
    triggerRequest({
      agentName: faultAgentName,
      name: uniqueName("triggers-failure"),
      prompt: "trigger crash mid-stream",
      retry: { strategy: "backoff", max_retries: 1, base_delay: "100ms" },
    })
  );
  const endpoint = endpointFor(trigger);

  await ensureGlobalWorkspace(runtime);
  await appPage.reload({ waitUntil: "domcontentloaded" });
  await useGlobalWorkspaceIfPrompted(shellUI);
  await appPage.goto(runtime.url("/triggers"), { waitUntil: "domcontentloaded" });
  await expect(ui.triggersShell).toBeVisible();
  await expect(ui.item(trigger.id)).toBeVisible({ timeout: 20_000 });
  await ui.itemLink(trigger.id).click();
  await expect(ui.detailPanel).toContainText("1 retries from 100ms");

  const delivery = await deliverWebhook(runtime, {
    deliveryID: uniqueName("delivery-failure"),
    endpoint,
    payload: triggerPayload("deploy", "main"),
    secret: webhookSecret,
    wantStatus: 500,
  });
  expect(delivery.body).not.toMatch(sensitivePattern);
  const failedRun = await waitForLatestTriggerRun(
    runtime,
    trigger.id,
    "failed",
    run => run.attempt > 1
  );
  expect(failedRun.attempt).toBeGreaterThan(1);
  const failureMessage = `${failedRun.error ?? ""} ${failedRun.delivery_error ?? ""}`;
  expect(failureMessage).toMatch(/peer disconnected before response|internal error/i);

  await appPage.reload({ waitUntil: "domcontentloaded" });
  await expect(windowTitle(triggersWin)).toContainText(trigger.name, { timeout: 20_000 });
  await expect(ui.run(failedRun.id)).toBeVisible();
  await expect(ui.run(failedRun.id)).toContainText("FAILED");
  await expect(ui.run(failedRun.id)).toContainText(
    /peer disconnected before response|internal error/i
  );

  const failedWorkspaceID = await resolveAutomationWorkspaceID(runtime, failedRun.workspace_id);
  const parity = await captureTriggerParity(runtime, trigger.id, failedRun.id, failedWorkspaceID);
  expect(parity.httpRun.run.status).toBe("failed");
  expect(parity.cliRun.status).toBe("failed");
  expect(parity.cliHistory.runs.some(run => run.id === failedRun.id && run.attempt > 1)).toBe(true);

  await runtime.artifactCollector.captureJSON("browser_api_snapshots", parity);
  await browserArtifacts.captureScreenshot("triggers-failure-diagnostics", appPage);
  await assertTriggerRunViewportMatrix(
    appPage,
    browserArtifacts,
    runtime,
    trigger.id,
    failedRun.id,
    "triggers-failure-diagnostics"
  );
  await browserArtifacts.persist(appPage);
  const routeState = await readRouteState(runtime);
  expect(routeState).toMatchObject({
    automation_active_tab: "triggers",
    automation_run_count: expect.any(Number),
    automation_run_history_visible: true,
    automation_selected_item: trigger.name,
    automation_view_visible: true,
  });
  await assertNoTriggerSensitiveLeak(appPage, runtime, { parity, routeState });
  await deleteSessionIfExists(runtime, failedWorkspaceID, failedRun.session_id);
  await deleteTriggerIfExists(runtime, trigger.id);
});

test("operator sees fire-limit rejection across browser and runtime surfaces", async ({
  appPage,
  browserArtifacts,
  runtime,
}) => {
  const triggersWin = appPage.getByTestId("os-window-app:triggers");
  const ui = automationOperatorSelectors(triggersWin, appPage);
  const shellUI = automationOperatorSelectors(appPage);
  const trigger = await createTrigger(
    runtime,
    triggerRequest({
      fireLimit: { max: 1, window: "1h" },
      name: uniqueName("triggers-fire-limit"),
      prompt: browserAutomationOperatorFlowScenario.trigger.prompt,
    })
  );
  const endpoint = endpointFor(trigger);

  await ensureGlobalWorkspace(runtime);
  await appPage.reload({ waitUntil: "domcontentloaded" });
  await useGlobalWorkspaceIfPrompted(shellUI);
  await appPage.goto(runtime.url("/triggers"), { waitUntil: "domcontentloaded" });
  await expect(ui.triggersShell).toBeVisible();
  await expect(ui.item(trigger.id)).toBeVisible({ timeout: 20_000 });
  await ui.itemLink(trigger.id).click();
  await expect(ui.detailPanel).toContainText("1 fires / 1h");

  const accepted = await deliverWebhook(runtime, {
    deliveryID: uniqueName("delivery-fire-limit-first"),
    endpoint,
    payload: triggerPayload("deploy", "main"),
    secret: webhookSecret,
    wantStatus: 200,
  });
  const acceptedRunID = accepted.json?.result.runs?.[0]?.id;
  expect(acceptedRunID).toBeTruthy();
  const acceptedRun = await waitForTriggerRun(
    runtime,
    trigger.id,
    acceptedRunID ?? "",
    "completed"
  );

  const limited = await deliverWebhook(runtime, {
    deliveryID: uniqueName("delivery-fire-limit-second"),
    endpoint,
    payload: triggerPayload("deploy", "main"),
    secret: webhookSecret,
    wantStatus: 409,
  });
  expect(limited.body).toMatch(/fire limit|limit/i);
  expect(await triggerRunCount(runtime, trigger.id)).toBe(2);

  await appPage.reload({ waitUntil: "domcontentloaded" });
  await expect(windowTitle(triggersWin)).toContainText(trigger.name, { timeout: 20_000 });
  await expect(ui.run(acceptedRun.id)).toBeVisible();
  const limitedRun = (await listTriggerRuns(runtime, trigger.id)).find(
    run => run.id !== acceptedRun.id
  );
  expect(limitedRun?.status).toBe("failed");
  expect([limitedRun?.error ?? "", limitedRun?.delivery_error ?? ""].join(" ")).toMatch(
    /fire limit/i
  );
  await expect(ui.run(limitedRun?.id ?? "")).toBeVisible();
  await expect(ui.run(limitedRun?.id ?? "")).toContainText("FAILED");
  await expect(ui.runHistory).toContainText("COMPLETED");
  const acceptedWorkspaceID = await resolveAutomationWorkspaceID(runtime, acceptedRun.workspace_id);
  const parity = await captureTriggerParity(
    runtime,
    trigger.id,
    acceptedRun.id,
    acceptedWorkspaceID
  );
  expect(parity.httpRuns.runs).toHaveLength(2);
  expect(parity.cliHistory.runs).toHaveLength(2);
  await runtime.artifactCollector.captureJSON("browser_api_snapshots", {
    fireLimit: { response: { status: limited.status }, parity },
  });
  await browserArtifacts.captureScreenshot("triggers-fire-limit-rejection", appPage);
  await assertTriggerRunViewportMatrix(
    appPage,
    browserArtifacts,
    runtime,
    trigger.id,
    acceptedRun.id,
    "triggers-fire-limit-rejection"
  );
  await browserArtifacts.persist(appPage);
  await assertNoTriggerSensitiveLeak(appPage, runtime, { limited, parity });
  await deleteSessionIfExists(runtime, acceptedWorkspaceID, acceptedRun.session_id);
  await deleteTriggerIfExists(runtime, trigger.id);
});

function triggerRequest(input: {
  agentName?: string;
  endpointSlug?: string;
  fireLimit?: TriggerRequest["fire_limit"];
  filter?: Record<string, string>;
  name: string;
  prompt?: string;
  retry?: TriggerRequest["retry"];
  webhookID?: string;
}): TriggerRequest {
  return {
    agent_name: input.agentName ?? automationAgentName,
    enabled: true,
    endpoint_slug: input.endpointSlug ?? uniqueName("browser-trigger"),
    event: "webhook",
    filter: input.filter ?? { "data.branch": "main" },
    fire_limit: input.fireLimit ?? { max: 12, window: "1h" },
    name: input.name,
    prompt: input.prompt ?? browserAutomationOperatorFlowScenario.trigger.prompt,
    retry: input.retry ?? { strategy: "none", max_retries: 0, base_delay: "" },
    scope: "global",
    webhook_id: input.webhookID ?? `wbh_${randomUUID().replaceAll("-", "_").slice(0, 18)}`,
    webhook_secret_value: webhookSecret,
  };
}

async function createTrigger(
  runtime: BrowserRuntime,
  request: TriggerRequest
): Promise<AutomationTrigger> {
  return (
    await runtime.requestJSON<TriggerResponse>("/api/automation/triggers", {
      method: "POST",
      body: JSON.stringify(request),
    })
  ).trigger;
}

async function getTrigger(runtime: BrowserRuntime, id: string): Promise<TriggerResponse> {
  return await runtime.requestJSON<TriggerResponse>(
    `/api/automation/triggers/${encodeURIComponent(id)}`
  );
}

async function getTriggerStatus(runtime: BrowserRuntime, id: string): Promise<number> {
  const response = await fetch(runtime.url(`/api/automation/triggers/${encodeURIComponent(id)}`));
  return response.status;
}

async function deleteTriggerIfExists(runtime: BrowserRuntime, id: string): Promise<void> {
  const response = await fetch(runtime.url(`/api/automation/triggers/${encodeURIComponent(id)}`), {
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
  const workspace = workspaceID?.trim();
  if (!workspace) {
    throw new Error(`delete session ${id} requires workspace_id`);
  }
  const response = await fetch(runtime.url(sessionAPIPath(workspace, id)), {
    method: "DELETE",
  });
  expect([204, 404]).toContain(response.status);
}

async function waitForTriggerByName(
  runtime: BrowserRuntime,
  name: string
): Promise<AutomationTrigger> {
  let matched: AutomationTrigger | undefined;
  await expect
    .poll(async () => {
      const payload = await runtime.requestJSON<TriggersResponse>(
        "/api/automation/triggers?limit=50"
      );
      matched = payload.triggers.find(trigger => trigger.name === name);
      return matched?.id ?? "";
    })
    .not.toBe("");
  if (!matched) {
    throw new Error(`Expected automation trigger ${name} to be visible.`);
  }
  return matched;
}

async function listTriggerRuns(
  runtime: BrowserRuntime,
  triggerID: string
): Promise<AutomationRun[]> {
  return (
    await runtime.requestJSON<RunsResponse>(
      `/api/automation/triggers/${encodeURIComponent(triggerID)}/runs?limit=50`
    )
  ).runs;
}

async function triggerRunCount(runtime: BrowserRuntime, triggerID: string): Promise<number> {
  return (await listTriggerRuns(runtime, triggerID)).length;
}

async function waitForTriggerRun(
  runtime: BrowserRuntime,
  triggerID: string,
  runID: string,
  status: AutomationRun["status"]
): Promise<AutomationRun> {
  let matched: AutomationRun | undefined;
  await expect
    .poll(
      async () => {
        const runs = await listTriggerRuns(runtime, triggerID);
        matched = runs.find(run => run.id === runID && run.status === status);
        return matched?.id ?? "";
      },
      { timeout: 45_000 }
    )
    .not.toBe("");
  if (!matched) {
    throw new Error(`Expected ${status} automation trigger run ${runID}.`);
  }
  return matched;
}

async function waitForLatestTriggerRun(
  runtime: BrowserRuntime,
  triggerID: string,
  status: AutomationRun["status"],
  predicate: (run: AutomationRun) => boolean = () => true
): Promise<AutomationRun> {
  let matched: AutomationRun | undefined;
  await expect
    .poll(
      async () => {
        const runs = await listTriggerRuns(runtime, triggerID);
        matched = runs.find(run => run.status === status && predicate(run));
        return matched?.id ?? "";
      },
      { timeout: 45_000 }
    )
    .not.toBe("");
  if (!matched) {
    throw new Error(`Expected latest ${status} automation trigger run for ${triggerID}.`);
  }
  return matched;
}

async function captureTriggerParity(
  runtime: BrowserRuntime,
  triggerID: string,
  runID: string,
  workspaceID: string
) {
  const http = await getTrigger(runtime, triggerID);
  const uds = await requestOperatorJSONOrThrow<TriggerResponse>(
    runtime,
    `/api/automation/triggers/${encodeURIComponent(triggerID)}`
  );
  const httpRuns = await runtime.requestJSON<RunsResponse>(
    `/api/automation/triggers/${encodeURIComponent(triggerID)}/runs?limit=50`
  );
  const httpRun = await runtime.requestJSON<RunResponse>(
    `/api/automation/runs/${encodeURIComponent(runID)}`
  );
  const cliGet = await automationCLI<AutomationTrigger>(runtime, [
    "automation",
    "triggers",
    "get",
    triggerID,
  ]);
  const cliHistory = await automationCLI<RunsResponse>(runtime, [
    "automation",
    "triggers",
    "history",
    triggerID,
    "--last",
    "50",
  ]);
  const cliRun = await automationCLI<AutomationRun>(runtime, ["automation", "runs", "get", runID]);
  const observe = httpRun.run.session_id
    ? await runtime.requestJSON<LogsListResponse>(
        workspaceListLogsPath(workspaceID, httpRun.run.session_id)
      )
    : { events: [] };
  return {
    cliGet,
    cliHistory,
    cliRun,
    http,
    httpRun,
    httpRuns,
    observe,
    uds,
  };
}

async function resolveAutomationWorkspaceID(
  runtime: BrowserRuntime,
  candidate?: string | null
): Promise<string> {
  const explicit = candidate?.trim();
  if (explicit) {
    return explicit;
  }
  const seeded = runtime.seeded.workspace?.id?.trim();
  if (seeded) {
    return seeded;
  }
  if (runtime.paths?.homeDir) {
    const workspace = await runtime.resolveWorkspace(runtime.paths.homeDir);
    return workspace.id;
  }
  const payload = await runtime.requestJSON<{ workspaces: Array<{ id: string }> }>(
    "/api/workspaces"
  );
  const workspaceID = payload.workspaces[0]?.id?.trim();
  if (!workspaceID) {
    throw new Error("automation trigger parity requires a workspace_id");
  }
  return workspaceID;
}

function workspaceListLogsPath(
  workspaceID: string | null | undefined,
  sessionID: string | null | undefined
): string {
  const workspace = workspaceID?.trim();
  const session = sessionID?.trim();
  if (!workspace || !session) {
    throw new Error("logs parity requires workspace_id and session_id");
  }
  return `/api/logs?workspace_id=${encodeURIComponent(workspace)}&session_id=${encodeURIComponent(session)}&limit=20`;
}

function sessionAPIPath(workspaceID: string, sessionID: string): string {
  return `/api/workspaces/${encodeURIComponent(workspaceID)}/sessions/${encodeURIComponent(sessionID)}`;
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

async function deliverWebhook(
  runtime: BrowserRuntime,
  input: {
    deliveryID: string;
    endpoint: string;
    payload: string;
    secret: string;
    timestamp?: Date;
    wantStatus: number;
  }
): Promise<WebhookDelivery> {
  const timestamp = input.timestamp ?? new Date();
  const body = input.payload;
  const response = await fetch(
    runtime.url(`/api/webhooks/global/${encodeURIComponent(input.endpoint)}`),
    {
      method: "POST",
      headers: {
        "content-type": "application/json",
        "x-agh-webhook-delivery-id": input.deliveryID,
        "x-agh-webhook-signature": signWebhook(input.secret, timestamp, body),
        "x-agh-webhook-timestamp": timestamp.toISOString(),
      },
      body,
    }
  );
  const responseBody = await response.text();
  expect(response.status).toBe(input.wantStatus);
  expect(responseBody).not.toMatch(sensitivePattern);
  return {
    body: responseBody,
    json: response.ok ? (JSON.parse(responseBody) as WebhookDeliveryResponse) : undefined,
    status: response.status,
  };
}

function signWebhook(secret: string, timestamp: Date, payload: string): string {
  const seconds = Math.floor(timestamp.getTime() / 1000);
  const signature = createHmac("sha256", secret).update(`${seconds}.${payload}`).digest("hex");
  return `sha256=${signature}`;
}

function triggerPayload(payload: string, branch: string): string {
  return JSON.stringify({
    branch,
    payload,
    secret_probe: payloadSecret,
  });
}

function endpointFor(trigger: AutomationTrigger): string {
  if (!trigger.endpoint_slug || !trigger.webhook_id) {
    throw new Error(`trigger ${trigger.id} does not expose a webhook endpoint`);
  }
  return `${trigger.endpoint_slug}--${trigger.webhook_id}`;
}

async function assertTriggersViewportMatrix(
  appPage: Page,
  browserArtifacts: { captureScreenshot: (name: string, page?: Page) => Promise<unknown> },
  runtime: BrowserRuntime,
  triggerID: string
): Promise<void> {
  const triggersWin = appPage.getByTestId("os-window-app:triggers");
  const ui = automationOperatorSelectors(triggersWin, appPage);
  for (const width of [375, 768, 1280]) {
    await appPage.setViewportSize({ width, height: 820 });
    await appPage.goto(runtime.url("/triggers"), { waitUntil: "domcontentloaded" });
    await expect(ui.triggersShell).toBeVisible();
    await expect(ui.item(triggerID)).toBeVisible({ timeout: 20_000 });
    await ui.itemLink(triggerID).click();
    await expect(ui.runHistory).toBeVisible();
    await browserArtifacts.captureScreenshot(
      `triggers-lifecycle-history-viewport-${width}`,
      appPage
    );
    await ui.detailOverflow.click();
    await appPage.getByTestId("edit-automation-btn").click();
    await expect(ui.editorDialog).toBeVisible();
    await expect(ui.triggerEndpointSlugInput).toBeVisible();
    await expect(ui.submitTriggerForm).toBeEnabled();
    await browserArtifacts.captureScreenshot(
      `triggers-lifecycle-editor-viewport-${width}`,
      appPage
    );
    await appPage.keyboard.press("Escape");
    await expect(ui.editorDialog).toBeHidden();
  }
}

async function assertTriggerRunViewportMatrix(
  appPage: Page,
  browserArtifacts: { captureScreenshot: (name: string, page?: Page) => Promise<unknown> },
  runtime: BrowserRuntime,
  triggerID: string,
  runID: string,
  prefix: string
): Promise<void> {
  const triggersWin = appPage.getByTestId("os-window-app:triggers");
  const ui = automationOperatorSelectors(triggersWin, appPage);
  for (const width of [375, 768, 1280]) {
    await appPage.setViewportSize({ width, height: 820 });
    await appPage.goto(runtime.url("/triggers"), { waitUntil: "domcontentloaded" });
    await expect(ui.triggersShell).toBeVisible();
    await expect(ui.item(triggerID)).toBeVisible({ timeout: 20_000 });
    await ui.itemLink(triggerID).click();
    await expect(ui.runHistory).toBeVisible();
    await expect(ui.run(runID)).toBeVisible();
    await browserArtifacts.captureScreenshot(`${prefix}-viewport-${width}`, appPage);
  }
}

async function assertNoTriggerSensitiveLeak(
  appPage: Page,
  runtime: BrowserRuntime,
  snapshot: unknown
): Promise<void> {
  expect(JSON.stringify(snapshot)).not.toMatch(sensitivePattern);
  expect((await appPage.textContent("body")) ?? "").not.toMatch(sensitivePattern);
  const routeStatePath = runtime.artifactCollector.artifactPath("browser_route_state");
  const apiSnapshotPath = runtime.artifactCollector.artifactPath("browser_api_snapshots");
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
    PATH: `${path.dirname(paths.cliShim)}:${process.env.PATH ?? ""}`,
  };
}

function uniqueName(prefix: string): string {
  return `${prefix}-${randomUUID().slice(0, 8)}`;
}

async function readRouteState(runtime: BrowserRuntime): Promise<Record<string, unknown>> {
  return JSON.parse(
    await readFile(runtime.artifactCollector.artifactPath("browser_route_state"), "utf8")
  ) as Record<string, unknown>;
}
