import { win32 } from "node:path";

/**
 * `desktop.editor_command` template handling. A template is split into argv like a shell
 * would for quotes only (no escapes, no expansion) and is never run through a shell.
 */

const PLACEHOLDER = /\{([^{}]*)\}/g;
const ALLOWED_PLACEHOLDERS = new Set(["path", "line", "col"]);

/** Why a template cannot be used; the message is safe to show the operator. */
export class EditorTemplateError {
  readonly message: string;

  constructor(message: string) {
    this.message = message;
  }
}

/** The file an editor opens: the checked real path and an optional 1-based position. */
export interface EditorTarget {
  path: string;
  line?: number | undefined;
  column?: number | undefined;
}

function splitTemplate(template: string): string[] | EditorTemplateError {
  const argv: string[] = [];
  let current = "";
  let started = false;
  let quote: '"' | "'" | null = null;
  for (const char of template) {
    if (quote) {
      if (char === quote) quote = null;
      else current += char;
      continue;
    }
    if (char === '"' || char === "'") {
      quote = char;
      started = true;
    } else if (/\s/.test(char)) {
      if (started) argv.push(current);
      current = "";
      started = false;
    } else {
      current += char;
      started = true;
    }
  }
  if (quote) return new EditorTemplateError("unbalanced quote");
  if (started) argv.push(current);
  return argv;
}

/**
 * Splits a template into argv and checks its placeholders. The template must contain
 * `{path}`, may use `{line}` and `{col}`, and its executable (argv[0]) must be literal so the
 * opened path can never become the program that runs.
 */
export function parseEditorCommand(template: string): string[] | EditorTemplateError {
  const argv = splitTemplate(template);
  if (argv instanceof EditorTemplateError) return argv;
  const [executable] = argv;
  if (executable === undefined || executable === "") return new EditorTemplateError("is empty");
  let hasPath = false;
  for (const arg of argv) {
    for (const [, name = ""] of arg.matchAll(PLACEHOLDER)) {
      if (!ALLOWED_PLACEHOLDERS.has(name)) {
        return new EditorTemplateError(
          `unknown placeholder {${name}}; allowed: {path}, {line}, {col}`
        );
      }
      if (name === "path") hasPath = true;
    }
  }
  if (!hasPath) return new EditorTemplateError("must contain {path}");
  if (executable.includes("{")) {
    return new EditorTemplateError("the executable cannot contain a placeholder");
  }
  return argv;
}

/**
 * Substitutes placeholders inside each argument in one pass, so text inside the path (for
 * example a literal `{line}`) is never expanded again. Missing line or column becomes `1`.
 */
export function expandEditorArgv(argv: string[], target: EditorTarget): string[] {
  const values: Record<string, string> = {
    path: target.path,
    line: String(target.line ?? 1),
    col: String(target.column ?? 1),
  };
  return argv.map(arg => arg.replace(PLACEHOLDER, (match, name: string) => values[name] ?? match));
}

/** Node refuses to spawn batch files without a shell, so a Windows `.cmd`/`.bat` is invalid. */
export function isWindowsBatchExecutable(platform: NodeJS.Platform, executable: string): boolean {
  return platform === "win32" && /\.(?:cmd|bat)$/i.test(executable);
}

/** Desktop entry IDs `gtk-launch` accepts; anything else from `xdg-mime` is ignored. */
export function isDesktopEntryID(value: string): boolean {
  return /^[A-Za-z0-9][A-Za-z0-9_.-]*\.desktop$/.test(value);
}

/** Where the unset-command fallback finds what it cannot resolve on its own. */
export interface FallbackEditorHost {
  /** The `xdg-mime` `text/plain` handler on Linux, or `null` when there is none. */
  linuxTextHandler: string | null;
  /** The win32 `SystemRoot` environment value; `C:\Windows` when unset. */
  systemRoot?: string | undefined;
}

/**
 * The fixed per-OS text-editor fallback for an unset command, or `null` when Linux has no
 * `text/plain` handler. darwin and win32 use fixed executable locations so no PATH or
 * current-directory entry can shadow them; Linux has none, so `gtk-launch` stays a bare name
 * resolved through the login-shell PATH. None of these supports a line.
 */
export function fallbackEditorArgv(
  platform: NodeJS.Platform,
  path: string,
  host: FallbackEditorHost
): string[] | null {
  if (platform === "darwin") return ["/usr/bin/open", "-t", path];
  if (platform === "win32") {
    const systemRoot = host.systemRoot?.trim() || "C:\\Windows";
    return [win32.join(systemRoot, "System32", "notepad.exe"), path];
  }
  const handler = host.linuxTextHandler;
  if (!handler || !isDesktopEntryID(handler)) return null;
  return ["gtk-launch", handler, path];
}
