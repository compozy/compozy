// Suite: integrated terminal agent handoff journeys.
// Invariant: agent terminal actions remain approval-gated, profile-bound, and
// observable through the same browser surfaces a human controls.
// Owning layer: hosted native tools + browser Terminal app. Canonical suite: this file.
import { chmod, writeFile } from "node:fs/promises";
import path from "node:path";
import { fileURLToPath } from "node:url";

import type { Client } from "@modelcontextprotocol/sdk/client/index.js";
import type { Locator, Page } from "@playwright/test";

import {
  connectHostedMcpClient,
  readHostedMcpDescriptor,
  teardownHostedMcp,
  type HostedMcpConnection,
} from "../fixtures/hosted-mcp";
import {
  ensureAppWindow,
  focusWindowThroughPalette,
  openAppWindow,
  sessionWindow,
  windowID,
} from "../fixtures/os-navigation";
import type { BrowserRuntime, RuntimePaths } from "../fixtures/runtime";
import { profilesOperatorSelectors, sessionWindowSelectors } from "../fixtures/selectors";
import { closeTerminalWatchers, connectTerminalWatcher } from "../fixtures/terminal-watcher";
import { expect, test } from "../fixtures/test";
import { completeOnboardingIfPrompted, ensureProjectWorkspace } from "../fixtures/workspace";

const MOCK_AGENT = "mock-integrated-terminal";
const FIXTURE_AGENT = "integrated-terminal";

function repoFixture(name: string): string {
  return path.resolve(
    path.dirname(fileURLToPath(import.meta.url)),
    "../../..",
    "internal/testutil/acpmock/testdata",
    name
  );
}

function assertLaunchRuntime(
  runtime: BrowserRuntime
): asserts runtime is BrowserRuntime & { paths: RuntimePaths } {
  if (runtime.mode !== "launch" || !runtime.paths) {
    throw new Error("Terminal agent E2E requires a launch-mode runtime with paths.");
  }
}

interface SessionEnvelope {
  session: { id: string; agent_name: string; workspace_id: string };
}

interface TerminalToolResult {
  terminal_id: string;
  still_running?: boolean;
  exit_code?: number | null;
  outcome?: "answered" | "rejected" | "superseded" | "expired";
  redacted?: boolean;
  length?: number;
}

interface AgentHarness {
  client: Client;
  connection: HostedMcpConnection;
  sessionId: string;
  sessionUI: ReturnType<typeof sessionWindowSelectors>;
  sessionWin: Locator;
  workspace: { id: string; name: string };
}

function readToolResult<T>(result: Awaited<ReturnType<Client["callTool"]>>): T {
  const structured = result.structuredContent;
  if (structured && typeof structured === "object") return structured as T;
  for (const block of Array.isArray(result.content) ? result.content : []) {
    if (block.type === "text" && typeof block.text === "string") {
      try {
        return JSON.parse(block.text) as T;
      } catch (error) {
        throw new Error(`Terminal tool call returned non-JSON text: ${block.text}`, {
          cause: error,
        });
      }
    }
  }
  throw new Error("Terminal tool call returned no structured content.");
}

async function startAgentHarness(appPage: Page, runtime: BrowserRuntime): Promise<AgentHarness> {
  assertLaunchRuntime(runtime);
  await completeOnboardingIfPrompted(appPage);
  await ensureProjectWorkspace(appPage, runtime);
  const workspace = await runtime.resolveWorkspace(runtime.paths.workspaceDir);
  const created = await runtime.requestJSON<SessionEnvelope>("/api/sessions", {
    method: "POST",
    body: JSON.stringify({ agent_name: MOCK_AGENT, workspace: workspace.id }),
  });
  const sessionId = created.session.id;
  await appPage.goto(runtime.url(`/agents/${MOCK_AGENT}/sessions/${sessionId}`), {
    waitUntil: "domcontentloaded",
  });
  const sessionWin = sessionWindow(appPage, sessionId);
  const sessionUI = sessionWindowSelectors(sessionWin, appPage);
  await expect(sessionWin).toBeVisible({ timeout: 20_000 });
  await sessionUI.composerTextarea.fill("hold native approval");
  await sessionUI.composerTextarea.press("Enter");
  await expect(appPage.getByText("native approval ready")).toBeVisible({ timeout: 20_000 });
  await expect(sessionUI.stopButton).toBeVisible({ timeout: 20_000 });
  const descriptor = await readHostedMcpDescriptor(
    path.join(runtime.paths.homeDir, "logs", "acpmock", `${MOCK_AGENT}.jsonl`)
  );
  const connection = await connectHostedMcpClient(descriptor);
  return { client: connection.client, connection, sessionId, sessionUI, sessionWin, workspace };
}

async function approveOnce(harness: AgentHarness, appPage: Page): Promise<void> {
  await expect(harness.sessionUI.permissionPrompt).toBeVisible({ timeout: 30_000 });
  await focusWindowThroughPalette(appPage, harness.sessionWin);
  await harness.sessionUI.permissionAllowOnce.click();
  await expect(harness.sessionUI.permissionPrompt).toBeHidden();
}

async function stopHoldingTurn(harness: AgentHarness, appPage: Page): Promise<void> {
  const sessionBase = `/api/workspaces/${encodeURIComponent(
    harness.workspace.id
  )}/sessions/${encodeURIComponent(harness.sessionId)}`;
  const stopped = appPage.waitForResponse(
    response =>
      response.request().method() === "POST" &&
      (response.url().endsWith(`${sessionBase}/prompt/cancel`) ||
        response.url().endsWith(`${sessionBase}/stop`))
  );
  await harness.sessionUI.stopButton.click();
  expect((await stopped).ok()).toBe(true);
  await harness.sessionUI.topbarOverflow.click();
  await expect(harness.sessionUI.composerClearButton).toBeEnabled({ timeout: 60_000 });
  await appPage.keyboard.press("Escape");
}

async function selectTerminalOutput(
  page: Page,
  window: Locator,
  screenContent: string
): Promise<void> {
  const lines = screenContent.split(/\r?\n/u);
  const firstRow = lines.findIndex(line => line.includes("quote-alpha"));
  const secondRow = lines.findIndex(line => line.includes("quote-beta"));
  if (firstRow < 0 || secondRow < 0) throw new Error("Terminal quote rows are absent from screen.");
  const firstColumn = lines[firstRow]!.indexOf("quote-alpha");
  const secondColumn = lines[secondRow]!.indexOf("quote-beta") + "quote-beta".length;
  const grid = await window.getByTestId("terminal-size-vote").innerText();
  const dimensions = /(\d+)×(\d+)\s*$/u.exec(grid.trim());
  if (!dimensions) throw new Error(`Unexpected terminal grid dimensions: ${grid}`);
  const columns = Number(dimensions[1]);
  const rows = Number(dimensions[2]);
  const screen = window.locator(".xterm-screen");
  await expect(screen).toBeVisible({ timeout: 20_000 });
  const box = await screen.boundingBox();
  if (!box) throw new Error("Terminal screen has no layout box.");
  const cellWidth = box.width / columns;
  const cellHeight = box.height / rows;
  await page.mouse.move(
    box.x + (firstColumn + 0.5) * cellWidth,
    box.y + (firstRow + 0.5) * cellHeight
  );
  await page.mouse.down();
  await page.mouse.move(
    box.x + (secondColumn + 0.5) * cellWidth,
    box.y + (secondRow + 0.5) * cellHeight,
    { steps: 8 }
  );
  await page.mouse.up();
}

async function takeTerminalControl(window: Locator): Promise<Locator> {
  const log = window.locator('[role="log"]:visible').last();
  await expect(async () => {
    if ((await log.getAttribute("data-readonly")) === "true") {
      const takeControl = window.getByTestId("terminal-take-control").last();
      await expect(takeControl).toBeVisible();
      await takeControl.click();
    }
    await expect(log).not.toHaveAttribute("data-readonly", "true");
  }).toPass({ timeout: 20_000 });
  return log;
}

async function ensureTerminalWindow(page: Page): Promise<Locator> {
  const terminalWindow = await ensureAppWindow(page, "Terminal", "terminal");
  await focusWindowThroughPalette(page, terminalWindow);
  return terminalWindow;
}

async function openAgentTerminal(
  harness: AgentHarness,
  title: string,
  extra: Record<string, unknown> = {}
): Promise<string> {
  const pending = harness.client.callTool({
    name: "compozy__terminal_open",
    arguments: { title, ...extra },
    _meta: { toolCallId: `e2e-open-${title}` },
  });
  await approveOnce(harness, harness.sessionWin.page());
  const result = await pending;
  expect(result.isError).toBeFalsy();
  return readToolResult<TerminalToolResult>(result).terminal_id;
}

async function terminalScreen(
  runtime: BrowserRuntime,
  workspaceId: string,
  terminalId: string,
  profile = "default"
) {
  return await runtime.requestJSON<{ content: string }>(
    `/api/workspaces/${encodeURIComponent(workspaceId)}/terminals/${encodeURIComponent(
      terminalId
    )}/read?view=screen&profile=${encodeURIComponent(profile)}`
  );
}

test.use({
  runtimeOptions: {
    seed: {
      mockAgents: [
        {
          fixturePath: repoFixture("integrated_terminal_fixture.json"),
          fixtureAgent: FIXTURE_AGENT,
          agentName: MOCK_AGENT,
        },
      ],
    },
  },
});

test("E2E-010: agent-reported output stays labeled and absent from the Terminal app", async ({
  appPage,
  runtime,
}) => {
  assertLaunchRuntime(runtime);
  await completeOnboardingIfPrompted(appPage);
  const workspace = await runtime.resolveWorkspace(runtime.paths.workspaceDir);
  const created = await runtime.requestJSON<SessionEnvelope>("/api/sessions", {
    method: "POST",
    body: JSON.stringify({ agent_name: MOCK_AGENT, workspace: workspace.id }),
  });
  const sessionId = created.session.id;
  await appPage.goto(runtime.url(`/agents/${MOCK_AGENT}/sessions/${sessionId}`), {
    waitUntil: "domcontentloaded",
  });
  const sessionWin = sessionWindow(appPage, sessionId);
  const sessionUI = sessionWindowSelectors(sessionWin, appPage);
  await expect(sessionWin).toBeVisible({ timeout: 20_000 });
  await sessionUI.composerTextarea.fill("show agent reported terminal");
  await sessionUI.composerTextarea.press("Enter");

  const reported = sessionWin.getByTestId("session-agent-reported-block-reported-terminal-1");
  await expect(reported).toBeVisible({ timeout: 20_000 });
  await expect(reported.getByText("reported by agent")).toBeVisible();
  await expect(reported.getByRole("log", { name: /reported by the agent/i })).toHaveAttribute(
    "data-readonly",
    "true"
  );
  await expect(reported.getByRole("button")).toHaveCount(0);
  await expect(reported).toContainText("12 tests passed");

  const catalog = await runtime.requestJSON<{ terminals: unknown[] }>(
    `/api/workspaces/${encodeURIComponent(workspace.id)}/terminals?profile=default`
  );
  expect(catalog.terminals).toEqual([]);
  await ensureProjectWorkspace(appPage, runtime);
  const terminalWindow = await openAppWindow(appPage, "Terminal", "terminal");
  await expect(terminalWindow.getByTestId("terminal-empty")).toBeVisible();
});

test("E2E-003: deliberate agent exec stays discoverable from approval through journal", async ({
  appPage,
  runtime,
}) => {
  const harness = await startAgentHarness(appPage, runtime);
  let activeCall: ReturnType<Client["callTool"]> | undefined;
  try {
    const command = "printf 'agent-live-1\\n'; sleep 2; printf 'agent-live-2\\n'";
    const pending = harness.client.callTool({
      name: "compozy__terminal_exec",
      arguments: {
        command: "/bin/sh",
        args: ["-c", command],
        visible: true,
        yield_ms: 250,
      },
      _meta: { toolCallId: "e2e-terminal-deliberate-exec" },
    });
    activeCall = pending;
    await expect(harness.sessionUI.permissionPrompt).toBeVisible({ timeout: 30_000 });
    const exactApproval = String.raw`/bin/sh -c 'printf '\''agent-live-1\n'\''; sleep 2; printf '\''agent-live-2\n'\'''`;
    await expect(harness.sessionWin.getByTestId("terminal-approval-command")).toHaveText(
      exactApproval
    );

    await harness.sessionWin.getByRole("button", { name: "Close window" }).click();
    const terminalLauncher = appPage.locator('[data-slot="os-dock-item"][data-app="terminal"]');
    await expect(terminalLauncher.locator('[data-slot="os-dock-badge"]')).toHaveText("1", {
      timeout: 20_000,
    });
    await appPage.goto(runtime.url(`/agents/${MOCK_AGENT}/sessions/${harness.sessionId}`), {
      waitUntil: "domcontentloaded",
    });
    await approveOnce(harness, appPage);

    const callResult = await pending;
    activeCall = undefined;
    expect(callResult.isError).toBeFalsy();
    const execution = readToolResult<TerminalToolResult>(callResult);
    expect(execution).toMatchObject({ still_running: true });
    const terminalId = execution.terminal_id;

    await ensureProjectWorkspace(appPage, runtime);
    // The dock adopts the agent's running terminal directly.
    let terminalWindow = await openAppWindow(appPage, "Terminal", "terminal");
    await expect(terminalWindow.getByTestId(`terminal-pane-${terminalId}`)).toBeVisible();
    await expect
      .poll(async () => (await terminalScreen(runtime, harness.workspace.id, terminalId)).content)
      .toContain("agent-live-1");
    await expect(terminalWindow.getByTestId("terminal-exit-bar")).toBeVisible({ timeout: 20_000 });

    const journal = await runtime.requestJSON<{
      entries: Array<{ approval: string; command_id: string; terminal_id: string | null }>;
    }>(
      `/api/workspaces/${encodeURIComponent(
        harness.workspace.id
      )}/terminals/journal?profile=default`
    );
    const row = journal.entries.find(entry => entry.terminal_id === terminalId);
    expect(row).toMatchObject({ approval: "approved_once" });
    if (!row) throw new Error("Approved terminal exec was absent from the journal.");
    await terminalWindow.getByTestId("terminal-journal-toggle").click();
    await expect(
      terminalWindow.getByTestId(`terminal-journal-row-${row.command_id}`)
    ).toBeVisible();
  } finally {
    await teardownHostedMcp(harness.connection, activeCall);
  }
});

test("E2E-004: watcher takeover and release update two browser contexts", async ({
  appPage,
  browser,
  runtime,
}) => {
  test.setTimeout(90_000);
  const harness = await startAgentHarness(appPage, runtime);
  const secondContext = await browser.newContext();
  try {
    const terminalId = await openAgentTerminal(harness, "agent-watch-control");
    await ensureProjectWorkspace(appPage, runtime);
    const firstWindow = await openAppWindow(appPage, "Terminal", "terminal");
    const firstGrid = firstWindow.getByRole("log", { name: "agent-watch-control" });
    await expect(firstGrid).toHaveAttribute("data-readonly", "true");
    await firstGrid.click();
    await appPage.keyboard.type("watcher-must-not-write");
    expect((await terminalScreen(runtime, harness.workspace.id, terminalId)).content).not.toContain(
      "watcher-must-not-write"
    );

    const secondPage = await secondContext.newPage();
    await secondPage.goto(runtime.url("/"), { waitUntil: "domcontentloaded" });
    await ensureProjectWorkspace(secondPage, runtime);
    const secondWindow = await openAppWindow(secondPage, "Terminal", "terminal");
    await expect(firstWindow.getByTestId("terminal-viewers")).toContainText("2");

    await firstWindow.getByTestId("terminal-take-control").click();
    await expect(firstWindow.getByTestId("terminal-lease-label")).toHaveText("You're in control");
    await expect(secondWindow.getByTestId("terminal-lease-label")).toHaveText(
      /^web-.+ is in control$/u
    );
    await firstWindow.getByRole("log").click();
    await appPage.keyboard.type("printf 'human-control-flowed\\n'");
    await appPage.keyboard.press("Enter");
    await expect
      .poll(async () => (await terminalScreen(runtime, harness.workspace.id, terminalId)).content)
      .toContain("human-control-flowed");

    await firstWindow.getByTestId("terminal-release-control").click();
    await expect
      .poll(
        async () => {
          const payload = await runtime.requestJSON<{
            terminal: { lease: string; controller: { kind: string } | null };
          }>(
            `/api/workspaces/${encodeURIComponent(
              harness.workspace.id
            )}/terminals/${encodeURIComponent(terminalId)}?profile=default`
          );
          return payload.terminal;
        },
        { timeout: 45_000 }
      )
      .toMatchObject({ lease: "agent_owned", controller: { kind: "agent" } });
  } finally {
    await secondContext.close();
    await teardownHostedMcp(harness.connection, undefined);
  }
});

test("E2E-005: hidden input is delivered by length and can be rejected cleanly", async ({
  appPage,
  runtime,
}) => {
  const harness = await startAgentHarness(appPage, runtime);
  assertLaunchRuntime(runtime);
  let activeCall: ReturnType<Client["callTool"]> | undefined;
  try {
    const promptShell = path.join(runtime.paths.workspaceDir, ".terminal-e2e-hidden-input.sh");
    await writeFile(
      promptShell,
      [
        "#!/bin/sh",
        "stty -echo",
        "printf 'Password:'",
        "IFS= read -r secret",
        "stty echo",
        "printf '\\n<redacted:%s>\\nagent-continued\\n' \"${#secret}\"",
        "exec /bin/sh",
        "",
      ].join("\n"),
      { mode: 0o700 }
    );
    await chmod(promptShell, 0o700);
    const terminalId = await openAgentTerminal(harness, "hidden-input", { shell: promptShell });
    await ensureProjectWorkspace(appPage, runtime);
    let terminalWindow = await openAppWindow(appPage, "Terminal", "terminal");

    const pending = harness.client.callTool({
      name: "compozy__terminal_request_input",
      arguments: {
        terminal_id: terminalId,
        reason: "sudo password",
        prompt_excerpt: "Password:",
        redact: true,
      },
      _meta: { toolCallId: "e2e-terminal-hidden-input" },
    });
    activeCall = pending;
    await approveOnce(harness, appPage);
    terminalWindow = await ensureTerminalWindow(appPage);
    const card = terminalWindow.locator('[data-testid^="terminal-input-request-"]').first();
    await expect(card).toBeVisible({ timeout: 20_000 });
    await expect(card).toContainText("Password:");
    const field = card.locator('input[type="password"]');
    await expect(field).toBeVisible();
    const secret = "s3cret-value";
    await field.fill(secret);
    expect(await field.evaluate(node => node.outerHTML)).not.toContain(secret);
    await expect(appPage.getByText(secret, { exact: true })).toHaveCount(0);
    await card.getByRole("button", { name: "Take control & send" }).click();
    const answered = readToolResult<TerminalToolResult>(await pending);
    activeCall = undefined;
    expect(answered).toMatchObject({ outcome: "answered", redacted: true, length: secret.length });
    await expect
      .poll(async () => (await terminalScreen(runtime, harness.workspace.id, terminalId)).content)
      .toContain(`<redacted:${secret.length}>`);
    expect((await terminalScreen(runtime, harness.workspace.id, terminalId)).content).not.toContain(
      secret
    );

    const rejectedCall = harness.client.callTool({
      name: "compozy__terminal_request_input",
      arguments: {
        terminal_id: terminalId,
        reason: "confirmation",
        prompt_excerpt: "Continue?",
        redact: false,
      },
      _meta: { toolCallId: "e2e-terminal-reject-input" },
    });
    activeCall = rejectedCall;
    await approveOnce(harness, appPage);
    terminalWindow = await ensureTerminalWindow(appPage);
    const rejectCard = terminalWindow.locator('[data-testid^="terminal-input-request-"]').first();
    await expect(rejectCard).toContainText("Continue?");
    await rejectCard.getByRole("button", { name: "Decline" }).click();
    expect(readToolResult<TerminalToolResult>(await rejectedCall)).toMatchObject({
      outcome: "rejected",
    });
    activeCall = undefined;
  } finally {
    await teardownHostedMcp(harness.connection, activeCall);
  }
});

test("E2E-006: typing grant is promptless only for its terminal generation", async ({
  appPage,
  runtime,
}) => {
  const harness = await startAgentHarness(appPage, runtime);
  let activeCall: ReturnType<Client["callTool"]> | undefined;
  try {
    const firstTerminal = await openAgentTerminal(harness, "typing-grant-a");
    const firstWrite = harness.client.callTool({
      name: "compozy__terminal_write",
      arguments: { terminal_id: firstTerminal, data: "printf 'grant-first\\n'\\n" },
      _meta: { toolCallId: "e2e-terminal-first-write" },
    });
    activeCall = firstWrite;
    await expect(harness.sessionUI.permissionPrompt).toBeVisible({ timeout: 30_000 });
    await expect(harness.sessionWin.getByTestId("terminal-typing-grant-detail")).toContainText(
      firstTerminal
    );
    await harness.sessionUI.permissionAllowAlways.click();
    await expect(harness.sessionUI.permissionPrompt).toBeHidden();
    expect((await firstWrite).isError).toBeFalsy();
    activeCall = undefined;

    const followUp = await harness.client.callTool({
      name: "compozy__terminal_write",
      arguments: { terminal_id: firstTerminal, data: "printf 'grant-follow-up\\n'\\n" },
      _meta: { toolCallId: "e2e-terminal-follow-up-write" },
    });
    expect(followUp.isError).toBeFalsy();
    await expect(harness.sessionUI.permissionPrompt).toBeHidden();

    const secondTerminal = await openAgentTerminal(harness, "typing-grant-b");
    const secondWrite = harness.client.callTool({
      name: "compozy__terminal_write",
      arguments: { terminal_id: secondTerminal, data: "printf 'grant-second-terminal\\n'\\n" },
      _meta: { toolCallId: "e2e-terminal-second-write" },
    });
    activeCall = secondWrite;
    await expect(harness.sessionUI.permissionPrompt).toBeVisible({ timeout: 30_000 });
    await expect(harness.sessionWin.getByTestId("terminal-typing-grant-detail")).toContainText(
      secondTerminal
    );
    await harness.sessionWin.getByTestId("permission-reject-once").click();
    expect((await secondWrite).isError).toBe(true);
    activeCall = undefined;
  } finally {
    await teardownHostedMcp(harness.connection, activeCall);
  }
});

test("E2E-008: a two-line terminal selection becomes a sourced conversation quote", async ({
  appPage,
  runtime,
}) => {
  const harness = await startAgentHarness(appPage, runtime);
  assertLaunchRuntime(runtime);
  try {
    const quoteShell = path.join(runtime.paths.workspaceDir, ".terminal-e2e-quote.sh");
    await writeFile(
      quoteShell,
      ["#!/bin/sh", "printf 'quote-alpha\\nquote-beta\\n'", "exec /bin/sh", ""].join("\n"),
      { mode: 0o700 }
    );
    await chmod(quoteShell, 0o700);
    const terminalId = await openAgentTerminal(harness, "quote-source", { shell: quoteShell });
    await stopHoldingTurn(harness, appPage);
    await ensureProjectWorkspace(appPage, runtime);
    const terminalWindow = await openAppWindow(appPage, "Terminal", "terminal");
    await expect
      .poll(async () => (await terminalScreen(runtime, harness.workspace.id, terminalId)).content)
      .toContain("quote-beta");

    const quoteScreen = await terminalScreen(runtime, harness.workspace.id, terminalId);
    await connectTerminalWatcher(appPage, runtime, harness.workspace.id, terminalId);
    try {
      await selectTerminalOutput(appPage, terminalWindow, quoteScreen.content);
      const actions = terminalWindow.getByTestId("terminal-selection-actions");
      await expect(actions).toBeVisible();
      await actions.getByRole("button", { name: "Send to conversation" }).click();
    } finally {
      await closeTerminalWatchers(appPage);
    }
    await expect(harness.sessionWin).toBeVisible();
    const quoteBlock = harness.sessionWin.getByTestId("terminal-quote-block");
    await expect(quoteBlock).toBeVisible();
    await expect(quoteBlock).toContainText(terminalId);
    await expect(quoteBlock).toContainText(/quote-alpha/u);
    await expect(quoteBlock).toContainText(/quote-beta/u);
    await expect(harness.sessionUI.composerTextarea).toHaveText("");
    await harness.sessionUI.composerSendButton.click();
    await expect(harness.sessionWin).toContainText("I received the sourced terminal excerpt.", {
      timeout: 20_000,
    });

    await harness.sessionWin.getByRole("button", { name: "Close window" }).click();
    await expect(harness.sessionWin).toBeHidden();
    const fallback = terminalWindow.getByTestId("terminal-selection-actions-no-session");
    await expect(fallback.getByRole("button", { name: "Choose a session…" })).toBeVisible();
    await expect(fallback.getByRole("button", { name: "Copy" })).toBeVisible();
  } finally {
    await teardownHostedMcp(harness.connection, undefined);
  }
});

test("E2E-020: profile switches isolate terminals and aggregate journal owners", async ({
  appPage,
  runtime,
}) => {
  test.setTimeout(120_000);
  const harness = await startAgentHarness(appPage, runtime);
  let activeCall: ReturnType<Client["callTool"]> | undefined;
  try {
    const executionCall = harness.client.callTool({
      name: "compozy__terminal_exec",
      arguments: {
        command: "/bin/sh",
        args: ["-c", "printf 'profile-default-row\\n'; sleep 1"],
        visible: true,
        yield_ms: 250,
      },
      _meta: { toolCallId: "e2e-profile-default-exec" },
    });
    activeCall = executionCall;
    await approveOnce(harness, appPage);
    const execution = readToolResult<TerminalToolResult>(await executionCall);
    activeCall = undefined;
    expect(execution).toMatchObject({ still_running: true });
    const defaultTerminalId = execution.terminal_id;
    await expect
      .poll(
        async () => (await terminalScreen(runtime, harness.workspace.id, defaultTerminalId)).content
      )
      .toContain("profile-default-row");
    await expect
      .poll(async () => {
        const journal = await runtime.requestJSON<{
          entries: Array<{ terminal_id: string | null }>;
        }>(
          `/api/workspaces/${encodeURIComponent(harness.workspace.id)}/terminals/journal?profile=default`
        );
        return journal.entries.some(entry => entry.terminal_id === defaultTerminalId);
      })
      .toBe(true);

    const inputTerminalId = await openAgentTerminal(harness, "profile-default-input");

    const inputCall = harness.client.callTool({
      name: "compozy__terminal_request_input",
      arguments: {
        terminal_id: inputTerminalId,
        reason: "profile-scoped confirmation",
        prompt_excerpt: "Default profile input:",
        redact: false,
      },
      _meta: { toolCallId: "e2e-profile-default-input" },
    });
    activeCall = inputCall;
    await approveOnce(harness, appPage);

    await ensureProjectWorkspace(appPage, runtime);
    // Adoption lands on the input terminal — the only running one — so the
    // pending question is pinned right under its grid; the dock badge is the
    // cross-window attention surface.
    let terminalWindow = await openAppWindow(appPage, "Terminal", "terminal");
    const terminalLauncher = appPage.locator('[data-slot="os-dock-item"][data-app="terminal"]');
    await expect(terminalWindow.getByTestId(`terminal-pane-${inputTerminalId}`)).toBeVisible();
    await expect(terminalWindow.getByTestId("terminal-input-request-stack")).toBeVisible();
    await expect(terminalLauncher.locator('[data-slot="os-dock-badge"]')).toHaveText("1");

    const profiles = profilesOperatorSelectors(appPage);
    await profiles.switcher.click();
    await profiles.switcherCreate.click();
    await profiles.createName.fill("terminal-b");
    const createdProfile = appPage.waitForResponse(
      response => response.request().method() === "POST" && response.url().endsWith("/api/profiles")
    );
    await profiles.createConfirm.click();
    expect((await createdProfile).ok()).toBe(true);
    await expect(profiles.switcher).toContainText("terminal-b");
    terminalWindow = await ensureTerminalWindow(appPage);
    // The other profile's terminals are hidden, not closed: the routed one is
    // simply not here under terminal-b.
    await expect(terminalWindow.getByTestId(`terminal-pane-${defaultTerminalId}`)).toHaveCount(0);
    await expect(terminalWindow.getByTestId(`terminal-pane-${inputTerminalId}`)).toHaveCount(0);
    await expect(terminalWindow.getByTestId("terminal-not-found")).toBeVisible();
    await expect(terminalLauncher.locator('[data-slot="os-dock-badge"]')).toHaveCount(0);

    const profileBWindow = appPage.getByTestId(`os-window-${await windowID(terminalWindow)}`);
    await profileBWindow.getByRole("button", { name: "Open a new terminal" }).click();
    const profileBPane = profileBWindow
      .locator('[data-testid^="terminal-pane-term-"]:visible')
      .first();
    await expect(profileBPane).toBeVisible({ timeout: 20_000 });
    const profileBTerminalId = (await profileBPane.getAttribute("data-testid"))?.replace(
      "terminal-pane-",
      ""
    );
    if (!profileBTerminalId) throw new Error("terminal-b did not expose its terminal id.");
    terminalWindow = profileBWindow;
    const profileBLog = await takeTerminalControl(terminalWindow);
    await profileBLog.click();
    await appPage.keyboard.type("printf 'profile-terminal-b-row\\n'");
    await appPage.keyboard.press("Enter");
    await expect
      .poll(
        async () =>
          (await terminalScreen(runtime, harness.workspace.id, profileBTerminalId, "terminal-b"))
            .content
      )
      .toContain("profile-terminal-b-row");
    await terminalWindow.getByTestId("terminal-journal-toggle").click();
    await expect(terminalWindow.getByTestId("terminal-journal")).toContainText(
      "profile-terminal-b-row"
    );
    await expect(terminalWindow.getByTestId("terminal-journal")).not.toContainText(
      "profile-default-row"
    );

    await profiles.switcher.click();
    await profiles.switcherOption("default").click();
    await expect(profiles.switcher).toContainText("default");
    terminalWindow = await ensureTerminalWindow(appPage);
    // Back on default, this window is still routed at terminal-b's terminal —
    // hidden here — while the pending question shows on the dock badge.
    await expect(terminalWindow.getByTestId("terminal-not-found")).toBeVisible();
    await expect(terminalLauncher.locator('[data-slot="os-dock-badge"]')).toHaveText("1");
    expect(
      (await terminalScreen(runtime, harness.workspace.id, defaultTerminalId)).content
    ).toContain("profile-default-row");

    await profiles.switcher.click();
    await profiles.switcherAll.click();
    await expect(profiles.switcher).toContainText("All profiles");
    terminalWindow = await ensureTerminalWindow(appPage);
    await expect(terminalWindow.getByTestId("terminal-journal-toggle")).toBeVisible();
    await terminalWindow.getByTestId("terminal-journal-toggle").click();
    const aggregateJournal = terminalWindow.getByTestId("terminal-journal");
    await expect(aggregateJournal).toContainText("profile-default-row");
    await expect(aggregateJournal).toContainText("profile-terminal-b-row");
    const ownerTags = aggregateJournal.getByTestId(/^terminal-journal-owner-/);
    await expect(ownerTags.filter({ hasText: "default" }).first()).toBeVisible();
    await expect(ownerTags.filter({ hasText: "terminal-b" }).first()).toBeVisible();

    await profiles.switcher.click();
    await profiles.switcherOption("default").click();
    // The deep link is the way to a specific terminal; it retargets the window.
    await appPage.goto(runtime.url(`/terminal/${encodeURIComponent(inputTerminalId)}`), {
      waitUntil: "domcontentloaded",
    });
    terminalWindow = await ensureTerminalWindow(appPage);
    await expect(terminalWindow.getByTestId(`terminal-pane-${inputTerminalId}`)).toBeVisible();
    const inputCard = terminalWindow.locator('[data-testid^="terminal-input-request-"]').first();
    await expect(inputCard).toBeVisible();
    await inputCard.getByRole("button", { name: "Decline" }).click();
    expect(readToolResult<TerminalToolResult>(await inputCall)).toMatchObject({
      outcome: "rejected",
    });
    activeCall = undefined;
  } finally {
    await teardownHostedMcp(harness.connection, activeCall);
  }
});
