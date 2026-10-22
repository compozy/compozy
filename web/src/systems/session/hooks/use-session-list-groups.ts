import { useQuery } from "@tanstack/react-query";

import { useProfiles, type ProfileOwner, type ProfileScopeParams } from "@/systems/profiles";
import { breadthQuery, sessionCatalogBreadthParams, type DataScope } from "@/systems/workspace";

import { sessionFacetsOptions } from "../lib/session-catalog-options";
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
}: SessionListGroupsInput): SessionListGroupsModel {
  const grouped = enabled && groupBy !== "none" && scope !== null;
  const facets = useQuery({
    ...sessionFacetsOptions({
      ...sessionCatalogBreadthParams(breadthQuery(scope ?? { kind: "all" })),
      subagents: sessionListSubagentsParam(search),
      ...(archived ? { archive: "only" as const } : {}),
      ...lens,
    }),
    enabled: grouped,
  });
  const profiles = useProfiles(grouped && groupBy === "profile");
  const owners: ProfileOwner[] = (profiles.data ?? []).map(row => ({
    id: row.id,
    name: row.name,
    color: row.color,
    icon: row.icon,
    emoji: row.emoji,
    archived: row.archived_at != null,
  }));
  const groups =
    !grouped || scope === null
      ? []
      : groupBy === "project"
        ? projectSessionGroups(facets.data, { workspaces, lens })
        : profileSessionGroups(profiles.data ? facets.data : undefined, {
            profiles: owners,
            scope,
          });
  const failed = facets.isError || (groupBy === "profile" && profiles.isError);
  return {
    groups,
    loading: grouped && (facets.isLoading || (groupBy === "profile" && profiles.isLoading)),
    error:
      grouped && failed
        ? {
            retry: () => {
              void facets.refetch();
              if (groupBy === "profile") void profiles.refetch();
            },
          }
        : undefined,
  };
}
