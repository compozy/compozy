// Suite: integrated terminal public activation.
// Invariant: the production browser loads Terminal only on demand and its CSP
// admits only the same-origin terminal socket.
// Owning layer: browser OS shell. Canonical suite: this terminal E2E file.
import { execFile, spawn, type ChildProcessWithoutNullStreams } from "node:child_process";
import { chmod, mkdtemp, writeFile } from "node:fs/promises";
import os from "node:os";
import path from "node:path";
import process from "node:process";
import { fileURLToPath } from "node:url";
import { promisify } from "node:util";

import type { Locator, Page } from "@playwright/test";

import { reloadDaemonServedPage } from "../fixtures/navigation";
import {
  focusWindowThroughPalette,
  openAppWindow,
  switchWorkspace,
  windowFrame,
  windowID,
} from "../fixtures/os-navigation";
import {
  seedBrowserSandboxProfiles,
  type BrowserRuntime,
  type RuntimePaths,
} from "../fixtures/runtime";
import { closeTerminalWatchers, connectTerminalWatcher } from "../fixtures/terminal-watcher";
import { expect, test } from "../fixtures/test";
import { ensureProjectWorkspace } from "../fixtures/workspace";
import { TERMINAL_SUBPROTOCOL } from "../../src/generated/terminal-wire";

const execFileAsync = promisify(execFile);
const repositoryRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "../../..");
const macOSPtyBridge = path.join(repositoryRoot, "web/e2e/fixtures/pty-bridge.exp");
const CLI_EXIT_FAILURE = 1;
const CLI_EXIT_DATA_ERROR = 65;
const CLI_EXIT_UNAVAILABLE = 69;
const CLI_EXIT_CONFIG_INVALID = 78;

interface TerminalRecord {
  capabilities: { interactive: boolean };
  controller: { kind: string; id: string } | null;
  cwd: string;
  id: string;
  lease: string;
  mode: string;
  profile_name: string;
  shell: string;
  state: string;
  title: string;
  viewers: number;
  workspace_id: string;
}

interface TerminalListEnvelope {
  terminals: TerminalRecord[];
}

interface TerminalEnvelope {
  terminal: TerminalRecord;
}

interface TerminalJournalEnvelope {
  entries: Array<{
    actor: { kind: string; id: string };
    approval: string;
    command: string;
    command_id: string;
    detected_by: string;
    exit_cause: string;
    exit_code: number | null;
    profile_name: string;
    recording?: string | null;
    terminal_id: string | null;
  }>;
  next: string | null;
}

interface TerminalReadEnvelope {
  busy: boolean;
  content: string;
  seq: number;
  truncated: boolean;
  untrusted: boolean;
}

interface SettingsRestartAction {
  status_url: string;
}

interface SettingsRestartStatus {
  status: string;
}

interface InteractiveCLI {
  child: ChildProcessWithoutNullStreams;
  output: () => string;
  waitForExit: () => Promise<number>;
  waitForOutput: (needle: string | RegExp) => Promise<string>;
  write: (input: string | Uint8Array) => Promise<void>;
}

function assertLaunchRuntime(
  runtime: BrowserRuntime
): asserts runtime is BrowserRuntime & { paths: RuntimePaths } {
  if (!runtime.paths) throw new Error("Terminal E2E requires a launch-mode runtime.");
}

function cliEnv(paths: RuntimePaths): NodeJS.ProcessEnv {
  return {
    ...process.env,
    COMPOZY_HOME: paths.homeDir,
    HOME: paths.operatorHomeDir,
    PATH: [path.dirname(paths.cliShim), process.env.PATH ?? ""]
      .filter(Boolean)
      .join(path.delimiter),
  };
}

async function runTerminalCLI<T>(paths: RuntimePaths, args: string[]): Promise<T> {
  const { stdout } = await execFileAsync(paths.cliShim, ["terminal", ...args], {
    env: cliEnv(paths),
  });
  return JSON.parse(stdout.trim()) as T;
}

async function runTerminalCLIFailure(paths: RuntimePaths, args: string[]) {
  try {
    await execFileAsync(paths.cliShim, ["terminal", ...args], { env: cliEnv(paths) });
  } catch (error) {
    const failure = error as { code?: number; stderr?: string };
    return { code: failure.code, payload: JSON.parse((failure.stderr ?? "").trim()) as unknown };
  }
  throw new Error(`terminal command unexpectedly succeeded: ${args.join(" ")}`);
}

async function runTerminalCLITextFailure(paths: RuntimePaths, args: string[]) {
  try {
    await execFileAsync(paths.cliShim, ["terminal", ...args], { env: cliEnv(paths) });
  } catch (error) {
    const failure = error as { code?: number; stderr?: string };
    return { code: failure.code, stderr: failure.stderr ?? "" };
  }
  throw new Error(`terminal command unexpectedly succeeded: ${args.join(" ")}`);
}

async function runtimeWorkspace(runtime: BrowserRuntime & { paths: RuntimePaths }) {
  return runtime.seeded.workspace ?? (await runtime.resolveWorkspace(runtime.paths.workspaceDir));
}

async function pollRestartStatus(runtime: BrowserRuntime, statusURL: string): Promise<string> {
  try {
    return (await runtime.requestJSON<SettingsRestartStatus>(statusURL)).status;
  } catch {
    return "restarting";
  }
}

function shellQuote(value: string): string {
  return `'${value.replaceAll("'", `'\\''`)}'`;
}

function startInteractiveCLI(paths: RuntimePaths, args: string[]): InteractiveCLI {
  const command = [paths.cliShim, ...args].map(shellQuote).join(" ");
  const launcher = process.platform === "darwin" ? "/usr/bin/expect" : "script";
  const scriptArgs =
    process.platform === "darwin"
      ? ["-f", macOSPtyBridge, paths.cliShim, ...args]
      : ["-qfec", command, "/dev/null"];
  const child = spawn(launcher, scriptArgs, {
    env: cliEnv(paths),
    stdio: "pipe",
  });
  let captured = "";
  const append = (chunk: Buffer) => {
    captured += chunk.toString("utf8").replaceAll("\r", "");
  };
  child.stdout.on("data", append);
  child.stderr.on("data", append);

  const waitForOutput = async (needle: string | RegExp): Promise<string> => {
    const matches = () =>
      typeof needle === "string" ? captured.includes(needle) : needle.test(captured);
    if (matches()) return captured;
    return await new Promise<string>((resolve, reject) => {
      const timeout = setTimeout(() => {
        cleanup();
        reject(new Error(`Timed out waiting for ${String(needle)} in:\n${captured}`));
      }, 20_000);
      const inspect = () => {
        if (!matches()) return;
        cleanup();
        resolve(captured);
      };
      const closed = (code: number | null) => {
        cleanup();
        reject(
          new Error(`Interactive CLI exited ${String(code)} before ${String(needle)}:\n${captured}`)
        );
      };
      const cleanup = () => {
        clearTimeout(timeout);
        child.stdout.off("data", inspect);
        child.stderr.off("data", inspect);
        child.off("close", closed);
      };
      child.stdout.on("data", inspect);
      child.stderr.on("data", inspect);
      // `exit` may precede the final stdout/stderr chunks. `close` is the first
      // lifecycle event that proves both streams have drained.
      child.on("close", closed);
    });
  };

  return {
    child,
    output: () => captured,
    waitForExit: async () =>
      await new Promise<number>((resolve, reject) => {
        if (child.exitCode !== null) {
          resolve(child.exitCode);
          return;
        }
        child.once("error", reject);
        child.once("exit", code => resolve(code ?? 1));
      }),
    waitForOutput,
    write: async input =>
      await new Promise<void>((resolve, reject) => {
        child.stdin.write(input, error => {
          if (error) {
            reject(error);
            return;
          }
          resolve();
        });
      }),
  };
}

async function visibleTerminalPaneID(window: Locator): Promise<string> {
  const pane = window.locator('[data-testid^="terminal-pane-term-"]:visible').first();
  await pane.waitFor({ state: "visible" });
  const testId = await pane.getAttribute("data-testid");
  const prefix = "terminal-pane-";
  if (!testId?.startsWith(`${prefix}term-`)) {
    throw new Error(`Unexpected terminal pane id: ${testId}`);
  }
  return testId.slice(prefix.length);
}

async function terminalScreen(runtime: BrowserRuntime, workspaceId: string, terminalId: string) {
  return await runtime.requestJSON<TerminalReadEnvelope>(
    `/api/workspaces/${encodeURIComponent(workspaceId)}/terminals/${encodeURIComponent(
      terminalId
    )}/read?view=screen&profile=default`
  );
}

async function takeTerminalControl(window: Locator): Promise<Locator> {
  const log = window.locator('[role="log"]:visible').last();
  await expect(log).toBeVisible();
  await expect(log).toHaveAttribute("data-readonly", /^(true|false)$/);
  if ((await log.getAttribute("data-readonly")) === "true") {
    const takeControl = window.getByTestId("terminal-take-control").last();
    await expect(takeControl).toBeVisible();
    await takeControl.click();
  }
  await expect(log).toHaveAttribute("data-readonly", "false");
  return log;
}

function focusedTerminalWindow(page: Page): Locator {
  return page.locator(
    '[data-slot="os-window-frame"][data-focused] [data-slot="os-window-surface"][data-app="terminal"][data-stack-active]'
  );
}

function structuredTerminalErrorCode(payload: unknown): string | undefined {
  if (payload === null || typeof payload !== "object") return undefined;
  const record = payload as Record<string, unknown>;
  if (typeof record.code === "string") return record.code;
  if (record.error === null || typeof record.error !== "object") return undefined;
  const nested = record.error as Record<string, unknown>;
  return typeof nested.code === "string" ? nested.code : undefined;
}

test("E2E-001: CLI golden path opens, runs, lists, and journals a terminal", async ({
  runtime,
}) => {
  assertLaunchRuntime(runtime);
  const workspace = await runtimeWorkspace(runtime);
  const attached = startInteractiveCLI(runtime.paths, [
    "terminal",
    "open",
    "--workspace",
    workspace.id,
    "--title",
    "e2e-golden",
  ]);
  await attached.waitForOutput(/opened .* — attached\. Detach: Ctrl-\\ Ctrl-\\/u);
  await attached.write("printf 'terminal-golden-path\\n'\n");
  await attached.waitForOutput("terminal-golden-path");
  await attached.write("\u001c\u001c");
  await attached.waitForOutput("[detached — terminal keeps running]");
  expect(await attached.waitForExit()).toBe(0);

  const listed = await runTerminalCLI<TerminalListEnvelope>(runtime.paths, [
    "list",
    "--workspace",
    workspace.id,
    "-o",
    "json",
  ]);
  const opened = listed.terminals.find(terminal => terminal.title === "e2e-golden");
  expect(opened).toMatchObject({ state: "running", mode: "pty" });
  if (!opened) throw new Error("Attached terminal was absent from the list.");
  const history = await runTerminalCLI<{ quote: string }>(runtime.paths, [
    "quote",
    opened.id,
    "--workspace",
    workspace.id,
    "--lines",
    "1-200",
    "-o",
    "json",
  ]);
  expect(history.quote).toContain("terminal-golden-path");
  await expect
    .poll(async () => {
      const journal = await runTerminalCLI<TerminalJournalEnvelope>(runtime.paths, [
        "journal",
        "--workspace",
        workspace.id,
        "--since",
        "1h",
        "-o",
        "json",
      ]);
      return journal.entries.find(entry => entry.command.includes("terminal-golden-path"));
    })
    .toMatchObject({
      terminal_id: opened.id,
      actor: { kind: "human" },
      approval: "human",
      detected_by: "marker",
      exit_code: 0,
    });
});

test("E2E-002: browser keeps two terminal windows across reload and reattaches after close", async ({
  appPage,
  runtime,
}) => {
  assertLaunchRuntime(runtime);
  const workspace = await runtimeWorkspace(runtime);
  await ensureProjectWorkspace(appPage, runtime);
  // Opening the Terminal app lands in a working terminal directly: the id-less
  // route creates one, with no launcher or empty state in between.
  const createResponsePromise = appPage.waitForResponse(
    response =>
      response.request().method() === "POST" &&
      new URL(response.url()).pathname === `/api/workspaces/${workspace.id}/terminals`
  );
  await openAppWindow(appPage, "Terminal", "terminal");
  expect((await createResponsePromise).status()).toBe(201);
  const firstActiveWindow = focusedTerminalWindow(appPage);
  const firstID = await visibleTerminalPaneID(firstActiveWindow);
  await focusWindowThroughPalette(appPage, firstActiveWindow);
  let window = firstActiveWindow;
  const firstLog = await takeTerminalControl(window);
  await firstLog.click();
  await appPage.keyboard.type("printf 'first-screen-intact\\n'");
  await appPage.keyboard.press("Enter");
  await expect
    .poll(async () => (await terminalScreen(runtime, workspace.id, firstID)).content)
    .toContain("first-screen-intact");

  // Another terminal joins this frame as an OS window tab — the deck is the
  // only tab strip anywhere.
  const secondCreatePromise = appPage.waitForResponse(
    response =>
      response.request().method() === "POST" &&
      new URL(response.url()).pathname === `/api/workspaces/${workspace.id}/terminals`
  );
  await window.getByTestId("terminal-new").click();
  expect((await secondCreatePromise).status()).toBe(201);
  const frame = appPage.locator('[data-slot="os-window-frame"][data-focused]');
  await expect(frame.locator('[data-testid^="os-window-tab-"]')).toHaveCount(2);
  const secondActiveWindow = focusedTerminalWindow(appPage);
  const secondID = await visibleTerminalPaneID(secondActiveWindow);
  expect(secondID).not.toBe(firstID);
  window = secondActiveWindow;
  const secondLog = await takeTerminalControl(window);
  await secondLog.click();
  await appPage.keyboard.type("printf 'second-screen-intact\\n'");
  await appPage.keyboard.press("Enter");
  await expect
    .poll(async () => (await terminalScreen(runtime, workspace.id, secondID)).content)
    .toContain("second-screen-intact");
  for (let index = 0; index < 8; index += 1) {
    await frame
      .locator('[data-testid^="os-window-tab-"] [role="tab"]')
      .nth(index % 2)
      .click();
  }

  await appPage.reload({ waitUntil: "domcontentloaded" });
  const restoredFrame = appPage.locator('[data-slot="os-window-frame"][data-focused]');
  await expect(restoredFrame.locator('[data-testid^="os-window-tab-"]')).toHaveCount(2);
  expect((await terminalScreen(runtime, workspace.id, firstID)).content).toContain(
    "first-screen-intact"
  );
  expect((await terminalScreen(runtime, workspace.id, secondID)).content).toContain(
    "second-screen-intact"
  );
  const running = await runTerminalCLI<TerminalListEnvelope>(runtime.paths, [
    "list",
    "--workspace",
    workspace.id,
    "-o",
    "json",
  ]);
  expect(running.terminals.filter(terminal => [firstID, secondID].includes(terminal.id))).toEqual(
    expect.arrayContaining([
      expect.objectContaining({ id: firstID, state: "running" }),
      expect.objectContaining({ id: secondID, state: "running" }),
    ])
  );

  // Closing the window is a window gesture: both sessions keep running.
  const restored = focusedTerminalWindow(appPage);
  await focusWindowThroughPalette(appPage, restored);
  const closingWindow = appPage.getByTestId(`os-window-${await windowID(restored)}`);
  await closingWindow.getByRole("button", { name: "Close window" }).click();
  await expect(closingWindow).toBeHidden();
  const survivors = await runTerminalCLI<TerminalListEnvelope>(runtime.paths, [
    "list",
    "--workspace",
    workspace.id,
    "-o",
    "json",
  ]);
  expect(
    survivors.terminals.filter(
      terminal => [firstID, secondID].includes(terminal.id) && terminal.state === "running"
    )
  ).toHaveLength(2);

  // Reopening from the dock adopts the newest running session instead of
  // opening a launcher: the screen is the same one, still intact.
  const reopened = await openAppWindow(appPage, "Terminal", "terminal");
  expect(await visibleTerminalPaneID(reopened)).toBe(secondID);
  const firstQuote = await runTerminalCLI<{ quote: string }>(runtime.paths, [
    "quote",
    firstID,
    "--workspace",
    workspace.id,
    "--lines",
    "1-200",
    "-o",
    "json",
  ]);
  expect(firstQuote.quote).toContain("first-screen-intact");
});

test("E2E-007: journal filters update the real browser query", async ({ appPage, runtime }) => {
  assertLaunchRuntime(runtime);
  const workspace = await runtimeWorkspace(runtime);
  const general = await runtime.requestJSON<{
    config: Record<string, unknown> & {
      terminal: Record<string, unknown>;
    };
  }>("/api/settings/general");
  await runtime.requestJSON("/api/settings/general", {
    method: "PATCH",
    body: JSON.stringify({
      config: {
        ...general.config,
        terminal: { ...general.config.terminal, shell_integration: false },
      },
    }),
  });
  const restart = await runtime.requestJSON<SettingsRestartAction>(
    "/api/settings/actions/restart",
    { method: "POST", body: "{}" }
  );
  await expect
    .poll(async () => await pollRestartStatus(runtime, restart.status_url), { timeout: 45_000 })
    .toBe("ready");
  await reloadDaemonServedPage(appPage, runtime, "/", { readyTestId: "os-desktop" });
  await runTerminalCLI<unknown>(runtime.paths, [
    "exec",
    "--workspace",
    workspace.id,
    "-o",
    "json",
    "--",
    "/bin/sh",
    "-c",
    "false",
  ]);
  const terminal = await runTerminalCLI<TerminalEnvelope>(runtime.paths, [
    "open",
    "--workspace",
    workspace.id,
    "--shell",
    "/bin/sh",
    "--detach",
    "-o",
    "json",
  ]);
  const terminalID = terminal.terminal.id;
  await ensureProjectWorkspace(appPage, runtime);
  await openAppWindow(appPage, "Terminal", "terminal");
  const window = focusedTerminalWindow(appPage);
  // The dock adopts the newest running detached terminal — the CLI-opened one.
  await expect(window.getByTestId(`terminal-pane-${terminalID}`)).toBeVisible();
  await takeTerminalControl(window);
  await runTerminalCLI(runtime.paths, [
    "record",
    "start",
    terminalID,
    "--workspace",
    workspace.id,
    "-o",
    "json",
  ]);
  await window.locator('[role="log"]:visible').last().click();
  await appPage.keyboard.type("false", { delay: 50 });
  await appPage.keyboard.press("Enter");
  await expect
    .poll(async () => (await terminalScreen(runtime, workspace.id, terminalID)).content)
    .toContain("false");
  let observedEntries: TerminalJournalEnvelope["entries"] = [];
  await expect
    .poll(async () => {
      const journal = await runTerminalCLI<TerminalJournalEnvelope>(runtime.paths, [
        "journal",
        "--workspace",
        workspace.id,
        "-o",
        "json",
      ]);
      observedEntries = journal.entries;
      return journal.entries;
    })
    .toEqual(
      expect.arrayContaining([
        expect.objectContaining({
          terminal_id: terminalID,
          command: "false",
          actor: expect.objectContaining({ kind: "human" }),
          detected_by: "idle",
        }),
      ])
    );
  const failedRow = observedEntries.find(
    entry => entry.terminal_id === terminalID && entry.command === "false"
  );
  if (!failedRow) throw new Error("Failed approximate journal row was not created.");
  const exactFailedRow = observedEntries.find(
    entry => entry.terminal_id === null && entry.detected_by === "exact" && entry.exit_code === 1
  );
  if (!exactFailedRow) throw new Error("Exact failed journal row was not created.");
  await runTerminalCLI(runtime.paths, [
    "record",
    "stop",
    terminalID,
    "--workspace",
    workspace.id,
    "-o",
    "json",
  ]);

  await window.getByTestId("terminal-journal-toggle").click();
  await expect(window.getByTestId("terminal-journal")).toBeVisible();
  await window.getByTestId("terminal-journal-filters-add").click();
  await appPage.getByRole("option", { name: "Who" }).click();
  await appPage.getByRole("option", { name: "A person" }).click();
  await expect(window.getByTestId(`terminal-journal-row-${failedRow.command_id}`)).toBeVisible();
  await expect(
    window.getByTestId(`terminal-journal-confidence-${failedRow.command_id}`)
  ).toHaveText("estimated");
  const replay = window.getByTestId(`terminal-journal-replay-${failedRow.command_id}`);
  await expect(replay).toBeVisible();
  await replay.click();
  await expect(window.getByTestId("terminal-recording-player")).toBeVisible();
  await window.getByTestId("terminal-recording-open-journal").click();
  await expect(window.getByTestId("terminal-journal-detail")).toBeVisible();
  await appPage.keyboard.press("Escape");
  await expect(window.getByTestId("terminal-journal-detail")).toBeHidden();

  await window.getByTestId("terminal-journal-filters-add").click();
  await appPage.getByRole("option", { name: "Result" }).click();
  await appPage.getByRole("option", { name: "Finished with errors" }).click();
  await expect(
    window.getByTestId(`terminal-journal-row-${exactFailedRow.command_id}`)
  ).toBeVisible();

  await window.getByTestId("terminal-journal-filters-add").click();
  await appPage.getByRole("option", { name: "Terminal" }).click();
  await appPage.getByPlaceholder("term-…").fill("term-000000000000");
  await expect(window.getByTestId("terminal-journal-filtered-empty")).toContainText(
    "No matches in the rows loaded"
  );
});

test("E2E-009: the workspace cap names the terminal that can be closed", async ({
  appPage,
  runtime,
}) => {
  assertLaunchRuntime(runtime);
  const workspace = await runtimeWorkspace(runtime);
  const terminals: TerminalRecord[] = [];
  for (let index = 1; index <= 8; index += 1) {
    const opened = await runTerminalCLI<TerminalEnvelope>(runtime.paths, [
      "open",
      "--workspace",
      workspace.id,
      "--title",
      `cap-owner-${index}`,
      "--detach",
      "-o",
      "json",
    ]);
    terminals.push(opened.terminal);
  }

  await ensureProjectWorkspace(appPage, runtime);
  const window = await openAppWindow(appPage, "Terminal", "terminal");
  // The dock adopts one of the running terminals; asking for a fresh one at
  // the cap surfaces the way out instead of a dead create.
  await visibleTerminalPaneID(window);
  await window.getByTestId("terminal-new").click();
  const dialog = appPage.getByTestId("terminal-limit-dialog");
  await expect(dialog).toBeVisible();
  await expect(dialog).toContainText(terminals[0]!.id);
  await expect(dialog).toContainText(terminals[7]!.id);
  await expect(dialog).toContainText("8 of 8 terminals are open");
  await expect(dialog).toContainText("terminal.max_per_workspace 8");
  await appPage.keyboard.press("Escape");
  const capFrame = appPage.locator('[data-slot="os-window-frame"][data-focused]');
  await capFrame.getByRole("button", { name: "Close window" }).click();

  try {
    for (let index = 0; index < 16; index += 1) {
      await connectTerminalWatcher(appPage, runtime, workspace.id, terminals[0]!.id);
    }
    const attachPath = `/api/workspaces/${encodeURIComponent(
      workspace.id
    )}/terminals/${encodeURIComponent(terminals[0]!.id)}/attach-ticket?profile=default`;
    const rejected = await appPage.request.post(runtime.url(attachPath), {
      data: { mode: "read" },
    });
    expect(rejected.status()).toBe(409);
    expect(await rejected.json()).toMatchObject({
      error: {
        code: "subscriber_limit_reached",
        message: "terminal subscriber limit reached",
        details: { current: 16, max: 16 },
      },
    });
  } finally {
    await closeTerminalWatchers(appPage);
  }
});

test("E2E-011: CLI attach supports watch, control, detach, and single SIGQUIT", async ({
  runtime,
}) => {
  assertLaunchRuntime(runtime);
  const workspace = await runtimeWorkspace(runtime);
  const shellFixture = path.join(runtime.paths.workspaceDir, ".terminal-e2e-sigquit.sh");
  await writeFile(
    shellFixture,
    [
      "#!/bin/sh",
      "trap 'printf \"single-sigquit-received\\n\"' QUIT",
      "while :; do",
      "  if IFS= read -r line; then",
      '    eval "$line"',
      "  fi",
      "done",
      "",
    ].join("\n"),
    { mode: 0o700 }
  );
  await chmod(shellFixture, 0o700);
  const opened = await runTerminalCLI<TerminalEnvelope>(runtime.paths, [
    "open",
    "--workspace",
    workspace.id,
    "--shell",
    shellFixture,
    "--title",
    "attach-modes",
    "--detach",
    "-o",
    "json",
  ]);

  const watching = startInteractiveCLI(runtime.paths, [
    "terminal",
    "attach",
    opened.terminal.id,
    "--workspace",
    workspace.id,
  ]);
  await watching.waitForOutput(`[watching ${opened.terminal.id} — controller: human operator.`);
  await watching.write("ignored-in-watch-mode");
  await watching.write("\u001c\u001c");
  await watching.waitForOutput("[detached — terminal keeps running]");
  expect(await watching.waitForExit()).toBe(0);
  expect((await terminalScreen(runtime, workspace.id, opened.terminal.id)).content).not.toContain(
    "ignored-in-watch-mode"
  );

  const controlling = startInteractiveCLI(runtime.paths, [
    "terminal",
    "attach",
    opened.terminal.id,
    "--workspace",
    workspace.id,
    "--control",
  ]);
  await controlling.waitForOutput("[control taken from human operator — you type now]");
  await controlling.write("printf 'controlled-input-received\\n'\n");
  await controlling.waitForOutput("controlled-input-received");
  await controlling.write("\u001c");
  await controlling.waitForOutput("single-sigquit-received");
  await controlling.write("\u001c\u001c");
  await controlling.waitForOutput("[detached — terminal keeps running]");
  expect(await controlling.waitForExit()).toBe(0);
  expect((await terminalScreen(runtime, workspace.id, opened.terminal.id)).content).toContain(
    "controlled-input-received"
  );

  await runTerminalCLI(runtime.paths, [
    "kill",
    opened.terminal.id,
    "--workspace",
    workspace.id,
    "-o",
    "json",
  ]);
  const exited = await runTerminalCLITextFailure(runtime.paths, [
    "attach",
    opened.terminal.id,
    "--workspace",
    workspace.id,
  ]);
  // HTTP 409 is a caller-state conflict and maps to the CLI's stable EX_DATAERR code.
  expect(exited.code).toBe(65);
  expect(exited.stderr).toContain("terminal_exited");
  expect(exited.stderr).toMatch(/exited|signaled/u);
});

test("E2E-012: a sandbox project hides interactive controls but still executes", async ({
  appPage,
  runtime,
}) => {
  assertLaunchRuntime(runtime);
  const sandboxProfile = "terminal-execute-only";
  await seedBrowserSandboxProfiles(runtime, [
    {
      name: sandboxProfile,
      profile: { backend: "local", persistence: "reuse", sync_mode: "none" },
    },
  ]);
  const sandboxRoot = await mkdtemp(path.join(os.tmpdir(), "compozy-terminal-sandbox-"));
  const workspace = await runtime.resolveWorkspace(sandboxRoot);
  await runtime.requestJSON(`/api/workspaces/${encodeURIComponent(workspace.id)}`, {
    method: "PATCH",
    body: JSON.stringify({ sandbox_ref: sandboxProfile }),
  });
  await ensureProjectWorkspace(appPage, runtime);
  await switchWorkspace(appPage, workspace.id, workspace.name);

  const window = await openAppWindow(appPage, "Terminal", "terminal");
  await expect(window.getByTestId("terminal-execute-only")).toBeVisible();
  await expect(window.getByTestId("terminal-empty-open")).toHaveCount(0);
  await expect(window.getByTestId("terminal-new")).toHaveCount(0);

  const execution = await runTerminalCLI<{ exit_code: number; output: string }>(runtime.paths, [
    "exec",
    "--workspace",
    workspace.id,
    "--yield",
    "5s",
    "-o",
    "json",
    "--",
    "printf",
    "sandbox-exec-output\\n",
  ]);
  expect(execution).toMatchObject({ exit_code: 0, output: "sandbox-exec-output\n" });
});

test("E2E-014: alternate-screen TUI reflows, matches a watcher, and restores primary", async ({
  appPage,
  runtime,
}) => {
  test.setTimeout(150_000);
  assertLaunchRuntime(runtime);
  const workspace = await runtimeWorkspace(runtime);
  const wrapper = path.join(runtime.paths.workspaceDir, ".terminal-e2e-tui.sh");
  await writeFile(
    wrapper,
    `#!/bin/sh\ncd ${shellQuote(repositoryRoot)} || exit 1\nexec go run ./internal/terminal/testdata/tui hold\n`,
    { mode: 0o700 }
  );
  await chmod(wrapper, 0o700);
  const opened = await runTerminalCLI<TerminalEnvelope>(runtime.paths, [
    "open",
    "--workspace",
    workspace.id,
    "--shell",
    wrapper,
    "--title",
    "tui-fidelity",
    "--detach",
    "-o",
    "json",
  ]);

  await ensureProjectWorkspace(appPage, runtime);
  await openAppWindow(appPage, "Terminal", "terminal");
  const firstWindow = focusedTerminalWindow(appPage);
  await expect(firstWindow.getByTestId(`terminal-pane-${opened.terminal.id}`)).toBeVisible();
  await expect
    .poll(async () => (await terminalScreen(runtime, workspace.id, opened.terminal.id)).content)
    .toContain("terminal tui");
  expect((await terminalScreen(runtime, workspace.id, opened.terminal.id)).content).toContain(
    "second row"
  );
  await takeTerminalControl(firstWindow);
  const originalGrid = await connectTerminalWatcher(
    appPage,
    runtime,
    workspace.id,
    opened.terminal.id
  );
  try {
    await expect(firstWindow.getByTestId("terminal-viewers")).toContainText("2");
    await expect(firstWindow.getByTestId("terminal-size-vote")).toContainText(
      `${originalGrid.cols}×${originalGrid.rows}`
    );
  } finally {
    await closeTerminalWatchers(appPage);
  }
  const resizeHandle = windowFrame(firstWindow)
    .locator("xpath=..")
    .locator('[style*="cursor: se-resize"]');
  await expect(resizeHandle).toBeVisible();
  const handleBox = await resizeHandle.boundingBox();
  if (!handleBox) throw new Error("Terminal window resize handle has no layout box.");
  const resizeX = handleBox.x + handleBox.width / 2;
  const resizeY = handleBox.y + handleBox.height / 2;
  await appPage.mouse.move(resizeX, resizeY);
  await appPage.mouse.down();
  await appPage.mouse.move(resizeX - 160, resizeY - 80, { steps: 12 });
  await appPage.mouse.up();
  const watcherGrid = await connectTerminalWatcher(
    appPage,
    runtime,
    workspace.id,
    opened.terminal.id
  );
  try {
    expect(watcherGrid).not.toEqual(originalGrid);
    await expect(firstWindow.getByTestId("terminal-viewers")).toContainText("2");
    await expect(firstWindow.getByTestId("terminal-size-vote")).toContainText(
      `${watcherGrid.cols}×${watcherGrid.rows}`
    );
  } finally {
    await closeTerminalWatchers(appPage);
  }

  await takeTerminalControl(firstWindow);
  await firstWindow.getByRole("log").click();
  await appPage.keyboard.type("x");
  await appPage.keyboard.press("Enter");
  await expect
    .poll(async () => (await terminalScreen(runtime, workspace.id, opened.terminal.id)).content)
    .toContain("primary screen");
  await expect(firstWindow.getByTestId("terminal-exit-bar")).toBeVisible({ timeout: 20_000 });
});

test("E2E-015: terminal settings expose defaults and reject an invalid limit", async ({
  appPage,
  runtime,
}) => {
  const general = await runtime.requestJSON<{
    config: { terminal: { max_per_workspace: number; recording_retention_days: number } };
  }>("/api/settings/general");
  expect(general.config.terminal).toMatchObject({
    max_per_workspace: 8,
    recording_retention_days: 30,
  });
  await ensureProjectWorkspace(appPage, runtime);
  const settings = await openAppWindow(appPage, "Settings", "settings");
  await settings.getByTestId("settings-section-nav").getByText("Terminal", { exact: true }).click();
  await expect(settings.getByTestId("settings-terminal-default-shell")).toBeVisible();
  const limit = settings.getByTestId("settings-terminal-max-per-workspace");
  await expect(limit).toHaveValue("8");
  await limit.fill("0");
  const limitRow = settings.getByTestId("settings-terminal-max-per-workspace-row");
  await expect(limitRow.getByRole("alert")).toHaveText("Value must be 1 or greater.");
  await expect(settings.getByTestId("settings-page-terminal-save")).toBeDisabled();
});

test("E2E-016: CLI returns structured selector, timeout, and missing-terminal failures", async ({
  runtime,
}) => {
  assertLaunchRuntime(runtime);
  const workspace = await runtimeWorkspace(runtime);
  expect(
    await runTerminalCLI<TerminalListEnvelope>(runtime.paths, [
      "list",
      "--workspace",
      workspace.id,
      "-o",
      "json",
    ])
  ).toEqual({ terminals: [] });
  expect(
    await runTerminalCLI<TerminalJournalEnvelope>(runtime.paths, [
      "journal",
      "--workspace",
      workspace.id,
      "-o",
      "json",
    ])
  ).toEqual({ entries: [], next: null });
  expect(
    await runTerminalCLI<{ pending: unknown[]; resolved: unknown[] }>(runtime.paths, [
      "input-requests",
      "--workspace",
      workspace.id,
      "-o",
      "json",
    ])
  ).toEqual({ pending: [], resolved: [] });

  const cases: Array<{ args: string[]; code: string; exitCode: number }> = [
    {
      args: [
        "list",
        "--workspace",
        workspace.id,
        "--profile",
        "default",
        "--all-profiles",
        "-o",
        "json",
      ],
      code: "profile_selection_conflict",
      exitCode: CLI_EXIT_FAILURE,
    },
    {
      args: [
        "open",
        "--workspace",
        workspace.id,
        "--cwd",
        "services/gone",
        "--detach",
        "-o",
        "json",
      ],
      code: "invalid_cwd",
      exitCode: CLI_EXIT_CONFIG_INVALID,
    },
    {
      args: ["exec", "--workspace", workspace.id, "--yield", "100ms", "-o", "json", "--", "true"],
      code: "timeout_out_of_range",
      exitCode: CLI_EXIT_FAILURE,
    },
    {
      args: ["get", "term-doesnotexist", "--workspace", workspace.id, "-o", "json"],
      code: "terminal_not_found",
      exitCode: CLI_EXIT_UNAVAILABLE,
    },
  ];
  for (const testCase of cases) {
    const failure = await runTerminalCLIFailure(runtime.paths, testCase.args);
    expect(failure.code).toBe(testCase.exitCode);
    expect(structuredTerminalErrorCode(failure.payload)).toBe(testCase.code);
  }

  const shellFixture = path.join(runtime.paths.workspaceDir, ".terminal-e2e-flags.sh");
  await writeFile(shellFixture, "#!/bin/sh\nprintf 'quote-one\\nquote-two\\n'\nexec /bin/sh\n", {
    mode: 0o700,
  });
  await chmod(shellFixture, 0o700);
  const opened = await runTerminalCLI<TerminalEnvelope>(runtime.paths, [
    "open",
    "--workspace",
    workspace.id,
    "--shell",
    shellFixture,
    "--title",
    "flag-matrix",
    "--detach",
    "-o",
    "json",
  ]);
  expect(opened.terminal).toMatchObject({
    workspace_id: workspace.id,
    shell: shellFixture,
    title: "flag-matrix",
    state: "running",
  });
  await expect
    .poll(async () => (await terminalScreen(runtime, workspace.id, opened.terminal.id)).content)
    .toContain("quote-two");
  const quote = await runTerminalCLI<{ quote: string }>(runtime.paths, [
    "quote",
    opened.terminal.id,
    "--workspace",
    workspace.id,
    "--lines",
    "1-2",
    "-o",
    "json",
  ]);
  expect(JSON.stringify(quote)).toContain(`<terminal_context terminal=\\"${opened.terminal.id}\\"`);

  const noInput = await runTerminalCLIFailure(runtime.paths, [
    "respond",
    opened.terminal.id,
    "--workspace",
    workspace.id,
    "-o",
    "json",
  ]);
  expect(noInput).toMatchObject({
    code: CLI_EXIT_FAILURE,
  });
  expect(structuredTerminalErrorCode(noInput.payload)).toBe("input_request_not_found");

  await runTerminalCLI(runtime.paths, [
    "record",
    "start",
    opened.terminal.id,
    "--workspace",
    workspace.id,
    "-o",
    "json",
  ]);
  const recordTwice = await runTerminalCLIFailure(runtime.paths, [
    "record",
    "start",
    opened.terminal.id,
    "--workspace",
    workspace.id,
    "-o",
    "json",
  ]);
  expect(structuredTerminalErrorCode(recordTwice.payload)).toBe("recording_already_started");
  await runTerminalCLI(runtime.paths, [
    "record",
    "stop",
    opened.terminal.id,
    "--workspace",
    workspace.id,
    "-o",
    "json",
  ]);
  const stopIdle = await runTerminalCLIFailure(runtime.paths, [
    "record",
    "stop",
    opened.terminal.id,
    "--workspace",
    workspace.id,
    "-o",
    "json",
  ]);
  expect(structuredTerminalErrorCode(stopIdle.payload)).toBe("recording_not_active");

  await runTerminalCLI(runtime.paths, [
    "kill",
    opened.terminal.id,
    "--workspace",
    workspace.id,
    "-o",
    "json",
  ]);
  // Close is idempotent: killing an already-ended terminal reports its
  // recorded exit instead of a conflict. Signaling a dead process stays a
  // structured failure — there is nothing left to deliver the signal to.
  const killedAgain = await runTerminalCLI<{ exit: { cause: string } }>(runtime.paths, [
    "kill",
    opened.terminal.id,
    "--workspace",
    workspace.id,
    "-o",
    "json",
  ]);
  expect(killedAgain.exit.cause).toMatch(/exited|signaled/u);
  const signalFailure = await runTerminalCLIFailure(runtime.paths, [
    "signal",
    opened.terminal.id,
    "--signal",
    "INT",
    "--workspace",
    workspace.id,
    "-o",
    "json",
  ]);
  expect(signalFailure.code).toBe(CLI_EXIT_DATA_ERROR);
  expect(structuredTerminalErrorCode(signalFailure.payload)).toBe("terminal_exited");
});

async function terminalSocketCSPProbe(page: Page) {
  return await page.evaluate(async terminalSubprotocol => {
    const violations: string[] = [];
    const onViolation = (event: SecurityPolicyViolationEvent) => {
      if (event.effectiveDirective === "connect-src") violations.push(event.blockedURI);
    };
    const attempt = async (url: string) => {
      await new Promise<void>(resolve => {
        const socket = new WebSocket(url, terminalSubprotocol);
        socket.onopen = () => {
          socket.close();
          queueMicrotask(resolve);
        };
        socket.onerror = () => queueMicrotask(resolve);
      });
    };
    document.addEventListener("securitypolicyviolation", onViolation);
    const protocol = window.location.protocol === "https:" ? "wss:" : "ws:";
    await attempt(
      `${protocol}//${window.location.host}/api/workspaces/csp-probe/terminals/term-probe/stream?mode=read&ticket=invalid`
    );
    const sameOriginViolations = violations.length;
    violations.length = 0;
    await attempt(`${protocol}//example.com/compozy-terminal-cross-origin`);
    const crossOriginViolations = violations.length;
    document.removeEventListener("securitypolicyviolation", onViolation);
    return { sameOriginViolations, crossOriginViolations };
  }, TERMINAL_SUBPROTOCOL);
}

test("E2E-017: production CSP admits the same-origin terminal socket and refuses cross-origin", async ({
  appPage,
}) => {
  const result = await terminalSocketCSPProbe(appPage);

  expect(result.sameOriginViolations).toBe(0);
  expect(result.crossOriginViolations).toBeGreaterThan(0);
});

test("E2E-019: the Terminal controller chunk loads only after its launcher opens", async ({
  appPage,
  runtime,
}) => {
  await ensureProjectWorkspace(appPage, runtime);
  const terminalChunkLoaded = async () =>
    await appPage.evaluate(() =>
      performance.getEntriesByType("resource").some(entry => entry.name.includes("terminal-window"))
    );

  expect(await terminalChunkLoaded()).toBe(false);
  const chunkResponse = appPage.waitForResponse(response =>
    /\/assets\/terminal-window-[^/]+\.js(?:\?|$)/u.test(response.url())
  );
  const [response, terminalWindow] = await Promise.all([
    chunkResponse,
    openAppWindow(appPage, "Terminal", "terminal"),
  ]);
  expect(response.ok()).toBe(true);
  await expect(terminalWindow.getByTestId("terminal-window")).toBeVisible();
});

test("E2E-018: keyboard activation opens a working terminal from the dock", async ({
  appPage,
  runtime,
}) => {
  await ensureProjectWorkspace(appPage, runtime);
  const launcher = appPage
    .locator('[data-slot="os-dock"]:visible, [data-slot="os-dock-tabbar"]:visible')
    .getByRole("button", { exact: true, name: "Terminal" });
  await launcher.focus();
  await appPage.keyboard.press("Enter");

  const terminalWindow = appPage.locator(
    '[data-slot="os-window-surface"][data-app="terminal"][data-stack-active]'
  );
  await expect(terminalWindow).toBeVisible();
  // One activation is the whole flow: the window resolves straight into a
  // terminal, with no launcher step in between.
  await visibleTerminalPaneID(terminalWindow);

  const journalToggle = terminalWindow.getByTestId("terminal-journal-toggle");
  await journalToggle.focus();
  await expect(journalToggle).toBeFocused();

  await expect(terminalWindow.getByTestId("terminal-lease-label")).toHaveText("You're in control");
  const release = terminalWindow.getByTestId("terminal-release-control");
  await release.press("Enter");
  // No agent is bound to this terminal, so release keeps human control
  // (US-009.EC-1) while still proving the action is keyboard reachable.
  await expect(terminalWindow.getByTestId("terminal-lease-label")).toHaveText("You're in control");
});
