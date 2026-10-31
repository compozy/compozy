/**
 * Path text helpers for the directory picker field. `/` always separates; `\`
 * separates only in Windows-style paths, because on POSIX it is a legal name
 * character.
 */

/** `C:…` drive paths and `\\server\share` UNC paths. */
export function isWindowsPath(path: string): boolean {
  return /^[A-Za-z]:/.test(path) || path.startsWith("\\\\");
}

function separatorPattern(windows: boolean): RegExp {
  return windows ? /[\\/]/ : /\//;
}

function lastSeparatorIndex(path: string, windows: boolean): number {
  return windows ? Math.max(path.lastIndexOf("/"), path.lastIndexOf("\\")) : path.lastIndexOf("/");
}

function trailingSeparators(windows: boolean): RegExp {
  return windows ? /[\\/]+$/ : /\/+$/;
}

/** `/x`, `C:\x` or `C:/x`, and `\\server\share` (a UNC path needs its share). */
export function isAbsolutePath(path: string): boolean {
  return (
    path.startsWith("/") || /^[A-Za-z]:[\\/]/.test(path) || /^\\\\[^\\/]+[\\/][^\\/]+/.test(path)
  );
}

/** `~` and `~/x` expand against the daemon's home; anything else is returned as typed. */
export function expandHome(path: string, home: string | null): string {
  if (!home || !path.startsWith("~")) return path;
  if (path === "~") return home;
  const windows = isWindowsPath(home);
  if (!separatorPattern(windows).test(path.charAt(1))) return path;
  const base = home.replace(trailingSeparators(windows), "");
  return `${base}${path.slice(1)}`;
}

/** Shows a path under home as `~/…`, the way the field accepts it. */
export function contractHome(path: string, home: string | null): string {
  if (!home) return path;
  const windows = isWindowsPath(home);
  const base = home.replace(trailingSeparators(windows), "");
  if (path === base) return "~";
  if (path.startsWith(base) && separatorPattern(windows).test(path.charAt(base.length))) {
    return `~${path.slice(base.length)}`;
  }
  return path;
}

/** Drops trailing separators, keeping a filesystem root (`/`, `C:\`) intact. */
export function normalizeDirectoryPath(path: string): string {
  const trimmed = path.trim();
  const windows = isWindowsPath(trimmed);
  const stripped = trimmed.replace(trailingSeparators(windows), "");
  if (stripped === "") return trimmed === "" ? "" : trimmed.charAt(0);
  if (/^[A-Za-z]:$/.test(stripped)) return `${stripped}${trimmed.charAt(stripped.length) || "\\"}`;
  return stripped;
}

/**
 * Splits an absolute path into the folder to list and the name prefix to match:
 * `/a/b/c` → `/a/b` + `c`; `/a/b/` → `/a/b` + ``.
 */
export function completionTarget(path: string): { dir: string; prefix: string } | null {
  const cut = lastSeparatorIndex(path, isWindowsPath(path));
  if (cut < 0) return null;
  const dir = normalizeDirectoryPath(path.slice(0, cut + 1));
  return { dir, prefix: path.slice(cut + 1) };
}

/**
 * The text a chosen suggestion leaves in the field: what the operator typed up
 * to the last separator, the folder name, and a separator to drill in. Keeping
 * the typed spelling preserves `~` and symlinked prefixes the daemon resolves.
 * `windows` says whether the typed text is a Windows-style path once expanded.
 */
export function suggestionValue(typed: string, name: string, windows: boolean): string {
  const cut = lastSeparatorIndex(typed, windows);
  const dir = cut < 0 ? `${typed}/` : typed.slice(0, cut + 1);
  const separator = windows && dir.includes("\\") && !dir.includes("/") ? "\\" : "/";
  return `${dir}${name}${separator}`;
}

/** Folder name of a path, used to autofill a project's display name. */
export function folderName(path: string): string {
  const trimmed = normalizeDirectoryPath(path);
  const segment = trimmed.split(separatorPattern(isWindowsPath(trimmed))).pop();
  return segment && segment.length > 0 ? segment : trimmed;
}
