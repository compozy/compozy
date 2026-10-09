import { useQuery } from "@tanstack/react-query";

import { sessionFacetsOptions } from "../lib/session-catalog-options";
import {
  sessionListSortParam,
  sessionListSubagentsParam,
  type SessionListSort,
} from "../lib/session-list-preferences";
import type { SessionListFilters, SessionPayload } from "../types";
import { useProfileReadScope } from "@/systems/profiles";

const WORKSPACE_GROUP_PAGE_SIZE = 100;

export interface WorkspaceSessionGroup {
  workspaceId: string;
  workspaceName: string;
  sessions: SessionPayload[];
  /** Daemon-reported total, not the loaded page length. */
  total: number | undefined;
  catalogFilters?: SessionListFilters;
  enabled?: boolean;
  loading: boolean;
  failed: boolean;
  retry: () => void;
}

export interface WorkspaceSessionGroupsInput {
  workspaces: ReadonlyArray<{ id: string; name: string }>;
  sort: SessionListSort;
  /** Read the archive instead of the active catalog. */
  archived: boolean;
  enabled: boolean;
  search?: string;
}

// Group counts share one scoped aggregate; each expanded group owns its bounded page.
export function useWorkspaceSessionGroups({
  workspaces,
  sort,
  archived,
  enabled,
  search,
}: WorkspaceSessionGroupsInput): WorkspaceSessionGroup[] {
  const { params } = useProfileReadScope();
  const facets = useQuery({
    ...sessionFacetsOptions({
      all_workspaces: true,
      ...(archived ? { archive: "only" as const } : {}),
      ...params,
    }),
    enabled,
  });
  return workspaces.map(workspace => ({
    workspaceId: workspace.id,
    workspaceName: workspace.name,
    sessions: [],
    total:
      facets.data?.by_workspace.find(row => row.workspace_id === workspace.id)?.facets.all ??
      (facets.data ? 0 : undefined),
    loading: facets.isLoading,
    failed: facets.isError,
    retry: () => void facets.refetch(),
    enabled,
    catalogFilters: {
      workspace_id: workspace.id,
      include_health: true,
      limit: WORKSPACE_GROUP_PAGE_SIZE,
      sort: sessionListSortParam(sort),
      q: search,
      search_fields: "title_agent",
      subagents: sessionListSubagentsParam(search),
      ...(archived ? { archive: "only" as const } : {}),
      ...params,
    },
  }));
}
