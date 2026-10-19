/**
 * Browser-local recents for the composer `@` picker: the files and folders the
 * operator mentioned most recently in one workspace, newest first. A
 * convenience only — never daemon state — so storage failures are ignored.
 */
export const SESSION_MENTION_RECENTS_LIMIT = 8;

const STORAGE_PREFIX = "compozy.mentions.recent.";

export interface SessionMentionRecent {
  /** Relative to the session's working directory; folders end in `/`. */
  path: string;
  kind: "file" | "folder";
}

export function sessionMentionRecentsKey(workspaceId: string): string {
  return `${STORAGE_PREFIX}${workspaceId}`;
}

function recentFrom(value: unknown): SessionMentionRecent | null {
  if (typeof value !== "object" || value === null) return null;
  const { path, kind } = value as Record<string, unknown>;
  if (typeof path !== "string" || path.length === 0) return null;
  if (kind !== "file" && kind !== "folder") return null;
  return { path, kind };
}

export function readSessionMentionRecents(workspaceId: string): SessionMentionRecent[] {
  if (typeof window === "undefined" || workspaceId === "") return [];
  try {
    const raw = window.localStorage.getItem(sessionMentionRecentsKey(workspaceId));
    if (!raw) return [];
    const parsed = JSON.parse(raw) as unknown;
    if (!Array.isArray(parsed)) return [];
    const recents: SessionMentionRecent[] = [];
    for (const value of parsed) {
      const recent = recentFrom(value);
      if (recent && !recents.some(existing => existing.path === recent.path)) recents.push(recent);
    }
    return recents.slice(0, SESSION_MENTION_RECENTS_LIMIT);
  } catch {
    return [];
  }
}

/** Moves `recent` to the front, dedupes by path, caps at 8, and persists. */
export function pushSessionMentionRecent(
  workspaceId: string,
  recent: SessionMentionRecent
): SessionMentionRecent[] {
  const next = [
    recent,
    ...readSessionMentionRecents(workspaceId).filter(existing => existing.path !== recent.path),
  ].slice(0, SESSION_MENTION_RECENTS_LIMIT);
  if (typeof window === "undefined" || workspaceId === "") return next;
  try {
    window.localStorage.setItem(sessionMentionRecentsKey(workspaceId), JSON.stringify(next));
  } catch {
    // Best-effort convenience; quota or private-mode errors leave recents unchanged.
  }
  return next;
}
