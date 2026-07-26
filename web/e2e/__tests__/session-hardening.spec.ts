import { execFile } from "node:child_process";
import { createHash } from "node:crypto";
import { mkdir, mkdtemp, readFile, writeFile } from "node:fs/promises";
import os from "node:os";
import path from "node:path";
import process from "node:process";
import { fileURLToPath } from "node:url";
import { promisify } from "node:util";

import { sessionWindow, switchWorkspace } from "../fixtures/os-navigation";
import { sessionLifecycleSelectors, sessionWindowSelectors } from "../fixtures/selectors";
import {
  cleanupBrowserSettingsFixtures,
  seedBrowserSettingsFixtures,
  type BrowserRuntime,
  type WorkspacePayload,
} from "../fixtures/runtime";
import { expect, test } from "../fixtures/test";
import { useGlobalWorkspaceIfPrompted } from "../fixtures/workspace";

const execFileAsync = promisify(execFile);
const fixtureRoot = path.resolve(
  path.dirname(fileURLToPath(import.meta.url)),
  "..",
  "..",
  "..",
  "internal",
  "testutil",
  "acpmock",
  "testdata"
);
const browserHardeningFixture = path.join(fixtureRoot, "browser_session_hardening_fixture.json");
const driverFaultFixture = path.join(fixtureRoot, "driver_fault_fixture.json");
const autoTitleFixture = path.join(fixtureRoot, "auto_title_fixture.json");
const costProvenanceFixture = path.join(fixtureRoot, "cost_provenance_fixture.json");
const toolArtifactFixture = path.join(fixtureRoot, "browser_tool_artifact_fixture.json");
const permissionAgent = "permission-hardening-agent";
const faultAgent = "faulty";
const autoTitleAgent = "auto-title-agent";
const costProvenanceAgent = "cost-provenance-agent";
const toolArtifactAgent = "tool-artifact-agent";
const costProvenancePrompt = "Summarize the cost provenance run";
const toolArtifactDigest = "c82d7447711d610d6c0d8fd52b8c8ee99f051a81e62f51bf052eaad467fca444";
const toolArtifactTail = "E2E-009 tool artifact tail";
const sensitivePattern =
  /agh_claim_|claim_token["':\s]|mcp[_-]?auth|telegram-bot-token|pkce|oauth|webhook_secret|provider[_-]?credentials?["'\s]*[:=]/i;

interface SessionPayload {
  id: string;
  agent_name: string;
  provider: string;
  state: string;
  workspace_id: string;
  name?: string | null;
}

interface SessionEnvelope {
  session: SessionPayload;
}

interface SessionListEnvelope {
  sessions: SessionPayload[];
}

interface UsageEnvelope {
  usage?: {
    cost_status?: string;
    cost_source?: string;
    total_cost?: number;
    cost_currency?: string;
  };
}

interface SessionEventEnvelope {
  events: unknown[];
}

interface SessionHistoryEnvelope {
  history: unknown[];
}

interface SessionRepairEnvelope {
  repair: {
    session_id: string;
    issues?: unknown[];
    actions?: unknown[];
  };
}

test.use({
  runtimeOptions: {
    seed: {
      mockAgents: [
        {
          fixturePath: browserHardeningFixture,
          fixtureAgent: permissionAgent,
        },
        {
          fixturePath: driverFaultFixture,
          fixtureAgent: faultAgent,
        },
        {
          fixturePath: autoTitleFixture,
          fixtureAgent: autoTitleAgent,
        },
        {
          fixturePath: toolArtifactFixture,
          fixtureAgent: toolArtifactAgent,
        },
      ],
    },
  },
});

test("first document navigation to a canonical session route loads the app shell and transcript", async ({
  page,
  runtime,
}) => {
  if (!runtime.paths?.homeDir) {
    throw new Error("cold session-route E2E requires launch-mode runtime paths.");
  }

  const workspace = await runtime.resolveWorkspace(runtime.paths.homeDir);
  await runtime.requestJSON("/api/onboarding/complete", { method: "POST" });
  const session = await createSession(runtime, permissionAgent, workspace.id);
  const sessionRequestPath = sessionAPIPath(workspace.id, session.id);
  const observedSessionRequests = new Set<string>();

  page.on("request", request => {
    const pathname = new URL(request.url()).pathname;
    if (request.method() === "GET" && pathname === sessionRequestPath) {
      observedSessionRequests.add(pathname);
    }
  });
  await page.addInitScript(
    ({ workspaceId }) => {
      localStorage.setItem(
        "agh:active-workspace",
        JSON.stringify({
          state: { selectedWorkspaceId: workspaceId },
          version: 0,
        })
      );
    },
    { workspaceId: workspace.id }
  );

  await page.goto(runtime.url(sessionPath(permissionAgent, session.id)), {
    waitUntil: "domcontentloaded",
  });

  const sessionWin = sessionWindow(page, session.id);
  const ui = sessionWindowSelectors(sessionWin, page);
  await expect
    .poll(async () => ({
      osDesktopVisible: await page.getByTestId("os-desktop").isVisible(),
      chatViewVisible: await ui.chatView.isVisible(),
      sessionRequestObserved: observedSessionRequests.has(sessionRequestPath),
    }))
    .toEqual({
      osDesktopVisible: true,
      chatViewVisible: true,
      sessionRequestObserved: true,
    });
});

test("E2E-009: operator pages an oversized tool result to its retained tail", async ({
  appPage,
  runtime,
}) => {
  const workspace = await prepareSessionRuntime(runtime, appPage);
  await seedToolArtifact(runtime, workspace.root_dir);
  const session = await createSession(runtime, toolArtifactAgent, workspace.id);
  const artifactOffsets: string[] = [];
  appPage.on("response", response => {
    const url = new URL(response.url());
    if (url.pathname.includes("/tool-artifacts/")) {
      artifactOffsets.push(url.searchParams.get("offset") ?? "0");
    }
  });

  await appPage.goto(runtime.url(sessionPath(toolArtifactAgent, session.id)), {
    waitUntil: "domcontentloaded",
  });
  const sessionWin = sessionWindow(appPage, session.id);
  const ui = sessionWindowSelectors(sessionWin, appPage);
  await expect(sessionWin).toBeVisible();
  await ui.composerTextarea.fill("exercise tool artifact recovery");
  await ui.composerTextarea.press("Enter");

  await expect(ui.chatView).toContainText("Retained result is ready for page-back.");
  await sessionWin.getByTestId("turn-fold-row").click();
  await sessionWin.getByRole("button", { name: "Toggle tool call (success)" }).click();
  await expect(ui.chatView).toContainText("E2E-009 bounded retained-result preview");
  await sessionWin.getByRole("button", { name: "Open full result" }).click();
  const loadMore = sessionWin.getByRole("button", { name: "Load more" });
  await expect(loadMore).toBeVisible();
  await loadMore.click();
  await expect(loadMore).toBeEnabled();
  await loadMore.click();

  await expect(loadMore).toBeHidden();
  await expect(sessionWin.getByTestId("full-tool-result")).toContainText(toolArtifactTail);
  await expect(sessionWin.getByText("140,084 of 140,084 bytes")).toBeVisible();
  expect(artifactOffsets).toEqual(["0", "65536", "131072"]);
});

test("operator rejects a permission request, records tool output, and keeps session artifacts private", async ({
  appPage,
  browserArtifacts,
  runtime,
}) => {
  const workspace = await prepareSessionRuntime(runtime, appPage);
  const session = await createSession(runtime, permissionAgent, workspace.id);
  const deepLinkLoadingPhases = new Set<string>();
  let sampleDeepLinkLoading = true;
  const deepLinkLoadingSampler = (async () => {
    while (sampleDeepLinkLoading) {
      if (
        await appPage
          .getByTestId("session-route-loading")
          .isVisible()
          .catch(() => false)
      ) {
        deepLinkLoadingPhases.add("route");
      }
      if (
        await appPage
          .getByTestId("thread-transcript-skeleton")
          .isVisible()
          .catch(() => false)
      ) {
        deepLinkLoadingPhases.add("transcript");
      }
      await appPage.waitForTimeout(25).catch(() => {
        sampleDeepLinkLoading = false;
      });
    }
  })();

  await appPage.goto(runtime.url(`/session/${session.id}`), {
    waitUntil: "domcontentloaded",
  });

  const sessionWin = sessionWindow(appPage, session.id);
  const ui = sessionWindowSelectors(sessionWin, appPage);
  await expect(sessionWin).toBeVisible();
  sampleDeepLinkLoading = false;
  await deepLinkLoadingSampler;
  await expect
    .poll(() => new URL(appPage.url()).pathname)
    .toBe(sessionPath(permissionAgent, session.id));
  expect(deepLinkLoadingPhases.size).toBeLessThanOrEqual(1);
  await expect(ui.composerTextarea).toBeEnabled();

  await ui.composerTextarea.fill("exercise permission hardening");
  await ui.composerTextarea.press("Enter");

  await expect(ui.chatView).toContainText("Permission hardening started.");
  await expect(ui.permissionPrompt).toBeVisible();
  await expect(sessionWin.getByTestId("permission-tool-input")).toContainText("hardening.txt");

  const approvalResponsePromise = appPage.waitForResponse(
    response =>
      response.request().method() === "POST" &&
      response.url().endsWith(sessionAPIPath(workspace.id, session.id, "/approve"))
  );
  await sessionWin.getByTestId("permission-reject-always").click();
  expect((await approvalResponsePromise).ok()).toBe(true);

  await expect(ui.permissionPrompt).toBeHidden();
  await expect(ui.topbarOverflow).toBeVisible();

  const snapshot = await captureSessionSnapshot(runtime, workspace.id, session.id);
  expect(JSON.stringify(snapshot.events)).toContain("tool-hardening-read-1");
  expect(JSON.stringify(snapshot.events)).toContain("hardening read complete");
  expect(JSON.stringify(snapshot.events)).toContain("reject-always");
  expect(JSON.stringify(snapshot.history)).toContain("exercise permission hardening");
  await runtime.artifactCollector.captureJSON("browser_api_snapshots", snapshot);
  await browserArtifacts.captureScreenshot("session-permission-rejected", appPage);
  const manifest = await browserArtifacts.persist(appPage);
  expect(manifest.artifacts).toEqual(
    expect.arrayContaining([
      expect.objectContaining({ kind: "browser_api_snapshots" }),
      expect.objectContaining({ kind: "browser_route_state" }),
      expect.objectContaining({ kind: "browser_screenshots" }),
      expect.objectContaining({ kind: "browser_trace" }),
    ])
  );

  const routeState = JSON.parse(
    await readFile(runtime.artifactCollector.artifactPath("browser_route_state"), "utf8")
  ) as Record<string, unknown>;
  expect(routeState).toMatchObject({
    chat_view_visible: true,
    session_topbar_overflow_visible: true,
    message_count: expect.any(Number),
    permission_prompt_visible: false,
  });
  await assertNoSensitiveLeak(appPage, runtime, snapshot);
});

test("operator cancels a running prompt, clears the transcript, and deletes the session across surfaces", async ({
  appPage,
  browserArtifacts,
  runtime,
}) => {
  const workspace = await prepareSessionRuntime(runtime, appPage);
  const session = await createSession(runtime, faultAgent, workspace.id);

  await appPage.goto(runtime.url(sessionPath(faultAgent, session.id)), {
    waitUntil: "domcontentloaded",
  });

  const sessionWin = sessionWindow(appPage, session.id);
  const ui = sessionWindowSelectors(sessionWin, appPage);
  await expect(sessionWin).toBeVisible();
  await ui.composerTextarea.fill("block until canceled");
  await ui.composerTextarea.press("Enter");
  await expect(ui.chatView).toContainText("block until canceled");

  const stopActionPaths = [
    sessionAPIPath(workspace.id, session.id, "/prompt/cancel"),
    sessionAPIPath(workspace.id, session.id, "/stop"),
  ];
  const stopActionResponsePromise = appPage.waitForResponse(
    response =>
      response.request().method() === "POST" &&
      stopActionPaths.some(pathname => response.url().endsWith(pathname))
  );
  await expect(ui.stopButton).toBeVisible();
  await ui.stopButton.click();
  expect((await stopActionResponsePromise).ok()).toBe(true);

  await expect(ui.topbarOverflow).toBeVisible({ timeout: 60_000 });
  const beforeClear = await captureSessionSnapshot(runtime, workspace.id, session.id);
  expect(JSON.stringify(beforeClear.history)).toContain("block until canceled");

  await ui.topbarOverflow.click();
  await ui.composerClearButton.click();
  await expect(appPage.getByTestId("composer-clear-dialog")).toBeVisible();
  const clearResponsePromise = appPage.waitForResponse(
    response =>
      response.request().method() === "POST" &&
      response.url().endsWith(sessionAPIPath(workspace.id, session.id, "/clear"))
  );
  await appPage.getByTestId("composer-clear-confirm").click();
  expect((await clearResponsePromise).ok()).toBe(true);
  await expect(ui.chatView).not.toContainText("block until canceled");
  await appPage.reload({ waitUntil: "domcontentloaded" });
  await expect(sessionWin).toBeVisible();
  await expect(ui.chatView).not.toContainText("block until canceled");

  const afterClear = await captureSessionSnapshot(runtime, workspace.id, session.id);
  expect(JSON.stringify(afterClear.history)).not.toContain("block until canceled");

  const deletableSession = await createSession(runtime, faultAgent, workspace.id);
  await appPage.goto(runtime.url(sessionPath(faultAgent, deletableSession.id)), {
    waitUntil: "domcontentloaded",
  });
  const deletableWin = sessionWindow(appPage, deletableSession.id);
  const deletableUi = sessionWindowSelectors(deletableWin, appPage);
  await expect(deletableWin).toBeVisible();
  await deletableUi.topbarOverflow.click();
  await deletableUi.deleteButton.click();
  await expect(appPage.getByTestId("delete-dialog")).toBeVisible();
  const deleteResponsePromise = appPage.waitForResponse(
    response =>
      response.request().method() === "DELETE" &&
      response.url().endsWith(sessionAPIPath(workspace.id, deletableSession.id))
  );
  await appPage.getByTestId("delete-dialog-confirm").click();
  expect((await deleteResponsePromise).ok()).toBe(true);
  await expect.poll(() => new URL(appPage.url()).pathname).toBe(`/agents/${faultAgent}`);

  await expect(
    runtime.requestJSON<SessionEnvelope>(sessionAPIPath(workspace.id, deletableSession.id))
  ).rejects.toThrow("404");
  if (!runtime.requestOperatorJSON) {
    throw new Error("session delete parity check requires launch-mode UDS access.");
  }
  await expect(
    runtime.requestOperatorJSON<SessionEnvelope>(sessionAPIPath(workspace.id, deletableSession.id))
  ).rejects.toThrow("404");
  const cliSessions = await listSessionsViaCLI(runtime);
  expect(cliSessions.some(record => record.id === deletableSession.id)).toBe(false);

  await runtime.artifactCollector.captureJSON("browser_api_snapshots", {
    after_clear: afterClear,
    before_clear: beforeClear,
    cli_sessions_after_delete: cliSessions,
    deleted_session_id: deletableSession.id,
  });
  await browserArtifacts.captureScreenshot("session-cancel-clear-delete", appPage);
  await browserArtifacts.persist(appPage);
  await assertNoSensitiveLeak(appPage, runtime, {
    afterClear,
    beforeClear,
    cli_sessions_after_delete: cliSessions,
    deleted_session_id: deletableSession.id,
  });
});

test("operator repairs an interrupted session through HTTP, UDS, and CLI without losing transcript evidence", async ({
  appPage,
  browserArtifacts,
  runtime,
}) => {
  const workspace = await prepareSessionRuntime(runtime, appPage);
  const session = await createSession(runtime, faultAgent, workspace.id);

  await appPage.goto(runtime.url(sessionPath(faultAgent, session.id)), {
    waitUntil: "domcontentloaded",
  });

  const sessionWin = sessionWindow(appPage, session.id);
  const ui = sessionWindowSelectors(sessionWin, appPage);
  await expect(sessionWin).toBeVisible();
  await ui.composerTextarea.fill("trigger crash mid-stream");
  await ui.composerTextarea.press("Enter");
  await expect(ui.chatView).toContainText("partial before crash", { timeout: 15_000 });
  await expect(ui.resumeButton).not.toBeVisible({ timeout: 20_000 });

  const beforeRepair = await captureSessionSnapshot(runtime, workspace.id, session.id);
  expect(JSON.stringify(beforeRepair.history)).toContain("trigger crash mid-stream");
  expect(JSON.stringify(beforeRepair.history)).toContain("partial before crash");

  const httpRepair = await runtime.requestJSON<SessionRepairEnvelope>(
    sessionAPIPath(workspace.id, session.id, "/repair?dry_run=true&force=true"),
    { method: "POST" }
  );
  expect(httpRepair.repair.session_id).toBe(session.id);

  if (!runtime.requestOperatorJSON) {
    throw new Error("session repair E2E requires launch-mode UDS access.");
  }
  const udsRepair = await runtime.requestOperatorJSON<SessionRepairEnvelope>(
    sessionAPIPath(workspace.id, session.id, "/repair?dry_run=true&force=true"),
    { method: "POST" }
  );
  expect(udsRepair.repair.session_id).toBe(session.id);

  const cliRepair = await repairSessionViaCLI(runtime, session.id);
  expect(JSON.stringify(cliRepair)).toContain(session.id);

  const afterRepair = await captureSessionSnapshot(runtime, workspace.id, session.id);
  expect(JSON.stringify(afterRepair.history)).toContain("trigger crash mid-stream");
  expect(JSON.stringify(afterRepair.history)).toContain("partial before crash");
  expect(afterRepair.session.session.state).toBe("stopped");

  await appPage.reload({ waitUntil: "domcontentloaded" });
  await expect(ui.chatView).toContainText("partial before crash");
  await expect(ui.resumeButton).not.toBeVisible();

  await runtime.artifactCollector.captureJSON("browser_api_snapshots", {
    after_repair: afterRepair,
    before_repair: beforeRepair,
    cli_repair: cliRepair,
    http_repair: httpRepair,
    uds_repair: udsRepair,
  });
  await browserArtifacts.captureScreenshot("session-repair-parity", appPage);
  await browserArtifacts.persist(appPage);
  await assertNoSensitiveLeak(appPage, runtime, { afterRepair, beforeRepair, cliRepair });
});

test("operator sees the daemon-generated session title and the file-mutation verifier marker", async ({
  appPage,
  browserArtifacts,
  runtime,
}) => {
  const workspace = await prepareSessionRuntime(runtime, appPage);
  const session = await createSession(runtime, autoTitleAgent, workspace.id);
  expect(session.name ?? "").toBe("");

  await appPage.goto(runtime.url(sessionPath(autoTitleAgent, session.id)), {
    waitUntil: "domcontentloaded",
  });

  const sessionWin = sessionWindow(appPage, session.id);
  const ui = sessionWindowSelectors(sessionWin, appPage);
  await expect(sessionWin).toBeVisible();
  await ui.composerTextarea.fill("Implement checkout retry fencing");
  await ui.composerTextarea.press("Enter");

  await expect(ui.chatView).toContainText("Implemented checkout retry fencing.");

  const markerNotice = sessionWin.getByTestId("transcript-marker-notice");
  await expect(markerNotice).toBeVisible();
  await expect(markerNotice).toHaveAttribute("data-tone", "warning");
  await expect(sessionWin.getByTestId("transcript-marker-kind")).toContainText(
    "transcript_marker.file_mutation_unverified"
  );
  await expect(sessionWin.getByTestId("transcript-marker-summary")).toContainText(
    "file mutation failed and was not recovered"
  );
  await browserArtifacts.captureScreenshot("session-auto-title-verifier-marker", appPage);

  await expect
    .poll(
      async () => {
        const detail = await runtime.requestJSON<SessionEnvelope>(
          sessionAPIPath(workspace.id, session.id)
        );
        return detail.session.name ?? "";
      },
      { timeout: 30_000 }
    )
    .toBe("Checkout Retry Fencing");

  await appPage.goto(runtime.url(`/agents/${autoTitleAgent}`), {
    waitUntil: "domcontentloaded",
  });
  await appPage.getByTestId("agent-tab-sessions").click();
  const sessionLink = appPage.getByTestId(`agent-session-link-${session.id}`);
  await expect(sessionLink).toContainText("Checkout Retry Fencing");
  await expect(sessionLink).not.toContainText("New session");
  await browserArtifacts.captureScreenshot("session-auto-title-list", appPage);

  const snapshot = await captureSessionSnapshot(runtime, workspace.id, session.id);
  await runtime.artifactCollector.captureJSON("browser_api_snapshots", snapshot);
  await browserArtifacts.persist(appPage);
  await assertNoSensitiveLeak(appPage, runtime, snapshot);
});

test.describe("E2E-010 truthful session cost provenance by auth mode", () => {
  test.use({
    runtimeOptions: {
      env: { PATH: ["/usr/bin", "/bin"].join(path.delimiter) },
      modelsDevEnabled: false,
      seed: {
        mockAgents: [{ fixturePath: costProvenanceFixture, fixtureAgent: costProvenanceAgent }],
      },
    },
  });

  test("operator sees an estimated cue for a metered provider and included for a subscription provider", async ({
    appPage,
    runtime,
  }) => {
    if (!runtime.paths?.homeDir) {
      throw new Error("cost provenance E2E requires launch-mode runtime paths.");
    }
    const ui = sessionLifecycleSelectors(appPage);
    const workspaceRoot = await mkdtemp(path.join(os.tmpdir(), "agh-cost-provenance-workspace-"));
    const driverCommand = await readSeededAgentCommand(runtime.paths.homeDir, costProvenanceAgent);

    // The model catalog is daemon-global: workspace config never reconciles it, so cost
    // provenance is seeded through the public Settings PUT, which applies active config and
    // reconciles the catalog. auth_mode="none" defaults to none_security="local_transport", and
    // the metered curated rates land in the global catalog before any turn runs.
    const seeded = await seedBrowserSettingsFixtures(runtime, {
      providers: [
        {
          name: "cost-metered",
          settings: {
            command: driverCommand,
            display_name: "Cost Metered",
            harness: "acp",
            auth_mode: "none",
            models: {
              default: "cost-metered-model",
              curated: [
                {
                  id: "cost-metered-model",
                  display_name: "Cost Metered Model",
                  cost_input_per_million: 3,
                  cost_output_per_million: 15,
                },
              ],
            },
          },
        },
        {
          name: "cost-included",
          settings: {
            command: driverCommand,
            display_name: "Cost Included",
            harness: "acp",
            auth_mode: "native_cli",
            models: {
              default: "cost-included-model",
              curated: [{ id: "cost-included-model", display_name: "Cost Included Model" }],
            },
          },
        },
      ],
    });

    try {
      const workspace = await runtime.resolveWorkspace(workspaceRoot);

      await appPage.goto(runtime.url("/"), { waitUntil: "domcontentloaded" });
      await useGlobalWorkspaceIfPrompted(ui);
      await expect(ui.osDesktop).toBeVisible();
      await switchWorkspace(appPage, workspace.id, workspace.name);
      await appPage.setViewportSize({ width: 1440, height: 900 });

      const estimated = await openUsageCostForProvider(appPage, runtime, workspace.id, {
        provider: "cost-metered",
        model: "cost-metered-model",
        status: "estimated",
      });
      await expect(estimated).toContainText("≈");
      await expect(estimated).toContainText("Estimated");
      await expect(estimated).toContainText("Catalog rate");
      await expect(estimated).toContainText("$");

      const included = await openUsageCostForProvider(appPage, runtime, workspace.id, {
        provider: "cost-included",
        model: "cost-included-model",
        status: "included",
      });
      await expect(included).toContainText("Included");
      await expect(included).not.toContainText("$");
      await expect(included).not.toContainText("≈");
    } finally {
      await cleanupBrowserSettingsFixtures(runtime, seeded);
    }
  });
});

async function openUsageCostForProvider(
  page: import("@playwright/test").Page,
  runtime: BrowserRuntime,
  workspaceID: string,
  opts: { provider: string; model: string; status: string }
): Promise<import("@playwright/test").Locator> {
  const session = await createProviderSession(runtime, workspaceID, opts.provider, opts.model);

  await page.goto(runtime.url(sessionPath(costProvenanceAgent, session.id)), {
    waitUntil: "domcontentloaded",
  });
  const sessionWin = sessionWindow(page, session.id);
  const sessionUi = sessionWindowSelectors(sessionWin, page);
  await expect(sessionWin).toBeVisible();
  await expect(sessionUi.composerTextarea).toBeEnabled();
  await sessionUi.composerTextarea.fill(costProvenancePrompt);
  await sessionUi.composerTextarea.press("Enter");
  await expect(sessionUi.chatView).toContainText("Cost provenance run recorded.");

  await expect
    .poll(
      async () => {
        const usage = await runtime.requestJSON<UsageEnvelope>(
          sessionAPIPath(workspaceID, session.id, "/usage")
        );
        return usage.usage?.cost_status ?? "";
      },
      { timeout: 30_000 }
    )
    .toBe(opts.status);

  // The page-level usage query stops refetching once the session goes idle, so
  // reload to mount the inspector against the now-populated usage summary.
  await page.reload({ waitUntil: "domcontentloaded" });
  await expect(sessionWin).toBeVisible();
  await sessionWin.getByTestId("session-inspector-toggle").click();
  await sessionWin.getByTestId("session-inspector-tab-usage").click();
  return sessionWin.getByTestId("session-inspector-usage-cost");
}

async function createProviderSession(
  runtime: BrowserRuntime,
  workspaceID: string,
  provider: string,
  model: string
): Promise<SessionPayload> {
  const payload = await runtime.requestJSON<SessionEnvelope>("/api/sessions", {
    method: "POST",
    body: JSON.stringify({
      agent_name: costProvenanceAgent,
      provider,
      model,
      workspace: workspaceID,
    }),
  });
  expect(payload.session.id).not.toBe("");
  expect(payload.session.provider).toBe(provider);
  return payload.session;
}

async function readSeededAgentCommand(homeDir: string, agentName: string): Promise<string> {
  const agentDefPath = path.join(homeDir, "agents", agentName, "AGENT.md");
  const agentDef = await readFile(agentDefPath, "utf8");
  const match = agentDef.match(/^command:\s+(.+)$/m);
  if (!match) {
    throw new Error(`agent definition ${agentDefPath} is missing a command line`);
  }
  return match[1].trim();
}

async function prepareSessionRuntime(
  runtime: BrowserRuntime,
  page: import("@playwright/test").Page
): Promise<WorkspacePayload> {
  if (!runtime.paths?.homeDir) {
    throw new Error("session hardening E2E requires launch-mode runtime paths.");
  }
  const workspace = await runtime.resolveWorkspace(runtime.paths.homeDir);
  const ui = sessionLifecycleSelectors(page);
  await page.goto(runtime.url("/"), { waitUntil: "domcontentloaded" });
  await useGlobalWorkspaceIfPrompted(ui);
  return workspace;
}

async function seedToolArtifact(runtime: BrowserRuntime, workspaceRoot: string): Promise<void> {
  if (!runtime.paths?.homeDir) {
    throw new Error("tool artifact E2E requires launch-mode runtime paths.");
  }
  const content = Buffer.from(
    JSON.stringify({
      content: [{ type: "text", text: `${"x".repeat(140_000)} ${toolArtifactTail}` }],
      truncated: false,
    }),
    "utf8"
  );
  const digest = createHash("sha256").update(content).digest("hex");
  expect(digest).toBe(toolArtifactDigest);
  const identity = await readFile(path.join(workspaceRoot, ".agh", "workspace.toml"), "utf8");
  const workspaceID = /^workspace_id\s*=\s*"([^"]+)"$/m.exec(identity)?.[1];
  if (!workspaceID) {
    throw new Error(`workspace identity is missing from ${workspaceRoot}`);
  }
  const workspaceDigest = createHash("sha256").update(workspaceID).digest("hex");
  const workspaceDir = path.join(runtime.paths.homeDir, "tool-artifacts", workspaceDigest);
  await mkdir(workspaceDir, { recursive: true, mode: 0o700 });
  await writeFile(path.join(workspaceDir, `art_${digest}.json`), content, { mode: 0o600 });
}

async function createSession(
  runtime: BrowserRuntime,
  agentName: string,
  workspaceID: string
): Promise<SessionPayload> {
  const payload = await runtime.requestJSON<SessionEnvelope>("/api/sessions", {
    method: "POST",
    body: JSON.stringify({
      agent_name: agentName,
      workspace: workspaceID,
    }),
  });
  expect(payload.session.id).not.toBe("");
  expect(payload.session.agent_name).toBe(agentName);
  expect(payload.session.workspace_id).toBe(workspaceID);
  return payload.session;
}

async function captureSessionSnapshot(
  runtime: BrowserRuntime,
  workspaceID: string,
  sessionID: string
): Promise<{
  events: SessionEventEnvelope;
  history: SessionHistoryEnvelope;
  session: SessionEnvelope;
  transcript: unknown;
  udsSession?: SessionEnvelope;
}> {
  const sessionPathname = sessionAPIPath(workspaceID, sessionID);
  const snapshot = {
    events: await runtime.requestJSON<SessionEventEnvelope>(`${sessionPathname}/events`),
    history: await runtime.requestJSON<SessionHistoryEnvelope>(`${sessionPathname}/history`),
    session: await runtime.requestJSON<SessionEnvelope>(sessionPathname),
    transcript: await runtime.requestJSON<unknown>(`${sessionPathname}/transcript`),
    udsSession: runtime.requestOperatorJSON
      ? await runtime.requestOperatorJSON<SessionEnvelope>(sessionPathname)
      : undefined,
  };
  if (snapshot.udsSession) {
    expect(snapshot.udsSession.session.id).toBe(snapshot.session.session.id);
    expect(snapshot.udsSession.session.state).toBe(snapshot.session.session.state);
  }
  return snapshot;
}

async function listSessionsViaCLI(runtime: BrowserRuntime): Promise<SessionPayload[]> {
  if (!runtime.paths) {
    throw new Error("session hardening CLI checks require launch-mode runtime paths.");
  }
  const { stdout } = await execFileAsync(
    runtime.paths.cliShim,
    ["session", "list", "--all", "-o", "json"],
    { env: cliEnv(runtime.paths) }
  );
  return (JSON.parse(stdout) as SessionListEnvelope).sessions;
}

async function repairSessionViaCLI(runtime: BrowserRuntime, sessionID: string): Promise<unknown> {
  if (!runtime.paths) {
    throw new Error("session hardening CLI checks require launch-mode runtime paths.");
  }
  const { stdout } = await execFileAsync(
    runtime.paths.cliShim,
    ["session", "repair", sessionID, "--dry-run", "--force", "-o", "json"],
    { env: cliEnv(runtime.paths) }
  );
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

function sessionPath(agentName: string, sessionID: string): string {
  return `/agents/${agentName}/sessions/${sessionID}`;
}

function sessionAPIPath(workspaceID: string, sessionID: string, suffix = ""): string {
  return `/api/workspaces/${encodeURIComponent(workspaceID)}/sessions/${encodeURIComponent(
    sessionID
  )}${suffix}`;
}

async function assertNoSensitiveLeak(
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
