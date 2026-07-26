import { execFile } from "node:child_process";
import { readFile } from "node:fs/promises";
import path from "node:path";
import process from "node:process";
import { fileURLToPath } from "node:url";
import { promisify } from "node:util";

import type { Page } from "@playwright/test";
import type {
  BridgeDetailResponse,
  BridgeHealth,
  BridgeRoute,
  BridgeSecretBinding,
  BridgeSummary,
  TestBridgeDeliveryResponse,
} from "@/systems/bridges";

import { captureRouteState } from "../fixtures/browser-artifact-session";
import { bridgeOperatorSelectors } from "../fixtures/selectors";
import { openAppWindow, windowTitle } from "../fixtures/os-navigation";
import type { BrowserRuntime } from "../fixtures/runtime";
import {
  browserBridgeOperatorFlowScenario,
  seedBrowserBridgeOperatorFlow,
  triggerBrowserBridgeIngress,
} from "../fixtures/runtime";
import { expect, test } from "../fixtures/test";
import { useGlobalWorkspaceIfPrompted } from "../fixtures/workspace";

const bridgeIngressFixture = path.resolve(
  path.dirname(fileURLToPath(import.meta.url)),
  "..",
  "..",
  "..",
  "internal",
  "testutil",
  "acpmock",
  "testdata",
  "bridge_ingress_fixture.json"
);

const execFileAsync = promisify(execFile);
const bridgeRawSecret = "telegram-super-secret-token-09";
const bridgeSensitiveValues = [
  browserBridgeOperatorFlowScenario.secretBinding.value,
  bridgeRawSecret,
  "agh_claim_bridge_secret_09",
  "mcp-auth-token-bridge-09",
  "oauth-bridge-secret-09",
  "pkce-bridge-secret-09",
  "bridge-webhook-secret-09",
] as const;

const bridgeRuntimeEnv = {
  AGH_TEST_TELEGRAM_TOKEN: "telegram-bot-token",
  ...(process.env.AGH_WEB_DIST_DIR?.trim()
    ? { AGH_WEB_DIST_DIR: process.env.AGH_WEB_DIST_DIR }
    : {}),
  ...(process.env.PATH?.trim() ? { PATH: process.env.PATH } : {}),
  ...(process.env.TMPDIR?.trim() ? { TMPDIR: process.env.TMPDIR } : {}),
  ...(process.env.TMP?.trim() ? { TMP: process.env.TMP } : {}),
  ...(process.env.TEMP?.trim() ? { TEMP: process.env.TEMP } : {}),
  ...(process.platform === "win32" && process.env.PATHEXT?.trim()
    ? { PATHEXT: process.env.PATHEXT }
    : {}),
  ...(process.platform === "win32" && process.env.SystemRoot?.trim()
    ? { SystemRoot: process.env.SystemRoot }
    : {}),
  ...(process.platform === "win32" && process.env.ComSpec?.trim()
    ? { ComSpec: process.env.ComSpec }
    : {}),
};

const createdBridgeName = "Telegram Bridge Create Smoke";
const createdBridgeProviderKey = "telegram-reference::telegram";

test.setTimeout(120_000);

test.use({
  runtimeOptions: {
    env: bridgeRuntimeEnv,
    extensionsAllowUnverified: true,
    seed: {
      mockAgents: [
        {
          fixturePath: bridgeIngressFixture,
          fixtureAgent: "bridge-runner",
          agentName: "general",
        },
      ],
    },
  },
});

test("operator can edit bridge config, enable runtime, observe status updates, and resolve delivery targets", async ({
  appPage,
  browserArtifacts,
  runtime,
}) => {
  const shellUI = bridgeOperatorSelectors(appPage);
  const seeded = await seedBrowserBridgeOperatorFlow(runtime);

  await useGlobalWorkspaceIfPrompted(shellUI);

  await expect(shellUI.osDesktop).toBeVisible();
  const bridgesWin = await openAppWindow(appPage, "Bridges", "bridges");
  const bridgeUI = bridgeOperatorSelectors(bridgesWin);
  const bridgeStatus = bridgesWin.locator('[data-slot="topbar-status"]');

  await expect(appPage).toHaveURL(/\/bridges$/);
  await expect(bridgeUI.listPanel).toBeVisible();
  await expect(bridgeUI.addListFilter).toBeVisible();

  await bridgeUI.createBridgeButton.click();
  await expect(bridgeUI.createDialog).toBeVisible();
  await bridgeUI.providerCard(createdBridgeProviderKey).click();
  await expect(bridgeUI.createWizardNext).toBeEnabled();
  await bridgeUI.createWizardNext.click();
  await bridgeUI.createDisplayNameInput.fill(createdBridgeName);
  await bridgeUI.createProviderConfigInput.fill("{invalid");
  await expect(bridgeUI.createWizardNext).toBeDisabled();
  await bridgeUI.createProviderConfigInput.fill(
    JSON.stringify(browserBridgeOperatorFlowScenario.bridge.initialProviderConfig, null, 2)
  );
  await expect(bridgeUI.createWizardNext).toBeEnabled();
  await bridgeUI.createWizardNext.click();
  await expect(bridgeUI.submitBridgeCreate).toBeEnabled();

  const createResponsePromise = appPage.waitForResponse(response => {
    return response.request().method() === "POST" && response.url().endsWith("/api/bridges");
  });
  await bridgeUI.submitBridgeCreate.click();
  const createResponse = await createResponsePromise;
  expect(createResponse.ok()).toBeTruthy();
  const createdPayload = (await createResponse.json()) as { bridge: { id: string } };
  await expect(bridgeUI.createDialog).toBeHidden();
  await expect(appPage).toHaveURL(
    new RegExp(`/bridges/${encodeURIComponent(createdPayload.bridge.id)}$`)
  );
  await expect(windowTitle(bridgesWin)).toContainText(createdBridgeName);
  await browserArtifacts.captureScreenshot("bridge-create-dialog-saved", appPage);

  await bridgeUI.backToList.click();
  await expect(appPage).toHaveURL(/\/bridges$/);
  await expect(bridgeUI.listPanel).toBeVisible();
  await expect(bridgeUI.item(createdPayload.bridge.id)).toBeVisible();
  await expect(bridgeUI.item(createdPayload.bridge.id)).toContainText(createdBridgeName);
  await expect(bridgeUI.item(seeded.bridge.id)).toBeVisible();
  await expect(bridgeUI.item(seeded.bridge.id)).toContainText(
    browserBridgeOperatorFlowScenario.bridge.initialName
  );
  await expect(bridgeUI.item(seeded.bridge.id)).toContainText("disabled");
  await expect(bridgeUI.item(seeded.bridge.id)).toContainText("0 routes");
  await bridgeUI.item(seeded.bridge.id).click();
  await expect(appPage).toHaveURL(new RegExp(`/bridges/${encodeURIComponent(seeded.bridge.id)}$`));
  await expect(windowTitle(bridgesWin)).toContainText(
    browserBridgeOperatorFlowScenario.bridge.initialName
  );
  await expect(bridgeStatus).toContainText("disabled");
  await expect(bridgeUI.detailPanel).toContainText(/Last delivery\s*Never/);
  await browserArtifacts.captureScreenshot("bridge-operator-seeded", appPage);

  await bridgeUI.detailOverflow.click();
  await appPage.getByTestId("edit-bridge-btn").click();
  await expect(bridgeUI.editDialog).toBeVisible();

  await bridgeUI.editDisplayNameInput.fill(browserBridgeOperatorFlowScenario.bridge.editedName);
  await bridgeUI.editProviderConfigInput.fill(
    JSON.stringify(browserBridgeOperatorFlowScenario.bridge.editedProviderConfig, null, 2)
  );
  await expect(bridgeUI.submitBridgeEdit).toBeEnabled();
  await bridgeUI.submitBridgeEdit.click();

  await expect(bridgeUI.editDialog).toBeHidden();
  await expect(windowTitle(bridgesWin)).toContainText(
    browserBridgeOperatorFlowScenario.bridge.editedName
  );
  await expect(bridgeUI.detailPanel).toContainText(
    browserBridgeOperatorFlowScenario.bridge.editedProviderConfig.webhook_url
  );
  await expect(bridgeUI.enableBridgeButton).toBeVisible();
  await browserArtifacts.captureScreenshot("bridge-operator-configured", appPage);

  await bridgeUI.enableBridgeButton.click();

  await expect
    .poll(
      async () => {
        const payload = await runtime.requestJSON<{
          health: { status?: string };
        }>(`/api/bridges/${encodeURIComponent(seeded.bridge.id)}`);
        return payload.health.status;
      },
      {
        timeout: 60_000,
      }
    )
    .toBe("ready");

  await expect
    .poll(async () => (await bridgeStatus.textContent()) ?? "", {
      timeout: 45_000,
    })
    .toContain("ready");
  await expect(bridgeUI.activeRoutesMetric).toContainText("0");
  await browserArtifacts.captureScreenshot("bridge-operator-enabled", appPage);

  // D2: the delivery test lives in the editor's Advanced tier, not a dialog.
  await bridgeUI.openTestDeliveryButton.click();
  await expect(bridgeUI.deliveryTestPanel).toBeVisible();

  await bridgeUI.testDeliveryMessage.fill(browserBridgeOperatorFlowScenario.testDelivery.message);
  await bridgeUI.testDeliveryModeSelect.selectOption(
    browserBridgeOperatorFlowScenario.testDelivery.mode
  );
  await bridgeUI.testDeliveryPeerInput.fill(browserBridgeOperatorFlowScenario.testDelivery.peerId);
  await bridgeUI.testDeliveryThreadInput.fill(
    browserBridgeOperatorFlowScenario.testDelivery.threadId
  );
  await expect(bridgeUI.submitTestDelivery).toBeEnabled();
  await bridgeUI.submitTestDelivery.click();

  await expect(bridgeUI.testDeliveryResult).toBeVisible();
  await expect(bridgeUI.testDeliveryResult).toContainText(
    browserBridgeOperatorFlowScenario.testDelivery.mode
  );
  await expect(bridgeUI.testDeliveryResult).toContainText(
    `peer:${browserBridgeOperatorFlowScenario.testDelivery.peerId}`
  );
  await expect(bridgeUI.testDeliveryResult).toContainText(
    `thread:${browserBridgeOperatorFlowScenario.testDelivery.threadId}`
  );
  await expect(bridgeUI.testDeliveryResult).toContainText(
    browserBridgeOperatorFlowScenario.testDelivery.message
  );
  await browserArtifacts.captureScreenshot("bridge-test-delivery-result", appPage);

  await bridgeUI.editDialog.getByRole("button", { name: "Cancel" }).click();
  await expect(bridgeUI.deliveryTestPanel).toBeHidden();

  const ingress = await triggerBrowserBridgeIngress(runtime, seeded);
  expect(ingress.transcript).toContain(browserBridgeOperatorFlowScenario.ingress.assistant);

  await expect(bridgeUI.route(ingress.sessionId)).toBeVisible();
  await expect(bridgeUI.activeRoutesMetric).toContainText("1");
  await expect
    .poll(async () =>
      /Last delivery\s*Never/.test((await bridgeUI.detailPanel.textContent()) ?? "")
    )
    .toBe(false);
  await expect.poll(async () => (await bridgeStatus.textContent()) ?? "").toContain("ready");
  const routeSnapshots = await collectBridgeRouteSnapshots(runtime, seeded.bridge.id);
  expect(routeSnapshots.http.routes.some(route => route.session_id === ingress.sessionId)).toBe(
    true
  );
  expect(routeSnapshots.uds.routes.some(route => route.session_id === ingress.sessionId)).toBe(
    true
  );
  expect(routeSnapshots.cliRoutes.some(route => route.session_id === ingress.sessionId)).toBe(true);
  assertNoSensitiveText("seeded bridge route snapshots", JSON.stringify(routeSnapshots));
  const routeState = await captureRouteState(appPage);
  expect(routeState.bridge_view_visible).toBe(true);
  expect(routeState.bridge_route_count).toBeGreaterThanOrEqual(1);
  expect(routeState.bridge_detail_visible).toBe(true);
  await browserArtifacts.captureScreenshot("bridge-health-stream-updated", appPage);
});

test("operator creates a bridge, rotates secrets, diagnoses auth failure, and recovers after restart", async ({
  appPage,
  browserArtifacts,
  runtime,
}) => {
  const shellUI = bridgeOperatorSelectors(appPage);
  const seeded = await seedBrowserBridgeOperatorFlow(runtime, {
    displayName: "Telegram Seed Provider Anchor",
  });
  const providerKey = `${seeded.provider.extension_name}::${seeded.provider.platform}`;
  const createdName = "Telegram Browser Lifecycle";

  await useGlobalWorkspaceIfPrompted(shellUI);
  const bridgesWin = await openAppWindow(appPage, "Bridges", "bridges");
  const bridgeUI = bridgeOperatorSelectors(bridgesWin);
  const bridgeStatus = bridgesWin.locator('[data-slot="topbar-status"]');
  await expect(appPage).toHaveURL(/\/bridges$/);
  await expect(bridgeUI.listPanel).toBeVisible();

  await bridgeUI.createBridgeButton.click();
  await expect(bridgeUI.createDialog).toBeVisible();
  await expect(bridgeUI.providerCard(providerKey)).toBeVisible();
  await bridgeUI.providerCard(providerKey).click();
  await expect(bridgeUI.createWizardNext).toBeEnabled();
  await bridgeUI.createWizardNext.click();

  await bridgeUI.createProviderConfigInput.fill("{invalid-json");
  await expect(bridgeUI.createProviderConfigError).toBeVisible();
  await expect(bridgeUI.createWizardNext).toBeDisabled();
  await browserArtifacts.captureScreenshot("bridge-create-invalid-provider-config", appPage);

  await bridgeUI.createProviderConfigInput.fill(
    JSON.stringify(
      {
        mode: "bot",
        webhook_url: "https://example.test/browser-bridge-lifecycle",
      },
      null,
      2
    )
  );
  await bridgeUI.createDisplayNameInput.fill(createdName);
  await bridgeUI.createScopeSelect.selectOption("workspace");
  await expect(bridgeUI.createWizardNext).toBeEnabled();
  await bridgeUI.createWizardNext.click();
  await bridgeUI.createDeliveryModeSelect.selectOption("direct-send");
  await bridgeUI.createDeliveryPeerInput.fill("telegram-peer-lifecycle");
  await bridgeUI.createDeliveryThreadInput.fill("777");
  await expect(bridgeUI.submitBridgeCreate).toBeEnabled();
  await bridgeUI.submitBridgeCreate.click();

  await expect(bridgeUI.createDialog).toBeHidden();
  const createdBridge = await waitForBridgeByName(runtime, createdName);
  await expect(appPage).toHaveURL(new RegExp(`/bridges/${encodeURIComponent(createdBridge.id)}$`));
  await expect(windowTitle(bridgesWin)).toContainText(createdName);
  await expect(bridgeUI.detailPanel).toContainText("UNBOUND");
  await browserArtifacts.captureScreenshot("bridge-created-unbound", appPage);

  const initialSnapshots = await waitForBridgeSnapshots(runtime, createdBridge.id, "disabled");
  expect(initialSnapshots.http.bridge.id).toBe(createdBridge.id);
  expect(initialSnapshots.uds.bridge.id).toBe(createdBridge.id);
  expect(initialSnapshots.cli.id).toBe(createdBridge.id);
  expect(initialSnapshots.http.bridge.enabled).toBe(false);
  expect(initialSnapshots.uds.bridge.enabled).toBe(false);
  expect(initialSnapshots.cli.enabled).toBe(false);
  expect(initialSnapshots.http.health.status).toBe("disabled");
  expect(initialSnapshots.uds.health.status).toBe("disabled");
  assertNoSensitiveText("initial bridge snapshots", JSON.stringify(initialSnapshots));

  await bridgeUI
    .secretEnvInput(browserBridgeOperatorFlowScenario.secretBinding.name)
    .fill(bridgeRawSecret);
  await expect(
    bridgeUI.saveSecret(browserBridgeOperatorFlowScenario.secretBinding.name)
  ).toBeEnabled();
  await bridgeUI.saveSecret(browserBridgeOperatorFlowScenario.secretBinding.name).click();
  await expect(
    bridgeUI.secretBinding(browserBridgeOperatorFlowScenario.secretBinding.name)
  ).toContainText("BOUND");
  await expect(
    bridgeUI.secretEnvInput(browserBridgeOperatorFlowScenario.secretBinding.name)
  ).toHaveValue("");
  await expect(bridgeUI.restartRequired).toBeVisible();

  const boundSnapshots = await collectBridgeSnapshots(runtime, createdBridge.id);
  expect(
    boundSnapshots.httpBindings.bindings.some(
      binding =>
        binding.binding_name === browserBridgeOperatorFlowScenario.secretBinding.name &&
        binding.secret_ref ===
          `vault:bridges/${createdBridge.id}/${browserBridgeOperatorFlowScenario.secretBinding.name}`
    )
  ).toBe(true);
  expect(
    boundSnapshots.cliBindings.bindings.some(
      binding => binding.binding_name === browserBridgeOperatorFlowScenario.secretBinding.name
    )
  ).toBe(true);
  assertNoSensitiveText("bound secret snapshots", JSON.stringify(boundSnapshots));

  await bridgeUI.enableBridgeButton.click();
  await waitForBridgeStatus(runtime, createdBridge.id, "ready");
  await expect
    .poll(async () => (await bridgeStatus.textContent()) ?? "", { timeout: 45_000 })
    .toContain("ready");

  await bridgeUI.detailOverflow.click();
  await appPage.getByTestId("disable-bridge-btn").click();
  await waitForBridgeStatus(runtime, createdBridge.id, "disabled");
  await expect.poll(async () => (await bridgeStatus.textContent()) ?? "").toContain("disabled");

  await bridgeUI.enableBridgeButton.click();
  await waitForBridgeStatus(runtime, createdBridge.id, "ready");
  await bridgeUI.detailOverflow.click();
  await appPage.getByTestId("restart-bridge-btn").click();
  await waitForBridgeStatus(runtime, createdBridge.id, "ready");
  await expect
    .poll(async () => (await bridgeStatus.textContent()) ?? "", { timeout: 45_000 })
    .toContain("ready");
  await browserArtifacts.captureScreenshot("bridge-ready-after-restart", appPage);

  const delivery = await testBridgeDelivery(runtime, createdBridge.id);
  expect(delivery.status).toBe("resolved");
  expect(delivery.delivery_target.bridge_instance_id).toBe(createdBridge.id);
  await browserArtifacts.captureScreenshot("bridge-created-outbound-resolved", appPage);

  await bridgeUI.deleteSecret(browserBridgeOperatorFlowScenario.secretBinding.name).click();
  const confirmDeleteSecret = appPage.getByTestId(
    `confirm-delete-bridge-secret-${browserBridgeOperatorFlowScenario.secretBinding.name}`
  );
  await confirmDeleteSecret.click();
  await expect(
    bridgeUI.secretBinding(browserBridgeOperatorFlowScenario.secretBinding.name)
  ).toContainText("UNBOUND");
  await appPage.keyboard.press("Escape");
  await expect(confirmDeleteSecret).toBeHidden();
  await expect(bridgeUI.restartRequired).toBeVisible();

  await bridgeUI.detailOverflow.click();
  await appPage.getByTestId("restart-bridge-btn").click();
  const authRequired = await waitForBridgeStatus(runtime, createdBridge.id, "auth_required");
  expect(authRequired.health.status).toBe("auth_required");
  await expect
    .poll(async () => (await bridgeStatus.textContent()) ?? "")
    .toContain("auth required");
  await browserArtifacts.captureScreenshot("bridge-auth-required-after-secret-delete", appPage);

  await bridgeUI
    .secretEnvInput(browserBridgeOperatorFlowScenario.secretBinding.name)
    .fill(bridgeRawSecret);
  await bridgeUI.saveSecret(browserBridgeOperatorFlowScenario.secretBinding.name).click();
  await expect(
    bridgeUI.secretBinding(browserBridgeOperatorFlowScenario.secretBinding.name)
  ).toContainText("BOUND");
  await bridgeUI.detailOverflow.click();
  await appPage.getByTestId("restart-bridge-btn").click();
  await waitForBridgeStatus(runtime, createdBridge.id, "ready");
  await expect
    .poll(async () => (await bridgeStatus.textContent()) ?? "", { timeout: 45_000 })
    .toContain("ready");

  await assertBridgeDetailResponsive(appPage, bridgeUI, createdBridge.id);
  const routeState = await captureRouteState(appPage);
  expect(routeState.bridge_view_visible).toBe(true);
  expect(routeState.bridge_selected_item).toBe(createdName);
  expect(routeState.bridge_secret_binding_count).toBeGreaterThanOrEqual(1);
  expect(routeState.bridge_route_count).toBeGreaterThanOrEqual(0);
  expect(routeState.bridge_detail_visible).toBe(true);

  await assertBridgeDeleteSurfaceAbsent(runtime);
  await assertNoBridgeSensitiveLeaks(appPage, runtime, "final bridge browser/runtime evidence");
});

interface BridgeListResponse {
  bridges: BridgeSummary[];
}

interface BridgeBindingsResponse {
  bindings: BridgeSecretBinding[];
}

interface BridgeRoutesResponse {
  routes: BridgeRoute[];
}

interface BridgeCLIRecord {
  id: string;
  display_name: string;
  enabled: boolean;
  platform: string;
  status: string;
}

interface BridgeCLIBindingsResponse {
  bindings: BridgeSecretBinding[];
}

interface BridgeCLIRoutesBundle {
  bridge_routes: BridgeRoute[];
}

type BridgeCLIRoutesResponse = BridgeRoute[] | BridgeCLIRoutesBundle;

async function waitForBridgeByName(
  runtime: BrowserRuntime,
  displayName: string
): Promise<BridgeSummary> {
  const deadline = Date.now() + 45_000;
  let lastError: Error | null = null;
  while (Date.now() < deadline) {
    try {
      const payload = await runtime.requestJSON<BridgeListResponse>("/api/bridges");
      lastError = null;
      const bridge = payload.bridges.find(candidate => candidate.display_name === displayName);
      if (bridge) {
        return bridge;
      }
    } catch (error) {
      lastError = error instanceof Error ? error : new Error(String(error));
    }
    await delay(250);
  }
  throw new Error(
    `bridge ${displayName} was not present after wait${
      lastError ? ` (last error: ${lastError.message})` : ""
    }`
  );
}

async function waitForBridgeStatus(
  runtime: BrowserRuntime,
  bridgeId: string,
  status: BridgeHealth["status"]
): Promise<BridgeDetailResponse> {
  const deadline = Date.now() + 60_000;
  let lastError: Error | null = null;
  let lastHealth: BridgeDetailResponse["health"] | undefined;
  let lastStatus: string | undefined;
  while (Date.now() < deadline) {
    try {
      const payload = await runtime.requestJSON<BridgeDetailResponse>(
        `/api/bridges/${encodeURIComponent(bridgeId)}`
      );
      lastError = null;
      lastHealth = payload.health;
      lastStatus = payload.health.status;
      if (lastStatus === status) {
        return payload;
      }
    } catch (error) {
      lastError = error instanceof Error ? error : new Error(String(error));
    }
    await delay(250);
  }
  throw new Error(
    `bridge ${bridgeId} status is ${lastStatus ?? "unknown"}; expected ${status}${
      lastError ? ` (last error: ${lastError.message})` : ""
    }${lastHealth ? ` (last health: ${JSON.stringify(lastHealth)})` : ""}`
  );
}

async function collectBridgeSnapshots(runtime: BrowserRuntime, bridgeId: string) {
  const http = await runtime.requestJSON<BridgeDetailResponse>(
    `/api/bridges/${encodeURIComponent(bridgeId)}`
  );
  const uds = await requestOperatorJSONOrThrow<BridgeDetailResponse>(
    runtime,
    `/api/bridges/${encodeURIComponent(bridgeId)}`
  );
  const httpBindings = await runtime.requestJSON<BridgeBindingsResponse>(
    `/api/bridges/${encodeURIComponent(bridgeId)}/secret-bindings`
  );
  const udsBindings = await requestOperatorJSONOrThrow<BridgeBindingsResponse>(
    runtime,
    `/api/bridges/${encodeURIComponent(bridgeId)}/secret-bindings`
  );
  const cli = await bridgeCLI<BridgeCLIRecord>(runtime, ["bridge", "get", bridgeId]);
  const cliBindings = await bridgeCLI<BridgeCLIBindingsResponse>(runtime, [
    "bridge",
    "secret-bindings",
    "list",
    bridgeId,
  ]);

  return { cli, cliBindings, http, httpBindings, uds, udsBindings };
}

async function waitForBridgeSnapshots(
  runtime: BrowserRuntime,
  bridgeId: string,
  status: BridgeHealth["status"]
) {
  const deadline = Date.now() + 60_000;
  let lastError: Error | null = null;
  let lastStatus = "unknown";
  while (Date.now() < deadline) {
    try {
      const snapshots = await collectBridgeSnapshots(runtime, bridgeId);
      lastError = null;
      lastStatus = `http=${snapshots.http.health.status} uds=${snapshots.uds.health.status}`;
      if (snapshots.http.health.status === status && snapshots.uds.health.status === status) {
        return snapshots;
      }
    } catch (error) {
      lastError = error instanceof Error ? error : new Error(String(error));
    }
    await delay(250);
  }

  throw new Error(
    `bridge ${bridgeId} snapshots status ${lastStatus}; expected ${status}${
      lastError ? ` (last error: ${lastError.message})` : ""
    }`
  );
}

async function collectBridgeRouteSnapshots(runtime: BrowserRuntime, bridgeId: string) {
  const http = await runtime.requestJSON<BridgeRoutesResponse>(
    `/api/bridges/${encodeURIComponent(bridgeId)}/routes`
  );
  const uds = await requestOperatorJSONOrThrow<BridgeRoutesResponse>(
    runtime,
    `/api/bridges/${encodeURIComponent(bridgeId)}/routes`
  );
  const cli = await bridgeCLI<BridgeCLIRoutesResponse>(runtime, ["bridge", "routes", bridgeId]);
  const cliRoutes = Array.isArray(cli) ? cli : cli.bridge_routes;
  return { cli, cliRoutes, http, uds };
}

async function testBridgeDelivery(
  runtime: BrowserRuntime,
  bridgeId: string
): Promise<TestBridgeDeliveryResponse> {
  return await runtime.requestJSON<TestBridgeDeliveryResponse>(
    `/api/bridges/${encodeURIComponent(bridgeId)}/test-delivery`,
    {
      method: "POST",
      body: JSON.stringify({
        message: "Lifecycle delivery dry-run",
        target: {
          mode: "direct-send",
          peer_id: "telegram-peer-lifecycle",
          thread_id: "777",
        },
      }),
    }
  );
}

async function bridgeCLI<T>(runtime: BrowserRuntime, args: string[]): Promise<T> {
  if (!runtime.paths) {
    throw new Error("bridge CLI parity checks require launch-mode runtime paths");
  }
  const { stdout } = await execFileAsync(runtime.paths.cliShim, [...args, "-o", "json"], {
    env: cliEnv(runtime.paths),
    timeout: 30_000,
  });
  assertNoSensitiveText(`bridge CLI ${args.join(" ")}`, stdout);
  return JSON.parse(stdout) as T;
}

async function requestOperatorJSONOrThrow<T>(
  runtime: BrowserRuntime,
  pathname: string,
  init?: RequestInit
): Promise<T> {
  if (!runtime.requestOperatorJSON) {
    throw new Error("bridge UDS parity checks require requestOperatorJSON");
  }
  return await runtime.requestOperatorJSON<T>(pathname, init);
}

async function assertBridgeDetailResponsive(
  appPage: Page,
  bridgeUI: ReturnType<typeof bridgeOperatorSelectors>,
  bridgeId: string
): Promise<void> {
  for (const viewport of [
    { height: 900, name: "mobile", width: 375 },
    { height: 900, name: "tablet", width: 768 },
    { height: 900, name: "desktop", width: 1280 },
  ]) {
    await appPage.setViewportSize({ height: viewport.height, width: viewport.width });
    await expect(bridgeUI.detailPanel).toBeVisible();
    await expect(appPage).toHaveURL(new RegExp(`/bridges/${encodeURIComponent(bridgeId)}$`));
    await expect(bridgeUI.detailOverflow).toBeVisible();
    await expect(bridgeUI.openTestDeliveryButton).toBeVisible();
  }
}

async function assertBridgeDeleteSurfaceAbsent(runtime: BrowserRuntime): Promise<void> {
  if (!runtime.paths) {
    throw new Error("bridge delete surface check requires launch-mode runtime paths");
  }
  const { stdout } = await execFileAsync(runtime.paths.cliShim, ["bridge", "--help"], {
    env: cliEnv(runtime.paths),
    timeout: 30_000,
  });
  expect(stdout).toContain("create");
  expect(stdout).toContain("secret-bindings");
  expect(stdout).not.toMatch(/^\s+delete\s/m);
}

async function assertNoBridgeSensitiveLeaks(
  page: Page,
  runtime: BrowserRuntime,
  label: string
): Promise<void> {
  assertNoSensitiveText(`${label}: page body`, await page.locator("body").textContent());
  if (runtime.paths?.daemonLog) {
    assertNoSensitiveText(`${label}: daemon log`, await readFile(runtime.paths.daemonLog, "utf8"));
  }
}

function assertNoSensitiveText(label: string, text: string | null | undefined): void {
  const content = text ?? "";
  for (const value of bridgeSensitiveValues) {
    expect(content, `${label} leaked ${value}`).not.toContain(value);
  }
}

function cliEnv(paths: { cliShim: string; homeDir: string }): NodeJS.ProcessEnv {
  return {
    ...process.env,
    AGH_HOME: paths.homeDir,
    AGH_E2E_CLI_BIN: paths.cliShim,
    HOME: paths.homeDir,
    PATH: [path.dirname(paths.cliShim), process.env.PATH ?? ""]
      .filter(Boolean)
      .join(path.delimiter),
  };
}

async function delay(ms: number): Promise<void> {
  await new Promise(resolve => setTimeout(resolve, ms));
}
