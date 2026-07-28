import { useSessions } from "@/systems/session";
import type { SessionPayload } from "@/systems/session";

interface UseAgentSessionsOptions {
  enabled?: boolean;
}

interface UseAgentSessionsResult {
  sessions: SessionPayload[];
  hasMore: boolean;
  isLoadingMore: boolean;
  loadMore: () => void;
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

  return {
    sessions: sessionsQuery.data ?? [],
    hasMore: sessionsQuery.hasNextPage,
    isLoadingMore: sessionsQuery.isFetchingNextPage,
    loadMore: () => {
      void sessionsQuery.fetchNextPage();
    },
    isLoading: sessionsQuery.isLoading,
    isError: sessionsQuery.isError,
  };
}
