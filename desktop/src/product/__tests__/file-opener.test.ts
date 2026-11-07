import { EventEmitter } from "node:events";
import type { ChildProcess, SpawnOptions } from "node:child_process";

import { describe, expect, it, vi } from "vitest";
import type { BrowserWindow, MessageBoxOptions } from "electron";

import { EditorTemplateError, expandEditorArgv, parseEditorCommand } from "../editor-command";
import { ElectronFileOpener, LAUNCHER_EXIT_TIMEOUT_MS, SETTINGS_TIMEOUT_MS } from "../file-opener";
import type { OpenInEditorParams, OpenWithParams } from "../open-in-editor-contract";
import settingsGeneral from "./fixtures/settings-general.json" with { type: "json" };

type Entry = "file" | "dir" | "fifo";
type SettingsReply = { body: unknown } | { status: number } | { reject: Error } | "hang";

function at<T>(items: readonly T[], index: number): T {
  const item = items[index];
  if (item === undefined) throw new Error(`Expected an item at ${index}.`);
  return item;
}

const WINDOW = { id: 1 } as unknown as BrowserWindow;

/**
 * A recorded `GET /api/settings/general` response whose `config.desktop.editor_command` is
 * `code -g {path}:{line}`. `null` removes `config.desktop`, as an older daemon returns it.
 */
function settingsBody(editorCommand?: string | null): unknown {
  const body = structuredClone(settingsGeneral) as { config: Record<string, unknown> };
  if (editorCommand === null) delete body.config.desktop;
  else if (editorCommand !== undefined) body.config.desktop = { editor_command: editorCommand };
  return body;
}

function harness(
  options: {
    platform?: NodeJS.Platform;
    home?: string;
    entries?: Record<string, Entry>;
    links?: Record<string, string>;
    /** Where a path resolves from its second `realpath` on, as if replaced mid-open. */
    swaps?: Record<string, string>;
    answers?: number[];
    settings?: SettingsReply;
    xdgMime?: string | Error;
    loginPath?: string | null | Error;
    executables?: string[];
    bundles?: Record<string, string>;
    spawnEvent?: "spawn" | "error";
    /** The child's exit code after `spawn`; `"never"` keeps it running. */
    exitCode?: number | "never";
    env?: NodeJS.ProcessEnv;
  } = {}
) {
  const home = options.home ?? "/Users/d";
  const entries = { [home]: "dir" as Entry, ...options.entries };
  const links = options.links ?? {};
  const resolved = new Set<string>();
  const fsCalls: string[] = [];
  const dialogs: MessageBoxOptions[] = [];
  const answers = [...(options.answers ?? [])];
  const spawned: { command: string; args: string[]; options: SpawnOptions }[] = [];
  const fetches: { url: string; init: RequestInit }[] = [];
  const logs: Record<string, unknown>[] = [];
  const loginShellPath = vi.fn(async () => {
    if (options.loginPath instanceof Error) throw options.loginPath;
    return options.loginPath === undefined ? null : options.loginPath;
  });
  const enoent = (path: string) =>
    Object.assign(new Error(`ENOENT: ${path}`), { code: "ENOENT" as const });
  const showItemInFolder = vi.fn();
  const opener = new ElectronFileOpener({
    showItemInFolder,
    platform: options.platform ?? "darwin",
    homedir: () => home,
    env: { PATH: "/usr/bin:/bin", HOME: home, ...options.env },
    fs: {
      async realpath(path) {
        fsCalls.push(`realpath ${path}`);
        const swap = options.swaps?.[path];
        if (swap && resolved.has(path)) return swap;
        resolved.add(path);
        if (links[path]) return links[path];
        if (entries[path]) return path;
        throw enoent(path);
      },
      async stat(path) {
        fsCalls.push(`stat ${path}`);
        const entry = entries[path];
        if (!entry) throw enoent(path);
        return { isFile: () => entry === "file", isDirectory: () => entry === "dir" };
      },
    },
    async showMessageBox(window, messageBox) {
      expect(window).toBe(WINDOW);
      dialogs.push(messageBox);
      return { response: answers.shift() ?? 1, checkboxChecked: false };
    },
    origin: () => "http://127.0.0.1:2123",
    async fetch(url, init) {
      fetches.push({ url, init });
      const reply = options.settings ?? { body: settingsBody() };
      if (reply === "hang") {
        return await new Promise<Response>((_resolve, reject) =>
          init.signal?.addEventListener("abort", () => reject(init.signal?.reason))
        );
      }
      if ("reject" in reply) throw reply.reject;
      if ("status" in reply) return new Response("{}", { status: reply.status });
      return Response.json(reply.body);
    },
    spawn(command: string, args: string[], spawnOptions: SpawnOptions) {
      spawned.push({ command, args, options: spawnOptions });
      const child = Object.assign(new EventEmitter(), { unref: vi.fn() });
      const event = options.spawnEvent ?? "spawn";
      const exitCode = options.exitCode ?? 0;
      queueMicrotask(() => {
        if (event === "error") {
          child.emit("error", Object.assign(new Error("spawn ENOENT"), { code: "ENOENT" }));
          return;
        }
        child.emit("spawn");
        if (exitCode !== "never") queueMicrotask(() => child.emit("exit", exitCode, null));
      });
      return child as unknown as ChildProcess;
    },
    async run(file, args) {
      if (file === "/usr/bin/mdfind") return options.bundles?.[args[0] ?? ""] ?? "";
      expect([file, ...args]).toEqual(["xdg-mime", "query", "default", "text/plain"]);
      if (options.xdgMime instanceof Error) throw options.xdgMime;
      return options.xdgMime ?? "";
    },
    loginShellPath,
    isExecutable: async path =>
      (options.executables ?? ["/usr/bin/code", "/usr/bin/gtk-launch"]).includes(path),
    log: (_message, fields) => logs.push(fields),
  });
  const open = async (params: Partial<OpenInEditorParams> & { path: string }) =>
    await opener.open({ roots: ["/repo"], ...params }, WINDOW);
  return {
    open,
    opener,
    showItemInFolder,
    fsCalls,
    dialogs,
    spawned,
    fetches,
    logs,
    loginShellPath,
  };
}

const REPO = { "/repo": "dir", "/repo/a.ts": "file" } as const;

describe("editor command template", () => {
  it("Should keep a hostile path as one argument and never run a shell", async () => {
    const path = '/tmp/a b;rm -rf ~/x".ts';
    expect(
      expandEditorArgv(parseEditorCommand("code -g {path}:{line}") as string[], { path, line: 3 })
    ).toEqual(["code", "-g", '/tmp/a b;rm -rf ~/x".ts:3']);
    const run = harness({ entries: { "/tmp": "dir", [path]: "file" } });
    expect(await run.open({ path, line: 3, roots: ["/tmp"] })).toEqual({ status: "opened" });
    expect(run.spawned).toHaveLength(1);
    expect(at(run.spawned, 0)).toMatchObject({
      command: "/usr/bin/code",
      args: ["-g", '/tmp/a b;rm -rf ~/x".ts:3'],
      options: { shell: false, detached: true, stdio: "ignore" },
    });
  });

  it("Should group quoted executables and default a missing line or column to 1", () => {
    const argv = parseEditorCommand(
      "'/Applications/My Editor.app/Contents/MacOS/ed' {path} +{line}"
    ) as string[];
    expect(expandEditorArgv(argv, { path: "/r/a.ts" })).toEqual([
      "/Applications/My Editor.app/Contents/MacOS/ed",
      "/r/a.ts",
      "+1",
    ]);
    expect(
      expandEditorArgv(parseEditorCommand("idea --line {line} --column {col} {path}") as string[], {
        path: "/r/{line}.ts",
        line: 4,
        column: 2,
      })
    ).toEqual(["idea", "--line", "4", "--column", "2", "/r/{line}.ts"]);
  });

  it("Should reject templates that are empty, unbalanced, pathless, unknown, or run the path", () => {
    const message = (template: string) =>
      (parseEditorCommand(template) as EditorTemplateError).message;
    expect(parseEditorCommand("  ")).toBeInstanceOf(EditorTemplateError);
    expect(message('code "{path}')).toBe("unbalanced quote");
    expect(message("code -g")).toBe("must contain {path}");
    expect(message("code {file}")).toBe(
      "unknown placeholder {file}; allowed: {path}, {line}, {col}"
    );
    expect(message("{path}")).toBe("the executable cannot contain a placeholder");
  });
});

describe("file opener", () => {
  it("Should open an inside file with the configured command and no dialog", async () => {
    const run = harness({ entries: REPO });
    expect(await run.open({ path: "/repo/a.ts", line: 42 })).toEqual({ status: "opened" });
    expect(run.dialogs).toEqual([]);
    expect(at(run.spawned, 0)).toMatchObject({
      command: "/usr/bin/code",
      args: ["-g", "/repo/a.ts:42"],
    });
    expect(at(run.fetches, 0).url).toBe("http://127.0.0.1:2123/api/settings/general");
    expect(run.logs).toEqual([
      { event: "file_open", status: "opened", code: "", inside: true, fallback: false },
    ]);
    expect(JSON.stringify(run.logs)).not.toContain("/repo");
  });

  it("Should report a missing target, a directory, and a special file", async () => {
    expect(await harness({ entries: REPO }).open({ path: "/repo/gone.ts" })).toEqual({
      status: "error",
      code: "not_found",
      message: "File not found: /repo/gone.ts",
    });
    const dir = harness({ entries: { ...REPO, "/repo/src": "dir" } });
    expect(await dir.open({ path: "/repo/src" })).toMatchObject({ code: "not_a_file" });
    const fifo = harness({ entries: { ...REPO, "/repo/pipe": "fifo" } });
    expect(await fifo.open({ path: "/repo/pipe" })).toMatchObject({ code: "not_a_file" });
    expect(fifo.spawned).toEqual([]);
  });

  it("Should report a drive path as missing off Windows before any dialog or filesystem call", async () => {
    for (const platform of ["darwin", "linux"] as const) {
      for (const path of ["C:\\Users\\a\\b.txt", "C:/Users/a/b.txt"]) {
        const run = harness({ platform, entries: REPO });
        expect(await run.open({ path })).toEqual({
          status: "error",
          code: "not_found",
          message: `File not found: ${path}`,
        });
        expect(run.dialogs).toEqual([]);
        expect(run.fsCalls).toEqual([]);
        expect(run.fetches).toEqual([]);
        expect(run.spawned).toEqual([]);
      }
    }
  });

  it("Should confirm a symlink escape with its real path and hand the editor only real paths", async () => {
    const escape = harness({
      entries: { ...REPO, "/Users/d/.ssh/config": "file" },
      links: { "/repo/link": "/Users/d/.ssh/config" },
    });
    expect(await escape.open({ path: "/repo/link" })).toEqual({ status: "cancelled" });
    expect(escape.dialogs).toHaveLength(1);
    expect(at(escape.dialogs, 0)).toMatchObject({
      type: "warning",
      message: "Open file outside this workspace?",
      detail:
        "/Users/d/.ssh/config\n\nAn agent mentioned this file. It is outside the session's workspace.",
      buttons: ["Open", "Cancel"],
      defaultId: 1,
      cancelId: 1,
    });
    expect(escape.spawned).toEqual([]);

    const alias = harness({
      entries: { ...REPO, "/repo/real.ts": "file" },
      links: { "/repo/alias.ts": "/repo/real.ts" },
    });
    expect(await alias.open({ path: "/repo/alias.ts" })).toEqual({ status: "opened" });
    expect(alias.dialogs).toEqual([]);
    expect(at(alias.spawned, 0).args).toEqual(["-g", "/repo/real.ts:1"]);
  });

  it("Should confirm a lexically outside target before any filesystem call", async () => {
    const cancelled = harness({ entries: { ...REPO, "/etc/hosts": "file" } });
    expect(await cancelled.open({ path: "/etc/hosts" })).toEqual({ status: "cancelled" });
    expect(cancelled.fsCalls).toEqual([]);
    expect(cancelled.fetches).toEqual([]);
    expect(at(cancelled.dialogs, 0).detail).toContain("/etc/hosts\n\n");

    const escape = harness({ entries: { ...REPO, "/etc/hosts": "file" } });
    expect(await escape.open({ path: "/repo/../etc/hosts" })).toEqual({ status: "cancelled" });
    expect(at(escape.dialogs, 0).detail?.startsWith("/etc/hosts\n")).toBe(true);
    expect(escape.fsCalls).toEqual([]);

    const opened = harness({ entries: { ...REPO, "/etc/hosts": "file" }, answers: [0] });
    expect(await opened.open({ path: "/etc/hosts" })).toEqual({ status: "opened" });
    expect(opened.dialogs).toHaveLength(1);
    expect(at(opened.spawned, 0).args).toEqual(["-g", "/etc/hosts:1"]);
    expect(at(opened.logs, 0)).toMatchObject({ status: "opened", inside: false });
  });

  it("Should ask again with the real path when a confirmed outside target resolves elsewhere", async () => {
    const run = harness({
      entries: { ...REPO, "/private/etc/hosts": "file" },
      links: { "/etc/hosts": "/private/etc/hosts" },
      answers: [0, 1],
    });
    expect(await run.open({ path: "/etc/hosts" })).toEqual({ status: "cancelled" });
    expect(run.dialogs.map(dialog => dialog.detail?.split("\n")[0])).toEqual([
      "/etc/hosts",
      "/private/etc/hosts",
    ]);
    expect(run.spawned).toEqual([]);
  });

  it("Should compare macOS paths case-sensitively and trust only the real path's case", async () => {
    const project = { "/Users/d/project": "dir" } as const;
    // Case-sensitive volume: a case variant of the root is a different, outside directory.
    const sensitive = harness({ entries: { ...project, "/Users/d/PROJECT/x.ts": "file" } });
    const roots = ["/Users/d/project"];
    expect(await sensitive.open({ path: "/Users/d/PROJECT/x.ts", roots })).toEqual({
      status: "cancelled",
    });
    expect(at(sensitive.dialogs, 0).detail).toContain("/Users/d/PROJECT/x.ts\n\n");
    expect(sensitive.fsCalls).toEqual([]);

    // Case-insensitive volume: the lexical mismatch still confirms first, and realpath's
    // on-disk case then places the target inside, so no second dialog follows.
    const insensitive = {
      entries: { ...project, "/Users/d/project/x.ts": "file" },
      links: { "/users/d/project/x.ts": "/Users/d/project/x.ts" },
    } as const;
    const cancelled = harness(insensitive);
    expect(await cancelled.open({ path: "/users/d/project/x.ts", roots })).toEqual({
      status: "cancelled",
    });
    expect(cancelled.fsCalls).toEqual([]);
    const opened = harness({ ...insensitive, answers: [0] });
    expect(await opened.open({ path: "/users/d/project/x.ts", roots })).toEqual({
      status: "opened",
    });
    expect(opened.dialogs).toHaveLength(1);
    expect(at(opened.logs, 0)).toMatchObject({ status: "opened", inside: true });
    expect(at(opened.spawned, 0).args).toEqual(["-g", "/Users/d/project/x.ts:1"]);
  });

  it("Should refuse a link that resolves to a network or device path before any stat or dialog", async () => {
    const cases = [
      {
        platform: "win32" as const,
        home: "C:\\Users\\d",
        root: "C:\\repo",
        path: "C:\\repo\\link.txt",
        real: "\\\\host\\share\\x.txt",
      },
      {
        platform: "darwin" as const,
        home: "/Users/d",
        root: "/repo",
        path: "/repo/link.ts",
        real: "//host/x.ts",
      },
    ];
    for (const { platform, home, root, path, real } of cases) {
      const run = harness({
        platform,
        home,
        entries: { [root]: "dir", [real]: "file" },
        links: { [path]: real },
        answers: [0],
      });
      expect(await run.open({ path, roots: [root] })).toEqual({
        status: "error",
        code: "not_a_file",
        message: `Not a local file: ${path}`,
      });
      expect(run.fsCalls.filter(call => call.startsWith("stat"))).toEqual([]);
      expect(run.dialogs).toEqual([]);
      expect(run.fetches).toEqual([]);
      expect(run.spawned).toEqual([]);
    }

    // A root whose real path is a network path never makes a target inside.
    const root = harness({
      entries: { "/host/a.ts": "file" },
      links: { "/net": "//host", "/net/a.ts": "/host/a.ts" },
    });
    expect(await root.open({ path: "/net/a.ts", roots: ["/net"] })).toEqual({
      status: "cancelled",
    });
    expect(at(root.dialogs, 0).detail).toContain("/host/a.ts\n\n");
    expect(root.spawned).toEqual([]);
  });

  it("Should discard home, its ancestors, and the filesystem root as inside roots", async () => {
    for (const roots of [["/Users/d"], ["/"], ["/Users"]]) {
      const notes = harness({ entries: { "/Users/d/notes.md": "file" } });
      expect(await notes.open({ path: "/Users/d/notes.md", roots })).toEqual({
        status: "cancelled",
      });
      expect(notes.fsCalls).toEqual([]);
    }
    const key = harness({ entries: { "/Users/d/.ssh/id_ed25519": "file" } });
    expect(await key.open({ path: "/Users/d/.ssh/id_ed25519", roots: ["/Users/d"] })).toEqual({
      status: "cancelled",
    });
    expect(at(key.dialogs, 0).detail).toContain("It is outside the session's workspace.");
  });

  it("Should always confirm the credential set even inside a planted root", async () => {
    const run = harness({
      entries: { "/Users/d/.aws": "dir", "/Users/d/.aws/credentials": "file" },
      answers: [0],
    });
    expect(await run.open({ path: "/Users/d/.aws/credentials", roots: ["/Users/d/.aws"] })).toEqual(
      { status: "opened" }
    );
    expect(run.dialogs).toHaveLength(1);
    expect(at(run.dialogs, 0).detail).toBe(
      "/Users/d/.aws/credentials\n\nIt may contain credentials, such as keys or tokens."
    );
    expect(at(run.dialogs, 0).message).toBe("Open a file that may contain credentials?");
  });

  it("Should expand ~/ with the operator home before touching the filesystem", async () => {
    const run = harness({
      entries: { "/Users/d/src": "dir", "/Users/d/src/notes.md": "file" },
    });
    expect(await run.open({ path: "~/src/notes.md", roots: ["/Users/d/src"] })).toEqual({
      status: "opened",
    });
    expect(run.fsCalls).toContain("realpath /Users/d/src/notes.md");
    expect(run.fsCalls).toContain("stat /Users/d/src/notes.md");
  });

  it("Should report the settings as unavailable without spawning", async () => {
    const replies: SettingsReply[] = [
      { reject: new TypeError("fetch failed") },
      { status: 500 },
      { body: { config: "nope" } },
      { body: [] },
      { body: { config: { desktop: { editor_command: 7 } } } },
    ];
    for (const settings of replies) {
      const run = harness({ entries: REPO, settings });
      expect(await run.open({ path: "/repo/a.ts" })).toEqual({
        status: "error",
        code: "editor_unavailable",
        message: "Couldn't read the editor command. Try again.",
      });
      expect(run.spawned).toEqual([]);
    }
  });

  it("Should give up on settings after the 2s timeout", async () => {
    vi.useFakeTimers();
    try {
      const run = harness({ entries: REPO, settings: "hang" });
      const result = run.open({ path: "/repo/a.ts" });
      await vi.advanceTimersByTimeAsync(SETTINGS_TIMEOUT_MS + 1);
      expect(await result).toMatchObject({ code: "editor_unavailable" });
      expect(run.spawned).toEqual([]);
    } finally {
      vi.useRealTimers();
    }
  });

  it("Should treat a daemon without config.desktop as unset and use the fallback", async () => {
    const run = harness({ entries: REPO, settings: { body: settingsBody(null) } });
    expect(await run.open({ path: "/repo/a.ts", line: 9 })).toEqual({ status: "opened" });
    expect(at(run.spawned, 0)).toMatchObject({
      command: "/usr/bin/open",
      args: ["-t", "/repo/a.ts"],
    });
    expect(at(run.logs, 0)).toMatchObject({ fallback: true });
  });

  it("Should use the fixed per-OS fallback with the real path", async () => {
    const unset = { body: settingsBody("") };
    const win = harness({
      platform: "win32",
      home: "C:\\Users\\d",
      entries: { "C:\\repo": "dir", "C:\\repo\\a.ts": "file" },
      settings: unset,
    });
    expect(await win.open({ path: "C:\\repo\\a.ts", roots: ["C:\\repo"] })).toEqual({
      status: "opened",
    });
    expect(at(win.spawned, 0)).toMatchObject({
      command: "C:\\Windows\\System32\\notepad.exe",
      args: ["C:\\repo\\a.ts"],
    });
    expect(win.loginShellPath).not.toHaveBeenCalled();

    const winRoot = harness({
      platform: "win32",
      home: "C:\\Users\\d",
      entries: { "C:\\repo": "dir", "C:\\repo\\a.ts": "file" },
      settings: unset,
      env: { SystemRoot: "D:\\WINNT" },
    });
    expect(await winRoot.open({ path: "C:\\repo\\a.ts", roots: ["C:\\repo"] })).toEqual({
      status: "opened",
    });
    expect(at(winRoot.spawned, 0).command).toBe("D:\\WINNT\\System32\\notepad.exe");

    const mac = harness({
      entries: REPO,
      settings: unset,
      loginPath: "/opt/shim/bin:/usr/bin",
      executables: ["/opt/shim/bin/open"],
    });
    expect(await mac.open({ path: "/repo/a.ts" })).toEqual({ status: "opened" });
    expect(at(mac.spawned, 0)).toMatchObject({
      command: "/usr/bin/open",
      args: ["-t", "/repo/a.ts"],
    });

    const linux = harness({
      platform: "linux",
      entries: REPO,
      settings: unset,
      xdgMime: "code.desktop\n",
    });
    expect(await linux.open({ path: "/repo/a.ts" })).toEqual({ status: "opened" });
    expect(at(linux.spawned, 0)).toMatchObject({
      command: "/usr/bin/gtk-launch",
      args: ["code.desktop", "/repo/a.ts"],
    });

    for (const xdgMime of ["", new Error("xdg-mime: not found"), "../evil sh"]) {
      const none = harness({ platform: "linux", entries: REPO, settings: unset, xdgMime });
      expect(await none.open({ path: "/repo/a.ts" })).toEqual({
        status: "error",
        code: "editor_not_configured",
        message: "No text editor found. Set an editor command in Settings.",
      });
      expect(none.spawned).toEqual([]);
    }
  });

  it("Should report an editor that fails to start", async () => {
    const run = harness({ entries: REPO, spawnEvent: "error" });
    expect(await run.open({ path: "/repo/a.ts" })).toEqual({
      status: "error",
      code: "editor_failed",
      message: "Couldn't open the editor. Check the editor command in Settings.",
    });
    expect(at(run.logs, 0)).toMatchObject({ status: "error", code: "editor_failed" });
  });

  it("Should refuse Windows batch shims and invalid templates without spawning", async () => {
    const shim =
      '"C:\\Users\\d\\AppData\\Local\\Programs\\Microsoft VS Code\\bin\\code.cmd" -g {path}';
    const win = harness({
      platform: "win32",
      home: "C:\\Users\\d",
      entries: { "C:\\repo": "dir", "C:\\repo\\a.ts": "file" },
      settings: { body: settingsBody(shim) },
    });
    expect(await win.open({ path: "C:\\repo\\a.ts", roots: ["C:\\repo"] })).toEqual({
      status: "error",
      code: "editor_invalid",
      message: "The editor command is invalid: use the editor's .exe path.",
    });
    expect(win.spawned).toEqual([]);

    const pathless = harness({ entries: REPO, settings: { body: settingsBody("{path}") } });
    expect(await pathless.open({ path: "/repo/a.ts" })).toMatchObject({ code: "editor_invalid" });
    expect(pathless.spawned).toEqual([]);
  });

  it("Should resolve a bare editor against the cached login-shell PATH", async () => {
    const run = harness({
      entries: REPO,
      loginPath: "/opt/homebrew/bin:/usr/bin",
      executables: ["/opt/homebrew/bin/code"],
    });
    expect(await run.open({ path: "/repo/a.ts" })).toEqual({ status: "opened" });
    expect(await run.open({ path: "/repo/a.ts" })).toEqual({ status: "opened" });
    expect(run.loginShellPath).toHaveBeenCalledTimes(1);
    expect(at(run.spawned, 0).command).toBe("/opt/homebrew/bin/code");
    expect(at(run.spawned, 0).options.env?.PATH).toBe("/opt/homebrew/bin:/usr/bin");

    const failed = harness({ entries: REPO, loginPath: new Error("timed out") });
    expect(await failed.open({ path: "/repo/a.ts" })).toEqual({ status: "opened" });
    expect(at(failed.spawned, 0).command).toBe("/usr/bin/code");
    expect(at(failed.spawned, 0).options.env?.PATH).toBe("/usr/bin:/bin");
  });

  it("Should fail instead of spawning a bare editor no absolute PATH directory supplies", async () => {
    for (const platform of ["darwin", "linux"] as const) {
      const run = harness({ platform, entries: REPO, loginPath: ".::/opt/bin", executables: [] });
      expect(await run.open({ path: "/repo/a.ts" })).toEqual({
        status: "error",
        code: "editor_failed",
        message: "Couldn't open the editor. Check the editor command in Settings.",
      });
      expect(run.spawned).toEqual([]);
    }
  });

  it("Should refuse a target whose real path changes while the editor is prepared", async () => {
    const run = harness({
      entries: { ...REPO, "/Users/d/.ssh/config": "file" },
      swaps: { "/repo/a.ts": "/Users/d/.ssh/config" },
    });
    expect(await run.open({ path: "/repo/a.ts" })).toEqual({
      status: "error",
      code: "not_found",
      message: "The file changed while opening: /repo/a.ts",
    });
    expect(run.fetches).toHaveLength(1);
    expect(run.dialogs).toEqual([]);
    expect(run.spawned).toEqual([]);
  });

  it("Should report a gtk-launch fallback that exits non-zero, but not a configured editor", async () => {
    const unset = { body: settingsBody("") };
    const broken = harness({
      platform: "linux",
      entries: REPO,
      settings: unset,
      xdgMime: "gone.desktop",
      exitCode: 1,
    });
    expect(await broken.open({ path: "/repo/a.ts" })).toEqual({
      status: "error",
      code: "editor_failed",
      message: "Couldn't open the editor. Check the editor command in Settings.",
    });
    expect(broken.spawned).toHaveLength(1);

    const configured = harness({ platform: "linux", entries: REPO, exitCode: 1 });
    expect(await configured.open({ path: "/repo/a.ts" })).toEqual({ status: "opened" });

    vi.useFakeTimers();
    try {
      const slow = harness({
        platform: "linux",
        entries: REPO,
        settings: unset,
        xdgMime: "code.desktop",
        exitCode: "never",
      });
      const result = slow.open({ path: "/repo/a.ts" });
      await vi.advanceTimersByTimeAsync(LAUNCHER_EXIT_TIMEOUT_MS + 1);
      expect(await result).toEqual({ status: "opened" });
    } finally {
      vi.useRealTimers();
    }
  });

  it("Should confirm credential folders whatever their case on macOS", async () => {
    for (const [root, path] of [
      ["/Users/d/.AWS", "/Users/d/.AWS/credentials"],
      ["/Users/d/.SSH", "/Users/d/.SSH/config"],
    ] as const) {
      const run = harness({ entries: { [root]: "dir", [path]: "file" }, answers: [0] });
      expect(await run.open({ path, roots: [root] })).toEqual({ status: "opened" });
      expect(run.dialogs).toHaveLength(1);
      expect(at(run.dialogs, 0).detail).toBe(
        `${path}\n\nIt may contain credentials, such as keys or tokens.`
      );
    }
  });
});

describe("Open in targets", () => {
  it("Should detect login PATH targets and omit absent binaries", async () => {
    const h = harness({ loginPath: "/opt/bin", executables: ["/opt/bin/cursor"] });
    const targets = await h.opener.listOpenTargets();
    expect(targets).toContainEqual({
      id: "cursor",
      label: "Cursor",
      kind: "editor",
      supportsLine: true,
    });
    expect(targets.some(target => target.id === "sublime")).toBe(false);
    expect(h.loginShellPath).toHaveBeenCalledOnce();
  });

  it("Should list the configured editor only when editor_command is set", async () => {
    const unset = harness({ settings: { body: settingsBody("") }, executables: [] });
    expect(await unset.opener.listOpenTargets()).toEqual([
      { id: "reveal", label: "Finder", kind: "reveal", supportsLine: false },
    ]);
    const older = harness({ settings: { body: settingsBody(null) }, executables: [] });
    expect((await older.opener.listOpenTargets()).map(target => target.id)).toEqual(["reveal"]);
    const failing = harness({ settings: { status: 500 }, executables: [] });
    expect((await failing.opener.listOpenTargets()).map(target => target.id)).toEqual(["reveal"]);
    const set = harness({ executables: [] });
    expect(at(await set.opener.listOpenTargets(), 0)).toEqual({
      id: "configured",
      label: "Configured editor",
      kind: "editor",
      supportsLine: true,
    });
  });

  it("Should detect every catalog editor in catalog order", async () => {
    const binaries = [
      "cursor",
      "code",
      "code-insiders",
      "windsurf",
      "zed",
      "subl",
      "idea",
      "webstorm",
      "goland",
      "pycharm",
      "xed",
      "studio",
      "fleet",
    ];
    const h = harness({
      settings: { body: settingsBody("") },
      executables: binaries.map(binary => `/usr/bin/${binary}`),
    });
    const targets = await h.opener.listOpenTargets();
    expect(targets.map(target => target.label)).toEqual([
      "Cursor",
      "VS Code",
      "VS Code Insiders",
      "Windsurf",
      "Zed",
      "Sublime Text",
      "IntelliJ IDEA",
      "WebStorm",
      "GoLand",
      "PyCharm",
      "Xcode",
      "Android Studio",
      "Fleet",
      "Finder",
    ]);
    expect(targets.find(target => target.id === "fleet")?.supportsLine).toBe(false);
  });

  it.each([
    ["linux", "Show in folder"],
    ["win32", "Show in Explorer"],
  ] as const)("Should label the %s file manager %s", async (platform, label) => {
    const h = harness({ platform, settings: { body: settingsBody("") }, executables: [] });
    expect(await h.opener.listOpenTargets()).toEqual([
      { id: "reveal", label, kind: "reveal", supportsLine: false },
    ]);
  });

  it("Should open a directory without a line in an editor, the configured editor, and Finder", async () => {
    const entries = { "/repo": "dir", "/repo/src": "dir" } as const;
    const editor = harness({ entries, executables: ["/usr/bin/cursor"] });
    expect(
      await editor.opener.openWith(
        { target: "cursor", path: "/repo/src", roots: ["/repo"] },
        WINDOW
      )
    ).toEqual({ status: "opened" });
    expect(at(editor.spawned, 0).args).toEqual(["/repo/src"]);
    const configured = harness({ entries });
    expect(
      await configured.opener.openWith(
        { target: "configured", path: "/repo/src", roots: ["/repo"] },
        WINDOW
      )
    ).toEqual({ status: "opened" });
    expect(at(configured.spawned, 0).command).toBe("/usr/bin/code");
    const reveal = harness({ entries });
    expect(
      await reveal.opener.openWith(
        { target: "reveal", path: "/repo/src", roots: ["/repo"] },
        WINDOW
      )
    ).toEqual({ status: "opened" });
    expect(reveal.showItemInFolder).toHaveBeenCalledWith("/repo/src");
  });

  it("Should detect and launch a macOS application bundle when its CLI is absent", async () => {
    const h = harness({
      entries: REPO,
      executables: [],
      bundles: { "kMDItemCFBundleIdentifier == 'dev.zed.Zed'": "/Applications/Zed.app\n" },
    });
    expect((await h.opener.listOpenTargets()).some(target => target.id === "zed")).toBe(true);
    expect(
      await h.opener.openWith({ target: "zed", path: "/repo/a.ts", roots: ["/repo"] }, WINDOW)
    ).toEqual({ status: "opened" });
    expect(h.spawned[0]).toMatchObject({
      command: "/usr/bin/open",
      args: ["-a", "/Applications/Zed.app", "--args", "/repo/a.ts"],
      options: { shell: false },
    });
  });
  it("Should detect a native Windows executable instead of launching a batch shim", async () => {
    const h = harness({
      platform: "win32",
      env: { PATH: "C:\\apps" },
      executables: ["C:\\apps\\cursor.exe"],
      entries: { "C:\\repo": "dir", "C:\\repo\\a.ts": "file" },
    });
    expect((await h.opener.listOpenTargets()).some(target => target.id === "cursor")).toBe(true);
    expect(
      await h.opener.openWith(
        { target: "cursor", path: "C:\\repo\\a.ts", roots: ["C:\\repo"] },
        WINDOW
      )
    ).toEqual({ status: "opened" });
    expect(h.spawned[0]).toMatchObject({
      command: "C:\\apps\\cursor.exe",
      options: { shell: false },
    });
  });
  it.each([
    ["vscode", "code", ["-g", "/repo/a.ts:12:3"]],
    ["zed", "zed", ["/repo/a.ts:12:3"]],
    ["idea", "idea", ["--line", "12", "/repo/a.ts"]],
    ["vscode-insiders", "code-insiders", ["-g", "/repo/a.ts:12:3"]],
    ["windsurf", "windsurf", ["-g", "/repo/a.ts:12:3"]],
    ["pycharm", "pycharm", ["--line", "12", "/repo/a.ts"]],
    ["android-studio", "studio", ["--line", "12", "/repo/a.ts"]],
    ["fleet", "fleet", ["/repo/a.ts"]],
  ])("Should launch %s with fixed argv and no shell", async (target, binary, args) => {
    const h = harness({ entries: REPO, executables: ["/usr/bin/" + binary] });
    expect(
      await h.opener.openWith(
        { target, path: "/repo/a.ts", line: 12, column: 3, roots: ["/repo"] },
        WINDOW
      )
    ).toEqual({ status: "opened" });
    expect(h.spawned[0]).toMatchObject({
      command: "/usr/bin/" + binary,
      args,
      options: { shell: false },
    });
  });
  it("Should reveal through the file manager without spawning", async () => {
    const h = harness({ entries: REPO });
    expect(
      await h.opener.openWith({ target: "reveal", path: "/repo/a.ts", roots: ["/repo"] }, WINDOW)
    ).toEqual({ status: "opened" });
    expect(h.showItemInFolder).toHaveBeenCalledWith("/repo/a.ts");
    expect(h.spawned).toEqual([]);
  });
  it("Should reject unknown and vanished targets", async () => {
    const h = harness({ entries: REPO, executables: [] });
    const request = { path: "/repo/a.ts", roots: ["/repo"] };
    expect(await h.opener.openWith({ ...request, target: "arbitrary" }, WINDOW)).toMatchObject({
      code: "target_unknown",
    });
    expect(await h.opener.openWith({ ...request, target: "cursor" }, WINDOW)).toEqual({
      status: "error",
      code: "target_unavailable",
      message: "Cursor is no longer available.",
    });
    expect(h.spawned).toEqual([]);
  });
  it.each([
    "/outside/a.ts",
    "/repo/.env",
    "/repo/.npmrc",
    "/repo/credentials.json",
    "/repo/id_rsa",
    "/repo/service-account-dev.json",
    "/repo/vault.keystore",
  ])("Should confirm and cancel %s before launching", async path => {
    const h = harness({ entries: { ...REPO, [path]: "file" } });
    expect(await h.opener.openWith({ target: "vscode", path, roots: ["/repo"] }, WINDOW)).toEqual({
      status: "cancelled",
    });
    expect(h.dialogs).toHaveLength(1);
    expect(h.spawned).toEqual([]);
  });
  it.each([".env.example", ".env.sample", ".env.template"])(
    "Should open the %s sample without a credential confirmation",
    async name => {
      const path = `/repo/${name}`;
      const h = harness({ entries: { ...REPO, [path]: "file" } });
      expect(await h.opener.openWith({ target: "vscode", path, roots: ["/repo"] }, WINDOW)).toEqual(
        { status: "opened" }
      );
      expect(h.dialogs).toEqual([]);
    }
  );
  it("Should delegate configured target to the existing editor command", async () => {
    const h = harness({ entries: REPO });
    const params: OpenWithParams = {
      target: "configured",
      path: "/repo/a.ts",
      line: 9,
      roots: ["/repo"],
    };
    expect(await h.opener.openWith(params, WINDOW)).toEqual({ status: "opened" });
    expect(h.spawned[0]).toMatchObject({ command: "/usr/bin/code", args: ["-g", "/repo/a.ts:9"] });
  });
});
