import { win32 } from "node:path";
import type { ChildProcess, SpawnOptions } from "node:child_process";

import type { BrowserWindow, MessageBoxOptions, MessageBoxReturnValue } from "electron";

import {
  EditorTemplateError,
  expandEditorArgv,
  fallbackEditorArgv,
  isWindowsBatchExecutable,
  parseEditorCommand,
} from "./editor-command";
import { OPEN_TARGETS, openTargetEntry, targetSupportsLine, targetTemplate } from "./open-targets";
import type { OpenTarget, OpenWithParams } from "./open-in-editor-contract";
import { FileOpenPolicy } from "./file-open-policy";
import { lookPath, type CommandRunner } from "./login-shell-path";
import {
  isNetworkOrDevicePath,
  isWindowsDrivePath,
  type OpenInEditorErrorCode,
  type OpenInEditorParams,
  type OpenInEditorResult,
} from "./open-in-editor-contract";

/** Settings fetch bound; the daemon is on loopback, so anything slower is unavailable. */
export const SETTINGS_TIMEOUT_MS = 2_000;
const XDG_MIME_TIMEOUT_MS = 2_000;
const XDG_MIME_MAX_BYTES = 4 * 1024;
/** How long the Linux `gtk-launch` fallback may take to report a broken desktop entry. */
export const LAUNCHER_EXIT_TIMEOUT_MS = 5_000;

export interface FileOpener {
  listOpenTargets(): Promise<OpenTarget[]>;
  openWith(params: OpenWithParams, window: BrowserWindow | null): Promise<OpenInEditorResult>;
  open(params: OpenInEditorParams, window: BrowserWindow | null): Promise<OpenInEditorResult>;
}

type Fetcher = (input: string, init: RequestInit) => Promise<Response>;
type Spawner = (command: string, args: string[], options: SpawnOptions) => ChildProcess;

/** Every boundary the opener touches, injected so the decision logic is testable. */
export interface FileOpenerDeps {
  showItemInFolder?: (path: string) => void;
  platform: NodeJS.Platform;
  homedir: () => string;
  env: NodeJS.ProcessEnv;
  fs: {
    realpath(path: string): Promise<string>;
    stat(path: string): Promise<{ isFile(): boolean; isDirectory?(): boolean }>;
  };
  showMessageBox(
    window: BrowserWindow | null,
    options: MessageBoxOptions
  ): Promise<MessageBoxReturnValue>;
  /** The product loopback origin the settings are read from, when a runtime is attached. */
  origin: () => string | null;
  fetch: Fetcher;
  spawn: Spawner;
  run: CommandRunner;
  /** The operator login-shell PATH; called at most once per opener (one per app run). */
  loginShellPath: () => Promise<string | null>;
  isExecutable: (path: string) => Promise<boolean>;
  log: (message: string, fields: Record<string, unknown>) => void;
}

type ConfirmReason = "outside" | "sensitive";

interface PreparedLaunch {
  /** argv with argv[0] resolved; `{path}`, `{line}`, `{col}` in the arguments stay unexpanded. */
  template: string[];
  env: NodeJS.ProcessEnv;
  /** Wait for the launcher's exit status (bounded) instead of returning at `spawn`. */
  awaitExit: boolean;
}

const ERROR_MESSAGES: Record<Exclude<OpenInEditorErrorCode, "not_found" | "not_a_file">, string> = {
  target_unknown: "Unknown application target.",
  target_unavailable: "This app is no longer available.",
  editor_invalid: "The editor command is invalid.",
  editor_unavailable: "Couldn't read the editor command. Try again.",
  editor_not_configured: "No text editor found. Set an editor command in Settings.",
  editor_failed: "Couldn't open the editor. Check the editor command in Settings.",
};

class OpenFailure extends Error {
  readonly code: OpenInEditorErrorCode;

  constructor(code: OpenInEditorErrorCode, message: string) {
    super(message);
    this.code = code;
  }
}

function failure(code: keyof typeof ERROR_MESSAGES, detail?: string): OpenFailure {
  return new OpenFailure(code, detail ? `${ERROR_MESSAGES[code]} ${detail}` : ERROR_MESSAGES[code]);
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

/**
 * Opens a transcript-mentioned file in the operator's editor. Owns every security decision:
 * the renderer supplies only the path, an optional position, and the inside roots.
 */
export class ElectronFileOpener implements FileOpener {
  readonly #deps: FileOpenerDeps;
  readonly #policy: FileOpenPolicy;
  #loginPath: Promise<string | null> | null = null;

  constructor(deps: FileOpenerDeps) {
    this.#deps = deps;
    this.#policy = new FileOpenPolicy(deps.platform);
  }

  /**
   * The configured editor only when `[desktop] editor_command` is set, then every detected
   * catalog editor, then the platform file manager (spec §3a).
   */
  async listOpenTargets(): Promise<OpenTarget[]> {
    const targets: OpenTarget[] = [];
    const command = await this.#editorCommand().catch(() => "");
    if (command.trim() !== "") {
      targets.push({
        id: "configured",
        label: "Configured editor",
        kind: "editor",
        supportsLine: true,
      });
    }
    const env = await this.#targetEnv();
    const detected = await Promise.all(
      OPEN_TARGETS.map(async target => ((await this.#resolveTarget(target, env)) ? target : null))
    );
    for (const target of detected) {
      if (!target) continue;
      targets.push({
        id: target.id,
        label: target.label,
        kind: "editor",
        supportsLine: targetSupportsLine(target.id),
      });
    }
    targets.push({ id: "reveal", label: this.#revealLabel(), kind: "reveal", supportsLine: false });
    return targets;
  }

  #revealLabel(): string {
    if (this.#deps.platform === "darwin") return "Finder";
    return this.#deps.platform === "win32" ? "Show in Explorer" : "Show in folder";
  }

  async openWith(
    params: OpenWithParams,
    window: BrowserWindow | null
  ): Promise<OpenInEditorResult> {
    const known = ["configured", "reveal"].includes(params.target);
    if (!known && !openTargetEntry(params.target)) {
      return { status: "error", code: "target_unknown", message: ERROR_MESSAGES.target_unknown };
    }
    return this.open(params, window);
  }

  async #resolveTarget(
    target: (typeof OPEN_TARGETS)[number],
    env: NodeJS.ProcessEnv
  ): Promise<string[] | null> {
    if (this.#deps.platform === "win32") {
      for (const directory of (env.PATH ?? "").split(";")) {
        if (!win32.isAbsolute(directory)) continue;
        const names = [win32.join(directory, `${target.binary}.exe`)];
        if (target.windowsExe) names.push(win32.join(directory, "..", target.windowsExe));
        for (const executable of names)
          if (await this.#deps.isExecutable(executable)) return [executable];
      }
      return null;
    }
    const binary = await lookPath(target.binary, env.PATH ?? "", this.#deps.isExecutable);
    if (binary) return [binary];
    if (this.#deps.platform !== "darwin") return null;
    try {
      const apps = await this.#deps.run(
        "/usr/bin/mdfind",
        ["kMDItemCFBundleIdentifier == '" + target.bundle + "'"],
        { env, timeoutMs: 2_000, maxBytes: 4096 }
      );
      const app = apps.split("\n").find(path => path.startsWith("/") && path.endsWith(".app"));
      return app ? ["/usr/bin/open", "-a", app, "--args"] : null;
    } catch {
      return null;
    }
  }

  async #targetEnv(): Promise<NodeJS.ProcessEnv> {
    this.#loginPath ??= this.#deps.loginShellPath().catch(() => null);
    return { ...this.#deps.env, PATH: (await this.#loginPath) ?? this.#deps.env.PATH ?? "" };
  }

  async open(
    params: OpenInEditorParams,
    window: BrowserWindow | null
  ): Promise<OpenInEditorResult> {
    const outcome = { inside: false, fallback: false };
    let result: OpenInEditorResult;
    try {
      result = await this.#open(params, window, outcome);
    } catch (error) {
      if (!(error instanceof OpenFailure)) {
        this.#deps.log("file_open", {
          event: "file_open",
          status: "error",
          code: "internal",
          ...outcome,
        });
        throw error;
      }
      result = { status: "error", code: error.code, message: error.message };
    }
    this.#deps.log("file_open", {
      event: "file_open",
      status: result.status,
      code: result.status === "error" ? result.code : "",
      ...outcome,
    });
    return result;
  }

  async #open(
    params: OpenInEditorParams,
    window: BrowserWindow | null,
    outcome: { inside: boolean; fallback: boolean }
  ): Promise<OpenInEditorResult> {
    // A drive path names no file off Windows; resolving it would show a dialog for a path
    // the agent never wrote.
    if (this.#deps.platform !== "win32" && isWindowsDrivePath(params.path)) {
      throw new OpenFailure("not_found", `File not found: ${params.path}`);
    }
    const policy = this.#policy;
    const home = policy.normalize(this.#deps.homedir());
    const lexical = policy.expand(params.path, home);
    if (isNetworkOrDevicePath(lexical)) throw new TypeError("The file path is not local.");
    const roots = params.roots.map(root => policy.normalize(root));
    const keptRoots = roots.filter(root => !policy.discardsRoot(root, [home]));

    // Lexical gate: nothing touches the filesystem until an outside target is confirmed.
    const lexicalInside = keptRoots.some(root => policy.contains(root, lexical));
    const lexicalSensitive = policy.isSensitive(lexical, [home]);
    let confirmed: string | null = null;
    if (!lexicalInside || lexicalSensitive) {
      const reason = lexicalInside ? "sensitive" : "outside";
      if (!(await this.#confirm(window, lexical, reason))) return { status: "cancelled" };
      confirmed = lexical;
    }

    const real = await this.#realTarget(lexical, "target" in params);
    const realHome = await this.#realpathOrNull(home);
    const homes = realHome && !policy.same(realHome, home) ? [home, realHome] : [home];
    const realRoots = (await Promise.all(keptRoots.map(root => this.#realpathOrNull(root)))).filter(
      (root): root is string =>
        root !== null && !isNetworkOrDevicePath(root) && !policy.discardsRoot(root, homes)
    );
    const inside = realRoots.some(root => policy.contains(root, real));
    const sensitive = policy.isSensitive(real, homes);
    outcome.inside = inside;
    if ((!inside || sensitive) && !(confirmed && policy.same(confirmed, real))) {
      const reason = inside ? "sensitive" : "outside";
      if (!(await this.#confirm(window, real, reason))) return { status: "cancelled" };
    }

    // Everything fallible (settings, xdg-mime, login-shell PATH) runs before the last check,
    // so nothing awaits between re-validating the target and handing it to the editor.
    const target = "target" in params ? String(params.target) : "configured";
    let launch: PreparedLaunch | null = null;
    if (target !== "reveal") {
      if (target === "configured") launch = await this.#prepareLaunch(outcome);
      else {
        const entry = openTargetEntry(target);
        const env = await this.#targetEnv();
        const resolved = entry && (await this.#resolveTarget(entry, env));
        if (!entry) throw failure("target_unknown");
        if (!resolved) {
          throw new OpenFailure("target_unavailable", `${entry.label} is no longer available.`);
        }
        launch = {
          template: [...resolved, ...targetTemplate(entry.id, params.line !== undefined)],
          env,
          awaitExit: false,
        };
      }
    }
    if ((await this.#realpathOrNull(lexical)) !== real) {
      throw new OpenFailure("not_found", `The file changed while opening: ${lexical}`);
    }
    if (!launch) {
      if (!this.#deps.showItemInFolder) throw failure("target_unavailable");
      this.#deps.showItemInFolder(real);
      return { status: "opened" };
    }
    const [executable = "", ...template] = launch.template;
    const args = expandEditorArgv(template, {
      path: real,
      line: params.line,
      column: params.column,
    });
    await this.#spawn(launch.env, launch.awaitExit, executable, args);
    return { status: "opened" };
  }

  async #confirm(
    window: BrowserWindow | null,
    path: string,
    reason: ConfirmReason
  ): Promise<boolean> {
    // A credential file inside the root is not "outside"; it gets its own question.
    const copy =
      reason === "sensitive"
        ? {
            message: "Open a file that may contain credentials?",
            detail: `${path}\n\nIt may contain credentials, such as keys or tokens.`,
          }
        : {
            message: "Open file outside this workspace?",
            detail: `${path}\n\nAn agent mentioned this file. It is outside the session's workspace.`,
          };
    const { response } = await this.#deps.showMessageBox(window, {
      type: "warning",
      ...copy,
      buttons: ["Open", "Cancel"],
      defaultId: 1,
      cancelId: 1,
      noLink: true,
    });
    return response === 0;
  }

  async #realTarget(lexical: string, allowDirectory: boolean): Promise<string> {
    let real: string;
    try {
      real = await this.#deps.fs.realpath(lexical);
    } catch {
      throw new OpenFailure("not_found", `File not found: ${lexical}`);
    }
    // A local link can resolve to a UNC or device path; never stat or open it.
    if (isNetworkOrDevicePath(real)) {
      throw new OpenFailure("not_a_file", `Not a local file: ${lexical}`);
    }
    let file = false;
    try {
      const entry = await this.#deps.fs.stat(real);
      file = entry.isFile() || (allowDirectory && entry.isDirectory?.() === true);
    } catch {
      throw new OpenFailure("not_found", `File not found: ${lexical}`);
    }
    if (!file) throw new OpenFailure("not_a_file", `Not a file: ${lexical}`);
    return real;
  }

  async #realpathOrNull(path: string): Promise<string | null> {
    try {
      return await this.#deps.fs.realpath(path);
    } catch {
      return null;
    }
  }

  /** Reads `config.desktop.editor_command`; an absent `desktop` (older daemon) means unset. */
  async #editorCommand(): Promise<string> {
    const origin = this.#deps.origin();
    if (!origin) throw failure("editor_unavailable");
    let body: unknown;
    try {
      const response = await this.#deps.fetch(new URL("/api/settings/general", origin).toString(), {
        method: "GET",
        headers: { accept: "application/json" },
        signal: AbortSignal.timeout(SETTINGS_TIMEOUT_MS),
      });
      if (!response.ok) throw new Error(`settings returned HTTP ${response.status}`);
      body = await response.json();
    } catch {
      throw failure("editor_unavailable");
    }
    if (!isRecord(body) || !isRecord(body.config)) throw failure("editor_unavailable");
    const desktop = body.config.desktop;
    if (desktop === undefined || desktop === null) return "";
    if (!isRecord(desktop)) throw failure("editor_unavailable");
    const command = desktop.editor_command;
    if (command === undefined) return "";
    if (typeof command !== "string") throw failure("editor_unavailable");
    return command;
  }

  /** Resolves the argv template and executable; `{path}` stays a placeholder until launch. */
  async #prepareLaunch(outcome: { fallback: boolean }): Promise<PreparedLaunch> {
    const command = await this.#editorCommand();
    let template: string[];
    let awaitExit = false;
    if (command.trim() === "") {
      outcome.fallback = true;
      template = await this.#fallbackTemplate();
      // gtk-launch reports an unknown or broken desktop entry only through its exit status.
      awaitExit = this.#deps.platform === "linux";
    } else {
      const parsed = parseEditorCommand(command);
      if (parsed instanceof EditorTemplateError) {
        throw failure("editor_invalid", `It ${parsed.message}.`);
      }
      template = parsed;
      if (isWindowsBatchExecutable(this.#deps.platform, template[0] ?? "")) {
        throw new OpenFailure(
          "editor_invalid",
          "The editor command is invalid: use the editor's .exe path."
        );
      }
    }
    const [executable = "", ...args] = template;
    if (this.#deps.platform === "win32") {
      return { template, env: this.#deps.env, awaitExit };
    }
    this.#loginPath ??= this.#deps.loginShellPath().catch(() => null);
    const path = (await this.#loginPath) ?? this.#deps.env.PATH ?? "";
    const env = { ...this.#deps.env, PATH: path };
    if (executable.includes("/")) return { template, env, awaitExit };
    // A bare name spawn would search relative PATH entries against main's cwd; only an
    // absolute PATH directory may supply the editor.
    const resolved = await lookPath(executable, path, this.#deps.isExecutable);
    if (!resolved) throw failure("editor_failed");
    return { template: [resolved, ...args], env, awaitExit };
  }

  async #fallbackTemplate(): Promise<string[]> {
    let handler: string | null = null;
    if (this.#deps.platform === "linux") {
      try {
        const output = await this.#deps.run("xdg-mime", ["query", "default", "text/plain"], {
          env: this.#deps.env,
          timeoutMs: XDG_MIME_TIMEOUT_MS,
          maxBytes: XDG_MIME_MAX_BYTES,
        });
        handler = output.trim() || null;
      } catch {
        handler = null;
      }
    }
    const template = fallbackEditorArgv(this.#deps.platform, "{path}", {
      linuxTextHandler: handler,
      systemRoot: this.#deps.env.SystemRoot,
    });
    if (!template) throw failure("editor_not_configured");
    return template;
  }

  /** Must be entered synchronously after the final target check. */
  async #spawn(
    env: NodeJS.ProcessEnv,
    awaitExit: boolean,
    executable: string,
    args: string[]
  ): Promise<void> {
    await new Promise<void>((resolve, reject) => {
      let child: ChildProcess;
      try {
        child = this.#deps.spawn(executable, args, {
          shell: false,
          detached: true,
          stdio: "ignore",
          env,
          windowsHide: false,
        });
      } catch {
        reject(failure("editor_failed"));
        return;
      }
      // Stays attached: an `error` after launch must not crash main, and is not tracked.
      child.on("error", () => reject(failure("editor_failed")));
      child.once("spawn", () => {
        child.unref();
        if (!awaitExit) {
          resolve();
          return;
        }
        // A launcher still running after the bound is treated as opened.
        const timer = setTimeout(resolve, LAUNCHER_EXIT_TIMEOUT_MS);
        child.once("exit", code => {
          clearTimeout(timer);
          if (code === 0) resolve();
          else reject(failure("editor_failed"));
        });
      });
    });
  }
}
