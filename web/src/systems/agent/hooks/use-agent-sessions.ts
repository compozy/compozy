import { type SessionPayload, useSessions } from "@/systems/session";

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
  const sessionsQuery = useSessions(workspaceId, {
    enabled,
    filters: { agent: agentName, sort: "last_activity" },
  });
  const archivedSessionsQuery = useSessions(workspaceId, {
    enabled,
    filters: { agent: agentName, archive: "only", sort: "last_activity" },
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
    isLoading: sessionsQuery.isLoading || archivedSessionsQuery.isLoading,
    isError: sessionsQuery.isError || archivedSessionsQuery.isError,
  };
}
