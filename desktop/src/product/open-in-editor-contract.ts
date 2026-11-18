/**
 * Wire contract of `files.open_in_editor`, shared by the preload (renderer side) and the
 * main-process controller. Both sides run these validators; the renderer cannot reach
 * `node:path`, so every check here is syntactic.
 */

/** Longest path or root the bridge accepts. */
export const MAX_OPEN_PATH_LENGTH = 1024;
/** Most inside roots one request may carry (worktree, workspace root, and spare room). */
export const MAX_OPEN_ROOTS = 8;
/** Largest line or column accepted; nine decimal digits. */
const MAX_POSITION = 999_999_999;

export const OPEN_IN_EDITOR_ERROR_CODES = [
  "target_unknown",
  "target_unavailable",
  "not_found",
  "not_a_file",
  "editor_invalid",
  "editor_unavailable",
  "editor_not_configured",
  "editor_failed",
] as const;
export type OpenInEditorErrorCode = (typeof OPEN_IN_EDITOR_ERROR_CODES)[number];
const ERROR_CODES = new Set<string>(OPEN_IN_EDITOR_ERROR_CODES);

export interface OpenInEditorParams {
  /** Absolute or `~/`-prefixed path of the file to open. */
  path: string;
  /** 1-based line, when known. */
  line?: number;
  /** 1-based column, when known. */
  column?: number;
  /** Absolute inside roots (bound worktree, workspace root); 0..8 entries. */
  roots: string[];
}

export type OpenInEditorResult =
  | { status: "opened" }
  | { status: "cancelled" }
  | { status: "error"; code: OpenInEditorErrorCode; message: string };

const PARAM_KEYS = new Set(["path", "line", "column", "roots"]);

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

/**
 * True for a path whose first two characters are both separators: UNC (`\\host\share`),
 * device namespaces (`\\?\`, `\\.\`), and `//host/…`. Touching one can contact an
 * attacker-chosen host before any confirmation, so they are rejected outright.
 */
export function isNetworkOrDevicePath(value: string): boolean {
  return /^[\\/]{2}/.test(value);
}

function isBoundedPathText(value: unknown): value is string {
  if (typeof value !== "string" || value.length === 0) return false;
  if (value.length > MAX_OPEN_PATH_LENGTH) return false;
  // oxlint-disable-next-line no-control-regex -- control characters are exactly what is rejected.
  if (/[\u0000-\u001f\u007f]/.test(value)) return false;
  return !isNetworkOrDevicePath(value);
}

/** A Windows drive path (`C:\x`, `C:/x`); it only names a file on a win32 host. */
export function isWindowsDrivePath(value: string): boolean {
  return /^[A-Za-z]:[\\/]/.test(value);
}

/** An absolute local POSIX (`/x`) or Windows drive (`C:\x`, `C:/x`) path; never UNC/device. */
export function isLocalAbsolutePath(value: unknown): value is string {
  if (!isBoundedPathText(value)) return false;
  return value.startsWith("/") || isWindowsDrivePath(value);
}

/** A target path: local absolute with at least one non-separator character, or `~/` + name. */
export function isOpenTargetPath(value: unknown): value is string {
  if (!isBoundedPathText(value)) return false;
  if (/^~[\\/]./.test(value)) return !isNetworkOrDevicePath(value.slice(1));
  return isLocalAbsolutePath(value) && /[^\\/]/.test(value.replace(/^[A-Za-z]:/, ""));
}

function isPosition(value: unknown): boolean {
  return (
    value === undefined ||
    (typeof value === "number" && Number.isInteger(value) && value >= 1 && value <= MAX_POSITION)
  );
}

export function validOpenInEditorParams(value: unknown): value is OpenInEditorParams {
  if (!isRecord(value)) return false;
  if (Object.keys(value).some(key => !PARAM_KEYS.has(key))) return false;
  return (
    isOpenTargetPath(value.path) &&
    isPosition(value.line) &&
    isPosition(value.column) &&
    Array.isArray(value.roots) &&
    value.roots.length <= MAX_OPEN_ROOTS &&
    value.roots.every(isLocalAbsolutePath)
  );
}

export function validOpenInEditorResult(value: unknown): value is OpenInEditorResult {
  if (!isRecord(value)) return false;
  const keys = Object.keys(value);
  if (value.status === "opened" || value.status === "cancelled") return keys.length === 1;
  return (
    value.status === "error" &&
    keys.length === 3 &&
    typeof value.code === "string" &&
    ERROR_CODES.has(value.code) &&
    typeof value.message === "string"
  );
}

export interface OpenTarget {
  id: string;
  label: string;
  kind: "editor" | "reveal";
  supportsLine: boolean;
}
export interface OpenWithParams extends OpenInEditorParams {
  target: string;
}
export function validOpenWithParams(value: unknown): value is OpenWithParams {
  if (!isRecord(value) || typeof value.target !== "string" || value.target.length > 64)
    return false;
  const { target: _target, ...params } = value;
  return validOpenInEditorParams(params);
}
export function validOpenTargets(value: unknown): value is OpenTarget[] {
  return (
    Array.isArray(value) &&
    value.every(
      item =>
        isRecord(item) &&
        Object.keys(item).length === 4 &&
        typeof item.id === "string" &&
        typeof item.label === "string" &&
        (item.kind === "editor" || item.kind === "reveal") &&
        typeof item.supportsLine === "boolean"
    )
  );
}
