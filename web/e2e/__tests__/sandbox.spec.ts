import { execFile } from "node:child_process";
import { mkdtemp, readFile, writeFile } from "node:fs/promises";
import os from "node:os";
import path from "node:path";
import process from "node:process";
import { fileURLToPath } from "node:url";
import { promisify } from "node:util";

import type { Locator, Page } from "@playwright/test";

import { sessionWindow, switchWorkspace } from "../fixtures/os-navigation";
import {
  sandboxOperatorSelectors,
  sessionLifecycleSelectors,
  sessionWindowSelectors,
} from "../fixtures/selectors";
import type { BrowserRuntime } from "../fixtures/runtime";
import { seedBrowserSandboxProfiles } from "../fixtures/runtime";
import { expect, test } from "../fixtures/test";
import { useGlobalWorkspaceIfPrompted } from "../fixtures/workspace";

const execFileAsync = promisify(execFile);
const sandboxFixture = path.resolve(
  path.dirname(fileURLToPath(import.meta.url)),
  "..",
  "..",
  "..",
  "internal",
  "testutil",
  "acpmock",
  "testdata",
  "browser_sandbox_fixture.json"
);
const allowedAgent = "browser-sandbox-allowed";
const blockedAgent = "browser-sandbox-blocked";
const sandboxProfileName = "browser-local-sandbox";
const sensitivePattern =
  /agh_claim_|["']claim_token["']\s*:|mcp[_-]?auth|telegram-bot-token|pkce|oauth|webhook_secret|provider[_-]?credentials?["'\s]*[:=]|DAYTONA_API_KEY|sandbox-secret/i;

interface SettingsSandboxProfile {
  backend: string;
  sync_mode?: string;
  persistence?: string;
  runtime_root?: string;
}

interface SettingsSandboxEntry {
  name: string;
  profile: SettingsSandboxProfile;
  workspace_usage_count: number;
}

interface SettingsSandboxesResponse {
  sandboxes: SettingsSandboxEntry[];
}

interface SettingsRestartAction {
  operation_id: string;
  status_url: string;
}

interface WorkspacePayload {
  id: string;
  name: string;
  root_dir: string;
  sandbox_ref?: string;
}

interface WorkspaceEnvelope {
  workspace: WorkspacePayload;
}

interface SessionSandboxPayload {
  backend?: string;
  profile?: string;
  sandbox_id?: string;
  state?: string;
}

interface SessionPayload {
  id: string;
  agent_name: string;
  state: string;
  workspace_id: string;
  sandbox?: SessionSandboxPayload | null;
}

interface SessionEnvelope {
  session: SessionPayload;
}

interface SessionEventsEnvelope {
  events: unknown[];
}

interface ConfigValueRecord {
  path: string;
  value: unknown;
  redacted: boolean;
}

test.use({
  runtimeOptions: {
    seed: {
      mockAgents: [
        {
          fixturePath: sandboxFixture,
          fixtureAgent: allowedAgent,
        },
        {
          fixturePath: sandboxFixture,
          fixtureAgent: blockedAgent,
        },
      ],
    },
  },
});

test("operator manages a local sandbox profile and binds it to real session execution", async ({
  appPage,
  browserArtifacts,
  runtime,
}) => {
  test.setTimeout(180_000);

  await assertLaunchRuntime(runtime);
  await useGlobalWorkspaceIfPrompted(sessionLifecycleSelectors(appPage));

  await appPage.goto(runtime.url("/sandbox"), { waitUntil: "domcontentloaded" });
  const sandboxWin = appPage.getByTestId("os-window-app:sandbox");
  await expect(sandboxWin).toBeVisible({ timeout: 20_000 });
  const sandboxUI = sandboxOperatorSelectors(sandboxWin);
  await expect(sandboxUI.shell).toBeVisible({ timeout: 20_000 });
  await createSandboxProfileThroughUI(sandboxWin, sandboxProfileName);
  await expect(sandboxUI.profile(sandboxProfileName)).toBeVisible();
  await expect(sandboxUI.profile(sandboxProfileName)).toContainText("local");
  await expect(sandboxUI.profileMetadata(sandboxProfileName)).toContainText("reuse");
  await expect(sandboxUI.profileSource(sandboxProfileName)).toContainText("CONFIG");
  await expect(sandboxUI.actionResult).toContainText(`Saved sandbox "${sandboxProfileName}"`);

  await assertDuplicateNameValidation(sandboxWin, sandboxProfileName);
  await expect(
    runtime.requestJSON<unknown>(
      `/api/settings/sandboxes/${encodeURIComponent("browser-invalid-sandbox")}`,
      {
        method: "PUT",
        body: JSON.stringify({
          profile: {
            backend: "invalid-backend",
          },
        }),
      }
    )
  ).rejects.toThrow("400");

  const restart = await runtime.requestJSON<SettingsRestartAction>(
    "/api/settings/actions/restart",
    { method: "POST" }
  );
  expect(restart.operation_id).toMatch(
    /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i
  );
  await expect
    .poll(async () => await pollRestartStatus(runtime, restart.status_url), { timeout: 45_000 })
    .toBe("ready");

  await appPage.goto(runtime.url("/sandbox"), { waitUntil: "domcontentloaded" });
  await expect(sandboxUI.profile(sandboxProfileName)).toBeVisible();

  const workspaceRoot = await mkdtemp(path.join(os.tmpdir(), "agh-browser-sandbox-workspace-"));
  await writeFile(
    path.join(workspaceRoot, "browser-allowed-source.txt"),
    "sandbox-browser-allowed"
  );
  const workspace = await runtime.resolveWorkspace(workspaceRoot);
  const updatedWorkspace = await setWorkspaceSandbox(runtime, workspace.id, sandboxProfileName);
  expect(updatedWorkspace.sandbox_ref).toBe(sandboxProfileName);

  await appPage.reload({ waitUntil: "domcontentloaded" });
  await expect(sandboxUI.profileUsage(sandboxProfileName)).toContainText(/1\s*workspace/);
  await sandboxUI.deleteProfile(sandboxProfileName).click();
  await expect(sandboxUI.deleteDialog).toBeVisible();
  await expect(sandboxUI.deleteUsage).toContainText("1 workspace currently reference this profile");
  await sandboxWin.getByTestId("settings-sandboxes-delete-cancel").click();

  const session = await createSession(runtime, allowedAgent, workspace.id);
  // The session lives in this freshly resolved workspace, while the active
  // workspace is still the global one. The session route redirects away when
  // the active workspace does not own the session, so activate the session's
  // workspace first, exactly as an operator would before opening it.
  await switchWorkspace(appPage, workspace.id, workspace.name);
  await appPage.goto(runtime.url(sessionPath(allowedAgent, session.id)), {
    waitUntil: "domcontentloaded",
  });
  const allowedSessionWin = sessionWindow(appPage, session.id);
  const sessionUI = sessionWindowSelectors(allowedSessionWin);
  await expect(allowedSessionWin).toBeVisible();
  await sessionUI.composerTextarea.fill("exercise sandbox allowed path");
  await sessionUI.composerTextarea.press("Enter");

  await expect(sessionUI.chatView).toContainText("Sandbox allowed started.");
  await expect(sessionUI.chatView).toContainText("sandbox-browser-allowed", {
    timeout: 30_000,
  });
  await expect(sessionUI.chatView).toContainText("Sandbox allowed complete.");

  const sideEffectPath = path.join(workspaceRoot, "browser-allowed.txt");
  await expect
    .poll(async () => await readFile(sideEffectPath, "utf8"))
    .toBe("sandbox-browser-allowed");
  const metadata = await assertSessionSandboxMetadata(
    runtime,
    workspace.id,
    session.id,
    sandboxProfileName
  );
  expect(metadata.session.sandbox?.backend).toBe("local");
  expect(metadata.session.sandbox?.state).toBe("prepared");

  const cliSession = await sessionViaCLI(runtime, session.id);
  expect(cliSession.sandbox?.profile).toBe(sandboxProfileName);
  expect(cliSession.sandbox?.backend).toBe("local");

  const cliWorkspace = await workspaceViaCLI(runtime, workspace.id);
  expect(cliWorkspace.sandbox_ref).toBe(sandboxProfileName);
  const configBackend = await configValueViaCLI(runtime, `sandboxes.${sandboxProfileName}.backend`);
  expect(configBackend).toMatchObject({
    path: `sandboxes.${sandboxProfileName}.backend`,
    redacted: false,
    value: "local",
  });

  const httpSandboxes =
    await runtime.requestJSON<SettingsSandboxesResponse>("/api/settings/sandboxes");
  const udsSandboxes = await operatorJSON<SettingsSandboxesResponse>(
    runtime,
    "/api/settings/sandboxes"
  );
  expect(sandboxByName(httpSandboxes.sandboxes, sandboxProfileName)?.workspace_usage_count).toBe(1);
  expect(sandboxByName(udsSandboxes.sandboxes, sandboxProfileName)?.profile.backend).toBe("local");

  await appPage.goto(runtime.url("/sandbox"), { waitUntil: "domcontentloaded" });
  await assertSandboxViewports(sandboxWin, browserArtifacts);
  await runtime.artifactCollector.captureJSON("browser_api_snapshots", {
    cli_session: cliSession,
    cli_workspace: cliWorkspace,
    config_backend: configBackend,
    http_sandboxes: httpSandboxes,
    session_metadata: metadata,
    uds_sandboxes: udsSandboxes,
    workspace: updatedWorkspace,
  });
  await browserArtifacts.persist(appPage);
  const routeState = JSON.parse(
    await readFile(runtime.artifactCollector.artifactPath("browser_route_state"), "utf8")
  ) as Record<string, unknown>;
  expect(routeState).toMatchObject({
    sandbox_profile_count: expect.any(Number),
    sandbox_view_visible: true,
  });
  expect(routeState.sandbox_profile_names).toEqual(expect.arrayContaining([sandboxProfileName]));
  await assertNoSensitiveLeak(appPage, runtime);
});

test("operator sees blocked sandbox diagnostics without leaking secrets or writing side effects", async ({
  appPage,
  browserArtifacts,
  runtime,
}) => {
  test.setTimeout(120_000);

  await assertLaunchRuntime(runtime);
  await seedBrowserSandboxProfiles(runtime, [
    {
      name: sandboxProfileName,
      profile: {
        backend: "local",
        persistence: "reuse",
        sync_mode: "none",
      },
    },
  ]);

  const workspaceRoot = await mkdtemp(path.join(os.tmpdir(), "agh-browser-sandbox-blocked-"));
  const workspace = await runtime.resolveWorkspace(workspaceRoot);
  await setWorkspaceSandbox(runtime, workspace.id, sandboxProfileName);
  const session = await createSession(runtime, blockedAgent, workspace.id);

  await appPage.goto(runtime.url(sessionPath(blockedAgent, session.id)), {
    waitUntil: "domcontentloaded",
  });
  await useGlobalWorkspaceIfPrompted(appPage);
  const blockedSessionWin = sessionWindow(appPage, session.id);
  const sessionUI = sessionWindowSelectors(blockedSessionWin);
  await expect(blockedSessionWin).toBeVisible();
  await sessionUI.composerTextarea.fill("exercise sandbox blocked path");
  await sessionUI.composerTextarea.press("Enter");
  await expect(sessionUI.chatView).toContainText("terminal/create denied", {
    timeout: 30_000,
  });

  await expect(
    readFile(path.join(workspaceRoot, "toolhost", "browser-blocked.txt"), "utf8")
  ).rejects.toThrow();
  const metadata = await assertSessionSandboxMetadata(
    runtime,
    workspace.id,
    session.id,
    sandboxProfileName
  );
  expect(metadata.session.sandbox?.state).toBe("prepared");
  const events = await runtime.requestJSON<SessionEventsEnvelope>(
    sessionAPIPath(workspace.id, session.id, "/events")
  );
  expect(JSON.stringify(events)).toContain("browser-sandbox-blocked-command");
  expect(JSON.stringify(events)).toContain("terminal/create");

  await appPage.goto(runtime.url("/sandbox"), { waitUntil: "domcontentloaded" });
  await browserArtifacts.captureScreenshot("sandbox-blocked-diagnostics", appPage);
  await runtime.artifactCollector.captureJSON("browser_api_snapshots", {
    blocked_events: events,
    blocked_session: metadata,
  });
  await browserArtifacts.persist(appPage);
  await assertNoSensitiveLeak(appPage, runtime);
});

async function createSandboxProfileThroughUI(win: Locator, name: string): Promise<void> {
  const ui = sandboxOperatorSelectors(win);
  const page = win.page();
  await ui.createButton.click();
  await expect(ui.editor).toBeVisible();
  await ui.editorNameInput.fill(name);
  await ui.editorBackendInput.selectOption("local");
  await ui.editorSyncModeInput.fill("none");
  await ui.editorPersistenceInput.fill("reuse");
  await ui.editorRuntimeRootInput.fill("");

  const response = page.waitForResponse(
    result =>
      result.request().method() === "PUT" &&
      result.url().endsWith(`/api/settings/sandboxes/${encodeURIComponent(name)}`)
  );
  await ui.editorSave.click();
  expect((await response).ok()).toBe(true);
  await expect(ui.editor).toBeHidden();
}

async function assertDuplicateNameValidation(win: Locator, name: string): Promise<void> {
  const ui = sandboxOperatorSelectors(win);
  await ui.createButton.click();
  await expect(ui.editor).toBeVisible();
  await ui.editorNameInput.fill(name);
  await expect(ui.editorError).toContainText(`A sandbox named "${name}" already exists.`);
  await expect(ui.editorSave).toBeDisabled();
  await win.getByTestId("settings-sandbox-editor-cancel").click();
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
  expect(payload.session.agent_name).toBe(agentName);
  expect(payload.session.workspace_id).toBe(workspaceID);
  return payload.session;
}

async function setWorkspaceSandbox(
  runtime: BrowserRuntime,
  workspaceID: string,
  sandboxRef: string
): Promise<WorkspacePayload> {
  const payload = await runtime.requestJSON<WorkspaceEnvelope>(
    `/api/workspaces/${encodeURIComponent(workspaceID)}`,
    {
      method: "PATCH",
      body: JSON.stringify({ sandbox_ref: sandboxRef }),
    }
  );
  return payload.workspace;
}

async function assertSessionSandboxMetadata(
  runtime: BrowserRuntime,
  workspaceID: string,
  sessionID: string,
  profileName: string
): Promise<SessionEnvelope> {
  const path = sessionAPIPath(workspaceID, sessionID);
  const httpSession = await runtime.requestJSON<SessionEnvelope>(path);
  const udsSession = await operatorJSON<SessionEnvelope>(runtime, path);

  expect(httpSession.session.sandbox).toMatchObject({
    backend: "local",
    profile: profileName,
  });
  expect(httpSession.session.sandbox?.sandbox_id).not.toBe("");
  expect(udsSession.session.sandbox?.backend).toBe(httpSession.session.sandbox?.backend);
  expect(udsSession.session.sandbox?.profile).toBe(httpSession.session.sandbox?.profile);
  expect(udsSession.session.sandbox?.state).toBe(httpSession.session.sandbox?.state);
  return httpSession;
}

function sessionAPIPath(workspaceID: string, sessionID: string, suffix = ""): string {
  return `/api/workspaces/${encodeURIComponent(workspaceID)}/sessions/${encodeURIComponent(
    sessionID
  )}${suffix}`;
}

async function operatorJSON<T>(runtime: BrowserRuntime, pathname: string): Promise<T> {
  if (!runtime.requestOperatorJSON) {
    throw new Error(`operator JSON ${pathname} requires launch-mode UDS access`);
  }
  return await runtime.requestOperatorJSON<T>(pathname);
}

async function sessionViaCLI(runtime: BrowserRuntime, sessionID: string): Promise<SessionPayload> {
  const sessions = (await runCLIJSON(runtime, ["session", "list", "--all", "-o", "json"])) as
    | SessionPayload[]
    | { sessions?: SessionPayload[] };
  const records = Array.isArray(sessions) ? sessions : (sessions.sessions ?? []);
  const found = records.find(session => session.id === sessionID);
  if (!found) {
    throw new Error(`CLI session list did not include ${sessionID}`);
  }
  return found;
}

async function workspaceViaCLI(
  runtime: BrowserRuntime,
  workspaceID: string
): Promise<WorkspacePayload> {
  const payload = (await runCLIJSON(runtime, ["workspace", "info", workspaceID, "-o", "json"])) as
    | WorkspacePayload
    | WorkspaceEnvelope;
  return "workspace" in payload ? payload.workspace : payload;
}

async function configValueViaCLI(
  runtime: BrowserRuntime,
  configPath: string
): Promise<ConfigValueRecord> {
  return (await runCLIJSON(runtime, [
    "config",
    "get",
    configPath,
    "-o",
    "json",
  ])) as ConfigValueRecord;
}

async function runCLIJSON(runtime: BrowserRuntime, args: string[]): Promise<unknown> {
  if (!runtime.paths) {
    throw new Error("sandbox CLI checks require launch-mode runtime paths");
  }
  const { stdout } = await execFileAsync(runtime.paths.cliShim, args, {
    env: cliEnv(runtime.paths),
  });
  return JSON.parse(stdout) as unknown;
}

function cliEnv(paths: { cliShim: string; homeDir: string }): NodeJS.ProcessEnv {
  return {
    ...process.env,
    AGH_HOME: paths.homeDir,
    HOME: paths.homeDir,
    PATH: `${path.dirname(paths.cliShim)}:${process.env.PATH ?? ""}`,
  };
}

async function pollRestartStatus(runtime: BrowserRuntime, statusURL: string): Promise<string> {
  try {
    return (await runtime.requestJSON<{ status: string }>(statusURL)).status;
  } catch {
    return "restarting";
  }
}

async function assertSandboxViewports(
  win: Locator,
  browserArtifacts: { captureScreenshot(name?: string, page?: Page): Promise<string | null> }
): Promise<void> {
  const page = win.page();
  for (const viewport of [
    { width: 1280, height: 900, name: "desktop" },
    { width: 768, height: 900, name: "tablet" },
    { width: 375, height: 812, name: "mobile" },
  ]) {
    await page.setViewportSize({ width: viewport.width, height: viewport.height });
    await expect(win.getByTestId("sandbox-shell")).toBeVisible();
    await expect(page.locator("body")).not.toContainText(sensitivePattern);
    await browserArtifacts.captureScreenshot(`sandbox-${viewport.name}`, page);
  }
}

async function assertLaunchRuntime(runtime: BrowserRuntime): Promise<void> {
  if (!runtime.paths?.homeDir) {
    throw new Error("sandbox E2E requires launch-mode runtime paths");
  }
}

function sandboxByName(
  sandboxes: SettingsSandboxEntry[],
  name: string
): SettingsSandboxEntry | undefined {
  return sandboxes.find(sandbox => sandbox.name === name);
}

function sessionPath(agentName: string, sessionID: string): string {
  return `/agents/${agentName}/sessions/${sessionID}`;
}

async function assertNoSensitiveLeak(page: Page, runtime: BrowserRuntime): Promise<void> {
  await expect(page.locator("body")).not.toContainText(sensitivePattern);
  const artifactPayloads = [
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
