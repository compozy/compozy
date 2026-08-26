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
import { openAppWindow, switchWorkspace, windowFrame } from "../fixtures/os-navigation";
import {
  seedBrowserSandboxProfiles,
  type BrowserRuntime,
  type RuntimePaths,
} from "../fixtures/runtime";
import { expect, test } from "../fixtures/test";
import { ensureProjectWorkspace } from "../fixtures/workspace";

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
  write: (input: string | Uint8Array) => void;
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
      const exited = (code: number | null) => {
        cleanup();
        reject(
          new Error(`Interactive CLI exited ${String(code)} before ${String(needle)}:\n${captured}`)
        );
      };
      const cleanup = () => {
        clearTimeout(timeout);
        child.stdout.off("data", inspect);
        child.stderr.off("data", inspect);
        child.off("exit", exited);
      };
      child.stdout.on("data", inspect);
      child.stderr.on("data", inspect);
      child.on("exit", exited);
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
    write: input => {
      child.stdin.write(input);
    },
  };
}

function terminalIDFromTab(testId: string | null): string {
  const prefix = "terminal-tab-term-";
  if (!testId?.startsWith(prefix)) throw new Error(`Unexpected terminal tab id: ${testId}`);
  return testId.slice("terminal-tab-".length);
}

async function terminalScreen(runtime: BrowserRuntime, workspaceId: string, terminalId: string) {
  return await runtime.requestJSON<TerminalReadEnvelope>(
    `/api/workspaces/${encodeURIComponent(workspaceId)}/terminals/${encodeURIComponent(
      terminalId
    )}/read?view=screen&profile=default`
  );
}

async function takeTerminalControl(window: Locator): Promise<void> {
  await expect(async () => {
    const log = window.locator('[role="log"]:visible').last();
    if ((await log.getAttribute("data-readonly")) === "true") {
      const takeControl = window.getByTestId("terminal-take-control").last();
      await expect(takeControl).toBeVisible();
      await takeControl.click();
    }
    await expect(log).not.toHaveAttribute("data-readonly", "true");
  }).toPass({ timeout: 20_000 });
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

async function connectTerminalWatcher(
  page: Page,
  runtime: BrowserRuntime,
  workspaceId: string,
  terminalId: string
): Promise<{ cols: number; rows: number }> {
  const ticket = await runtime.requestJSON<{ ticket: string }>(
    `/api/workspaces/${encodeURIComponent(workspaceId)}/terminals/${encodeURIComponent(
      terminalId
    )}/attach-ticket?profile=default`,
    { method: "POST", body: JSON.stringify({ mode: "read" }) }
  );
  return await page.evaluate(
    async input => {
      const protocol = window.location.protocol === "https:" ? "wss:" : "ws:";
      const socket = new WebSocket(
        `${protocol}//${window.location.host}/api/workspaces/${encodeURIComponent(
          input.workspaceId
        )}/terminals/${encodeURIComponent(input.terminalId)}/stream?mode=read&flow=drop&ticket=${encodeURIComponent(
          input.ticket
        )}`,
        "compozy.terminal.v1"
      );
      socket.binaryType = "arraybuffer";
      const attached = await new Promise<{ cols: number; rows: number }>((resolve, reject) => {
        const timeout = window.setTimeout(
          () => reject(new Error("Terminal watcher did not receive its attached frame.")),
          20_000
        );
        socket.onmessage = event => {
          if (!(event.data instanceof ArrayBuffer)) return;
          const bytes = new Uint8Array(event.data);
          if (bytes[0] !== 0x02) return;
          window.clearTimeout(timeout);
          const payload = JSON.parse(new TextDecoder().decode(bytes.subarray(1))) as {
            cols: number;
            rows: number;
          };
          resolve(payload);
        };
        socket.onerror = () => reject(new Error("Terminal watcher failed to connect."));
      });
      const key = "__compozyTerminalE2EWatchers";
      const watchers = (Reflect.get(globalThis, key) as WebSocket[] | undefined) ?? [];
      watchers.push(socket);
      Reflect.set(globalThis, key, watchers);
      return attached;
    },
    { ticket: ticket.ticket, terminalId, workspaceId }
  );
}

async function closeTerminalWatchers(page: Page): Promise<void> {
  await page.evaluate(() => {
    const key = "__compozyTerminalE2EWatchers";
    const watchers = (Reflect.get(globalThis, key) as WebSocket[] | undefined) ?? [];
    for (const watcher of watchers) watcher.close();
    Reflect.deleteProperty(globalThis, key);
  });
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
  attached.write("printf 'terminal-golden-path\\n'\n");
  await attached.waitForOutput("terminal-golden-path");
  attached.write("\u001c\u001c");
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
  expect((await terminalScreen(runtime, workspace.id, opened.id)).content).toContain(
    "terminal-golden-path"
  );
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

test("E2E-002: browser keeps two terminal tabs across reload and window reattach", async ({
  appPage,
  runtime,
}) => {
  assertLaunchRuntime(runtime);
  const workspace = await runtimeWorkspace(runtime);
  await ensureProjectWorkspace(appPage, runtime);
  const window = await openAppWindow(appPage, "Terminal", "terminal");
  await expect(window.getByTestId("terminal-empty")).toBeVisible();
  await window.getByTestId("terminal-empty-open").click();
  await expect(window.locator('[data-testid^="terminal-tab-term-"]')).toHaveCount(1);
  await takeTerminalControl(window);
  const firstTab = window.locator('[data-testid^="terminal-tab-term-"]').first();
  const firstID = terminalIDFromTab(await firstTab.getAttribute("data-testid"));
  await window.locator('[role="log"]:visible').last().click();
  await appPage.keyboard.type("printf 'first-screen-intact\\n'");
  await appPage.keyboard.press("Enter");
  await expect
    .poll(async () => (await terminalScreen(runtime, workspace.id, firstID)).content)
    .toContain("first-screen-intact");

  await window.getByTestId("terminal-open").click();
  await expect(window.locator('[data-testid^="terminal-tab-term-"]')).toHaveCount(2);
  const secondTab = window.locator('[data-testid^="terminal-tab-term-"]').nth(1);
  const secondID = terminalIDFromTab(await secondTab.getAttribute("data-testid"));
  await window.getByTestId(`terminal-tab-select-${secondID}`).click();
  await takeTerminalControl(window);
  await window.locator('[role="log"]:visible').last().click();
  await appPage.keyboard.type("printf 'second-screen-intact\\n'");
  await appPage.keyboard.press("Enter");
  await expect
    .poll(async () => (await terminalScreen(runtime, workspace.id, secondID)).content)
    .toContain("second-screen-intact");
  const tabIds = await window
    .locator('[data-testid^="terminal-tab-term-"]')
    .evaluateAll(nodes => nodes.map(node => node.getAttribute("data-testid")));
  for (let index = 0; index < 8; index += 1) {
    await window.getByTestId(`terminal-tab-select-${index % 2 === 0 ? firstID : secondID}`).click();
  }

  await appPage.reload({ waitUntil: "domcontentloaded" });
  const restored = appPage.locator(
    '[data-slot="os-window-surface"][data-app="terminal"][data-stack-active]'
  );
  await expect(restored.locator('[data-testid^="terminal-tab-term-"]')).toHaveCount(2);
  expect(
    await restored
      .locator('[data-testid^="terminal-tab-term-"]')
      .evaluateAll(nodes => nodes.map(node => node.getAttribute("data-testid")))
  ).toEqual(tabIds);
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

  await restored.getByRole("button", { name: "Close window" }).click();
  await expect(restored).toBeHidden();
  const reopened = await openAppWindow(appPage, "Terminal", "terminal");
  await expect(reopened.locator('[data-testid^="terminal-tab-term-"]')).toHaveCount(2);
  expect((await terminalScreen(runtime, workspace.id, firstID)).content).toContain(
    "first-screen-intact"
  );
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
  await window.getByTestId(`terminal-tab-select-${terminalID}`).click();
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

  await window.getByTestId("terminal-tab-journal").click();
  await expect(window.getByTestId("terminal-journal")).toBeVisible();
  await window.getByRole("button", { name: "Filter", exact: true }).click();
  await appPage.getByTestId("terminal-journal-filter-actor").click();
  await appPage.getByRole("option", { name: "Human" }).click();
  await appPage.getByTestId("terminal-journal-filter-apply").click();
  await expect(window.getByTestId(`terminal-journal-row-${failedRow.command_id}`)).toBeVisible();
  await expect(
    window.getByTestId(`terminal-journal-confidence-${failedRow.command_id}`)
  ).toHaveText("estimated");
  const replay = window.getByTestId(`terminal-journal-replay-${failedRow.command_id}`);
  await expect(replay).toBeVisible();
  await replay.click();
  await expect(window.getByTestId("terminal-recording-player")).toBeVisible();
  await window.getByTestId("terminal-recording-open-journal").click();

  await window.getByRole("button", { name: "Filter", exact: true }).click();
  await appPage.getByTestId("terminal-journal-filter-failed").check();
  await appPage.getByTestId("terminal-journal-filter-apply").click();
  await expect(
    window.getByTestId(`terminal-journal-row-${exactFailedRow.command_id}`)
  ).toBeVisible();

  await window.getByRole("button", { name: "Filter", exact: true }).click();
  await appPage.getByTestId("terminal-journal-filter-terminal").fill("term-000000000000");
  await appPage.getByTestId("terminal-journal-filter-apply").click();
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
  await window.getByTestId("terminal-open").click();
  const dialog = appPage.getByTestId("terminal-limit-dialog");
  await expect(dialog).toBeVisible();
  await expect(dialog).toContainText(terminals[0]!.id);
  await expect(dialog).toContainText(terminals[7]!.id);
  await expect(dialog).toContainText("8 of 8 terminals are open");
  await expect(dialog).toContainText("terminal.max_per_workspace 8");
  await appPage.keyboard.press("Escape");
  await window.getByRole("button", { name: "Close window" }).click();

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
      error: "terminal subscriber limit reached",
      code: "subscriber_limit_reached",
      details: { current: "16", max: "16" },
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
      "trap 'printf \\\"single-sigquit-received\\\\n\\\"' QUIT",
      "while IFS= read -r line; do",
      '  eval "$line"',
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
  watching.write("ignored-in-watch-mode");
  watching.write("\u001c\u001c");
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
  controlling.write("printf 'controlled-input-received\\n'\n");
  await controlling.waitForOutput("controlled-input-received");
  controlling.write("\u001c");
  await controlling.waitForOutput("single-sigquit-received");
  controlling.write("\u001c\u001c");
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
  await expect(window.getByTestId("terminal-open")).toHaveCount(0);

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
  await firstWindow.getByTestId(`terminal-tab-select-${opened.terminal.id}`).click();
  await expect
    .poll(async () => (await terminalScreen(runtime, workspace.id, opened.terminal.id)).content)
    .toContain("terminal tui");
  expect((await terminalScreen(runtime, workspace.id, opened.terminal.id)).content).toContain(
    "second row"
  );
  await takeTerminalControl(firstWindow);
  const originalGrid = await firstWindow.getByTestId("terminal-grid-chip").textContent();
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
  await expect
    .poll(async () => await firstWindow.getByTestId("terminal-grid-chip").textContent())
    .not.toBe(originalGrid);

  const watcherGrid = await connectTerminalWatcher(
    appPage,
    runtime,
    workspace.id,
    opened.terminal.id
  );
  try {
    await expect(firstWindow.getByTestId("terminal-viewers")).toContainText("2");
    await expect(firstWindow.getByTestId("terminal-grid-chip")).toHaveText(
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
  await expect(settings.getByTestId("settings-terminal-default-shell")).toBeVisible();
  const limit = settings.getByTestId("settings-terminal-max-per-workspace");
  await expect(limit).toHaveValue("8");
  await limit.fill("0");
  const limitRow = settings.getByTestId("settings-terminal-max-per-workspace-row");
  await expect(limitRow.getByRole("alert")).toHaveText("Value must be 1 or greater.");
  await expect(settings.getByTestId("settings-page-general-save")).toBeDisabled();
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
    await runTerminalCLI<{ requests: unknown[] }>(runtime.paths, [
      "input-requests",
      "--workspace",
      workspace.id,
      "-o",
      "json",
    ])
  ).toEqual({ requests: [] });

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
  for (const args of [
    ["kill", opened.terminal.id],
    ["signal", opened.terminal.id, "--signal", "INT"],
  ]) {
    const failure = await runTerminalCLIFailure(runtime.paths, [
      ...args,
      "--workspace",
      workspace.id,
      "-o",
      "json",
    ]);
    expect(failure.code).toBe(CLI_EXIT_DATA_ERROR);
    expect(structuredTerminalErrorCode(failure.payload)).toBe("terminal_exited");
  }
});

async function terminalSocketCSPProbe(page: Page) {
  return await page.evaluate(async () => {
    const violations: string[] = [];
    const onViolation = (event: SecurityPolicyViolationEvent) => {
      if (event.effectiveDirective === "connect-src") violations.push(event.blockedURI);
    };
    const attempt = async (url: string) => {
      await new Promise<void>(resolve => {
        const socket = new WebSocket(url, "compozy.terminal.v1");
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
  });
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

test("E2E-018: keyboard activation reaches the Terminal empty-state action", async ({
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
  const open = terminalWindow.getByRole("button", { name: "Open a terminal" });
  await open.focus();
  await expect(open).toBeFocused();
  await appPage.keyboard.press("Enter");

  const terminalTab = terminalWindow.locator('[data-testid^="terminal-tab-select-term-"]');
  await expect(terminalTab).toBeVisible();
  await terminalTab.focus();
  await appPage.keyboard.press("ArrowRight");
  await expect(terminalWindow.getByTestId("terminal-tab-journal")).toBeFocused();
  await appPage.keyboard.press("ArrowLeft");
  await expect(terminalTab).toBeFocused();

  const takeControl = terminalWindow.getByTestId("terminal-take-control");
  await takeControl.press("Enter");
  await expect(terminalWindow.getByTestId("terminal-lease-label")).toHaveText("You're in control");
  const release = terminalWindow.getByTestId("terminal-release-control");
  await release.press("Enter");
  // No agent is bound to this terminal, so release keeps human control
  // (US-009.EC-1) while still proving the action is keyboard reachable.
  await expect(terminalWindow.getByTestId("terminal-lease-label")).toHaveText("You're in control");
});
