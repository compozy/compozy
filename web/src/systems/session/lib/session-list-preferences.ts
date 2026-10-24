/**
 * Operator preferences for every session list surface: how rows are ordered and
 * how they group.
 *
 * Both persist per operator through the daemon's `[shell.sessions]` config
 * section, so the choice survives a reload, a restart, and a second client —
 * and round-trips with `compozy config get/set shell.sessions.*` like every
 * other setting. The vocabulary below mirrors `internal/config/shell.go`
 * exactly; the daemon validates it and rejects anything else. Breadth is not a
 * list preference: the topbar globe owns it (ADR-004).
 */

export const SESSION_LIST_SORTS = [
  "created",
  "name",
  "last_activity",
  "attention",
  "manual",
] as const;
export const SESSION_LIST_GROUP_BYS = ["none", "project", "profile"] as const;

export type SessionListSort = (typeof SESSION_LIST_SORTS)[number];
export type SessionListGroupBy = (typeof SESSION_LIST_GROUP_BYS)[number];

export interface SessionListPreferences {
  sort: SessionListSort;
  groupBy: SessionListGroupBy;
}

/** Calm defaults: newest created first, one flat list. */
export const DEFAULT_SESSION_LIST_PREFERENCES: SessionListPreferences = {
  sort: "created",
  groupBy: "none",
};

const SORTS: ReadonlySet<string> = new Set(SESSION_LIST_SORTS);
const GROUP_BYS: ReadonlySet<string> = new Set(SESSION_LIST_GROUP_BYS);

export function toSessionListSort(value: string | null | undefined): SessionListSort {
  return value !== null && value !== undefined && SORTS.has(value)
    ? (value as SessionListSort)
    : DEFAULT_SESSION_LIST_PREFERENCES.sort;
}

export function toSessionListGroupBy(value: string | null | undefined): SessionListGroupBy {
  return value !== null && value !== undefined && GROUP_BYS.has(value)
    ? (value as SessionListGroupBy)
    : DEFAULT_SESSION_LIST_PREFERENCES.groupBy;
}

/** Which aggregate axes the list is reading; each one unlocks its own grouping. */
export interface SessionListBreadth {
  allProjects: boolean;
  allProfiles: boolean;
}

/** Group-by options offered now: `project` only under All projects, `profile` only under All profiles. */
export function availableSessionListGroupBys(
  breadth: SessionListBreadth
): readonly SessionListGroupBy[] {
  return SESSION_LIST_GROUP_BYS.filter(
    value =>
      value === "none" ||
      (value === "project" && breadth.allProjects) ||
      (value === "profile" && breadth.allProfiles)
  );
}

/** The grouping that renders: an inapplicable stored value renders flat, unchanged in storage. */
export function effectiveSessionListGroupBy(
  stored: SessionListGroupBy,
  breadth: SessionListBreadth
): SessionListGroupBy {
  return availableSessionListGroupBys(breadth).includes(stored) ? stored : "none";
}

/**
 * The order a query is served: manual order lives in one (workspace, profile)
 * pair's folders, so any wider query reads `last_activity` instead. The stored
 * sort is never rewritten.
 */
export function sessionListSortParam(sort: SessionListSort, singlePair = true): SessionListSort {
  return sort === "manual" && !singlePair ? "last_activity" : sort;
}

type SessionBoardSort = Exclude<SessionListSort, "manual">;

/**
 * Sorts the board offers. Manual order is the List's drag order inside one
 * project and profile's folders; it has no meaning across board columns.
 */
export const SESSION_BOARD_SORTS: readonly SessionBoardSort[] = SESSION_LIST_SORTS.filter(
  (sort): sort is SessionBoardSort => sort !== "manual"
);

/** The order board columns read: a stored `manual` reads `last_activity`, unchanged in storage. */
export function sessionBoardSort(sort: SessionListSort): SessionBoardSort {
  return sort === "manual" ? "last_activity" : sort;
}

/**
 * Subagent sessions stay out of the sidebar page (ADR-005): the parent row
 * carries their chip. A search widens the population so matching subagents
 * appear, nested under their parent.
 */
export function sessionListSubagentsParam(search: string | undefined): "include" | "exclude" {
  return search?.trim() ? "include" : "exclude";
}
