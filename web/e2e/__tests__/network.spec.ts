import { execFile } from "node:child_process";
import { readFile } from "node:fs/promises";
import { fileURLToPath } from "node:url";
import path from "node:path";
import process from "node:process";
import { promisify } from "node:util";

import { networkOperatorSelectors } from "../fixtures/selectors";
import { ensureAppWindow, openAppWindow } from "../fixtures/os-navigation";
import {
  browserNetworkOperatorFlowScenario,
  seedBrowserNetworkOperatorFlow,
  type BrowserRuntime,
  type WorkspacePayload,
} from "../fixtures/runtime";
import { expect, test } from "../fixtures/test";
import { ensureGlobalWorkspace, useGlobalWorkspaceIfPrompted } from "../fixtures/workspace";

const execFileAsync = promisify(execFile);
const networkCollaborationFixture = path.resolve(
  path.dirname(fileURLToPath(import.meta.url)),
  "..",
  "..",
  "..",
  "internal",
  "testutil",
  "acpmock",
  "testdata",
  "network_collaboration_fixture.json"
);

const channelName = "browser-builders";
const initiatorAgentName = "mock-ops-coordinator";
const responderAgentName = "mock-patch-worker";
const createdChannelPurpose = "Coordinate browser e2e work";
const openWorkId = "browser_work_needs_input_17";
const openWorkRequestMessageId = "browser_msg_needs_input_request_01";
const openWorkMessageId = "browser_msg_needs_input_01";
const sensitivePattern =
  /agh_claim_|claim_token["':\s]|mcp[_-]?auth|telegram-bot-token|pkce|oauth|webhook_secret|provider[_-]?credentials?["'\s]*[:=]|proof["':\s]|signature["':\s]/i;

test.use({
  runtimeOptions: {
    networkEnabled: true,
    seed: {
      mockAgents: [
        {
          fixturePath: networkCollaborationFixture,
          fixtureAgent: "ops-coordinator",
          agentName: initiatorAgentName,
        },
        {
          fixturePath: networkCollaborationFixture,
          fixtureAgent: "patch-worker",
          agentName: responderAgentName,
        },
      ],
    },
  },
});

test.describe("network disabled state", () => {
  test.use({ runtimeOptions: { networkEnabled: false } });

  test("operator diagnoses disabled network and reaches matching settings state", async ({
    appPage,
    browserArtifacts,
    runtime,
  }) => {
    await ensureGlobalWorkspace(runtime);

    await appPage.goto(runtime.url("/network"), { waitUntil: "domcontentloaded" });
    await useGlobalWorkspaceIfPrompted(appPage);
    const networkWin = await ensureAppWindow(appPage, "Network", "network");
    const ui = networkOperatorSelectors(networkWin);
    await expect(ui.disabledState).toBeVisible();
    await expect(networkWin.getByTestId("network-empty")).toContainText("Network is disabled.");
    await expect(networkWin.getByTestId("network-empty")).toContainText(
      "Local work remains available"
    );
    await expect(networkWin.getByTestId("network-empty-open-settings")).toBeVisible();

    const status = await runtime.requestJSON<NetworkStatusEnvelope>("/api/network/status");
    expect(status.network.enabled).toBe(false);
    expect(status.network.status).toBe("disabled");

    await browserArtifacts.captureScreenshot("network-disabled", appPage);
    await networkWin.getByTestId("network-empty-open-settings").click();
    await expect.poll(() => new URL(appPage.url()).pathname).toBe("/settings/network");
    const settingsWin = appPage.getByTestId("os-window-app:settings");
    await expect(settingsWin.getByTestId("settings-page-network-hero")).toContainText("disabled");

    await runtime.artifactCollector.captureJSON("browser_api_snapshots", {
      http_status: status,
      route: "disabled-network-settings",
    });
    await browserArtifacts.persist(appPage);
    await assertNoNetworkSensitiveLeak(appPage, runtime, { status });
  });
});

test("operator verifies thread and direct network surfaces with final conversation artifacts", async ({
  appPage,
  browserArtifacts,
  runtime,
}) => {
  const workspace = await prepareNetworkRuntime(runtime, appPage);
  const networkWin = await openAppWindow(appPage, "Network", "network");
  const ui = networkOperatorSelectors(networkWin);
  await expect(ui.noChannelsState).toBeVisible();
  await expect(networkWin.getByTestId("network-empty")).toContainText(
    "Executions stay Local by default"
  );
  await expect(networkWin.getByTestId("network-empty")).toContainText("Choose Live explicitly");
  await expect(networkWin.getByTestId("network-empty")).toContainText(
    "Network availability never enrolls an execution"
  );
  await expect(networkWin.getByTestId("network-empty-open-settings")).toBeVisible();
  await createChannelFromUI(appPage, ui, {
    agents: [initiatorAgentName, responderAgentName],
    channel: channelName,
    purpose: createdChannelPurpose,
    workspaceId: workspace.id,
  });

  const operatorFlow = await seedBrowserNetworkOperatorFlow(runtime, {
    channel: channelName,
    initiatorAgentName,
    responderAgentName,
    workspaceId: workspace.id,
  });
  await seedOpenNetworkWork(runtime, operatorFlow);
  await appPage.goto(runtime.url(`/network/${workspace.id}/${channelName}/threads`), {
    waitUntil: "domcontentloaded",
  });

  await expect(networkWin).toBeVisible();
  await expect(networkWin.getByTestId("network-shell")).toBeVisible();
  await expect(ui.channelItem(channelName)).toBeVisible({ timeout: 15_000 });
  await ui.channelItem(channelName).getByTestId(`network-channel-link-${channelName}`).click();

  await expect(ui.channelHeader.getByTestId("network-channel-title")).toContainText(channelName);
  const channelMeta = ui.channelHeader.getByTestId("network-channel-meta");
  await expect(channelMeta).toContainText("2 agents");
  await expect(ui.channelHeader.getByTestId("network-channel-purpose")).toContainText(
    createdChannelPurpose
  );
  await expect(ui.threadTab).toHaveAttribute("aria-selected", "true");
  await expect(ui.threadsTab).toHaveAttribute("aria-label", `Threads in #${channelName}`);
  await expect(ui.threadList).toHaveAttribute("aria-label", `Threads in #${channelName}`);
  await ui.channelInspectorToggle.click();
  await expect(ui.inspector).toBeVisible();
  await expect(ui.inspectorMembersTab).toHaveAttribute("aria-pressed", "true");
  await expect(ui.inspectorPanelMembers).toBeVisible();
  await ui.inspectorWorkTab.click();
  await expect(ui.inspectorWorkTab).toHaveAttribute("aria-pressed", "true");
  await expect(ui.inspectorPanelWork).toBeVisible();
  await ui.channelInspectorToggle.click();
  await expect(ui.inspector).toHaveCount(0);
  await expect(ui.threadItem(operatorFlow.threadId)).toBeVisible();
  await expect(ui.threadList).toHaveAttribute("aria-label", `Threads in #${channelName}`);
  await expect(networkWin.getByTestId("network-composer-channel-thread")).toBeVisible();
  await expect(ui.threadItem(operatorFlow.threadId)).toContainText(
    browserNetworkOperatorFlowScenario.texts.say
  );

  await ui.threadItem(operatorFlow.threadId).click();
  await expect
    .poll(() => new URL(appPage.url()).pathname)
    .toContain(`/network/${workspace.id}/${channelName}/threads/${operatorFlow.threadId}`);
  await expect(ui.threadOverlay).toBeVisible();
  await expect(ui.threadList).toHaveAttribute("data-dim", "true");
  await expect(ui.channelMessage(operatorFlow.messageIds.say)).toContainText(
    browserNetworkOperatorFlowScenario.texts.say
  );
  await expect(ui.channelMessage(operatorFlow.messageIds.summary)).toContainText(
    browserNetworkOperatorFlowScenario.texts.summary
  );
  await expect(ui.channelMessage(operatorFlow.messageIds.direct)).toHaveCount(0);
  await browserArtifacts.captureScreenshot("network-thread-detail", appPage);

  const threadComposer = networkWin.getByTestId("network-composer-textarea-thread");
  await threadComposer.click();
  await expect(threadComposer).toBeFocused();

  await appPage.setViewportSize({ width: 375, height: 812 });
  await expect(ui.channelTabs).toBeVisible();
  await expect(networkWin.getByTestId("network-composer-thread")).toBeVisible();
  await browserArtifacts.captureScreenshot("network-mobile-composer", appPage);
  await networkWin.getByTestId("network-thread-overlay-close").click();
  await expect
    .poll(() => new URL(appPage.url()).pathname)
    .toBe("/network/" + workspace.id + "/" + channelName + "/threads");
  await appPage.setViewportSize({ width: 768, height: 900 });
  await expect(ui.threadList).toBeVisible();
  await browserArtifacts.captureScreenshot("network-tablet-thread-list", appPage);
  await appPage.setViewportSize({ width: 1280, height: 900 });

  await networkWin.getByTestId("network-tab-activity").click();
  await expect
    .poll(() => new URL(appPage.url()).pathname)
    .toBe(`/network/${workspace.id}/${channelName}/activity`);
  await expect(ui.activityFeed).toHaveAttribute("aria-label", `Activity in #${channelName}`);
  await expect(
    networkWin.getByTestId(`network-activity-entry-thread:${operatorFlow.threadId}`)
  ).toBeVisible();
  await browserArtifacts.captureScreenshot("network-activity", appPage);

  await ui.directTab.click();
  await expect
    .poll(() => new URL(appPage.url()).pathname)
    .toBe(`/network/${workspace.id}/${channelName}/directs`);
  await expect(ui.directTab).toHaveAttribute("aria-selected", "true");
  await expect(ui.directsTab).toHaveAttribute("aria-label", `Direct rooms in #${channelName}`);
  await expect(ui.directList).toHaveAttribute("aria-label", `Direct rooms in #${channelName}`);
  await expect(ui.directItem(operatorFlow.directId)).toBeVisible();

  await ui.directItem(operatorFlow.directId).click();
  await expect
    .poll(() => new URL(appPage.url()).pathname)
    .toBe(`/network/${workspace.id}/${channelName}/directs/${operatorFlow.directId}`);
  await expect(ui.directRoom).toBeVisible();
  await expect(ui.directRoom).toHaveAttribute("aria-label", /Direct room with @/);
  await expect(ui.channelMessage(operatorFlow.messageIds.direct)).toContainText(
    browserNetworkOperatorFlowScenario.texts.direct
  );
  await expect(ui.channelMessage(operatorFlow.messageIds.trace)).toContainText(
    browserNetworkOperatorFlowScenario.texts.trace
  );
  await expect(ui.channelMessage(openWorkMessageId)).toContainText("Need operator input");
  await expect(ui.channelMessage(operatorFlow.messageIds.say)).toHaveCount(0);

  await ui.inspectorToggle.click();
  await expect(networkWin.getByTestId("network-inspector")).toBeVisible();
  await networkWin.getByTestId("network-inspector-tab-work").click();
  await expect(ui.workInspector).toBeVisible();
  await expect(ui.workInspectorRow(openWorkId)).toContainText("needs input");
  await browserArtifacts.captureScreenshot("network-work-inspector", appPage);

  await appPage.goto(runtime.url("/network/" + workspace.id + "/" + channelName + "/directs"), {
    waitUntil: "domcontentloaded",
  });
  await expect(ui.directList).toBeVisible();
  await expect(ui.newDirectButton).toBeEnabled();
  await ui.newDirectButton.click();
  await expect(ui.newDirectDialog).toBeVisible();
  const resolvePeerId = (await ui
    .newDirectPeer(operatorFlow.responder.peerId)
    .isVisible()
    .catch(() => false))
    ? operatorFlow.responder.peerId
    : operatorFlow.initiator.peerId;
  await ui.newDirectPeer(resolvePeerId).click();
  await expect
    .poll(() => new URL(appPage.url()).pathname)
    .toBe(`/network/${workspace.id}/${channelName}/directs/${operatorFlow.directId}`);

  await ui.threadTab.click();
  await ui.threadItem(operatorFlow.threadId).click();
  await expect(ui.channelMessage(operatorFlow.messageIds.summary)).toContainText(
    browserNetworkOperatorFlowScenario.texts.summary
  );

  await ui.directTab.click();
  await ui.directItem(operatorFlow.directId).click();
  await browserArtifacts.captureScreenshot("network-direct-detail", appPage);

  const parity = await captureNetworkParity(runtime, {
    channel: channelName,
    directId: operatorFlow.directId,
    threadId: operatorFlow.threadId,
    workId: openWorkId,
    workspaceId: workspace.id,
  });
  assertNetworkParity(parity, {
    channel: channelName,
    directId: operatorFlow.directId,
    threadId: operatorFlow.threadId,
    workId: openWorkId,
  });
  expect(JSON.stringify(parity)).toContain(workspace.id);
  await runtime.artifactCollector.captureJSON("browser_api_snapshots", parity);
  await browserArtifacts.persist(appPage);

  const routeStateBytes = await readFile(
    runtime.artifactCollector.artifactPath("browser_route_state"),
    "utf8"
  );
  const routeState = JSON.parse(routeStateBytes) as Record<string, unknown>;
  expect(routeState).toMatchObject({
    network_selected_channel: channelName,
    network_selected_direct: operatorFlow.directId,
    network_work_count: expect.any(Number),
  });
  expect(Number(routeState.network_work_count)).toBeGreaterThanOrEqual(1);
  expect(routeState).not.toHaveProperty("network_selected_peer");
  expect(routeState).not.toHaveProperty("interaction_id");

  const browserArtifactText = [
    routeStateBytes,
    await readFile(runtime.artifactCollector.artifactPath("browser_console"), "utf8"),
    await readFile(runtime.artifactCollector.artifactPath("browser_network"), "utf8"),
    await readFile(runtime.artifactCollector.artifactPath("browser_api_snapshots"), "utf8"),
  ].join("\n");
  expect(browserArtifactText).not.toContain("network_selected_peer");
  expect(browserArtifactText).not.toContain("interaction_id");
  expect(browserArtifactText).not.toMatch(sensitivePattern);
  if (runtime.paths?.daemonLog) {
    expect(await readFile(runtime.paths.daemonLog, "utf8")).not.toMatch(sensitivePattern);
  }
});

interface NetworkStatusEnvelope {
  network: {
    enabled: boolean;
    status: string;
  };
}

interface NetworkChannelEnvelope {
  channel: {
    channel: string;
    purpose?: string | null;
    sessions?: Array<{ id: string; agent_name: string; workspace_id?: string }>;
  };
}

interface NetworkThreadEnvelope {
  thread: {
    thread_id: string;
    channel: string;
  };
}

interface NetworkDirectEnvelope {
  direct: {
    direct_id: string;
    channel: string;
  };
}

interface NetworkMessagesEnvelope {
  messages: Array<{
    message_id: string;
    channel: string;
    thread_id?: string;
    direct_id?: string;
    work_id?: string;
    body?: unknown;
  }>;
}

interface NetworkWorkEnvelope {
  work: {
    work_id: string;
    channel: string;
    state?: string;
  };
}

interface NetworkParitySnapshot {
  cli: {
    channels: unknown;
    direct: unknown;
    directMessages: unknown;
    status: unknown;
    thread: unknown;
    threadMessages: unknown;
    work: unknown;
  };
  http: {
    channel: NetworkChannelEnvelope;
    direct: NetworkDirectEnvelope;
    directMessages: NetworkMessagesEnvelope;
    status: NetworkStatusEnvelope;
    thread: NetworkThreadEnvelope;
    threadMessages: NetworkMessagesEnvelope;
    work: NetworkWorkEnvelope;
  };
  uds: {
    channel?: NetworkChannelEnvelope;
    direct?: NetworkDirectEnvelope;
    directMessages?: NetworkMessagesEnvelope;
    status?: NetworkStatusEnvelope;
    thread?: NetworkThreadEnvelope;
    threadMessages?: NetworkMessagesEnvelope;
    work?: NetworkWorkEnvelope;
  };
}

async function prepareNetworkRuntime(
  runtime: BrowserRuntime,
  page: import("@playwright/test").Page
): Promise<WorkspacePayload> {
  if (!runtime.paths?.homeDir) {
    throw new Error("network e2e requires launch-mode runtime paths.");
  }
  const workspace = await runtime.resolveWorkspace(runtime.paths.homeDir);
  const ui = networkOperatorSelectors(page);
  await page.goto(runtime.url("/"), { waitUntil: "domcontentloaded" });
  await useGlobalWorkspaceIfPrompted(ui);
  return workspace;
}

async function createChannelFromUI(
  page: import("@playwright/test").Page,
  ui: ReturnType<typeof networkOperatorSelectors>,
  input: { agents: string[]; channel: string; purpose: string; workspaceId: string }
): Promise<void> {
  await expect(ui.openCreateDialog).toBeEnabled();
  await ui.openCreateDialog.click();
  await expect(ui.createDialog).toBeVisible();
  await ui.channelNameInput.fill(input.channel);
  await ui.channelPurposeInput.fill(input.purpose);
  await ui.createAgentTrigger.click();
  for (const agent of input.agents) {
    await ui.agentOption(agent).click();
  }
  const firstAgent = input.agents[0];
  if (!firstAgent) throw new Error("channel creation requires at least one agent");
  await ui.createAgentTrigger.click();
  await expect(ui.agentOption(firstAgent)).toBeHidden();
  const createResponse = page.waitForResponse(
    response =>
      response.request().method() === "POST" &&
      response
        .url()
        .endsWith(`/api/workspaces/${encodeURIComponent(input.workspaceId)}/network/channels`)
  );
  await expect(ui.createSubmit).toBeEnabled();
  await ui.createSubmit.click();
  expect((await createResponse).status()).toBe(201);
  await expect(ui.createDialog).toBeHidden();
  await expect
    .poll(() => new URL(page.url()).pathname)
    .toBe(`/network/${input.workspaceId}/${input.channel}/threads`);
}

async function seedOpenNetworkWork(
  runtime: BrowserRuntime,
  operatorFlow: {
    channel: string;
    directId: string;
    initiator: { peerId: string };
    messageIds: typeof browserNetworkOperatorFlowScenario.messageIds;
    responder: { id: string };
    workspaceId: string;
  }
): Promise<void> {
  await runtime.requestJSON(networkWorkspacePath(operatorFlow.workspaceId, "/send"), {
    method: "POST",
    body: JSON.stringify({
      id: openWorkRequestMessageId,
      session_id: operatorFlow.responder.id,
      channel: operatorFlow.channel,
      kind: "say",
      surface: "direct",
      direct_id: operatorFlow.directId,
      to: operatorFlow.initiator.peerId,
      work_id: openWorkId,
      reply_to: operatorFlow.messageIds.direct,
      trace_id: "browser_trace_needs_input_17",
      causation_id: operatorFlow.messageIds.direct,
      body: {
        text: "Need operator input before applying patch.",
        intent: "needs-input-request",
        artifact_refs: [],
      },
    }),
  });
  await runtime.requestJSON(networkWorkspacePath(operatorFlow.workspaceId, "/send"), {
    method: "POST",
    body: JSON.stringify({
      id: openWorkMessageId,
      session_id: operatorFlow.responder.id,
      channel: operatorFlow.channel,
      kind: "trace",
      surface: "direct",
      direct_id: operatorFlow.directId,
      to: operatorFlow.initiator.peerId,
      work_id: openWorkId,
      reply_to: openWorkRequestMessageId,
      trace_id: "browser_trace_needs_input_17",
      causation_id: openWorkRequestMessageId,
      body: {
        state: "needs_input",
        message: "Need operator input before applying patch.",
        artifact_refs: [],
      },
    }),
  });
  await expect
    .poll(async () => {
      const payload = await runtime.requestJSON<NetworkMessagesEnvelope>(
        networkWorkspacePath(
          operatorFlow.workspaceId,
          `/channels/${encodeURIComponent(operatorFlow.channel)}/directs/${encodeURIComponent(
            operatorFlow.directId
          )}/messages?work_id=${encodeURIComponent(openWorkId)}`
        )
      );
      return payload.messages.map(message => message.message_id);
    })
    .toContain(openWorkMessageId);
}

async function captureNetworkParity(
  runtime: BrowserRuntime,
  ids: { channel: string; directId: string; threadId: string; workId: string; workspaceId: string }
): Promise<NetworkParitySnapshot> {
  if (!runtime.requestOperatorJSON) {
    throw new Error("network parity requires launch-mode UDS access.");
  }
  const threadPath = networkWorkspacePath(
    ids.workspaceId,
    `/channels/${encodeURIComponent(ids.channel)}/threads/${encodeURIComponent(ids.threadId)}`
  );
  const directPath = networkWorkspacePath(
    ids.workspaceId,
    `/channels/${encodeURIComponent(ids.channel)}/directs/${encodeURIComponent(ids.directId)}`
  );
  const threadMessagesPath = `${threadPath}/messages`;
  const directMessagesPath = `${directPath}/messages?work_id=${encodeURIComponent(ids.workId)}`;
  const workPath = networkWorkspacePath(ids.workspaceId, `/work/${encodeURIComponent(ids.workId)}`);
  return {
    http: {
      status: await runtime.requestJSON<NetworkStatusEnvelope>("/api/network/status"),
      channel: await runtime.requestJSON<NetworkChannelEnvelope>(
        networkWorkspacePath(ids.workspaceId, `/channels/${encodeURIComponent(ids.channel)}`)
      ),
      thread: await runtime.requestJSON<NetworkThreadEnvelope>(threadPath),
      threadMessages: await runtime.requestJSON<NetworkMessagesEnvelope>(threadMessagesPath),
      direct: await runtime.requestJSON<NetworkDirectEnvelope>(directPath),
      directMessages: await runtime.requestJSON<NetworkMessagesEnvelope>(directMessagesPath),
      work: await runtime.requestJSON<NetworkWorkEnvelope>(workPath),
    },
    uds: {
      status: await runtime.requestOperatorJSON<NetworkStatusEnvelope>("/api/network/status"),
      channel: await runtime.requestOperatorJSON<NetworkChannelEnvelope>(
        networkWorkspacePath(ids.workspaceId, `/channels/${encodeURIComponent(ids.channel)}`)
      ),
      thread: await runtime.requestOperatorJSON<NetworkThreadEnvelope>(threadPath),
      threadMessages:
        await runtime.requestOperatorJSON<NetworkMessagesEnvelope>(threadMessagesPath),
      direct: await runtime.requestOperatorJSON<NetworkDirectEnvelope>(directPath),
      directMessages:
        await runtime.requestOperatorJSON<NetworkMessagesEnvelope>(directMessagesPath),
      work: await runtime.requestOperatorJSON<NetworkWorkEnvelope>(workPath),
    },
    cli: {
      status: await networkCLI(runtime, ["network", "status"]),
      channels: await networkCLI(runtime, ["network", "--workspace", ids.workspaceId, "channels"]),
      thread: await networkCLI(runtime, [
        "network",
        "--workspace",
        ids.workspaceId,
        "threads",
        "show",
        "--channel",
        ids.channel,
        "--thread",
        ids.threadId,
      ]),
      threadMessages: await networkCLI(runtime, [
        "network",
        "--workspace",
        ids.workspaceId,
        "threads",
        "messages",
        "--channel",
        ids.channel,
        "--thread",
        ids.threadId,
      ]),
      direct: await networkCLI(runtime, [
        "network",
        "--workspace",
        ids.workspaceId,
        "directs",
        "show",
        "--channel",
        ids.channel,
        "--direct",
        ids.directId,
      ]),
      directMessages: await networkCLI(runtime, [
        "network",
        "--workspace",
        ids.workspaceId,
        "directs",
        "messages",
        "--channel",
        ids.channel,
        "--direct",
        ids.directId,
        "--work",
        ids.workId,
      ]),
      work: await networkCLI(runtime, [
        "network",
        "--workspace",
        ids.workspaceId,
        "work",
        "lookup",
        "--work",
        ids.workId,
      ]),
    },
  };
}

function networkWorkspacePath(workspaceId: string, suffix: string): string {
  const normalizedWorkspaceID = workspaceId.trim();
  if (normalizedWorkspaceID === "") {
    throw new Error("network workspace path requires a workspace_id");
  }
  return `/api/workspaces/${encodeURIComponent(normalizedWorkspaceID)}/network${
    suffix.startsWith("/") ? suffix : `/${suffix}`
  }`;
}

function assertNetworkParity(
  snapshot: NetworkParitySnapshot,
  ids: { channel: string; directId: string; threadId: string; workId: string }
): void {
  expect(snapshot.http.status.network.enabled).toBe(true);
  expect(snapshot.uds.status?.network.enabled).toBe(true);
  expect(snapshot.http.channel.channel.channel).toBe(ids.channel);
  expect(snapshot.uds.channel?.channel.channel).toBe(ids.channel);
  expect(snapshot.http.thread.thread.thread_id).toBe(ids.threadId);
  expect(snapshot.uds.thread?.thread.thread_id).toBe(ids.threadId);
  expect(snapshot.http.direct.direct.direct_id).toBe(ids.directId);
  expect(snapshot.uds.direct?.direct.direct_id).toBe(ids.directId);
  expect(
    snapshot.http.directMessages.messages.some(message => message.work_id === ids.workId)
  ).toBe(true);
  expect(
    snapshot.uds.directMessages?.messages.some(message => message.work_id === ids.workId)
  ).toBe(true);
  expect(snapshot.http.work.work.work_id).toBe(ids.workId);
  expect(snapshot.uds.work?.work.work_id).toBe(ids.workId);

  const cliPayload = JSON.stringify(snapshot.cli);
  for (const value of [ids.channel, ids.threadId, ids.directId, ids.workId]) {
    expect(cliPayload).toContain(value);
  }
}

async function networkCLI(runtime: BrowserRuntime, args: string[]): Promise<unknown> {
  if (!runtime.paths) {
    throw new Error("network CLI checks require launch-mode runtime paths.");
  }
  const { stdout } = await execFileAsync(runtime.paths.cliShim, [...args, "-o", "json"], {
    env: cliEnv(runtime.paths),
    maxBuffer: 1024 * 1024,
    timeout: 15_000,
  });
  return JSON.parse(stdout) as unknown;
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

async function assertNoNetworkSensitiveLeak(
  page: import("@playwright/test").Page,
  runtime: BrowserRuntime,
  snapshot: unknown
): Promise<void> {
  await expect(page.locator("body")).not.toContainText(sensitivePattern);
  const artifactPayloads = [
    JSON.stringify(snapshot),
    await readFile(runtime.artifactCollector.artifactPath("browser_console"), "utf8"),
    await readFile(runtime.artifactCollector.artifactPath("browser_network"), "utf8"),
    await readFile(runtime.artifactCollector.artifactPath("browser_route_state"), "utf8"),
    await readFile(runtime.artifactCollector.artifactPath("browser_api_snapshots"), "utf8"),
  ];
  for (const payload of artifactPayloads) {
    expect(payload).not.toMatch(sensitivePattern);
  }
  if (runtime.paths?.daemonLog) {
    expect(await readFile(runtime.paths.daemonLog, "utf8")).not.toMatch(sensitivePattern);
  }
}
