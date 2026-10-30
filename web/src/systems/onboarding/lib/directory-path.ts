/** Path text helpers for the directory picker field. Separators: `/` and `\`. */

const SEPARATOR = /[\\/]/;

function lastSeparatorIndex(path: string): number {
  return Math.max(path.lastIndexOf("/"), path.lastIndexOf("\\"));
}

function separatorFor(path: string): string {
  return path.includes("\\") && !path.includes("/") ? "\\" : "/";
}

/** `/x`, `C:\x` or `C:/x`, and `\\server\share`. */
export function isAbsolutePath(path: string): boolean {
  return path.startsWith("/") || /^[A-Za-z]:[\\/]/.test(path) || path.startsWith("\\\\");
}

/** `~` and `~/x` expand against the daemon's home; anything else is returned as typed. */
export function expandHome(path: string, home: string | null): string {
  if (!home || !path.startsWith("~")) return path;
  if (path === "~") return home;
  if (!SEPARATOR.test(path.charAt(1))) return path;
  const base = home.replace(/[\\/]+$/, "");
  return `${base}${path.slice(1)}`;
}

/** Shows a path under home as `~/…`, the way the field accepts it. */
export function contractHome(path: string, home: string | null): string {
  if (!home) return path;
  const base = home.replace(/[\\/]+$/, "");
  if (path === base) return "~";
  if (path.startsWith(base) && SEPARATOR.test(path.charAt(base.length))) {
    return `~${path.slice(base.length)}`;
  }
  return path;
}

/** Drops trailing separators, keeping a filesystem root (`/`, `C:\`) intact. */
export function normalizeDirectoryPath(path: string): string {
  const trimmed = path.trim();
  const stripped = trimmed.replace(/[\\/]+$/, "");
  if (stripped === "") return trimmed === "" ? "" : trimmed.charAt(0);
  if (/^[A-Za-z]:$/.test(stripped)) return `${stripped}${trimmed.charAt(stripped.length) || "\\"}`;
  return stripped;
}

/**
 * Splits typed text into the folder to list and the name prefix to match:
 * `/a/b/c` → `/a/b` + `c`; `/a/b/` → `/a/b` + ``.
 */
export function completionTarget(path: string): { dir: string; prefix: string } | null {
  const cut = lastSeparatorIndex(path);
  if (cut < 0) return null;
  const dir = normalizeDirectoryPath(path.slice(0, cut + 1));
  return { dir, prefix: path.slice(cut + 1) };
}

/**
 * The text a chosen suggestion leaves in the field: what the operator typed up
 * to the last separator, the folder name, and a separator to drill in. Keeping
 * the typed spelling preserves `~` and symlinked prefixes the daemon resolves.
 */
export function suggestionValue(typed: string, name: string): string {
  const cut = lastSeparatorIndex(typed);
  const dir = cut < 0 ? `${typed}/` : typed.slice(0, cut + 1);
  return `${dir}${name}${separatorFor(dir)}`;
}

/** Folder name of a path, used to autofill a project's display name. */
export function folderName(path: string): string {
  const trimmed = normalizeDirectoryPath(path);
  const segment = trimmed.split(SEPARATOR).pop();
  return segment && segment.length > 0 ? segment : trimmed;
}
