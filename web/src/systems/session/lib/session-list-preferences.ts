/**
 * Operator preferences for every session list surface: how rows are ordered and
 * how wide the list reaches.
 *
 * Both persist per operator through the daemon's `[shell.sessions]` config
 * section, so the choice survives a reload, a restart, and a second client —
 * and round-trips with `compozy config get/set shell.sessions.*` like every
 * other setting. The vocabulary below mirrors `internal/config/shell.go`
 * exactly; the daemon validates it and rejects anything else.
 */

export const SESSION_LIST_SORTS = ["last_activity", "attention"] as const;
export const SESSION_LIST_SCOPES = ["workspace", "all-workspaces"] as const;

export type SessionListSort = (typeof SESSION_LIST_SORTS)[number];
export type SessionListScope = (typeof SESSION_LIST_SCOPES)[number];

export interface SessionListPreferences {
  sort: SessionListSort;
  scope: SessionListScope;
}

/** Calm defaults: last activity, this workspace's own sessions. */
export const DEFAULT_SESSION_LIST_PREFERENCES: SessionListPreferences = {
  sort: "last_activity",
  scope: "workspace",
};

const SORTS: ReadonlySet<string> = new Set(SESSION_LIST_SORTS);
const SCOPES: ReadonlySet<string> = new Set(SESSION_LIST_SCOPES);

export function toSessionListSort(value: string | null | undefined): SessionListSort {
  return value !== null && value !== undefined && SORTS.has(value)
    ? (value as SessionListSort)
    : DEFAULT_SESSION_LIST_PREFERENCES.sort;
}

export function toSessionListScope(value: string | null | undefined): SessionListScope {
  return value !== null && value !== undefined && SCOPES.has(value)
    ? (value as SessionListScope)
    : DEFAULT_SESSION_LIST_PREFERENCES.scope;
}

/** The list order the daemon should serve for a given preference. */
export function sessionListSortParam(sort: SessionListSort): "last_activity" | "attention" {
  return sort;
}
