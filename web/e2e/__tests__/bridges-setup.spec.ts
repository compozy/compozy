// Suite: Bridges setup mocked-daemon browser contract
// Invariant: browser-visible setup actions call only their matching bridge HTTP contracts and
// project the returned daemon facts without pre-create side effects or cross-action leakage.
// Boundary IN: daemon-served React app, router, hooks, adapters, clipboard, and browser rendering.
// Boundary OUT: bridge daemon/provider implementations (HTTP-intercepted here) and visual styling.

import type { Page, Route } from "@playwright/test";
import type {
  BridgeDetailResponse,
  BridgeHealth,
  BridgeProvider,
  BridgeSecretBinding,
  BridgeSummary,
  BridgeTargetsResponse,
  BridgeVerifyResponse,
  BridgesListResponse,
  CreateBridgeRequest,
  SendBridgeTestRequest,
  SendBridgeTestResponse,
  SlackBridgeManifestResponse,
  TestBridgeDeliveryRequest,
  TestBridgeDeliveryResponse,
} from "@/systems/bridges";
import { slackBridgeManifestFixture } from "@/systems/bridges/mocks";

import { bridgeOperatorSelectors } from "../fixtures/selectors";
import { expect, test } from "../fixtures/test";
import { ensureGlobalWorkspace, useGlobalWorkspaceIfPrompted } from "../fixtures/workspace";

const NOW = "2026-07-12T12:00:00Z";
const CREATED_BRIDGE_ID = "brg_browser_slack_setup";
const CREATED_WEBHOOK_PUBLIC_URL = "https://hooks.example.com/browser-slack-setup";
const PROVIDER_KEY = "ext-slack::slack";

const slackProvider: BridgeProvider = {
  config_schema: { schema: "agh.bridge.slack", version: "1" },
  description: "Slack bridge adapter",
  display_name: "Slack",
  enabled: true,
  extension_name: "ext-slack",
  health: "healthy",
  platform: "slack",
  secret_slots: [
    { description: "Slack bot OAuth token", name: "bot_token", required: true },
    {
      description: "Slack request signing secret",
      name: "signing_secret",
      required: true,
    },
  ],
  state: "active",
};

interface BridgeRequestCounts {
  create: number;
  dryRun: number;
  manifest: number;
  sendTest: number;
  verify: number;
}

interface BridgeDaemonScenario {
  bindings: BridgeSecretBinding[];
  bridge: BridgeSummary | null;
  counts: BridgeRequestCounts;
  createBodies: CreateBridgeRequest[];
  createdBridgeID: string;
  dryRunBodies: TestBridgeDeliveryRequest[];
  dryRunResponse: TestBridgeDeliveryResponse | null;
  events: string[];
  health: BridgeHealth | null;
  manifestInstances: string[];
  manifestResponse: SlackBridgeManifestResponse;
  provider: BridgeProvider;
  sendTestBodies: SendBridgeTestRequest[];
  sendTestResponse: SendBridgeTestResponse | null;
  unexpectedRequests: string[];
  verifyResponse: BridgeVerifyResponse | null;
}

interface BridgeDaemonScenarioOptions {
  bindings?: BridgeSecretBinding[];
  bridge?: BridgeSummary | null;
  dryRunResponse?: TestBridgeDeliveryResponse | null;
  health?: BridgeHealth | null;
  sendTestResponse?: SendBridgeTestResponse | null;
  verifyResponse?: BridgeVerifyResponse | null;
}

test.setTimeout(120_000);
test.use({ viewport: { height: 900, width: 1440 } });

test("Should wait for Slack create 201 before manifest, copy daemon JSON, and open setup", async ({
  context,
  page,
  runtime,
}) => {
  const daemon = createBridgeDaemonScenario();
  await installBridgeDaemonRoutes(page, daemon);
  await context.grantPermissions(["clipboard-read", "clipboard-write"], {
    origin: runtime.baseURL,
  });
  await openBridgesPage(page, runtime.url("/bridges"), runtime);
  const ui = bridgeOperatorSelectors(page);

  await expect(ui.createBridgeButton).toBeVisible();
  await ui.createBridgeButton.click();
  await expect(ui.createDialog).toBeVisible();
  await ui.createDialog.getByTestId(`bridge-provider-card-${PROVIDER_KEY}`).click();
  await ui.createWizardNext.click();

  await expect(ui.createDisplayNameInput).toBeVisible();
  await ui.createDisplayNameInput.fill("Browser Slack Setup");
  await ui.createScopeSelect.selectOption("global");
  await ui.createProviderConfigInput.fill(
    JSON.stringify({
      webhook: { public_url: CREATED_WEBHOOK_PUBLIC_URL },
    })
  );
  await ui.createWizardNext.click();
  await expect(ui.submitBridgeCreate).toBeEnabled();

  expect(daemon.counts.create).toBe(0);
  expect(daemon.counts.manifest).toBe(0);
  await expect(ui.manifestHandoff).toHaveCount(0);

  await ui.submitBridgeCreate.click();
  await expect(ui.manifestHandoff).toBeVisible();
  await expect(ui.manifestJson).toContainText("Launch room dispatch");
  await expect.poll(() => daemon.counts.manifest).toBe(1);

  expect(daemon.counts.create).toBe(1);
  expect(daemon.events).toEqual(["create:request", "create:201", "manifest"]);
  expect(daemon.manifestInstances).toEqual([CREATED_BRIDGE_ID]);
  expect(daemon.createBodies).toHaveLength(1);
  expect(daemon.createBodies[0]).toMatchObject({
    enabled: false,
    extension_name: "ext-slack",
    platform: "slack",
  });

  await ui.manifestHandoff.getByRole("button", { name: "Copy Slack app manifest" }).click();
  await expect
    .poll(() => page.evaluate(() => navigator.clipboard.readText()))
    .toBe(JSON.stringify(daemon.manifestResponse.manifest, null, 2));
  await expect(
    ui.manifestHandoff.getByRole("link", { name: "Open Slack app dashboard" })
  ).toHaveAttribute("href", "https://api.slack.com/apps");

  await ui.manifestOpenBridge.click();
  await expect(page).toHaveURL(new RegExp(`/bridges/${CREATED_BRIDGE_ID}$`));
  await expect(ui.setupChecklist).toBeVisible();
  await expect(ui.setupItem("provider")).toContainText("COMPLETE");
  await expect(ui.setupItem("secrets")).toContainText("ACTION NEEDED");
  await expect(ui.setupItem("webhook")).toContainText("COMPLETE");
  await expect(ui.setupItem("verified")).toContainText("ACTION NEEDED");
  await expect(ui.setupItem("enabled")).toContainText("ACTION NEEDED");
  await expect(ui.setupItem("healthy")).toContainText("SKIPPED");
  expect(daemon.unexpectedRequests).toEqual([]);
});

test("Should render verify pass and failure remediation on the exact Slack secret cards", async ({
  page,
  runtime,
}) => {
  const bridge = makeSlackBridge({ id: "brg_verify_browser" });
  const signingRemediation = "Replace the signing secret from the Slack app dashboard.";
  const daemon = createBridgeDaemonScenario({
    bindings: makeSlackBindings(bridge.id),
    bridge,
    health: makeBridgeHealth(bridge, "disabled"),
    verifyResponse: {
      bridge_instance_id: bridge.id,
      checks: [
        { check: "provider.identity", remediation: "", status: "pass" },
        { check: "scope.chat:write", remediation: "", status: "pass" },
        {
          check: "webhook.signing_secret",
          remediation: signingRemediation,
          status: "fail",
        },
        { check: "webhook.configuration", remediation: "", status: "pass" },
        {
          check: "webhook.reachability",
          remediation: "Enable the bridge before checking reachability.",
          status: "skipped",
        },
      ],
    },
  });
  await installBridgeDaemonRoutes(page, daemon);
  await openBridgeDetail(page, runtime, bridge.id);
  const ui = bridgeOperatorSelectors(page);

  await expect(ui.setupChecklist).toBeVisible();
  await ui.verifyBridgeButton.click();
  await expect.poll(() => daemon.counts.verify).toBe(1);

  const identityCheck = ui.secretCheck("bot_token", "provider.identity");
  const scopeCheck = ui.secretCheck("bot_token", "scope.chat:write");
  const signingCheck = ui.secretCheck("signing_secret", "webhook.signing_secret");
  await expect(identityCheck).toContainText("PASS");
  await expect(scopeCheck).toContainText("PASS");
  await expect(signingCheck).toContainText("FAIL");
  await expect(signingCheck).toContainText(signingRemediation);
  await expect(ui.secretBinding("bot_token")).not.toContainText(signingRemediation);
  await expect(ui.setupItem("verified")).toContainText("FAILED");
  expect(daemon.unexpectedRequests).toEqual([]);
});

test("Should call distinct dry-run and send-test endpoints and render the delivery result", async ({
  page,
  runtime,
}) => {
  const bridge = makeSlackBridge({ enabled: true, id: "brg_delivery_browser" });
  const daemon = createBridgeDaemonScenario({
    bindings: makeSlackBindings(bridge.id),
    bridge,
    dryRunResponse: {
      delivery_target: {
        bridge_instance_id: bridge.id,
        mode: "direct-send",
        peer_id: "preview-peer",
      },
      message: "Resolved only; no provider message was sent.",
      status: "resolved",
    },
    health: makeBridgeHealth(bridge, "ready"),
    sendTestResponse: {
      bridge_instance_id: bridge.id,
      delivery_id: "delivery-browser-001",
      delivery_target: {
        bridge_instance_id: bridge.id,
        mode: "direct-send",
        peer_id: "real-peer",
      },
      remote_message_id: "slack-message-9001",
      status: "delivered",
    },
  });
  await installBridgeDaemonRoutes(page, daemon);
  await openBridgeDetail(page, runtime, bridge.id);
  const ui = bridgeOperatorSelectors(page);

  // D2: one entry point opens the editor in Advanced, where both verbs live.
  await expect(ui.openTestDeliveryButton).toBeVisible();
  await ui.openTestDeliveryButton.click();
  await expect(ui.deliveryTestPanel).toBeVisible();
  await expect(ui.editModeAdvanced).toHaveAttribute("aria-pressed", "true");
  await ui.testDeliveryMessage.fill("Dry-run browser preview");
  await ui.testDeliveryModeSelect.selectOption("direct-send");
  await ui.testDeliveryPeerInput.fill("preview-peer");
  await ui.submitTestDelivery.click();

  await expect(ui.testDeliveryResult).toContainText("preview-peer");
  await expect(ui.testDeliveryResult).toContainText("no provider message was sent");
  await expect.poll(() => daemon.counts.dryRun).toBe(1);
  expect(daemon.counts.sendTest).toBe(0);
  expect(daemon.dryRunBodies[0]).toMatchObject({
    message: "Dry-run browser preview",
    target: { bridge_instance_id: bridge.id, peer_id: "preview-peer" },
  });

  // The real send reuses the same panel and target draft — no second surface.
  await ui.testDeliveryMessage.fill("Real browser provider ping");
  await ui.testDeliveryModeSelect.selectOption("direct-send");
  await ui.testDeliveryPeerInput.fill("real-peer");
  await ui.submitSendTest.click();

  await expect(ui.sendTestResult).toContainText("delivered");
  await expect(ui.sendTestResult).toContainText("delivery-browser-001");
  await expect(ui.sendTestResult).toContainText("slack-message-9001");
  await expect(ui.sendTestResult).toContainText("real-peer");
  await expect.poll(() => daemon.counts.sendTest).toBe(1);
  expect(daemon.counts.dryRun).toBe(1);
  expect(daemon.sendTestBodies[0]).toMatchObject({
    message: "Real browser provider ping",
    target: { bridge_instance_id: bridge.id, peer_id: "real-peer" },
  });
  expect(daemon.unexpectedRequests).toEqual([]);
});

function createBridgeDaemonScenario(
  options: BridgeDaemonScenarioOptions = {}
): BridgeDaemonScenario {
  const bridge = options.bridge ?? null;
  return {
    bindings: options.bindings ?? [],
    bridge,
    counts: { create: 0, dryRun: 0, manifest: 0, sendTest: 0, verify: 0 },
    createBodies: [],
    createdBridgeID: CREATED_BRIDGE_ID,
    dryRunBodies: [],
    dryRunResponse: options.dryRunResponse ?? null,
    events: [],
    health: options.health ?? (bridge ? makeBridgeHealth(bridge) : null),
    manifestInstances: [],
    manifestResponse: slackBridgeManifestFixture,
    provider: slackProvider,
    sendTestBodies: [],
    sendTestResponse: options.sendTestResponse ?? null,
    unexpectedRequests: [],
    verifyResponse: options.verifyResponse ?? null,
  };
}

async function installBridgeDaemonRoutes(page: Page, daemon: BridgeDaemonScenario): Promise<void> {
  await page.route("**/api/bridges**", async route => {
    const request = route.request();
    const url = new URL(request.url());
    const method = request.method();
    const pathname = url.pathname;

    if (method === "GET" && pathname === "/api/bridges/health/stream") {
      await fulfillHealthStream(route, daemon);
      return;
    }
    if (method === "GET" && pathname === "/api/bridges/providers/slack/manifest") {
      daemon.counts.manifest += 1;
      daemon.events.push("manifest");
      const instance = url.searchParams.get("instance") ?? "";
      daemon.manifestInstances.push(instance);
      if (daemon.bridge === null || instance !== daemon.bridge.id) {
        await fulfillJSON(route, 404, { error: "Slack bridge not found" });
        return;
      }
      await fulfillJSON(route, 200, daemon.manifestResponse);
      return;
    }
    if (method === "GET" && pathname === "/api/bridges/providers") {
      await fulfillJSON(route, 200, { providers: [daemon.provider] });
      return;
    }
    if (pathname === "/api/bridges" && method === "GET") {
      await fulfillJSON(route, 200, bridgeListResponse(daemon));
      return;
    }
    if (pathname === "/api/bridges" && method === "POST") {
      daemon.counts.create += 1;
      daemon.events.push("create:request");
      const body = request.postDataJSON() as CreateBridgeRequest;
      daemon.createBodies.push(body);
      daemon.bridge = bridgeFromCreateRequest(
        body,
        daemon.createdBridgeID,
        CREATED_WEBHOOK_PUBLIC_URL
      );
      daemon.health = makeBridgeHealth(daemon.bridge, "disabled");
      await fulfillJSON(route, 201, { bridge: daemon.bridge, health: daemon.health });
      daemon.events.push("create:201");
      return;
    }

    const match = /^\/api\/bridges\/([^/]+)(?:\/(.+))?$/.exec(pathname);
    if (match === null || daemon.bridge === null) {
      await fulfillUnexpected(route, daemon, method, pathname);
      return;
    }
    const bridgeID = decodeURIComponent(match[1] ?? "");
    const action = match[2] ?? "";
    if (bridgeID !== daemon.bridge.id) {
      await fulfillJSON(route, 404, { error: "Bridge not found" });
      return;
    }

    if (method === "GET" && action === "") {
      const detail: BridgeDetailResponse = {
        bridge: daemon.bridge,
        health: daemon.health ?? makeBridgeHealth(daemon.bridge),
      };
      await fulfillJSON(route, 200, detail);
      return;
    }
    if (method === "GET" && action === "routes") {
      await fulfillJSON(route, 200, { routes: [] });
      return;
    }
    if (method === "GET" && action === "targets") {
      await fulfillJSON(route, 200, emptyTargetsResponse(bridgeID));
      return;
    }
    if (method === "GET" && action === "secret-bindings") {
      await fulfillJSON(route, 200, { bindings: daemon.bindings });
      return;
    }
    if (method === "POST" && action === "verify" && daemon.verifyResponse !== null) {
      daemon.counts.verify += 1;
      await fulfillJSON(route, 200, daemon.verifyResponse);
      return;
    }
    if (method === "POST" && action === "test-delivery" && daemon.dryRunResponse !== null) {
      daemon.counts.dryRun += 1;
      daemon.dryRunBodies.push(request.postDataJSON() as TestBridgeDeliveryRequest);
      await fulfillJSON(route, 200, daemon.dryRunResponse);
      return;
    }
    if (method === "POST" && action === "send-test" && daemon.sendTestResponse !== null) {
      daemon.counts.sendTest += 1;
      daemon.sendTestBodies.push(request.postDataJSON() as SendBridgeTestRequest);
      await fulfillJSON(route, 200, daemon.sendTestResponse);
      return;
    }
    await fulfillUnexpected(route, daemon, method, pathname);
  });
}

async function openBridgesPage(
  page: Page,
  targetURL: string,
  runtime: Parameters<typeof ensureGlobalWorkspace>[0]
): Promise<void> {
  await ensureGlobalWorkspace(runtime);
  await page.goto(targetURL, { waitUntil: "domcontentloaded" });
  await useGlobalWorkspaceIfPrompted(page);
  await expect(page.getByTestId("os-desktop")).toBeVisible();
}

async function openBridgeDetail(
  page: Page,
  runtime: Parameters<typeof ensureGlobalWorkspace>[0],
  bridgeID: string
): Promise<void> {
  await openBridgesPage(page, runtime.url("/bridges"), runtime);
  const ui = bridgeOperatorSelectors(page);
  await expect(ui.item(bridgeID)).toBeVisible();
  await ui.item(bridgeID).click();
  await expect(page).toHaveURL(new RegExp(`/bridges/${bridgeID}$`));
  await expect(ui.detailPanel).toBeVisible();
}

function bridgeListResponse(daemon: BridgeDaemonScenario): BridgesListResponse {
  const statuses = {
    auth_required: 0,
    degraded: 0,
    disabled: 0,
    error: 0,
    ready: 0,
    starting: 0,
  };
  if (daemon.bridge) {
    statuses[daemon.bridge.status] += 1;
  }
  return {
    bridge_health: daemon.bridge && daemon.health ? { [daemon.bridge.id]: daemon.health } : {},
    bridges: daemon.bridge ? [daemon.bridge] : [],
    facets: {
      platforms: daemon.bridge ? { [daemon.bridge.platform]: 1 } : {},
      statuses,
    },
    page: { has_more: false, limit: 50, total: daemon.bridge ? 1 : 0 },
  };
}

function bridgeFromCreateRequest(
  body: CreateBridgeRequest,
  id: string,
  webhookPublicURL: string
): BridgeSummary {
  return {
    ...body,
    created_at: NOW,
    id,
    notification_suppress: body.notification_suppress ?? false,
    source: "dynamic",
    status: body.enabled ? "starting" : "disabled",
    updated_at: NOW,
    webhook_public_url: webhookPublicURL,
  };
}

function makeSlackBridge({
  displayName = "Browser Slack Bridge",
  enabled = false,
  id,
}: {
  displayName?: string;
  enabled?: boolean;
  id: string;
}): BridgeSummary {
  return {
    created_at: NOW,
    delivery_defaults: { mode: "direct-send", peer_id: "default-peer" },
    display_name: displayName,
    enabled,
    extension_name: "ext-slack",
    id,
    notification_suppress: false,
    platform: "slack",
    provider_config: {
      webhook: { public_url: `https://hooks.example.com/${id}` },
    },
    routing_policy: { include_group: true, include_peer: true, include_thread: true },
    scope: "global",
    source: "dynamic",
    status: enabled ? "ready" : "disabled",
    updated_at: NOW,
    webhook_public_url: `https://hooks.example.com/${id}`,
  };
}

function makeBridgeHealth(
  bridge: BridgeSummary,
  status: BridgeHealth["status"] = bridge.enabled ? "ready" : "disabled"
): BridgeHealth {
  return {
    auth_failures_total: 0,
    bridge_instance_id: bridge.id,
    delivery_backlog: 0,
    delivery_dropped_total: 0,
    delivery_failures_total: 0,
    route_count: 0,
    status,
  };
}

function makeSlackBindings(bridgeID: string): BridgeSecretBinding[] {
  return ["bot_token", "signing_secret"].map(bindingName => ({
    binding_name: bindingName,
    bridge_instance_id: bridgeID,
    created_at: NOW,
    kind: "vault",
    secret_ref: `vault:bridges/${bridgeID}/${bindingName}`,
    updated_at: NOW,
  }));
}

function emptyTargetsResponse(bridgeID: string): BridgeTargetsResponse {
  return {
    bridge_id: bridgeID,
    cache_stale: false,
    generated_at: NOW,
    last_successful_refresh_at: NOW,
    targets: [],
    total: 0,
  };
}

async function fulfillHealthStream(route: Route, daemon: BridgeDaemonScenario): Promise<void> {
  const snapshot = {
    bridge_health: daemon.bridge && daemon.health ? { [daemon.bridge.id]: daemon.health } : {},
    generated_at: NOW,
  };
  await route.fulfill({
    body: `event: snapshot\ndata: ${JSON.stringify(snapshot)}\n\n`,
    headers: {
      "Cache-Control": "no-cache",
      "Content-Type": "text/event-stream",
    },
    status: 200,
  });
}

async function fulfillUnexpected(
  route: Route,
  daemon: BridgeDaemonScenario,
  method: string,
  pathname: string
): Promise<void> {
  const label = `${method} ${pathname}`;
  daemon.unexpectedRequests.push(label);
  await fulfillJSON(route, 500, { error: `Unexpected mocked bridge request: ${label}` });
}

async function fulfillJSON(route: Route, status: number, payload: unknown): Promise<void> {
  await route.fulfill({
    body: JSON.stringify(payload),
    contentType: "application/json",
    status,
  });
}
