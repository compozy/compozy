// Suite: integrated terminal agent collaboration journeys.
// Invariant: agent terminal actions remain approval-gated, profile-bound, and
// observable through the same shared browser surface a human can use concurrently.
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
  sessionWindow,
  windowFrame,
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

async function interactiveTerminalLog(window: Locator): Promise<Locator> {
  const log = window.locator('[role="log"]:visible').last();
  await expect(log).toHaveAttribute("data-readonly", "false", { timeout: 20_000 });
  return log;
}

async function ensureTerminalWindow(page: Page): Promise<Locator> {
  const terminalWindow = await ensureAppWindow(page, "Terminal", "terminal");
  await focusWindowThroughPalette(page, terminalWindow);
  return terminalWindow;
}

/**
 * The window hosting one terminal, keyed by the authority's `instance_key`.
 * Every agent-opened terminal materializes its own window, so the app-wide
 * locator is ambiguous once a session has opened more than one.
 */
function terminalWindowFor(page: Page, terminalId: string): Locator {
  return page.locator(
    `[data-slot="os-window-surface"][data-app="terminal"][data-instance-key="${terminalId}"]`
  );
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

    // The daemon materializes a Terminal window for the agent-opened pty
    // without stealing focus from the session the human is reading.
    const materialized = appPage.locator('[data-slot="os-window-surface"][data-app="terminal"]');
    await expect(materialized).toBeVisible({ timeout: 20_000 });
    await expect(windowFrame(harness.sessionWin)).toHaveAttribute("data-focused", "");

    await ensureProjectWorkspace(appPage, runtime);
    // The deep link is the deterministic way to this exact terminal; ambient
    // adoption is covered by E2E-002/018/020.
    await appPage.goto(runtime.url(`/terminal/${encodeURIComponent(terminalId)}`), {
      waitUntil: "domcontentloaded",
    });
    const terminalWindow = appPage.locator(
      '[data-slot="os-window-surface"][data-app="terminal"][data-stack-active]'
    );
    await expect(terminalWindow.getByTestId(`terminal-pane-${terminalId}`)).toBeVisible({
      timeout: 20_000,
    });
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

test("E2E-004: two browser contexts write to one agent terminal concurrently", async ({
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
    await appPage.goto(runtime.url(`/terminal/${encodeURIComponent(terminalId)}`), {
      waitUntil: "domcontentloaded",
    });
    const firstWindow = appPage.locator(
      '[data-slot="os-window-surface"][data-app="terminal"][data-stack-active]'
    );
    const firstGrid = firstWindow.getByRole("log", { name: "agent-watch-control" });
    await expect(firstGrid).toHaveAttribute("data-readonly", "false");
    await firstGrid.click();
    await appPage.keyboard.type("printf 'first-shared-writer\\n'");
    await appPage.keyboard.press("Enter");

    const secondPage = await secondContext.newPage();
    await secondPage.goto(runtime.url("/"), { waitUntil: "domcontentloaded" });
    await ensureProjectWorkspace(secondPage, runtime);
    await secondPage.goto(runtime.url(`/terminal/${encodeURIComponent(terminalId)}`), {
      waitUntil: "domcontentloaded",
    });
    const secondWindow = secondPage.locator(
      '[data-slot="os-window-surface"][data-app="terminal"][data-stack-active]'
    );
    await expect(firstWindow.getByTestId("terminal-viewers")).toContainText("2");
    const secondGrid = secondWindow.getByRole("log", { name: "agent-watch-control" });
    await expect(secondGrid).toHaveAttribute("data-readonly", "false");
    await secondGrid.click();
    await secondPage.keyboard.type("printf 'second-shared-writer\\n'");
    await secondPage.keyboard.press("Enter");
    await expect
      .poll(async () => (await terminalScreen(runtime, harness.workspace.id, terminalId)).content)
      .toContain("first-shared-writer");
    await expect
      .poll(async () => (await terminalScreen(runtime, harness.workspace.id, terminalId)).content)
      .toContain("second-shared-writer");
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
    await appPage.goto(runtime.url(`/terminal/${encodeURIComponent(terminalId)}`), {
      waitUntil: "domcontentloaded",
    });
    let terminalWindow = appPage.locator(
      '[data-slot="os-window-surface"][data-app="terminal"][data-stack-active]'
    );
    await expect(terminalWindow.getByTestId(`terminal-pane-${terminalId}`)).toBeVisible({
      timeout: 20_000,
    });

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
    await card.getByRole("button", { name: "Send" }).click();
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

test("E2E-006: terminal writes stay promptless across terminal generations", async ({
  appPage,
  runtime,
}) => {
  const harness = await startAgentHarness(appPage, runtime);
  try {
    const firstTerminal = await openAgentTerminal(harness, "shared-input-a");
    const firstWrite = await harness.client.callTool({
      name: "compozy__terminal_write",
      arguments: { terminal_id: firstTerminal, data: "printf 'shared-first\\n'\\n" },
      _meta: { toolCallId: "e2e-terminal-first-write" },
    });
    expect(firstWrite.isError).toBeFalsy();
    await expect(harness.sessionUI.permissionPrompt).toBeHidden();

    const followUp = await harness.client.callTool({
      name: "compozy__terminal_write",
      arguments: { terminal_id: firstTerminal, data: "printf 'shared-follow-up\\n'\\n" },
      _meta: { toolCallId: "e2e-terminal-follow-up-write" },
    });
    expect(followUp.isError).toBeFalsy();
    await expect(harness.sessionUI.permissionPrompt).toBeHidden();

    const secondTerminal = await openAgentTerminal(harness, "shared-input-b");
    const secondWrite = await harness.client.callTool({
      name: "compozy__terminal_write",
      arguments: { terminal_id: secondTerminal, data: "printf 'shared-second-terminal\\n'\\n" },
      _meta: { toolCallId: "e2e-terminal-second-write" },
    });
    expect(secondWrite.isError).toBeFalsy();
    await expect(harness.sessionUI.permissionPrompt).toBeHidden();
  } finally {
    await teardownHostedMcp(harness.connection);
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
    // The deep link is the deterministic way to this exact terminal; ambient
    // adoption is covered by E2E-002/003/004/018.
    await appPage.goto(runtime.url(`/terminal/${encodeURIComponent(terminalId)}`), {
      waitUntil: "domcontentloaded",
    });
    // The deep link focuses the reconciled window itself; ensure would dock-
    // click a focused window and minimize it.
    const terminalWindow = appPage.locator(
      '[data-slot="os-window-surface"][data-app="terminal"][data-stack-active]'
    );
    await expect(terminalWindow).toBeVisible({ timeout: 20_000 });
    await expect(terminalWindow.getByTestId(`terminal-pane-${terminalId}`)).toBeVisible({
      timeout: 20_000,
    });
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
    // Both agent-opened terminals were materialized as their own windows, so
    // the dock activation restores them rather than adopting one. The pending
    // question is pinned right under the input terminal's grid; the dock badge
    // is the cross-window attention surface.
    const terminalLauncher = appPage.locator('[data-slot="os-dock-item"][data-app="terminal"]');
    await terminalLauncher.click();
    let terminalWindow = terminalWindowFor(appPage, inputTerminalId);
    await expect(terminalWindow.getByTestId(`terminal-pane-${inputTerminalId}`)).toBeVisible({
      timeout: 20_000,
    });
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
    // Profile-b has its own desktop and no terminals: the ensured window
    // resolves straight into a fresh terminal owned by terminal-b — the other
    // profile's terminals are hidden, not closed, and never adopted here.
    terminalWindow = await ensureTerminalWindow(appPage);
    const profileBPane = terminalWindow
      .locator('[data-testid^="terminal-pane-term-"]:visible')
      .first();
    await expect(profileBPane).toBeVisible({ timeout: 20_000 });
    const profileBTerminalId = (await profileBPane.getAttribute("data-testid"))?.replace(
      "terminal-pane-",
      ""
    );
    if (!profileBTerminalId) throw new Error("terminal-b did not expose its terminal id.");
    expect([defaultTerminalId, inputTerminalId]).not.toContain(profileBTerminalId);
    await expect(terminalLauncher.locator('[data-slot="os-dock-badge"]')).toHaveCount(0);
    const profileBLog = await interactiveTerminalLog(terminalWindow);
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
    // Back on default, its own desktop returns with the original window still
    // showing the input terminal and its pending question.
    terminalWindow = terminalWindowFor(appPage, inputTerminalId);
    await expect(terminalWindow.getByTestId(`terminal-pane-${inputTerminalId}`)).toBeVisible({
      timeout: 20_000,
    });
    await expect(terminalLauncher.locator('[data-slot="os-dock-badge"]')).toHaveText("1");
    expect(
      (await terminalScreen(runtime, harness.workspace.id, defaultTerminalId)).content
    ).toContain("profile-default-row");

    await profiles.switcher.click();
    await profiles.switcherAll.click();
    await expect(profiles.switcher).toContainText("All profiles");
    // The aggregate lens changes the data, not the desktop: the default
    // profile's windows stay, and any of them journals every owner.
    terminalWindow = terminalWindowFor(appPage, inputTerminalId);
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
    // The deep link is the way to a specific terminal; a dock activation then
    // restores the window in case an earlier gesture left it minimized.
    await appPage.goto(runtime.url(`/terminal/${encodeURIComponent(inputTerminalId)}`), {
      waitUntil: "domcontentloaded",
    });
    terminalWindow = terminalWindowFor(appPage, inputTerminalId);
    await expect(terminalWindow.getByTestId(`terminal-pane-${inputTerminalId}`)).toBeVisible({
      timeout: 20_000,
    });
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
