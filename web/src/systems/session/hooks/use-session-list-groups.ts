import { useQuery } from "@tanstack/react-query";

import { useProfiles, type ProfileOwner, type ProfileScopeParams } from "@/systems/profiles";
import { breadthQuery, sessionCatalogBreadthParams, type DataScope } from "@/systems/workspace";

import { sessionFacetsOptions } from "../lib/session-catalog-options";
import type { SessionListFilters } from "../types";
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
  archived: boolean;
  search?: string;
  enabled: boolean;
  /**
   * The population filters of a surface that reads its own rows differently from
   * the list (the board: every subagent, children per "Hide child sessions"), so
   * group membership and totals count exactly the rows that surface shows.
   */
  population?: Pick<SessionListFilters, "children" | "subagents">;
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
  archived,
  search,
  enabled,
  population,
}: SessionListGroupsInput): SessionListGroupsModel {
  const grouped = enabled && groupBy !== "none" && scope !== null;
  const facets = useQuery({
    ...sessionFacetsOptions({
      ...sessionCatalogBreadthParams(breadthQuery(scope ?? { kind: "all" })),
      ...(population ?? { subagents: sessionListSubagentsParam(search) }),
      ...(archived ? { archive: "only" as const } : {}),
      ...lens,
    }),
    enabled: grouped,
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
