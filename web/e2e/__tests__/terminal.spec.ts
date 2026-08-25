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

import type { Page } from "@playwright/test";

import { openAppWindow, switchWorkspace } from "../fixtures/os-navigation";
import {
  seedBrowserSandboxProfiles,
  type BrowserRuntime,
  type RuntimePaths,
} from "../fixtures/runtime";
import { expect, test } from "../fixtures/test";
import { ensureProjectWorkspace } from "../fixtures/workspace";

const execFileAsync = promisify(execFile);
const repositoryRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "../../..");

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

async function runtimeWorkspace(runtime: BrowserRuntime & { paths: RuntimePaths }) {
  return runtime.seeded.workspace ?? (await runtime.resolveWorkspace(runtime.paths.workspaceDir));
}

function shellQuote(value: string): string {
  return `'${value.replaceAll("'", `'\\''`)}'`;
}

function startInteractiveCLI(paths: RuntimePaths, args: string[]): InteractiveCLI {
  const command = [paths.cliShim, ...args].map(shellQuote).join(" ");
  const scriptArgs =
    process.platform === "darwin"
      ? ["-q", "/dev/null", paths.cliShim, ...args]
      : ["-qfec", command, "/dev/null"];
  const child = spawn("script", scriptArgs, {
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

async function connectTerminalWatcher(
  page: Page,
  runtime: BrowserRuntime,
  workspaceId: string,
  terminalId: string
): Promise<void> {
  const ticket = await runtime.requestJSON<{ ticket: string }>(
    `/api/workspaces/${encodeURIComponent(workspaceId)}/terminals/${encodeURIComponent(
      terminalId
    )}/attach-ticket?profile=default`,
    { method: "POST", body: JSON.stringify({ mode: "read" }) }
  );
  await page.evaluate(
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
      await new Promise<void>((resolve, reject) => {
        socket.onopen = () => resolve();
        socket.onerror = () => reject(new Error("Terminal watcher failed to connect."));
      });
      const key = "__compozyTerminalE2EWatchers";
      const watchers = (Reflect.get(globalThis, key) as WebSocket[] | undefined) ?? [];
      watchers.push(socket);
      Reflect.set(globalThis, key, watchers);
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
      detected_by: "exact",
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
  const firstTab = window.locator('[data-testid^="terminal-tab-term-"]').first();
  const firstID = terminalIDFromTab(await firstTab.getAttribute("data-testid"));
  await window.getByRole("log").click();
  await appPage.keyboard.type("printf 'first-screen-intact\\n'");
  await appPage.keyboard.press("Enter");
  await expect
    .poll(async () => (await terminalScreen(runtime, workspace.id, firstID)).content)
    .toContain("first-screen-intact");

  await window.getByTestId("terminal-open").click();
  await expect(window.locator('[data-testid^="terminal-tab-term-"]')).toHaveCount(2);
  const secondTab = window.locator('[data-testid^="terminal-tab-term-"]').nth(1);
  const secondID = terminalIDFromTab(await secondTab.getAttribute("data-testid"));
  await window.getByRole("log").click();
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
  await ensureProjectWorkspace(appPage, runtime);
  const window = await openAppWindow(appPage, "Terminal", "terminal");
  await window.getByTestId("terminal-empty-open").click();
  const terminalID = terminalIDFromTab(
    await window.locator('[data-testid^="terminal-tab-term-"]').getAttribute("data-testid")
  );
  await runTerminalCLI(runtime.paths, [
    "record",
    "start",
    terminalID,
    "--workspace",
    workspace.id,
    "-o",
    "json",
  ]);
  await window.getByRole("log").click();
  await appPage.keyboard.type("false");
  await appPage.keyboard.press("Enter");
  let failedRow: TerminalJournalEnvelope["entries"][number] | undefined;
  await expect
    .poll(async () => {
      const journal = await runTerminalCLI<TerminalJournalEnvelope>(runtime.paths, [
        "journal",
        "--workspace",
        workspace.id,
        "-o",
        "json",
      ]);
      failedRow = journal.entries.find(
        entry =>
          entry.terminal_id === terminalID && entry.exit_code !== null && entry.exit_code !== 0
      );
      return failedRow;
    })
    .toMatchObject({ actor: { kind: "human" }, detected_by: "idle" });
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
  await window.getByRole("button", { name: "Filter" }).click();
  await appPage.getByTestId("terminal-journal-filter-actor").selectOption("human");
  await appPage.getByTestId("terminal-journal-filter-failed").check();
  await appPage.getByTestId("terminal-journal-filter-apply").click();
  if (!failedRow) throw new Error("Failed approximate journal row was not created.");
  await expect(window.getByTestId(`terminal-journal-row-${failedRow.command_id}`)).toBeVisible();
  await expect(
    window.getByTestId(`terminal-journal-confidence-${failedRow.command_id}`)
  ).toHaveText("approx");
  const replay = window.getByTestId(`terminal-journal-replay-${failedRow.command_id}`);
  await expect(replay).toBeVisible();
  await replay.click();
  await expect(window.getByTestId("terminal-recording-player")).toBeVisible();
  await window.getByTestId("terminal-recording-open-journal").click();

  await window.getByRole("button", { name: "Filter" }).click();
  await appPage.getByTestId("terminal-journal-filter-terminal").fill("term-000000000000");
  await appPage.getByTestId("terminal-journal-filter-apply").click();
  await expect(window.getByTestId("terminal-journal-filtered-empty")).toContainText(
    /0 matches of \d+/u
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
      error: {
        code: "subscriber_limit_reached",
        current: 16,
        max: 16,
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
  const exited = await runTerminalCLIFailure(runtime.paths, [
    "attach",
    opened.terminal.id,
    "--workspace",
    workspace.id,
    "-o",
    "json",
  ]);
  expect(exited.code).toBe(1);
  expect(exited.payload).toMatchObject({
    error: { code: "terminal_exited" },
  });
  expect(JSON.stringify(exited.payload)).toMatch(/exited|signaled/u);
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
  browser,
  runtime,
}) => {
  assertLaunchRuntime(runtime);
  const workspace = await runtimeWorkspace(runtime);
  const wrapper = path.join(runtime.paths.workspaceDir, ".terminal-e2e-tui.sh");
  const fixture = path.join(repositoryRoot, "internal", "terminal", "testdata", "tui");
  await writeFile(wrapper, `#!/bin/sh\nexec go run ${shellQuote(fixture)} hold\n`, { mode: 0o700 });
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
  const firstWindow = await openAppWindow(appPage, "Terminal", "terminal");
  await firstWindow.getByTestId(`terminal-tab-select-${opened.terminal.id}`).click();
  await expect(firstWindow.getByRole("log")).toContainText("terminal tui", { timeout: 30_000 });
  await expect(firstWindow.getByRole("log")).toContainText("second row");
  const originalGrid = await firstWindow.getByTestId("terminal-grid-chip").textContent();
  await appPage.setViewportSize({ width: 1100, height: 720 });
  await expect
    .poll(async () => await firstWindow.getByTestId("terminal-grid-chip").textContent())
    .not.toBe(originalGrid);

  const secondContext = await browser.newContext();
  try {
    const watcherPage = await secondContext.newPage();
    await watcherPage.goto(runtime.url("/"), { waitUntil: "domcontentloaded" });
    await ensureProjectWorkspace(watcherPage, runtime);
    const watcherWindow = await openAppWindow(watcherPage, "Terminal", "terminal");
    await watcherWindow.getByTestId(`terminal-tab-select-${opened.terminal.id}`).click();
    await expect(watcherWindow.getByRole("log")).toContainText("terminal tui", {
      timeout: 20_000,
    });
    expect((await terminalScreen(runtime, workspace.id, opened.terminal.id)).content).toContain(
      "terminal tui"
    );
  } finally {
    await secondContext.close();
  }

  await firstWindow.getByRole("log").click();
  await appPage.keyboard.type("x");
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
  await settings.getByTestId("settings-page-general-save").click();
  await expect(settings.getByTestId("settings-terminal-max-per-workspace-row")).toContainText(
    "max_per_workspace"
  );
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

  const cases: Array<{ args: string[]; code: string }> = [
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
    },
    {
      args: ["exec", "--workspace", workspace.id, "--yield", "100ms", "-o", "json", "--", "true"],
      code: "timeout_out_of_range",
    },
    {
      args: ["get", "term-doesnotexist", "--workspace", workspace.id, "-o", "json"],
      code: "terminal_not_found",
    },
  ];
  for (const testCase of cases) {
    const failure = await runTerminalCLIFailure(runtime.paths, testCase.args);
    expect(failure.code).toBe(1);
    expect(failure.payload).toMatchObject({ error: { code: testCase.code } });
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
    code: 1,
    payload: { error: { code: "input_request_not_found" } },
  });

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
  expect(recordTwice.payload).toMatchObject({ error: { code: "recording_already_started" } });
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
  expect(stopIdle.payload).toMatchObject({ error: { code: "recording_not_active" } });

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
    expect(failure).toMatchObject({ code: 1, payload: { error: { code: "terminal_exited" } } });
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
  const terminalWindow = await openAppWindow(appPage, "Terminal", "terminal");
  await expect(terminalWindow.getByTestId("terminal-window")).toBeVisible();
  await expect.poll(terminalChunkLoaded).toBe(true);
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
  await takeControl.focus();
  await appPage.keyboard.press("Enter");
  await expect(terminalWindow.getByTestId("terminal-lease-label")).toHaveText("You're in control");
  const release = terminalWindow.getByTestId("terminal-release-control");
  await release.focus();
  await appPage.keyboard.press("Enter");
  await expect(terminalWindow.getByTestId("terminal-lease-label")).toHaveText("No one in control");
});
