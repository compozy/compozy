import { useQueries } from "@tanstack/react-query";

import { sessionsCompleteListOptions } from "../lib/query-options";
import { sessionListSortParam, type SessionListSort } from "../lib/session-list-preferences";
import type { SessionPayload } from "../types";
import { useProfileReadScope } from "@/systems/profiles";

const WORKSPACE_GROUP_PAGE_SIZE = 100;

export interface WorkspaceSessionGroup {
  workspaceId: string;
  workspaceName: string;
  sessions: SessionPayload[];
  /** Daemon-reported total, not the loaded page length. */
  total: number;
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
}

/**
 * One bounded query per workspace for the all-workspaces scope.
 *
 * Per-workspace queries — rather than a single cross-workspace read — are what
 * make the honest failure mode possible: a workspace that cannot be reached
 * shows an inline error inside its own group and offers a retry, while every
 * other workspace still lists (US-031.EC-1). A single query would blank the
 * whole list on one failure. Workspaces joining or leaving are picked up
 * automatically because the query set is derived from the live workspace list.
 *
 * Group counts read the daemon's `page.total`, so a collapsed group never
 * claims the loaded page is the whole story.
 */
export function useWorkspaceSessionGroups({
  workspaces,
  sort,
  archived,
  enabled,
}: WorkspaceSessionGroupsInput): WorkspaceSessionGroup[] {
  const { params } = useProfileReadScope();
  const results = useQueries({
    queries: workspaces.map(workspace => ({
      ...sessionsCompleteListOptions({
        workspace_id: workspace.id,
        include_health: true,
        limit: WORKSPACE_GROUP_PAGE_SIZE,
        sort: sessionListSortParam(sort),
        ...(archived ? { archive: "only" as const } : {}),
        ...params,
      }),
      enabled,
    })),
  });

  return workspaces.map((workspace, index) => {
    const query = results[index];
    return {
      workspaceId: workspace.id,
      workspaceName: workspace.name,
      sessions: query?.data?.sessions ?? [],
      total: query?.data?.page.total ?? 0,
      loading: query?.isLoading ?? false,
      failed: query?.isError ?? false,
      retry: () => void query?.refetch(),
    };
  });
}
