// Suite: daemon-served Home application
// Invariant: Home renders the current overview truth, degrades honestly, and preserves
// HTTP/UDS/CLI parity through real browser flows.
// Boundary IN: daemon-served Web, overview transports, workspace switching, and QA artifacts.
// Boundary OUT: overview aggregation internals (owned by internal/observe suites).
import { execFile } from "node:child_process";
import { mkdtemp, readFile } from "node:fs/promises";
import os from "node:os";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { promisify } from "node:util";

import type { Locator, Page } from "@playwright/test";

import type { OperationResponse } from "@/lib/api-contract";

import { reloadDaemonServedPage } from "../fixtures/navigation";
import { ensureAppWindow, openAppWindow, switchWorkspace } from "../fixtures/os-navigation";
import type { BrowserRuntime, WorkspacePayload } from "../fixtures/runtime";
import { waitForSeedSessionActive } from "../fixtures/runtime";
import { sessionLifecycleTestIds } from "../fixtures/selectors";
import { expect, test } from "../fixtures/test";
import { useGlobalWorkspaceIfPrompted } from "../fixtures/workspace";

const execFileAsync = promisify(execFile);
const homeAgentAlpha = "home-agent-alpha";
const homeAgentBeta = "home-agent-beta";
const browserLifecycleFixture = path.resolve(
  path.dirname(fileURLToPath(import.meta.url)),
  "..",
  "..",
  "..",
  "internal",
  "testutil",
  "acpmock",
  "testdata",
  "browser_session_lifecycle_fixture.json"
);
const sensitivePattern =
  /agh_claim_|claim_token["':\s]|telegram-bot-token|pkce|oauth|webhook_secret|provider[_-]?credentials?["'\s]*[:=]/i;

type HomeOverview = OperationResponse<"getObserveOverview", 200>["overview"];
type HomeAgentCatalog = OperationResponse<"listAgentCatalog", 200>;

interface SessionSummary {
  id: string;
  state: string;
  workspace_id: string;
}

interface SettingsRestartAction {
  operation_id: string;
  status_url: string;
}

interface SettingsRestartStatus {
  status: string;
}

interface HomeSnapshot {
  catalog: HomeAgentCatalog;
  cli?: HomeOverview;
  http: HomeOverview;
  uds?: HomeOverview;
  workspace: WorkspacePayload;
}

test.use({
  runtimeOptions: {
    seed: {
      mockAgents: [
        {
          agentName: homeAgentAlpha,
          fixtureAgent: "browser-lifecycle-agent",
          fixturePath: browserLifecycleFixture,
        },
        {
          agentName: homeAgentBeta,
          fixtureAgent: "browser-lifecycle-agent",
          fixturePath: browserLifecycleFixture,
        },
      ],
    },
  },
});

test("operator sees truthful Home overview, navigation, artifacts, and transport parity", async ({
  appPage,
  browserArtifacts,
  runtime,
}) => {
  const workspace = await prepareHomeRuntime(runtime);
  await useGlobalWorkspaceIfPrompted(workspaceShell(appPage));
  await appPage.goto(runtime.url("/"), { waitUntil: "domcontentloaded" });
  const home = await ensureAppWindow(appPage, "Home", "dashboard");
  const snapshot = await captureHomeSnapshot(runtime, workspace);

  await expect(home.getByTestId("home-body")).toBeVisible();
  await expect(home.getByTestId("home-connection-indicator")).toHaveAttribute(
    "data-status",
    "connected"
  );
  await expect(home.locator('[data-slot="home-page-meta"]')).toBeVisible();
  await expect(home.locator('[data-slot="home-kpi-strip"] [data-slot="metric"]')).toHaveCount(4);
  await expect(homeMetricValue(home, "Needs you")).toHaveText(
    String(snapshot.http.attention.total)
  );
  await expect(homeMetricValue(home, "Completed today")).toHaveText(
    String(snapshot.http.today.runs_completed + snapshot.http.today.tasks_closed)
  );
  await expect(homeMetricValue(home, "Usage")).toHaveText(
    formatTokenCount(snapshot.http.usage.total_tokens)
  );
  await expect
    .poll(async () => Number(await homeMetricValue(home, "Working now").textContent()))
    .toBeGreaterThan(0);
  await expect(home.locator('[data-slot="home-run-card"]')).toBeVisible();
  await expect(home.locator('[data-slot="home-agent-row"]')).toHaveCount(
    snapshot.catalog.agents.length
  );
  await expect(
    home.locator('[data-slot="home-agent-row"]').filter({ hasText: homeAgentAlpha })
  ).toBeVisible();
  await expect(home.locator('[data-slot="home-pulse"]')).toBeVisible();
  await expect(home.locator('[data-slot="home-outcomes"]')).toBeVisible();
  await expect(home.locator('[data-slot="home-usage"]')).toBeVisible();
  await expect(home.locator('[data-slot="home-activity"]')).toBeVisible();

  assertOverviewParity(snapshot);
  await assertHomeNavigation(appPage, home);
  await assertHomeViewportMatrix(appPage, browserArtifacts, home);
  await assertHomeFocus(appPage);

  await runtime.artifactCollector.captureJSON("browser_api_snapshots", snapshot);
  await browserArtifacts.captureScreenshot("home-truthful", appPage);
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
    home_view_visible: true,
    home_connection_status: "connected",
    home_metric_count: 4,
    home_needs_you_value: String(snapshot.http.attention.total),
    home_completed_today_value: String(
      snapshot.http.today.runs_completed + snapshot.http.today.tasks_closed
    ),
    home_usage_value: formatTokenCount(snapshot.http.usage.total_tokens),
    home_agent_count: snapshot.catalog.agents.length,
  });

  await assertNoSensitiveLeak(appPage, runtime, snapshot);
});

test("Home reports an overview failure without misreporting daemon connectivity", async ({
  page,
  runtime,
}) => {
  await prepareHomeRuntime(runtime);
  await page.route("**/api/observe/overview**", async route => {
    await route.fulfill({
      contentType: "application/json",
      status: 503,
      body: JSON.stringify({ error: "overview unavailable" }),
    });
  });

  await page.goto(runtime.url("/"), { waitUntil: "domcontentloaded" });
  await useGlobalWorkspaceIfPrompted(workspaceShell(page));
  const home = await ensureAppWindow(page, "Home", "dashboard");

  await expect(home.getByTestId("home-connection-indicator")).toHaveAttribute(
    "data-status",
    "connected"
  );
  await expect(home.getByRole("heading", { name: "Unable to load the home overview" })).toBeVisible(
    { timeout: 20_000 }
  );
  await expect(home.locator('[data-slot="home-kpi-strip"]')).toHaveCount(0);
});

test("Home preserves its loaded overview and recovers when health requests resume", async ({
  page,
  runtime,
}) => {
  await prepareHomeRuntime(runtime);
  await page.goto(runtime.url("/"), { waitUntil: "domcontentloaded" });
  await useGlobalWorkspaceIfPrompted(workspaceShell(page));
  const home = await ensureAppWindow(page, "Home", "dashboard");
  await expect(home.getByTestId("home-connection-indicator")).toHaveAttribute(
    "data-status",
    "connected"
  );

  await page.route("**/api/status", async route => {
    await route.abort("failed");
  });

  await expect(home.getByTestId("home-connection-indicator")).toHaveAttribute(
    "data-status",
    /reconnecting|disconnected|error/,
    { timeout: 15_000 }
  );
  await expect(home.getByTestId("home-body")).toBeVisible();
  await expect(home.locator('[data-slot="home-kpi-strip"] [data-slot="metric"]')).toHaveCount(4);

  await page.unroute("**/api/status");
  await expect(home.getByTestId("home-connection-indicator")).toHaveAttribute(
    "data-status",
    "connected",
    { timeout: 20_000 }
  );
  await expect(home.getByTestId("home-body")).toBeVisible();
});

test("Home recovers after a daemon restart without retaining a stale overview", async ({
  appPage,
  browserArtifacts,
  runtime,
}) => {
  await prepareHomeRuntime(runtime);
  await useGlobalWorkspaceIfPrompted(workspaceShell(appPage));
  await appPage.goto(runtime.url("/"), { waitUntil: "domcontentloaded" });
  const home = await ensureAppWindow(appPage, "Home", "dashboard");
  const beforeRestart = await requestOverview(runtime);

  await expect(homeMetricValue(home, "Needs you")).toHaveText(
    String(beforeRestart.attention.total)
  );
  const restart = await runtime.requestJSON<SettingsRestartAction>(
    "/api/settings/actions/restart",
    { method: "POST", body: "{}" }
  );
  expect(restart.operation_id).toMatch(
    /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i
  );

  await expect
    .poll(async () => await pollRestartStatus(runtime, restart.status_url), { timeout: 45_000 })
    .toBe("ready");
  await reloadDaemonServedPage(appPage, runtime, "/", { readyTestId: "home-body" });
  const afterRestart = await requestOverview(runtime);

  await expect(home.getByTestId("home-connection-indicator")).toHaveAttribute(
    "data-status",
    "connected"
  );
  await expect(homeMetricValue(home, "Needs you")).toHaveText(String(afterRestart.attention.total));
  expect(afterRestart.schema_version).toBe(beforeRestart.schema_version);
  expect(afterRestart.generated_at).not.toBe(beforeRestart.generated_at);
  await browserArtifacts.captureScreenshot("home-restart-ready", appPage);
});

test("Home scope follows the active workspace", async ({ appPage, runtime }) => {
  if (!runtime.paths?.homeDir) {
    throw new Error("Home workspace switching requires launch-mode runtime paths.");
  }

  const alpha = await prepareHomeRuntime(runtime);
  const betaRoot = await mkdtemp(path.join(os.tmpdir(), "agh-home-workspace-beta-"));
  const beta = await runtime.resolveWorkspace(betaRoot);

  await useGlobalWorkspaceIfPrompted(workspaceShell(appPage));
  await appPage.goto(runtime.url("/"), { waitUntil: "domcontentloaded" });
  let home = await ensureAppWindow(appPage, "Home", "dashboard");
  await switchWorkspace(appPage, beta.id, beta.name);
  home = await ensureAppWindow(appPage, "Home", "dashboard");

  await expect(home.locator('[data-slot="home-page-meta"]')).toContainText(
    `workspace ${beta.name}`
  );
  await expect(homeMetricValue(home, "Working now")).toHaveText("0");

  await switchWorkspace(appPage, alpha.id, alpha.name);
  home = await ensureAppWindow(appPage, "Home", "dashboard");
  await expect(home.locator('[data-slot="home-page-meta"]')).not.toContainText("workspace");
  await expect
    .poll(async () => Number(await homeMetricValue(home, "Working now").textContent()))
    .toBeGreaterThan(0);
});

async function prepareHomeRuntime(runtime: BrowserRuntime): Promise<WorkspacePayload> {
  if (!runtime.paths?.homeDir) {
    throw new Error("Home E2E requires launch-mode runtime paths.");
  }

  const workspace = await runtime.resolveWorkspace(runtime.paths.homeDir);
  const sessions = await runtime.requestJSON<{ sessions: SessionSummary[] }>(
    `/api/sessions?workspace=${encodeURIComponent(workspace.id)}`
  );
  const pending = sessions.sessions.find(
    session =>
      session.workspace_id === workspace.id &&
      (session.state === "active" || session.state === "starting")
  );
  const sessionId =
    pending?.id ??
    (
      await runtime.requestJSON<{ session: SessionSummary }>("/api/sessions", {
        method: "POST",
        body: JSON.stringify({ agent_name: homeAgentAlpha, workspace: workspace.id }),
      })
    ).session.id;
  await waitForSeedSessionActive(runtime, sessionId);
  return workspace;
}

async function captureHomeSnapshot(
  runtime: BrowserRuntime,
  workspace: WorkspacePayload
): Promise<HomeSnapshot> {
  const [http, uds, cli, catalog] = await Promise.all([
    requestOverview(runtime),
    runtime.requestOperatorJSON?.<{ overview: HomeOverview }>(overviewPath()),
    captureCLIOverview(runtime),
    runtime.requestJSON<HomeAgentCatalog>(
      `/api/agents/catalog?workspace=${encodeURIComponent(workspace.id)}&limit=6`
    ),
  ]);
  return { catalog, http, uds: uds?.overview, cli, workspace };
}

function overviewPath(workspace?: string): string {
  const search = new URLSearchParams({ usage_window: "30" });
  if (workspace) search.set("workspace", workspace);
  return `/api/observe/overview?${search.toString()}`;
}

async function requestOverview(runtime: BrowserRuntime, workspace?: string): Promise<HomeOverview> {
  const response = await runtime.requestJSON<{ overview: HomeOverview }>(overviewPath(workspace));
  return response.overview;
}

async function captureCLIOverview(runtime: BrowserRuntime): Promise<HomeOverview | undefined> {
  if (!runtime.paths) return undefined;
  return await runCLIJSON<HomeOverview>(runtime, [
    "observe",
    "overview",
    "--usage-window",
    "30",
    "-o",
    "json",
  ]);
}

async function runCLIJSON<T>(runtime: BrowserRuntime, args: string[]): Promise<T> {
  if (!runtime.paths) {
    throw new Error(`CLI snapshot ${args.join(" ")} requires launch-mode runtime paths.`);
  }
  const { stdout } = await execFileAsync(runtime.paths.cliShim, args, {
    env: { ...process.env, AGH_HOME: runtime.paths.homeDir, HOME: runtime.paths.homeDir },
    maxBuffer: 10 * 1024 * 1024,
  });
  return JSON.parse(stdout) as T;
}

function assertOverviewParity(snapshot: HomeSnapshot): void {
  const expected = comparableOverview(snapshot.http);
  if (snapshot.uds) expect(comparableOverview(snapshot.uds)).toEqual(expected);
  if (snapshot.cli) expect(comparableOverview(snapshot.cli)).toEqual(expected);
}

function comparableOverview(overview: HomeOverview) {
  return {
    schema_version: overview.schema_version,
    attention: overview.attention,
    today: overview.today,
    outcomes: overview.outcomes,
    usage: overview.usage,
    pulse: overview.pulse,
    network: overview.network,
    system: overview.system,
  };
}

function homeMetric(root: Locator, label: string): Locator {
  return root
    .locator('[data-slot="home-kpi-strip"] [data-slot="metric"]')
    .filter({ hasText: label })
    .first();
}

function homeMetricValue(root: Locator, label: string): Locator {
  return homeMetric(root, label).locator('[data-slot="metric-value"]');
}

function formatTokenCount(total: number): string {
  if (total >= 1_000_000) {
    const millions = total / 1_000_000;
    return `${Number.isInteger(millions) ? millions : millions.toFixed(1)}M`;
  }
  if (total >= 1_000) return `${Math.round(total / 1_000)}K`;
  return String(total);
}

async function pollRestartStatus(runtime: BrowserRuntime, statusURL: string): Promise<string> {
  try {
    return (await runtime.requestJSON<SettingsRestartStatus>(statusURL)).status;
  } catch {
    // Transport loss is the observable restart state until the operation endpoint returns.
    return "restarting";
  }
}

function workspaceShell(page: Page) {
  return {
    osDesktop: page.getByTestId(sessionLifecycleTestIds.osDesktop),
    workspaceOnboarding: page.getByTestId(sessionLifecycleTestIds.workspaceOnboarding),
    workspaceUseGlobal: page.getByTestId(sessionLifecycleTestIds.workspaceUseGlobal),
  };
}

async function assertHomeNavigation(page: Page, home: Locator): Promise<void> {
  await home.getByRole("link", { name: /Needs you/i }).click();
  await expect.poll(() => new URL(page.url()).pathname).toBe("/tasks");
  await expect.poll(() => new URL(page.url()).searchParams.get("mode")).toBe("inbox");

  const focusedHome = await openAppWindow(page, "Home", "dashboard");
  await focusedHome
    .locator('[data-slot="home-agents"]')
    .getByRole("link", { name: new RegExp(homeAgentAlpha) })
    .click();
  await expect.poll(() => new URL(page.url()).pathname).toBe(`/agents/${homeAgentAlpha}`);

  const returnedHome = await openAppWindow(page, "Home", "dashboard");
  await expect(returnedHome.getByTestId("home-body")).toBeVisible();
}

async function assertHomeViewportMatrix(
  page: Page,
  browserArtifacts: {
    captureScreenshot(name?: string, page?: Page): Promise<unknown>;
  },
  home: Locator
): Promise<void> {
  const viewports = [
    { width: 375, height: 812, name: "mobile" },
    { width: 768, height: 1024, name: "tablet" },
    { width: 1280, height: 900, name: "desktop" },
  ];

  for (const viewport of viewports) {
    await page.setViewportSize({ width: viewport.width, height: viewport.height });
    await expect(home.getByTestId("home-body")).toBeVisible();
    await expect(home.locator('[data-slot="home-kpi-strip"] [data-slot="metric"]')).toHaveCount(4);
    await expect(home.locator('[data-slot="home-run-card"]').first()).toBeVisible();
    await expect(home.locator('[data-slot="home-agents"]')).toBeVisible();
    await browserArtifacts.captureScreenshot(`home-${viewport.name}`, page);
  }
}

async function assertHomeFocus(page: Page): Promise<void> {
  const homeLauncher = page.getByRole("button", { name: "Home", exact: true });
  await expect(homeLauncher).toHaveAccessibleName("Home");
  await homeLauncher.focus();
  await expect(homeLauncher).toBeFocused();
}

async function assertNoSensitiveLeak(
  page: Page,
  runtime: BrowserRuntime,
  snapshot: HomeSnapshot
): Promise<void> {
  await expect(page.locator("body")).not.toContainText(sensitivePattern);
  const artifactPayloads = [
    JSON.stringify(snapshot),
    await readFile(runtime.artifactCollector.artifactPath("browser_console"), "utf8"),
    await readFile(runtime.artifactCollector.artifactPath("browser_network"), "utf8"),
    await readFile(runtime.artifactCollector.artifactPath("browser_route_state"), "utf8"),
    await readFile(runtime.artifactCollector.artifactPath("browser_api_snapshots"), "utf8"),
  ];
  for (const payload of artifactPayloads) expect(payload).not.toMatch(sensitivePattern);
  if (runtime.paths?.daemonLog) {
    expect(await readFile(runtime.paths.daemonLog, "utf8")).not.toMatch(sensitivePattern);
  }
}
