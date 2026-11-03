import type { ChildProcess, SpawnOptions } from "node:child_process";

import type { BrowserWindow, MessageBoxOptions, MessageBoxReturnValue } from "electron";

import {
  EditorTemplateError,
  expandEditorArgv,
  fallbackEditorArgv,
  isWindowsBatchExecutable,
  parseEditorCommand,
} from "./editor-command";
import { FileOpenPolicy } from "./file-open-policy";
import { lookPath, type CommandRunner } from "./login-shell-path";
import {
  isNetworkOrDevicePath,
  type OpenInEditorErrorCode,
  type OpenInEditorParams,
  type OpenInEditorResult,
} from "./open-in-editor-contract";

/** Settings fetch bound; the daemon is on loopback, so anything slower is unavailable. */
export const SETTINGS_TIMEOUT_MS = 2_000;
const XDG_MIME_TIMEOUT_MS = 2_000;
const XDG_MIME_MAX_BYTES = 4 * 1024;

export interface FileOpener {
  open(params: OpenInEditorParams, window: BrowserWindow | null): Promise<OpenInEditorResult>;
}

type Fetcher = (input: string, init: RequestInit) => Promise<Response>;
type Spawner = (command: string, args: string[], options: SpawnOptions) => ChildProcess;

/** Every boundary the opener touches, injected so the decision logic is testable. */
export interface FileOpenerDeps {
  platform: NodeJS.Platform;
  homedir: () => string;
  env: NodeJS.ProcessEnv;
  fs: {
    realpath(path: string): Promise<string>;
    stat(path: string): Promise<{ isFile(): boolean }>;
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

const ERROR_MESSAGES: Record<Exclude<OpenInEditorErrorCode, "not_found" | "not_a_file">, string> = {
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

    const real = await this.#realTarget(lexical);
    const realHome = await this.#realpathOrNull(home);
    const homes = realHome && !policy.same(realHome, home) ? [home, realHome] : [home];
    const realRoots = (await Promise.all(keptRoots.map(root => this.#realpathOrNull(root)))).filter(
      (root): root is string => root !== null && !policy.discardsRoot(root, homes)
    );
    const inside = realRoots.some(root => policy.contains(root, real));
    const sensitive = policy.isSensitive(real, homes);
    outcome.inside = inside;
    if ((!inside || sensitive) && !(confirmed && policy.same(confirmed, real))) {
      const reason = inside ? "sensitive" : "outside";
      if (!(await this.#confirm(window, real, reason))) return { status: "cancelled" };
    }

    const command = await this.#editorCommand();
    const target = { path: real, line: params.line, column: params.column };
    let argv: string[];
    if (command.trim() === "") {
      outcome.fallback = true;
      argv = await this.#fallbackArgv(real);
    } else {
      const template = parseEditorCommand(command);
      if (template instanceof EditorTemplateError) {
        throw failure("editor_invalid", `It ${template.message}.`);
      }
      argv = expandEditorArgv(template, target);
      if (isWindowsBatchExecutable(this.#deps.platform, argv[0] ?? "")) {
        throw new OpenFailure(
          "editor_invalid",
          "The editor command is invalid: use the editor's .exe path."
        );
      }
    }
    await this.#launch(argv);
    return { status: "opened" };
  }

  async #confirm(
    window: BrowserWindow | null,
    path: string,
    reason: ConfirmReason
  ): Promise<boolean> {
    const why =
      reason === "sensitive"
        ? "It may contain credentials."
        : "It is outside the session's workspace.";
    const { response } = await this.#deps.showMessageBox(window, {
      type: "warning",
      message: "Open file outside this workspace?",
      detail: `${path}\n\nAn agent mentioned this file. ${why}`,
      buttons: ["Open", "Cancel"],
      defaultId: 1,
      cancelId: 1,
      noLink: true,
    });
    return response === 0;
  }

  async #realTarget(lexical: string): Promise<string> {
    let real: string;
    try {
      real = await this.#deps.fs.realpath(lexical);
    } catch {
      throw new OpenFailure("not_found", `File not found: ${lexical}`);
    }
    let file = false;
    try {
      file = (await this.#deps.fs.stat(real)).isFile();
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

  async #fallbackArgv(path: string): Promise<string[]> {
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
    const argv = fallbackEditorArgv(this.#deps.platform, path, handler);
    if (!argv) throw failure("editor_not_configured");
    return argv;
  }

  async #launch(argv: string[]): Promise<void> {
    let [executable = ""] = argv;
    let env = this.#deps.env;
    if (this.#deps.platform !== "win32") {
      this.#loginPath ??= this.#deps.loginShellPath().catch(() => null);
      const path = (await this.#loginPath) ?? this.#deps.env.PATH ?? "";
      env = { ...env, PATH: path };
      if (!executable.includes("/")) {
        executable = (await lookPath(executable, path, this.#deps.isExecutable)) ?? executable;
      }
    }
    await new Promise<void>((resolve, reject) => {
      let child: ChildProcess;
      try {
        child = this.#deps.spawn(executable, argv.slice(1), {
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
        resolve();
      });
    });
  }
}
