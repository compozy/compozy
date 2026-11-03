/**
 * File-path tokens in assistant markdown (spec #1167 Business Rules 2–3).
 *
 * Detection is lexical only: rendering never touches the filesystem, and the
 * desktop shell owns every security decision about the resolved path.
 */

export type FilePathSource = "inline_code" | "href";

export interface FilePathToken {
  /** The trimmed source text: inline-code content or the authored href. */
  raw: string;
  /** The path without its `:line[:col]` suffix; `file://` hrefs are decoded. */
  path: string;
  line?: number;
  column?: number;
  /** True when the path resolves against the session's working directory. */
  relative: boolean;
}

export interface FilePathBase {
  workingDir?: string;
  worktreePath?: string;
  workspaceRoot?: string;
}

const MAX_TOKEN_LENGTH = 1024;
// Whitespace and C0/C1 control characters never appear in a path token.
// eslint-disable-next-line no-control-regex
const FORBIDDEN_CHARS = /[\s\u0000-\u001f\u007f-\u009f]/;
// eslint-disable-next-line no-control-regex
const CONTROL_CHARS = /[\u0000-\u001f\u007f-\u009f]/;
const URL_SCHEME = /^[a-z][a-z0-9+.-]*:\/\//i;
const LOCATION_SUFFIX = /:(\d{1,9})(?::(\d{1,9}))?$/;
const WINDOWS_DRIVE = /^[A-Za-z]:[\\/]/;
const EXTENSION = /\.[A-Za-z0-9]{1,10}$/;

/** UNC, device-namespace, and `//host` paths can contact a network host before any prompt. */
function isNetworkPath(path: string): boolean {
  return path.startsWith("\\\\") || path.startsWith("//");
}

function decodeFileUrl(href: string): string | null {
  let url: URL;
  try {
    url = new URL(href);
  } catch {
    return null;
  }
  if (url.protocol !== "file:") return null;
  if (url.hostname !== "" && url.hostname !== "localhost") return null;
  let pathname: string;
  try {
    pathname = decodeURIComponent(url.pathname);
  } catch {
    return null;
  }
  // `file:///C:/x` carries the drive after the leading slash.
  return /^\/[A-Za-z]:\//.test(pathname) ? pathname.slice(1) : pathname;
}

function lastSegment(path: string): string {
  const parts = path.split(/[\\/]/);
  return parts[parts.length - 1] ?? "";
}

export interface FilePathDetectOptions {
  /** Whether Windows drive paths count; only a Windows desktop can open them. Default true. */
  drivePaths?: boolean;
}

function classify(
  path: string,
  hasLine: boolean,
  drivePaths: boolean
): { relative: boolean } | null {
  if (path === "" || isNetworkPath(path) || path.startsWith("@")) return null;
  if (path.startsWith("/")) return /[^/]/.test(path) ? { relative: false } : null;
  if (path.startsWith("~/")) return path.length > 2 ? { relative: false } : null;
  if (WINDOWS_DRIVE.test(path)) return drivePaths && path.length > 3 ? { relative: false } : null;
  if (path.startsWith("./") || path.startsWith("../")) {
    return path.replace(/^(\.\.?\/)+/, "") === "" ? null : { relative: true };
  }
  const hasExtension = EXTENSION.test(lastSegment(path));
  if (path.includes("/")) return hasExtension ? { relative: true } : null;
  return hasExtension && hasLine ? { relative: true } : null;
}

function positive(value: string | undefined): number | null | undefined {
  if (value === undefined) return undefined;
  const parsed = Number(value);
  return parsed >= 1 ? parsed : null;
}

/** Returns the path token `text` names, or `null` when it does not look like a file path. */
export function detectFilePath(
  text: string,
  source: FilePathSource,
  options: FilePathDetectOptions = {}
): FilePathToken | null {
  const raw = text.trim();
  if (raw.length === 0 || raw.length > MAX_TOKEN_LENGTH || FORBIDDEN_CHARS.test(raw)) return null;

  let candidate = raw;
  if (URL_SCHEME.test(raw)) {
    if (source !== "href" || !/^file:\/\//i.test(raw)) return null;
    const decoded = decodeFileUrl(raw);
    // A decoded `file://` path may hold spaces (`a%20b.md`), never control characters.
    if (decoded === null || CONTROL_CHARS.test(decoded)) return null;
    candidate = decoded;
  }

  let path = candidate;
  let line: number | undefined;
  let column: number | undefined;
  const suffix = LOCATION_SUFFIX.exec(candidate);
  if (suffix) {
    const parsedLine = positive(suffix[1]);
    const parsedColumn = positive(suffix[2]);
    if (parsedLine === null || parsedColumn === null) return null;
    line = parsedLine;
    column = parsedColumn;
    path = candidate.slice(0, suffix.index);
  }

  const kind = classify(path, line !== undefined, options.drivePaths ?? true);
  if (!kind) return null;
  return {
    raw,
    path,
    ...(line !== undefined ? { line } : {}),
    ...(column !== undefined ? { column } : {}),
    relative: kind.relative,
  };
}

/** Lexically resolves `.` and `..` segments; `..` never climbs above the root. */
function normalizeLexically(path: string): string {
  const drive = WINDOWS_DRIVE.test(path) ? path.slice(0, 2) : "";
  const separator = drive !== "" || (path.includes("\\") && !path.includes("/")) ? "\\" : "/";
  const rest = drive === "" ? path : path.slice(2);
  const absolute = /^[\\/]/.test(rest);
  const segments: string[] = [];
  for (const segment of rest.split(/[\\/]/)) {
    if (segment === "" || segment === ".") continue;
    if (segment === "..") {
      if (segments.length > 0 && segments[segments.length - 1] !== "..") segments.pop();
      else if (!absolute) segments.push(segment);
      continue;
    }
    segments.push(segment);
  }
  return `${drive}${absolute ? separator : ""}${segments.join(separator)}`;
}

function firstBase(base: FilePathBase): string | undefined {
  for (const candidate of [base.workingDir, base.worktreePath, base.workspaceRoot]) {
    const trimmed = candidate?.trim();
    if (trimmed) return trimmed;
  }
  return undefined;
}

/**
 * Resolves `token` to the path the desktop shell receives: absolute and `~/` paths pass
 * through (the shell expands `~`), and relative paths join the session's working directory,
 * then its bound worktree, then the workspace root. `null` when a relative path has no base.
 */
export function resolveFilePath(token: FilePathToken, base: FilePathBase): string | null {
  if (!token.relative) {
    return token.path.startsWith("~/") ? token.path : normalizeLexically(token.path);
  }
  const root = firstBase(base);
  if (root === undefined) return null;
  return normalizeLexically(`${root}/${token.path}`);
}

/** The basename shown in the open button's accessible name. */
export function filePathBasename(path: string): string {
  return lastSegment(path) || path;
}
