import { useQuery } from "@tanstack/react-query";

import { useProfiles, type ProfileOwner, type ProfileScopeParams } from "@/systems/profiles";
import { breadthQuery, sessionCatalogBreadthParams, type DataScope } from "@/systems/workspace";

import { sessionFacetsOptions } from "../lib/session-catalog-options";
import type { SessionListFilters } from "../types";
import { sessionListShelfFilters, type SessionListShelf } from "../lib/session-list-shelf";
import {
  profileSessionGroups,
  projectSessionGroups,
  type SessionListGroupSpec,
} from "../lib/session-list-groups";
import {
  sessionListSubagentsParam,
  type SessionListGroupBy,
} from "../lib/session-list-preferences";

export interface SessionListGroupsInput {
  /** The grouping that renders; `none` reads nothing here. */
  groupBy: SessionListGroupBy;
  scope: DataScope | null;
  lens: ProfileScopeParams;
  workspaces: ReadonlyArray<{ id: string; name: string }>;
  /** Active, the archive, or Later: groups list only that population. */
  shelf: SessionListShelf;
  search?: string;
  /** The list's worktree restriction: groups and counts cover only its sessions. */
  worktreeId?: string;
  enabled: boolean;
  /**
   * The population filters of a surface that reads its own rows differently from
   * the list (the board: every subagent, children per "Hide child sessions"), so
   * group membership and totals count exactly the rows that surface shows.
   */
  population?: Pick<SessionListFilters, "children" | "subagents">;
  /** A surface with no stream of its own for a group re-reads the counts on this interval. */
  refetchInterval?: number;
}

export interface SessionListGroupsModel {
  groups: SessionListGroupSpec[];
  loading: boolean;
  /** The facets read failed: no group can be listed until a retry. */
  error?: { retry: () => void };
}

/**
 * Group-by groups for one session list (ADR-004). Membership and counts come
 * from one facets read (`by_workspace`, `by_profile`) — complete, never derived
 * from a loaded page — so an off-page group and its needs-you dot still show.
 * Each group then reads its own catalog page, independently.
 */
export function useSessionListGroups({
  groupBy,
  scope,
  lens,
  workspaces,
  shelf,
  search,
  worktreeId,
  enabled,
  population,
  refetchInterval,
}: SessionListGroupsInput): SessionListGroupsModel {
  const grouped = enabled && groupBy !== "none" && scope !== null;
  const facets = useQuery({
    ...sessionFacetsOptions(
      sessionListFacetFilters({ scope, lens, shelf, search, worktreeId, population })
    ),
    enabled: grouped,
    ...(refetchInterval ? { refetchInterval } : {}),
  });
  const byProfile = grouped && groupBy === "profile";
  const profiles = useProfiles(byProfile);
  const groups =
    !grouped || scope === null
      ? []
      : groupBy === "project"
        ? projectSessionGroups(facets.data, { workspaces, lens })
        : profileSessionGroups(profiles.data ? facets.data : undefined, {
            profiles: profileOwners(profiles.data),
            scope,
          });
  return {
    groups,
    loading: grouped && (facets.isLoading || (byProfile && profiles.isLoading)),
    error: groupsFailure(grouped, facets, byProfile ? profiles : null),
  };
}

/**
 * The one facets query a list's counts read — group membership and the totals
 * footer alike: the list's breadth and lens over its population.
 */
export function sessionListFacetFilters({
  scope,
  lens,
  shelf,
  search,
  worktreeId,
  population,
}: Pick<
  SessionListGroupsInput,
  "scope" | "lens" | "shelf" | "search" | "worktreeId" | "population"
>): SessionListFilters {
  return {
    ...sessionCatalogBreadthParams(breadthQuery(scope ?? { kind: "all" })),
    ...(population ?? { subagents: sessionListSubagentsParam(search) }),
    ...(worktreeId ? { worktree: worktreeId } : {}),
    ...sessionListShelfFilters(shelf),
    ...lens,
  };
}

function profileOwners(rows: ReturnType<typeof useProfiles>["data"]): ProfileOwner[] {
  return (rows ?? []).map(row => ({
    id: row.id,
    name: row.name,
    color: row.color,
    icon: row.icon,
    emoji: row.emoji,
    archived: row.archived_at != null,
  }));
}

/** The facets read, and under profile grouping the profile list, retry together. */
function groupsFailure(
  grouped: boolean,
  facets: { isError: boolean; refetch: () => unknown },
  profiles: { isError: boolean; refetch: () => unknown } | null
): SessionListGroupsModel["error"] {
  if (!grouped || !(facets.isError || profiles?.isError)) return undefined;
  return {
    retry: () => {
      void facets.refetch();
      if (profiles) void profiles.refetch();
    },
  };
}

/** Session totals by derived state for the sidebar footer (issue #1044). */
export interface SessionListTotals {
  needsYou: number;
  working: number;
  idle: number;
}

/**
 * The footer's totals over the list's whole scope — exact facet counts, never a
 * loaded page. Grouped lists read the same facets query as their groups.
 */
export function useSessionListTotals({
  scope,
  lens,
  search,
  worktreeId,
  shelf,
  enabled: listEnabled,
}: {
  scope: DataScope | null;
  lens: ProfileScopeParams;
  search: string;
  worktreeId?: string;
  /** The archive and Later count nothing live, so they show no totals. */
  shelf: SessionListShelf;
  enabled: boolean;
}): SessionListTotals | undefined {
  const enabled = listEnabled && shelf === "active";
  const facets = useQuery({
    ...sessionFacetsOptions(
      sessionListFacetFilters({ scope, lens, shelf: "active", search, worktreeId })
    ),
    enabled: enabled && scope !== null,
  }).data?.facets;
  if (!enabled || !facets) return undefined;
  return { needsYou: facets.needs_you, working: facets.working, idle: facets.idle };
}

/**
 * How many sessions the list's scope holds in Later — the toolbar toggle's
 * count. The daemon reports it on the active facets read the footer totals
 * already make, so the toggle adds no request per catalog wake; `undefined`
 * until it loads.
 */
export function useSessionListLaterCount({
  scope,
  lens,
  worktreeId,
  enabled,
}: {
  scope: DataScope | null;
  lens: ProfileScopeParams;
  worktreeId?: string;
  enabled: boolean;
}): number | undefined {
  const data = useQuery({
    ...sessionFacetsOptions(sessionListFacetFilters({ scope, lens, shelf: "active", worktreeId })),
    enabled: enabled && scope !== null,
  }).data;
  // The daemon omits the total when nothing is parked.
  return data ? (data.later ?? 0) : undefined;
}
