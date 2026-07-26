import { execFile } from "node:child_process";
import { mkdir, mkdtemp, readFile, writeFile } from "node:fs/promises";
import os from "node:os";
import path from "node:path";
import process from "node:process";
import { fileURLToPath } from "node:url";
import { promisify } from "node:util";

import { openAppWindow, sessionWindow, switchWorkspace } from "../fixtures/os-navigation";
import { waitForSeedSessionActive } from "../fixtures/runtime";
import {
  SESSION_CREATE_FIRST_MESSAGE,
  sessionLifecycleSelectors,
  sessionWindowSelectors,
} from "../fixtures/selectors";
import { expect, test } from "../fixtures/test";
import { useGlobalWorkspaceIfPrompted } from "../fixtures/workspace";

const execFileAsync = promisify(execFile);

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

const browserLifecycleAgent = "browser-lifecycle-agent";
const mockAgentProvider = "acpmock";
const catalogModel = "qa-browser-model-alt";
const catalogModelLabel = "QA Browser Model Alt";
const overrideProvider = "qa-browser-override";
const driftedDefaultProvider = "gemini";

function browserLifecycleSessionPath(sessionId: string): string {
  return `/agents/${browserLifecycleAgent}/sessions/${sessionId}`;
}

interface SessionPayload {
  id: string;
  agent_name: string;
  provider: string;
  model?: string | null;
  reasoning_effort?: string | null;
  state: string;
  workspace_id: string;
}

interface SessionEnvelope {
  session: SessionPayload;
}

interface SessionListEnvelope {
  sessions: SessionPayload[];
}

interface WorkspaceDetailPayload {
  id: string;
  providers: Array<{ name: string }>;
}

test.use({
  runtimeOptions: {
    env: { PATH: ["/usr/bin", "/bin"].join(path.delimiter) },
    modelsDevEnabled: false,
    seed: {
      mockAgents: [
        {
          fixturePath: browserLifecycleFixture,
          fixtureAgent: browserLifecycleAgent,
        },
      ],
    },
  },
});

test("operator can create a provider/model override session and attach without losing provider truth", async ({
  appPage,
  browserArtifacts,
  runtime,
}) => {
  if (!runtime.paths) {
    throw new Error("provider override browser test requires launch-mode runtime paths");
  }

  const ui = sessionLifecycleSelectors(appPage);
  const workspaceRoot = await mkdtemp(path.join(os.tmpdir(), "agh-provider-override-workspace-"));
  const overrideCommand = await readAgentCommand(runtime.paths.homeDir, browserLifecycleAgent);

  await writeWorkspaceConfig({
    rootDir: workspaceRoot,
    defaultProvider: mockAgentProvider,
    overrideCommand,
    includeOverride: true,
  });

  const workspace = await runtime.resolveWorkspace(workspaceRoot);
  const workspaceDetail = await runtime.requestJSON<WorkspaceDetailPayload>(
    `/api/workspaces/${encodeURIComponent(workspace.id)}`
  );

  await appPage.goto(runtime.url("/"), { waitUntil: "domcontentloaded" });
  await useGlobalWorkspaceIfPrompted(ui);
  await expect(ui.osDesktop).toBeVisible();
  await switchWorkspace(appPage, workspace.id, workspace.name);
  const agentsWin = await openAppWindow(appPage, "Agents", "agents");
  const fleet = sessionLifecycleSelectors(agentsWin);
  await expect.poll(() => new URL(appPage.url()).pathname).toBe("/agents");
  await expect(fleet.agentRow(browserLifecycleAgent)).toBeVisible();

  await fleet.agentRow(browserLifecycleAgent).click();
  await expect.poll(() => new URL(appPage.url()).pathname).toBe(`/agents/${browserLifecycleAgent}`);
  await expect(fleet.agentPageNewSession).toBeVisible();
  await fleet.agentPageNewSession.click();

  await expect(appPage.getByTestId("session-create-dialog")).toBeVisible();
  await expect(appPage.getByTestId("session-create-agent-select")).toContainText(
    browserLifecycleAgent
  );
  // The unified runtime selector trigger shows the agent's default provider truth.
  const runtimeTrigger = appPage.getByTestId("session-create-runtime-select");
  await expect(runtimeTrigger).toContainText("ACP Mock");

  // Open the popup; the provider rail is a LOCAL filter listing every workspace provider.
  await runtimeTrigger.click();
  await expect(appPage.getByTestId("runtime-selector-popup")).toBeVisible();
  const railProviders = await appPage
    .locator('[role="radiogroup"] [data-rail]')
    .evaluateAll(items =>
      items
        .map(item => item.getAttribute("data-rail"))
        .filter((value): value is string => Boolean(value) && value !== "all" && value !== "fav")
        .sort()
    );
  expect(railProviders).toEqual(workspaceDetail.providers.map(provider => provider.name).sort());

  await browserArtifacts.captureScreenshot("session-provider-dialog-desktop", appPage);
  await appPage.setViewportSize({ width: 375, height: 812 });
  await expect(appPage.getByTestId("runtime-selector-popup")).toBeVisible();
  await browserArtifacts.captureScreenshot("session-provider-dialog-mobile", appPage);
  await appPage.setViewportSize({ width: 1280, height: 800 });

  // Filter the list to the override provider, then refresh the AGGREGATE catalog
  // (the selector owns one truthful global refresh, not a per-provider fan-out).
  await appPage.locator(`[data-rail="${overrideProvider}"]`).click();
  const catalogRefreshResponse = appPage.waitForResponse(
    response =>
      response.request().method() === "POST" &&
      response.url().endsWith(`/api/model-catalog/models/refresh`)
  );
  const refreshCatalog = appPage.getByTestId("runtime-selector-refresh");
  await expect(refreshCatalog).toBeEnabled();
  await refreshCatalog.click();
  expect((await catalogRefreshResponse).ok()).toBe(true);

  // The workspace override provider has no global-catalog rows → enter a custom id
  // scoped to the active (override) provider tuple, then close the popup.
  await appPage.getByTestId("runtime-selector-search").fill("qa-browser-model");
  await expect(appPage.getByTestId("runtime-selector-custom")).toBeVisible();
  await appPage.getByTestId("runtime-selector-custom").click();
  await appPage.keyboard.press("Escape");
  await expect(appPage.getByTestId("runtime-selector-popup")).toHaveCount(0);
  await expect(runtimeTrigger).toContainText("qa-browser-model");
  // The custom id is unknown to the catalog, so no reasoning segment is offered.
  await expect(runtimeTrigger).not.toContainText(/Low|Medium|High/);

  const createRequestPromise = appPage.waitForRequest(
    request => request.method() === "POST" && request.url().endsWith("/api/sessions")
  );
  const createResponsePromise = appPage.waitForResponse(
    response => response.request().method() === "POST" && response.url().endsWith("/api/sessions")
  );

  await appPage.getByTestId("session-create-prompt").fill(SESSION_CREATE_FIRST_MESSAGE);
  await appPage.getByTestId("session-create-send").click();

  const createRequest = await createRequestPromise;
  const createResponse = await createResponsePromise;
  const createRequestBody = createRequest.postDataJSON() as {
    agent_name?: string;
    model?: string;
    provider?: string;
    reasoning_effort?: string;
    workspace?: string;
  };
  expect(createRequestBody).toMatchObject({
    agent_name: browserLifecycleAgent,
    model: "qa-browser-model",
    provider: overrideProvider,
    workspace: workspace.id,
  });
  expect(createRequestBody).not.toHaveProperty("reasoning_effort");
  expect(createResponse.ok()).toBeTruthy();

  const createdSession = (await createResponse.json()) as SessionEnvelope;
  expect(createdSession.session.provider).toBe(overrideProvider);
  await waitForSeedSessionActive(runtime, createdSession.session.id);

  await expect
    .poll(() => new URL(appPage.url()).pathname)
    .toBe(browserLifecycleSessionPath(createdSession.session.id));
  const sessionWin = sessionWindow(appPage, createdSession.session.id);
  const sessionUi = sessionWindowSelectors(sessionWin, appPage);
  await expect(sessionWin).toBeVisible();
  await expect(sessionWin.getByTestId("session-status-provider")).toHaveText(overrideProvider);
  await browserArtifacts.captureScreenshot("session-provider-created", appPage);

  await assertSessionParity(
    runtime,
    createdSession.session.workspace_id,
    createdSession.session.id,
    overrideProvider
  );

  await writeWorkspaceConfig({
    rootDir: workspaceRoot,
    defaultProvider: driftedDefaultProvider,
    overrideCommand,
    includeOverride: true,
  });

  const attachResponsePromise = appPage.waitForResponse(
    response =>
      response.request().method() === "POST" &&
      response
        .url()
        .endsWith(
          sessionAPIPath(createdSession.session.workspace_id, createdSession.session.id, "/attach")
        )
  );

  await expect(sessionUi.resumeButton).toBeVisible();
  await sessionUi.resumeButton.click();
  expect((await attachResponsePromise).ok()).toBe(true);
  await expect(sessionUi.stopButton).toBeVisible();
  await expect(sessionWin.getByTestId("session-status-provider")).toHaveText(overrideProvider);
  await assertSessionParity(
    runtime,
    createdSession.session.workspace_id,
    createdSession.session.id,
    overrideProvider
  );

  await sessionUi.stopButton.click();
  await expect(sessionUi.resumeButton).not.toBeVisible();
});

test("operator persists an advertised model and non-empty reasoning effort on the created session", async ({
  appPage,
  runtime,
}) => {
  const ui = sessionLifecycleSelectors(appPage);

  await useGlobalWorkspaceIfPrompted(ui);
  await expect(ui.osDesktop).toBeVisible();
  const agentsWin = await openAppWindow(appPage, "Agents", "agents");
  const fleet = sessionLifecycleSelectors(agentsWin);
  await expect.poll(() => new URL(appPage.url()).pathname).toBe("/agents");
  await expect(fleet.agentRow(browserLifecycleAgent)).toBeVisible();

  await fleet.agentRow(browserLifecycleAgent).click();
  await expect.poll(() => new URL(appPage.url()).pathname).toBe(`/agents/${browserLifecycleAgent}`);
  await fleet.agentPageNewSession.click();
  await expect(appPage.getByTestId("session-create-dialog")).toBeVisible();

  const runtimeTrigger = appPage.getByTestId("session-create-runtime-select");
  await runtimeTrigger.click();
  await expect(appPage.getByTestId("runtime-selector-popup")).toBeVisible();

  const catalogRow = appPage.locator(
    `[data-provider="${mockAgentProvider}"][data-model="${catalogModel}"]`
  );
  await expect(catalogRow).toBeVisible();
  await catalogRow.click();

  const reasoningStrip = appPage.getByTestId("runtime-selector-reasoning");
  await expect(reasoningStrip).toHaveAttribute("data-reasoning-mode", "levels");
  await reasoningStrip.locator('button[data-rz="high"]').click();
  await expect(reasoningStrip.locator('button[data-rz="high"]')).toHaveAttribute("data-on", "true");

  await appPage.keyboard.press("Escape");
  await expect(appPage.getByTestId("runtime-selector-popup")).toHaveCount(0);
  await expect(runtimeTrigger).toContainText(catalogModelLabel);
  await expect(runtimeTrigger).toContainText("High");

  const createRequestPromise = appPage.waitForRequest(
    request => request.method() === "POST" && request.url().endsWith("/api/sessions")
  );
  const createResponsePromise = appPage.waitForResponse(
    response => response.request().method() === "POST" && response.url().endsWith("/api/sessions")
  );
  await appPage.getByTestId("session-create-prompt").fill(SESSION_CREATE_FIRST_MESSAGE);
  await appPage.getByTestId("session-create-send").click();

  const createRequest = await createRequestPromise;
  const createResponse = await createResponsePromise;
  expect(createRequest.postDataJSON()).toMatchObject({
    agent_name: browserLifecycleAgent,
    provider: mockAgentProvider,
    model: catalogModel,
    reasoning_effort: "high",
  });
  expect(createResponse.ok()).toBe(true);

  const created = (await createResponse.json()) as SessionEnvelope;
  expect(created.session).toMatchObject({
    provider: mockAgentProvider,
    model: catalogModel,
    reasoning_effort: "high",
  });
  await expect
    .poll(() => new URL(appPage.url()).pathname)
    .toBe(browserLifecycleSessionPath(created.session.id));
  const sessionWin = sessionWindow(appPage, created.session.id);
  await expect(sessionWin).toBeVisible();

  const rehydrated = await runtime.requestJSON<SessionEnvelope>(
    sessionAPIPath(created.session.workspace_id, created.session.id)
  );
  expect(rehydrated.session).toMatchObject({
    provider: mockAgentProvider,
    model: catalogModel,
    reasoning_effort: "high",
  });
});

async function assertSessionParity(
  runtime: {
    requestJSON: <T>(pathname: string, init?: RequestInit) => Promise<T>;
    requestOperatorJSON?: <T>(pathname: string, init?: RequestInit) => Promise<T>;
    paths?: { cliShim: string; homeDir: string };
  },
  workspaceID: string,
  sessionID: string,
  expectedProvider: string
): Promise<void> {
  const path = sessionAPIPath(workspaceID, sessionID);
  const httpRecord = await runtime.requestJSON<SessionEnvelope>(path);
  expect(httpRecord.session.provider).toBe(expectedProvider);

  if (!runtime.requestOperatorJSON) {
    throw new Error("provider override parity check requires operator UDS access");
  }
  const udsRecord = await runtime.requestOperatorJSON<SessionEnvelope>(path);
  expect(udsRecord.session.provider).toBe(expectedProvider);

  if (!runtime.paths) {
    throw new Error("provider override parity check requires runtime CLI paths");
  }
  const { stdout } = await execFileAsync(
    runtime.paths.cliShim,
    ["session", "list", "--all", "-o", "json"],
    {
      env: cliEnv(runtime.paths),
    }
  );
  const cliRecords = (JSON.parse(stdout) as SessionListEnvelope).sessions;
  const cliRecord = cliRecords.find(session => session.id === sessionID);
  expect(cliRecord?.provider).toBe(expectedProvider);
}

function sessionAPIPath(workspaceID: string, sessionID: string, suffix = ""): string {
  return `/api/workspaces/${encodeURIComponent(workspaceID)}/sessions/${encodeURIComponent(
    sessionID
  )}${suffix}`;
}

function cliEnv(paths: { cliShim: string; homeDir: string }): NodeJS.ProcessEnv {
  return {
    AGH_HOME: paths.homeDir,
    HOME: paths.homeDir,
    PATH: `${path.dirname(paths.cliShim)}:${process.env.PATH ?? ""}`,
  };
}

async function readAgentCommand(homeDir: string, agentName: string): Promise<string> {
  const agentDefPath = path.join(homeDir, "agents", agentName, "AGENT.md");
  const agentDef = await readFile(agentDefPath, "utf8");
  const match = agentDef.match(/^command:\s+(.+)$/m);
  if (!match) {
    throw new Error(`agent definition ${agentDefPath} is missing a command line`);
  }
  return match[1].trim();
}

async function writeWorkspaceConfig(input: {
  rootDir: string;
  defaultProvider: string;
  overrideCommand: string;
  includeOverride: boolean;
}): Promise<void> {
  const configDir = path.join(input.rootDir, ".agh");
  const configPath = path.join(configDir, "config.toml");

  await mkdir(configDir, { recursive: true });

  const lines = [
    "[defaults]",
    `agent = "${browserLifecycleAgent}"`,
    `provider = "${input.defaultProvider}"`,
    "",
  ];

  if (input.includeOverride) {
    lines.push(
      `[providers.${overrideProvider}]`,
      `command = "${escapeTomlString(input.overrideCommand)}"`,
      `display_name = "QA Browser Override"`,
      `harness = "acp"`,
      `auth_mode = "none"`,
      `none_security = "local_transport"`,
      `[providers.${overrideProvider}.models]`,
      `default = "qa-browser-model"`,
      `[providers.${overrideProvider}.models.reasoning]`,
      `apply = "acp_option"`,
      `[[providers.${overrideProvider}.models.curated]]`,
      `id = "qa-browser-model"`,
      `display_name = "QA Browser Model"`,
      `supports_reasoning = true`,
      `reasoning_efforts = ["low", "medium", "high"]`,
      `default_reasoning_effort = "medium"`,
      ""
    );
  }

  await writeFile(configPath, `${lines.join("\n")}\n`, "utf8");
}

function escapeTomlString(value: string): string {
  return value.replaceAll("\\", "\\\\").replaceAll('"', '\\"');
}
