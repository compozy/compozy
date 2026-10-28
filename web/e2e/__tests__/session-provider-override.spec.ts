import { execFile } from "node:child_process";
import { chmod, mkdir, mkdtemp, readFile, writeFile } from "node:fs/promises";
import os from "node:os";
import path from "node:path";
import process from "node:process";
import { fileURLToPath } from "node:url";
import { promisify } from "node:util";

import {
  closeAppWindow,
  openAppWindow,
  sessionWindow,
  switchWorkspace,
} from "../fixtures/os-navigation";
import { reloadDaemonServedPage } from "../fixtures/navigation";
import {
  cleanupBrowserSettingsFixtures,
  seedBrowserSettingsFixtures,
  waitForSeedSessionActive,
} from "../fixtures/runtime";
import {
  SESSION_CREATE_FIRST_MESSAGE,
  sessionLifecycleSelectors,
  sessionWindowSelectors,
} from "../fixtures/selectors";
import { expect, test as base } from "../fixtures/test";
import { ensureProjectWorkspace, completeOnboardingIfPrompted } from "../fixtures/workspace";

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
  profile_name: string;
  name: string;
  agent_name: string;
  runtime: {
    effective?: {
      model?: string | null;
      provider: string;
      reasoning_effort?: string | null;
    } | null;
    status: string;
  };
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

interface ProviderCatalogFixture {
  overrideCommand: string;
}

const test = base.extend<{ providerCatalog: ProviderCatalogFixture }>({
  providerCatalog: async ({ runtime }, use) => {
    if (!runtime.paths) {
      throw new Error("provider override browser test requires launch-mode runtime paths");
    }
    const overrideCommand = await readAgentCommand(runtime.paths.homeDir, browserLifecycleAgent);
    const settingsSeed = await seedBrowserSettingsFixtures(runtime, {
      providers: [
        {
          name: overrideProvider,
          settings: {
            command: overrideCommand,
            display_name: "QA Browser Override",
            harness: "acp",
            auth_mode: "none",
            models: {
              default: "qa-browser-model",
              reasoning: { apply: "acp_option" },
              curated: [
                {
                  id: "qa-browser-model",
                  display_name: "QA Browser Model",
                  supports_reasoning: true,
                  reasoning_efforts: ["low", "medium", "high"],
                  default_reasoning_effort: "medium",
                },
                ...Array.from({ length: 30 }, (_, index) => {
                  const suffix = (index + 1).toString().padStart(2, "0");
                  return {
                    id: `qa-browser-model-${suffix}`,
                    display_name: `QA Browser Model ${suffix}`,
                  };
                }),
              ],
            },
          },
        },
      ],
    });

    try {
      await use({ overrideCommand });
    } finally {
      await cleanupBrowserSettingsFixtures(runtime, settingsSeed);
    }
  },
});

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
  providerCatalog,
  runtime,
}) => {
  if (!runtime.paths) {
    throw new Error("provider override browser test requires launch-mode runtime paths");
  }

  const ui = sessionLifecycleSelectors(appPage);
  const workspaceRoot = await mkdtemp(
    path.join(os.tmpdir(), "compozy-provider-override-workspace-")
  );
  const { overrideCommand } = providerCatalog;
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
  await completeOnboardingIfPrompted(ui);
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
  const createRequestPromise = appPage.waitForRequest(
    request => request.method() === "POST" && new URL(request.url()).pathname === "/api/sessions"
  );
  const createResponsePromise = appPage.waitForResponse(
    response =>
      response.request().method() === "POST" && new URL(response.url()).pathname === "/api/sessions"
  );

  await appPage.getByTestId("session-create-submit").click();

  const createRequest = await createRequestPromise;
  const createResponse = await createResponsePromise;
  expect(createRequest.postDataJSON()).toMatchObject({
    agent_name: browserLifecycleAgent,
    workspace: workspace.id,
  });
  expect(createResponse.ok()).toBeTruthy();

  const createdSession = (await createResponse.json()) as SessionEnvelope;
  expect(createdSession.session.runtime.status).toBe("unbound");

  await expect
    .poll(() => new URL(appPage.url()).pathname)
    .toBe(browserLifecycleSessionPath(createdSession.session.id));
  const sessionWin = sessionWindow(appPage, createdSession.session.id);
  const sessionUi = sessionWindowSelectors(sessionWin, appPage);
  await expect(sessionWin).toBeVisible();
  await sessionUi.sessionRowActions(createdSession.session.id).click();
  await sessionUi.sessionRowMenuItem("rename", createdSession.session.id).click();
  const renameResponsePromise = appPage.waitForResponse(
    response =>
      response.request().method() === "PATCH" &&
      new URL(response.url()).pathname ===
        sessionAPIPath(createdSession.session.workspace_id, createdSession.session.id)
  );
  await appPage.getByTestId("session-rename-name").fill("Provider override review");
  await appPage.getByTestId("session-rename-confirm").click();
  const renameResponse = await renameResponsePromise;
  expect(new URL(renameResponse.url()).searchParams.get("profile")).toBe(
    createdSession.session.profile_name
  );
  expect(renameResponse.ok()).toBe(true);
  await expect(sessionWin.locator('[data-slot="topbar-title"]')).toContainText(
    "Provider override review"
  );
  await assertSessionNameParity(
    runtime,
    createdSession.session.workspace_id,
    createdSession.session.id,
    "Provider override review"
  );
  const runtimeTrigger = sessionWin.getByTestId("session-prompt-runtime-select");
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
  await appPage.locator(`[data-rail="${overrideProvider}"]`).click();
  const selectorScroll = appPage.getByTestId("runtime-selector-scroll");
  await expect
    .poll(() => selectorScroll.evaluate(element => element.scrollHeight > element.clientHeight))
    .toBe(true);
  const documentScrollBefore = await appPage.evaluate(() => document.scrollingElement?.scrollTop);
  await selectorScroll.hover();
  await appPage.mouse.wheel(0, 100_000);
  await expect.poll(() => selectorScroll.evaluate(element => element.scrollTop)).toBeGreaterThan(0);
  await appPage.mouse.wheel(0, 1_000);
  expect(await appPage.evaluate(() => document.scrollingElement?.scrollTop)).toBe(
    documentScrollBefore
  );
  await appPage.getByTestId("runtime-selector-search").fill("qa-browser-model");
  await appPage
    .locator(`[data-provider="${overrideProvider}"][data-model="qa-browser-model"]`)
    .click();
  await appPage.keyboard.press("Escape");
  await sessionUi.composerTextarea.fill(SESSION_CREATE_FIRST_MESSAGE);
  await expect(appPage.getByTestId("runtime-selector-popup")).toBeHidden();
  await expect(sessionUi.composerTextarea).toBeFocused();
  const promptRequestPromise = appPage.waitForRequest(
    request => request.method() === "POST" && request.url().endsWith("/prompt")
  );
  await sessionUi.composerTextarea.press("Enter");
  expect((await promptRequestPromise).postDataJSON()).toMatchObject({
    message: SESSION_CREATE_FIRST_MESSAGE,
    runtime: { model: "qa-browser-model", provider: overrideProvider },
  });
  await waitForSeedSessionActive(runtime, createdSession.session.id);
  // The window topbar spends its status zone on the PR chip; the row mark names the provider.
  await expect(
    sessionWin.getByTestId(`session-provider-mark-${createdSession.session.id}`)
  ).toHaveAttribute("data-kind", overrideProvider);
  await browserArtifacts.captureScreenshot("session-provider-created", appPage);

  await assertSessionParity(
    runtime,
    createdSession.session.workspace_id,
    createdSession.session.id,
    overrideProvider
  );
  const focusedAgentsWin = await openAppWindow(appPage, "Agents", "agents");
  await closeAppWindow(appPage, focusedAgentsWin);

  await writeWorkspaceConfig({
    rootDir: workspaceRoot,
    defaultProvider: driftedDefaultProvider,
    overrideCommand,
    includeOverride: true,
  });

  await expect(sessionUi.stopButton).toBeVisible();
  await expect(
    sessionWin.getByTestId(`session-provider-mark-${createdSession.session.id}`)
  ).toHaveAttribute("data-kind", overrideProvider);
  await assertSessionParity(
    runtime,
    createdSession.session.workspace_id,
    createdSession.session.id,
    overrideProvider
  );

  await sessionUi.stopButton.click();
  await expect(sessionUi.stopButton).not.toBeVisible();
});

test("operator selects an exact AGY catalog model and keeps agy session attribution", async ({
  appPage,
  browserArtifacts,
  providerCatalog,
  runtime,
}) => {
  test.setTimeout(300_000);
  if (!runtime.paths) {
    throw new Error("provider override browser test requires launch-mode runtime paths");
  }

  const agyModel = "qa-browser-model-alt";
  // The overlay points the built-in `agy` at the deterministic peer and curates
  // one exact peer-advertised ID. Provider overlays apply on restart, so the
  // test restarts the daemon once, then drives the real picker-to-session path
  // with no Google account.
  const agySeed = await seedBrowserSettingsFixtures(runtime, {
    providers: [
      {
        name: "agy",
        settings: {
          command: providerCatalog.overrideCommand,
          auth_acp_method: "oauth-personal",
          models: {
            curated: [{ id: agyModel, display_name: "QA Browser Model Alt" }],
          },
        },
      },
    ],
  });
  try {
    const restartResponse = await runtime.requestJSON<{ operation_id: string }>(
      "/api/settings/actions/restart",
      { method: "POST", body: JSON.stringify({}) }
    );
    await expect
      .poll(
        async () => {
          try {
            const payload = await runtime.requestJSON<{ status: string }>(
              `/api/settings/actions/restart/${encodeURIComponent(restartResponse.operation_id)}`
            );
            return payload.status;
          } catch {
            return "unavailable";
          }
        },
        { timeout: 120_000 }
      )
      .toBe("ready");
    await reloadDaemonServedPage(appPage, runtime, "/", {});

    await expect
      .poll(
        async () => {
          const catalog = await runtime.requestJSON<{
            models: Array<{ provider_id: string; model_id: string }>;
          }>("/api/model-catalog/models?view=all&include_stale=true");
          return catalog.models
            .filter(row => row.provider_id === "agy")
            .map(row => row.model_id)
            .join(",");
        },
        { timeout: 60_000 }
      )
      .toContain(agyModel);

    const ui = sessionLifecycleSelectors(appPage);
    const workspaceRoot = await mkdtemp(path.join(os.tmpdir(), "compozy-agy-override-workspace-"));
    await writeWorkspaceConfig({
      rootDir: workspaceRoot,
      defaultProvider: mockAgentProvider,
      overrideCommand: providerCatalog.overrideCommand,
      includeOverride: false,
    });

    const workspace = await runtime.resolveWorkspace(workspaceRoot);

    await appPage.goto(runtime.url("/"), { waitUntil: "domcontentloaded" });
    await completeOnboardingIfPrompted(ui);
    await expect(ui.osDesktop).toBeVisible();
    await switchWorkspace(appPage, workspace.id, workspace.name);
    const agentsWin = await openAppWindow(appPage, "Agents", "agents");
    const fleet = sessionLifecycleSelectors(agentsWin);
    await expect.poll(() => new URL(appPage.url()).pathname).toBe("/agents");
    await expect(fleet.agentRow(browserLifecycleAgent)).toBeVisible();

    await fleet.agentRow(browserLifecycleAgent).click();
    await expect
      .poll(() => new URL(appPage.url()).pathname)
      .toBe(`/agents/${browserLifecycleAgent}`);
    await fleet.agentPageNewSession.click();
    await expect(appPage.getByTestId("session-create-dialog")).toBeVisible();
    const createResponsePromise = appPage.waitForResponse(
      response =>
        response.request().method() === "POST" &&
        new URL(response.url()).pathname === "/api/sessions"
    );
    await appPage.getByTestId("session-create-submit").click();
    const createdSession = (await (await createResponsePromise).json()) as SessionEnvelope;
    expect(createdSession.session.runtime.status).toBe("unbound");
    await expect
      .poll(() => new URL(appPage.url()).pathname)
      .toBe(browserLifecycleSessionPath(createdSession.session.id));
    const sessionWin = sessionWindow(appPage, createdSession.session.id);
    const sessionUi = sessionWindowSelectors(sessionWin, appPage);
    await expect(sessionWin).toBeVisible();

    const runtimeTrigger = sessionWin.getByTestId("session-prompt-runtime-select");
    await runtimeTrigger.click();
    await expect(appPage.getByTestId("runtime-selector-popup")).toBeVisible();
    await appPage.locator('[data-rail="agy"]').click();
    const agyRow = appPage.locator(`[data-provider="agy"][data-model="${agyModel}"]`);
    await expect(agyRow).toBeVisible({ timeout: 30_000 });
    await agyRow.click();
    const reasoningStrip = appPage.getByTestId("runtime-selector-reasoning");
    await expect(reasoningStrip).toHaveAttribute("data-reasoning-mode", "levels");
    const reasoningSlider = reasoningStrip.getByRole("slider");
    await expect(reasoningSlider).toBeEnabled();
    await reasoningSlider.press("Home");
    await expect(reasoningSlider).toHaveAttribute("aria-valuetext", "Low");
    await appPage.keyboard.press("Escape");
    await expect(appPage.getByTestId("runtime-selector-popup")).toBeHidden();

    await sessionUi.composerTextarea.fill(SESSION_CREATE_FIRST_MESSAGE);
    const promptRequestPromise = appPage.waitForRequest(
      request => request.method() === "POST" && request.url().endsWith("/prompt")
    );
    await sessionUi.composerTextarea.press("Enter");
    expect((await promptRequestPromise).postDataJSON()).toMatchObject({
      message: SESSION_CREATE_FIRST_MESSAGE,
      runtime: { model: agyModel, provider: "agy", reasoning_effort: "low" },
    });

    await waitForSeedSessionActive(runtime, createdSession.session.id);
    await expect
      .poll(async () => {
        const rehydrated = await runtime.requestJSON<SessionEnvelope>(
          sessionAPIPath(createdSession.session.workspace_id, createdSession.session.id)
        );
        return rehydrated.session.runtime.effective;
      })
      .toMatchObject({ provider: "agy", model: agyModel, reasoning_effort: "low" });
    await expect(runtimeTrigger).toContainText("QA Browser Model Alt");
    await expect(
      sessionWin.getByTestId(`session-provider-mark-${createdSession.session.id}`)
    ).toHaveAttribute("data-kind", "agy");
    await assertSessionParity(
      runtime,
      createdSession.session.workspace_id,
      createdSession.session.id,
      "agy"
    );
    await browserArtifacts.captureScreenshot("session-provider-agy-attribution", appPage);

    if (await sessionUi.stopButton.isVisible().catch(() => false)) {
      await sessionUi.stopButton.click();
      await expect(sessionUi.stopButton).not.toBeVisible();
    }
  } finally {
    await cleanupBrowserSettingsFixtures(runtime, agySeed);
  }
});

test("operator persists an advertised model and non-empty reasoning effort on the created session", async ({
  appPage,
  runtime,
}) => {
  const ui = sessionLifecycleSelectors(appPage);

  await ensureProjectWorkspace(appPage, runtime);
  await expect(ui.osDesktop).toBeVisible();
  const agentsWin = await openAppWindow(appPage, "Agents", "agents");
  const fleet = sessionLifecycleSelectors(agentsWin);
  await expect.poll(() => new URL(appPage.url()).pathname).toBe("/agents");
  await expect(fleet.agentRow(browserLifecycleAgent)).toBeVisible();

  await fleet.agentRow(browserLifecycleAgent).click();
  await expect.poll(() => new URL(appPage.url()).pathname).toBe(`/agents/${browserLifecycleAgent}`);
  await fleet.agentPageNewSession.click();
  await expect(appPage.getByTestId("session-create-dialog")).toBeVisible();

  const createRequestPromise = appPage.waitForRequest(
    request => request.method() === "POST" && new URL(request.url()).pathname === "/api/sessions"
  );
  const createResponsePromise = appPage.waitForResponse(
    response =>
      response.request().method() === "POST" && new URL(response.url()).pathname === "/api/sessions"
  );
  await appPage.getByTestId("session-create-submit").click();

  const createRequest = await createRequestPromise;
  const createResponse = await createResponsePromise;
  expect(createRequest.postDataJSON()).toMatchObject({
    agent_name: browserLifecycleAgent,
  });
  expect(createResponse.ok()).toBe(true);

  const created = (await createResponse.json()) as SessionEnvelope;
  expect(created.session.runtime.status).toBe("unbound");
  await expect
    .poll(() => new URL(appPage.url()).pathname)
    .toBe(browserLifecycleSessionPath(created.session.id));
  const sessionWin = sessionWindow(appPage, created.session.id);
  await expect(sessionWin).toBeVisible();
  const sessionUi = sessionWindowSelectors(sessionWin, appPage);
  const runtimeTrigger = sessionWin.getByTestId("session-prompt-runtime-select");
  await runtimeTrigger.click();
  const catalogRow = appPage.locator(
    `[data-provider="${mockAgentProvider}"][data-model="${catalogModel}"]`
  );
  await expect(catalogRow).toBeVisible();
  await catalogRow.click();
  const reasoningStrip = appPage.getByTestId("runtime-selector-reasoning");
  await expect(reasoningStrip).toHaveAttribute("data-reasoning-mode", "levels");
  const reasoningSlider = reasoningStrip.getByRole("slider");
  await expect(reasoningSlider).toBeEnabled();
  await reasoningSlider.press("End");
  await expect(reasoningSlider).toHaveAttribute("aria-valuetext", "High");
  await appPage.keyboard.press("Escape");
  await expect(appPage.getByRole("dialog", { name: "Choose model" })).toBeHidden();
  await expect(runtimeTrigger).toContainText(catalogModelLabel);
  await expect(runtimeTrigger).toContainText("High");
  const promptRequestPromise = appPage.waitForRequest(
    request => request.method() === "POST" && request.url().endsWith("/prompt")
  );
  await sessionUi.composerTextarea.fill(SESSION_CREATE_FIRST_MESSAGE);
  await sessionUi.composerTextarea.press("Enter");
  expect((await promptRequestPromise).postDataJSON()).toMatchObject({
    message: SESSION_CREATE_FIRST_MESSAGE,
    runtime: {
      model: catalogModel,
      provider: mockAgentProvider,
      reasoning_effort: "high",
    },
  });

  await expect
    .poll(async () => {
      const rehydrated = await runtime.requestJSON<SessionEnvelope>(
        sessionAPIPath(created.session.workspace_id, created.session.id)
      );
      return rehydrated.session.runtime.effective;
    })
    .toMatchObject({
      provider: mockAgentProvider,
      model: catalogModel,
      reasoning_effort: "high",
    });
});

const openCodeJourneyAgent = "opencode-journey";
const openCodeJourneyModel = "openrouter/acme/coding-default";
const openCodeJourneyModelLabel = "Coding default";
const openCodeJourneyEffort = "high";
const openCodeJourneyV2Model = "openai/qa-next";
const openCodeBrowserFixture = path.resolve(
  path.dirname(fileURLToPath(import.meta.url)),
  "..",
  "..",
  "..",
  "internal",
  "testutil",
  "acpmock",
  "testdata",
  "opencode_browser_fixture.json"
);

function shellEscape(value: string): string {
  return `'${value.replaceAll("'", `'\\''`)}'`;
}

async function writeOpenCodeBrowserData(dataDir: string, major: "1" | "2"): Promise<void> {
  await writeFile(path.join(dataDir, "major"), `${major}\n`, "utf8");
  await writeFile(
    path.join(dataDir, "version"),
    major === "1" ? "opencode v1.18.33\n" : "opencode v2.0.32\n",
    "utf8"
  );
  await writeFile(
    path.join(dataDir, "v1.txt"),
    [
      "openrouter/acme/coding-default",
      '{"name":"Coding default","capabilities":{"reasoning":true,"toolcall":true},"limit":{"context":262144},"variants":{"high":{},"low":{}}}',
      "openai/qa-next",
      '{"name":"QA Next","limit":{"context":128000}}',
      "",
    ].join("\n"),
    "utf8"
  );
  await writeFile(
    path.join(dataDir, "v2.json"),
    [
      '{"location":{},"data":[',
      '{"providerID":"openrouter","id":"acme/coding-default","name":"Coding default","enabled":true,',
      '"capabilities":{"tools":true},"limit":{"context":262144,"input":200000,"output":32768},',
      '"variants":[{"id":"high"},{"id":"low"}]},',
      '{"providerID":"openai","id":"qa-next","name":"QA Next","enabled":true,"limit":{"context":128000},"variants":[]}',
      "]}",
      "",
    ].join("\n"),
    "utf8"
  );
}

async function installOpenCodeBrowserFixture(input: {
  homeDir: string;
  dataDir: string;
  callsLog: string;
  diagnosticsPath: string;
}): Promise<string> {
  const driverPath = path.join(input.homeDir, "bin", "acpmock-driver");
  const opencodePath = path.join(input.homeDir, "bin", "opencode");
  const script = [
    "#!/bin/sh",
    `printf 'exe=%s\\nargs=%s\\n' "$0" "$*" >> ${shellEscape(input.callsLog)}`,
    `data=${shellEscape(input.dataDir)}`,
    'major="$(cat "$data/major")"',
    'case "$1" in',
    '  --version) cat "$data/version";;',
    '  models) if [ "$major" = "1" ]; then cat "$data/v1.txt"; else echo "unexpected opencode discovery: $* (fixture major $major)" >&2; exit 1; fi;;',
    '  api) if [ "$major" = "2" ]; then cat "$data/v2.json"; else echo "unexpected opencode discovery: $* (fixture major $major)" >&2; exit 1; fi;;',
    '  acp) if [ "$#" -ne 1 ]; then echo "unexpected opencode acp args: $*" >&2; exit 1; fi',
    `    exec ${shellEscape(driverPath)} --fixture ${shellEscape(openCodeBrowserFixture)} --agent opencode-browser --diagnostics ${shellEscape(input.diagnosticsPath)} --provider opencode;;`,
    '  *) echo "unexpected opencode command: $*" >&2; exit 1;;',
    "esac",
    "",
  ].join("\n");
  await writeFile(opencodePath, script, "utf8");
  await chmod(opencodePath, 0o755);
  return opencodePath;
}

async function createOpenCodeJourneyAgent(homeDir: string): Promise<void> {
  // No command override: the journey exercises the inherited `opencode acp` launch.
  const agentDir = path.join(homeDir, "agents", openCodeJourneyAgent);
  await mkdir(agentDir, { recursive: true });
  await writeFile(
    path.join(agentDir, "AGENT.md"),
    [
      "---",
      `name: ${openCodeJourneyAgent}`,
      "provider: opencode",
      "---",
      "",
      "You are a coding assistant.",
      "",
    ].join("\n"),
    "utf8"
  );
}

async function readTranscriptText(
  runtime: {
    requestJSON: <T>(pathname: string, init?: RequestInit) => Promise<T>;
  },
  workspaceID: string,
  sessionID: string
): Promise<string> {
  const payload = await runtime.requestJSON<{ entries: Array<{ message: unknown }> }>(
    sessionAPIPath(workspaceID, sessionID, "/transcript")
  );
  return JSON.stringify(payload.entries.map(entry => entry.message));
}

async function pollSessionEffective(
  runtime: {
    requestJSON: <T>(pathname: string, init?: RequestInit) => Promise<T>;
  },
  workspaceID: string,
  sessionID: string
): Promise<NonNullable<SessionPayload["runtime"]["effective"]>> {
  const record = await runtime.requestJSON<SessionEnvelope>(sessionAPIPath(workspaceID, sessionID));
  return record.session.runtime.effective as NonNullable<SessionPayload["runtime"]["effective"]>;
}

async function assertOpenCodeJourneyParity(
  runtime: {
    requestJSON: <T>(pathname: string, init?: RequestInit) => Promise<T>;
    requestOperatorJSON?: <T>(pathname: string, init?: RequestInit) => Promise<T>;
    paths?: { cliShim: string; homeDir: string; operatorHomeDir: string };
  },
  workspaceID: string,
  sessionID: string,
  expected: { provider: string; model: string; reasoning_effort?: string }
): Promise<void> {
  const httpRecord = await runtime.requestJSON<SessionEnvelope>(
    sessionAPIPath(workspaceID, sessionID)
  );
  expect(httpRecord.session.runtime.effective).toMatchObject(expected);

  if (!runtime.requestOperatorJSON) {
    throw new Error("opencode journey parity check requires operator UDS access");
  }
  const udsRecord = await runtime.requestOperatorJSON<SessionEnvelope>(
    sessionAPIPath(workspaceID, sessionID)
  );
  expect(udsRecord.session.runtime.effective).toMatchObject(expected);

  if (!runtime.paths) {
    throw new Error("opencode journey parity check requires runtime CLI paths");
  }
  const { stdout } = await execFileAsync(
    runtime.paths.cliShim,
    ["session", "list", "--all", "-o", "json"],
    { env: cliEnv(runtime.paths) }
  );
  const cliRecord = (JSON.parse(stdout) as SessionListEnvelope).sessions.find(
    session => session.id === sessionID
  );
  expect(cliRecord?.runtime.effective).toMatchObject(expected);
}

async function countOpenCodePromptTurns(
  diagnosticsPath: string,
  sessionID: string
): Promise<number> {
  const raw = await readFile(diagnosticsPath, "utf8");
  let count = 0;
  for (const line of raw.split("\n")) {
    if (line.trim() === "") {
      continue;
    }
    const record = JSON.parse(line) as {
      compozy_session_id?: string;
      lifecycle_event?: string;
      prompt_index?: number;
    };
    if (
      record.compozy_session_id === sessionID &&
      (record.lifecycle_event ?? "") === "" &&
      (record.prompt_index ?? 0) > 0
    ) {
      count += 1;
    }
  }
  return count;
}

test("operator discovers, selects, and completes prompts on both native OpenCode majors", async ({
  appPage,
  browserArtifacts,
  runtime,
}) => {
  if (!runtime.paths) {
    throw new Error("opencode journey browser test requires launch-mode runtime paths");
  }

  const dataDir = await mkdtemp(path.join(os.tmpdir(), "compozy-opencode-e2e-"));
  const callsLog = path.join(dataDir, "calls.log");
  const diagnosticsPath = path.join(
    runtime.paths.homeDir,
    "logs",
    "acpmock",
    "opencode-browser.jsonl"
  );
  await writeOpenCodeBrowserData(dataDir, "1");
  const opencodePath = await installOpenCodeBrowserFixture({
    homeDir: runtime.paths.homeDir,
    dataDir,
    callsLog,
    diagnosticsPath,
  });
  await createOpenCodeJourneyAgent(runtime.paths.homeDir);
  const agentDef = await readFile(
    path.join(runtime.paths.homeDir, "agents", openCodeJourneyAgent, "AGENT.md"),
    "utf8"
  );
  expect(agentDef).not.toMatch(/^command:/m);

  // A retired curated model must not hide live rows in the provider rail.
  await seedBrowserSettingsFixtures(runtime, {
    providers: [
      {
        name: "opencode",
        settings: {
          models: {
            curated: [{ id: "opencode/retired-model", deprecated: true }],
          },
        },
      },
    ],
  });

  const ui = sessionLifecycleSelectors(appPage);
  const workspaceRoot = await mkdtemp(
    path.join(os.tmpdir(), "compozy-opencode-journey-workspace-")
  );
  const workspace = await runtime.resolveWorkspace(workspaceRoot);

  await appPage.goto(runtime.url("/"), { waitUntil: "domcontentloaded" });
  await completeOnboardingIfPrompted(ui);
  await expect(ui.osDesktop).toBeVisible();
  await switchWorkspace(appPage, workspace.id, workspace.name);
  const agentsWin = await openAppWindow(appPage, "Agents", "agents");
  const fleet = sessionLifecycleSelectors(agentsWin);
  await expect.poll(() => new URL(appPage.url()).pathname).toBe("/agents");
  await expect(fleet.agentRow(openCodeJourneyAgent)).toBeVisible();

  await fleet.agentRow(openCodeJourneyAgent).click();
  await expect.poll(() => new URL(appPage.url()).pathname).toBe(`/agents/${openCodeJourneyAgent}`);
  await expect(fleet.agentPageNewSession).toBeVisible();
  await fleet.agentPageNewSession.click();

  await expect(appPage.getByTestId("session-create-dialog")).toBeVisible();
  const createResponsePromise = appPage.waitForResponse(
    response =>
      response.request().method() === "POST" && new URL(response.url()).pathname === "/api/sessions"
  );
  await appPage.getByTestId("session-create-submit").click();
  const createResponse = await createResponsePromise;
  expect(createResponse.ok()).toBeTruthy();

  const created = (await createResponse.json()) as SessionEnvelope;
  expect(created.session.runtime.status).toBe("unbound");
  await expect
    .poll(() => new URL(appPage.url()).pathname)
    .toBe(`/agents/${openCodeJourneyAgent}/sessions/${created.session.id}`);
  const sessionWin = sessionWindow(appPage, created.session.id);
  const sessionUi = sessionWindowSelectors(sessionWin, appPage);
  await expect(sessionWin).toBeVisible();

  const composer = sessionWin.getByTestId("composer-input").getByRole("textbox");
  const runtimeTrigger = sessionWin.getByTestId("session-prompt-runtime-select");
  await runtimeTrigger.click();
  await expect(appPage.getByTestId("runtime-selector-popup")).toBeVisible();
  await appPage.locator('[data-rail="opencode"]').click();
  await appPage.getByTestId("runtime-selector-refresh").click();
  const v1Row = appPage.locator(`[data-provider="opencode"][data-model="${openCodeJourneyModel}"]`);
  await expect(v1Row).toBeVisible({ timeout: 30_000 });
  const catalog = await runtime.requestJSON<{
    models: Array<{ provider_id: string; model_id: string; curated: boolean }>;
  }>("/api/model-catalog/models?view=all&include_stale=true");
  expect(catalog.models.find(model => model.model_id === openCodeJourneyModel)).toMatchObject({
    provider_id: "opencode",
    curated: false,
  });
  await expect(appPage.getByTestId("runtime-selector-search")).toHaveValue("");
  await expect(appPage.locator('[data-provider="opencode"][data-model]')).toHaveCount(3);
  await expect(v1Row.getByText("openrouter", { exact: true })).toBeVisible();
  expect(
    await appPage
      .locator('[data-provider="opencode"][data-model]')
      .evaluateAll(rows => rows.map(row => row.getAttribute("data-model")))
  ).toEqual([openCodeJourneyV2Model, "opencode/retired-model", openCodeJourneyModel]);
  await v1Row.click();
  const reasoningStrip = appPage.getByTestId("runtime-selector-reasoning");
  await expect(reasoningStrip).toHaveAttribute("data-reasoning-mode", "levels");
  const reasoningSlider = reasoningStrip.getByRole("slider");
  await expect(reasoningSlider).toBeEnabled();
  await reasoningSlider.press("End");
  await expect(reasoningSlider).toHaveAttribute("aria-valuetext", "High");
  await appPage.keyboard.press("Escape");
  await expect(runtimeTrigger).toContainText(openCodeJourneyModelLabel);
  await expect(runtimeTrigger).toContainText("High");

  await composer.fill("opencode browser first prompt");
  const firstPromptRequestPromise = appPage.waitForRequest(
    request => request.method() === "POST" && request.url().endsWith("/prompt")
  );
  await composer.press("Enter");
  expect((await firstPromptRequestPromise).postDataJSON()).toMatchObject({
    message: "opencode browser first prompt",
    runtime: {
      model: openCodeJourneyModel,
      provider: "opencode",
      reasoning_effort: openCodeJourneyEffort,
    },
  });

  await expect
    .poll(() => readTranscriptText(runtime, workspace.id, created.session.id), {
      timeout: 30_000,
    })
    .toContain("opencode browser v1 first response");
  await expect
    .poll(() => pollSessionEffective(runtime, workspace.id, created.session.id), {
      timeout: 30_000,
    })
    .toMatchObject({
      provider: "opencode",
      model: openCodeJourneyModel,
      reasoning_effort: openCodeJourneyEffort,
    });
  await assertOpenCodeJourneyParity(runtime, workspace.id, created.session.id, {
    provider: "opencode",
    model: openCodeJourneyModel,
    reasoning_effort: openCodeJourneyEffort,
  });
  await browserArtifacts.captureScreenshot("opencode-journey-first-prompt", appPage);

  // Hold a turn open so stop and the next prompt prove a fresh process bind.
  await composer.fill("opencode browser interrupted prompt");
  await composer.press("Enter");
  await expect(
    sessionWin.getByText("opencode browser ready for stop", { exact: true })
  ).toBeVisible();
  await expect(sessionUi.stopButton).toBeVisible({ timeout: 30_000 });
  await sessionUi.stopButton.click();
  await expect
    .poll(async () => {
      const record = await runtime.requestJSON<SessionEnvelope>(
        sessionAPIPath(workspace.id, created.session.id)
      );
      return record.session.state;
    })
    .toBe("stopped");
  await expect(sessionWin.getByTestId("session-stopping-row")).not.toBeVisible();
  await composer.fill("opencode browser second prompt");
  const secondPromptRequestPromise = appPage.waitForRequest(
    request => request.method() === "POST" && request.url().endsWith("/prompt")
  );
  await composer.press("Enter");
  expect((await secondPromptRequestPromise).postDataJSON()).toMatchObject({
    message: "opencode browser second prompt",
    runtime: { model: openCodeJourneyModel, provider: "opencode" },
  });
  await expect
    .poll(() => readTranscriptText(runtime, workspace.id, created.session.id), {
      timeout: 30_000,
    })
    .toContain("opencode browser v1 second response");
  await expect
    .poll(() => countOpenCodePromptTurns(diagnosticsPath, created.session.id), {
      timeout: 30_000,
    })
    .toBeGreaterThanOrEqual(2);

  const calls = await readFile(callsLog, "utf8");
  expect(calls).toContain(`exe=${opencodePath}`);
  expect(calls).toContain("args=acp");

  await writeOpenCodeBrowserData(dataDir, "2");
  await runtimeTrigger.click();
  await expect(appPage.getByTestId("runtime-selector-popup")).toBeVisible();
  await appPage.locator('[data-rail="opencode"]').click();
  await appPage.getByTestId("runtime-selector-refresh").click();
  const v2Row = appPage.locator(
    `[data-provider="opencode"][data-model="${openCodeJourneyV2Model}"]`
  );
  await expect(v2Row).toBeVisible({ timeout: 30_000 });
  await expect(appPage.getByTestId("runtime-selector-search")).toHaveValue("");
  await expect(appPage.locator('[data-provider="opencode"][data-model]')).toHaveCount(3);
  await expect(v2Row.getByText("openai", { exact: true })).toBeVisible();
  await v2Row.click();
  await appPage.keyboard.press("Escape");
  await composer.fill("opencode browser v2 prompt");
  const v2PromptRequestPromise = appPage.waitForRequest(
    request => request.method() === "POST" && request.url().endsWith("/prompt")
  );
  await composer.press("Enter");
  expect((await v2PromptRequestPromise).postDataJSON()).toMatchObject({
    message: "opencode browser v2 prompt",
    runtime: { model: openCodeJourneyV2Model, provider: "opencode" },
  });
  await expect
    .poll(() => readTranscriptText(runtime, workspace.id, created.session.id), {
      timeout: 30_000,
    })
    .toContain("opencode browser v2 response");
  await expect
    .poll(() => pollSessionEffective(runtime, workspace.id, created.session.id), {
      timeout: 30_000,
    })
    .toMatchObject({ provider: "opencode", model: openCodeJourneyV2Model });
  await assertOpenCodeJourneyParity(runtime, workspace.id, created.session.id, {
    provider: "opencode",
    model: openCodeJourneyV2Model,
  });

  if (await sessionUi.stopButton.isVisible()) {
    await sessionUi.stopButton.click();
    await expect(sessionUi.stopButton).not.toBeVisible();
  }
});

async function assertSessionParity(
  runtime: {
    requestJSON: <T>(pathname: string, init?: RequestInit) => Promise<T>;
    requestOperatorJSON?: <T>(pathname: string, init?: RequestInit) => Promise<T>;
    paths?: { cliShim: string; homeDir: string; operatorHomeDir: string };
  },
  workspaceID: string,
  sessionID: string,
  expectedProvider: string
): Promise<void> {
  const path = sessionAPIPath(workspaceID, sessionID);
  // The runtime binds after the first prompt is admitted; wait for the daemon's effective truth.
  await expect
    .poll(async () => {
      const httpRecord = await runtime.requestJSON<SessionEnvelope>(path);
      return httpRecord.session.runtime.effective?.provider;
    })
    .toBe(expectedProvider);

  if (!runtime.requestOperatorJSON) {
    throw new Error("provider override parity check requires operator UDS access");
  }
  const udsRecord = await runtime.requestOperatorJSON<SessionEnvelope>(path);
  expect(udsRecord.session.runtime.effective?.provider).toBe(expectedProvider);

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
  expect(cliRecord?.runtime.effective?.provider).toBe(expectedProvider);
}

async function assertSessionNameParity(
  runtime: {
    requestJSON: <T>(pathname: string, init?: RequestInit) => Promise<T>;
    requestOperatorJSON?: <T>(pathname: string, init?: RequestInit) => Promise<T>;
    paths?: { cliShim: string; homeDir: string; operatorHomeDir: string };
  },
  workspaceID: string,
  sessionID: string,
  expectedName: string
): Promise<void> {
  const apiPath = sessionAPIPath(workspaceID, sessionID);
  expect((await runtime.requestJSON<SessionEnvelope>(apiPath)).session.name).toBe(expectedName);
  if (!runtime.requestOperatorJSON || !runtime.paths) {
    throw new Error("rename parity check requires UDS and CLI access");
  }
  expect((await runtime.requestOperatorJSON<SessionEnvelope>(apiPath)).session.name).toBe(
    expectedName
  );
  const { stdout } = await execFileAsync(
    runtime.paths.cliShim,
    ["session", "list", "--all", "-o", "json"],
    { env: cliEnv(runtime.paths) }
  );
  const renamed = (JSON.parse(stdout) as SessionListEnvelope).sessions.find(
    session => session.id === sessionID
  );
  expect(renamed?.name).toBe(expectedName);
}

function sessionAPIPath(workspaceID: string, sessionID: string, suffix = ""): string {
  return `/api/workspaces/${encodeURIComponent(workspaceID)}/sessions/${encodeURIComponent(
    sessionID
  )}${suffix}`;
}

function cliEnv(paths: {
  cliShim: string;
  homeDir: string;
  operatorHomeDir: string;
}): NodeJS.ProcessEnv {
  return {
    COMPOZY_HOME: paths.homeDir,
    HOME: paths.operatorHomeDir,
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
  const configDir = path.join(input.rootDir, ".compozy");
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
