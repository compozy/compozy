import { type SessionPayload, useSessions } from "@/systems/session";
import { useScopedWorktreeFilter } from "@/systems/workspace";
import { useWorktreeScopeId } from "@/hooks/use-window-scope";

interface UseAgentSessionsOptions {
  enabled?: boolean;
}

interface UseAgentSessionsResult {
  sessions: SessionPayload[];
  archivedSessions: SessionPayload[];
  archivedTotal: number;
  hasMore: boolean;
  isLoadingMore: boolean;
  loadMore: () => void;
  hasMoreArchived: boolean;
  isLoadingMoreArchived: boolean;
  loadMoreArchived: () => void;
  isLoading: boolean;
  isError: boolean;
}

/**
 * Paginated session rows for an agent. Metrics (Active/Failed/Runtime/Last activity)
 * come from `useAgentCatalogMetrics` — never derive them from this page.
 */
export function useAgentSessions(
  workspaceId: string | null,
  agentName: string | undefined,
  options?: UseAgentSessionsOptions
): UseAgentSessionsResult {
  const enabled = (options?.enabled ?? true) && Boolean(workspaceId) && Boolean(agentName);
  // This window's scope. `undefined` when the scope is the workspace root or the
  // selection fell back, which drops the filter instead of sending an empty one.
  const scopeId = useWorktreeScopeId();
  const worktree = useScopedWorktreeFilter(workspaceId, scopeId, { enabled });
  const queryEnabled = enabled && worktree.resolved;
  const sessionsQuery = useSessions(workspaceId, {
    enabled: queryEnabled,
    filters: { agent: agentName, sort: "last_activity", worktree: worktree.worktreeId },
  });
  const archivedSessionsQuery = useSessions(workspaceId, {
    enabled: queryEnabled,
    filters: {
      agent: agentName,
      archive: "only",
      sort: "last_activity",
      worktree: worktree.worktreeId,
    },
  });

  return {
    sessions: sessionsQuery.data ?? [],
    archivedSessions: archivedSessionsQuery.data ?? [],
    archivedTotal: archivedSessionsQuery.total,
    hasMore: sessionsQuery.hasNextPage,
    isLoadingMore: sessionsQuery.isFetchingNextPage,
    loadMore: () => {
      void sessionsQuery.fetchNextPage();
    },
    hasMoreArchived: archivedSessionsQuery.hasNextPage,
    isLoadingMoreArchived: archivedSessionsQuery.isFetchingNextPage,
    loadMoreArchived: () => {
      void archivedSessionsQuery.fetchNextPage();
    },
    isLoading:
      (enabled && !worktree.resolved) || sessionsQuery.isLoading || archivedSessionsQuery.isLoading,
    isError: sessionsQuery.isError || archivedSessionsQuery.isError,
  };
}
