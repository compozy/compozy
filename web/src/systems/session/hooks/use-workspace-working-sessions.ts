import { useQuery } from "@tanstack/react-query";

import { useProfileReadScope } from "@/systems/profiles";

import { sessionFacetsOptions } from "../lib/session-catalog-options";
import { sessionListSubagentsParam } from "../lib/session-list-preferences";
import type { fetchSessionFacets } from "../adapters/session-catalog-api";

/** Working-session counts keyed by workspace id; an absent workspace has none. */
export type WorkspaceWorkingSessions = Readonly<Record<string, number>>;

type SessionFacets = Awaited<ReturnType<typeof fetchSessionFacets>>;

const NONE: WorkspaceWorkingSessions = {};

export function workingSessionsByWorkspace(
  facets: Pick<SessionFacets, "by_workspace"> | undefined
): WorkspaceWorkingSessions {
  if (!facets) return NONE;
  const counts: Record<string, number> = {};
  for (const row of facets.by_workspace) {
    if (row.facets.working > 0) counts[row.workspace_id] = row.facets.working;
  }
  return counts;
}

/**
 * Which workspaces have sessions working right now, from the one exact
 * cross-workspace aggregate the grouped sidebar already reads. Catalog wakes
 * re-read it, so a turn starting or settling anywhere updates every switcher.
 */
export function useWorkspaceWorkingSessions(enabled: boolean): WorkspaceWorkingSessions {
  const { params } = useProfileReadScope();
  const facets = useQuery({
    ...sessionFacetsOptions({
      all_workspaces: true,
      // Same population as the sidebar's Working count: a parent row carries its subagents.
      subagents: sessionListSubagentsParam(undefined),
      ...params,
    }),
    enabled,
  });
  return workingSessionsByWorkspace(facets.data);
}
