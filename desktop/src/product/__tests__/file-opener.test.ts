import { EventEmitter } from "node:events";
import type { ChildProcess, SpawnOptions } from "node:child_process";

import { describe, expect, it, vi } from "vitest";
import type { BrowserWindow, MessageBoxOptions } from "electron";

import { EditorTemplateError, expandEditorArgv, parseEditorCommand } from "../editor-command";
import { ElectronFileOpener, SETTINGS_TIMEOUT_MS } from "../file-opener";
import type { OpenInEditorParams } from "../open-in-editor-contract";
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
    answers?: number[];
    settings?: SettingsReply;
    xdgMime?: string | Error;
    loginPath?: string | null | Error;
    executables?: string[];
    spawnEvent?: "spawn" | "error";
  } = {}
) {
  const home = options.home ?? "/Users/d";
  const entries = { [home]: "dir" as Entry, ...options.entries };
  const links = options.links ?? {};
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
  const opener = new ElectronFileOpener({
    platform: options.platform ?? "darwin",
    homedir: () => home,
    env: { PATH: "/usr/bin:/bin", HOME: home },
    fs: {
      async realpath(path) {
        fsCalls.push(`realpath ${path}`);
        if (links[path]) return links[path];
        if (entries[path]) return path;
        throw enoent(path);
      },
      async stat(path) {
        fsCalls.push(`stat ${path}`);
        const entry = entries[path];
        if (!entry) throw enoent(path);
        return { isFile: () => entry === "file" };
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
      queueMicrotask(() =>
        event === "spawn"
          ? child.emit("spawn")
          : child.emit("error", Object.assign(new Error("spawn ENOENT"), { code: "ENOENT" }))
      );
      return child as unknown as ChildProcess;
    },
    async run(file, args) {
      expect([file, ...args]).toEqual(["xdg-mime", "query", "default", "text/plain"]);
      if (options.xdgMime instanceof Error) throw options.xdgMime;
      return options.xdgMime ?? "";
    },
    loginShellPath,
    isExecutable: async path => (options.executables ?? []).includes(path),
    log: (_message, fields) => logs.push(fields),
  });
  const open = async (params: Partial<OpenInEditorParams> & { path: string }) =>
    await opener.open({ roots: ["/repo"], ...params }, WINDOW);
  return { open, fsCalls, dialogs, spawned, fetches, logs, loginShellPath };
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
      command: "code",
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
    expect(at(run.spawned, 0)).toMatchObject({ command: "code", args: ["-g", "/repo/a.ts:42"] });
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
      "/Users/d/.aws/credentials\n\nAn agent mentioned this file. It may contain credentials."
    );
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
    expect(at(run.spawned, 0)).toMatchObject({ command: "open", args: ["-t", "/repo/a.ts"] });
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
    expect(at(win.spawned, 0)).toMatchObject({ command: "notepad.exe", args: ["C:\\repo\\a.ts"] });
    expect(win.loginShellPath).not.toHaveBeenCalled();

    const linux = harness({
      platform: "linux",
      entries: REPO,
      settings: unset,
      xdgMime: "code.desktop\n",
    });
    expect(await linux.open({ path: "/repo/a.ts" })).toEqual({ status: "opened" });
    expect(at(linux.spawned, 0)).toMatchObject({
      command: "gtk-launch",
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
    expect(at(failed.spawned, 0).command).toBe("code");
    expect(at(failed.spawned, 0).options.env?.PATH).toBe("/usr/bin:/bin");
  });
});
