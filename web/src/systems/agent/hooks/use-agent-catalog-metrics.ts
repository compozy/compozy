import { useAgentCatalog } from "./use-agents";

interface UseAgentCatalogMetricsOptions {
  enabled?: boolean;
}

export interface AgentCatalogMetrics {
  /** Workspace-scoped session total for this agent. */
  total: number;
  /** Workspace-scoped active session count. */
  active: number;
  /** Null when the catalog reports sessions unavailable. */
  failed: number | null;
  /** Null when the catalog reports sessions unavailable. */
  runtimeSeconds: number | null;
  lastActivityAt: string | null;
  /** True when catalog returned exact session aggregates for this agent. */
  sessionsAvailable: boolean;
  isLoading: boolean;
  isError: boolean;
}

const EMPTY_METRICS = {
  total: 0,
  active: 0,
  failed: null,
  runtimeSeconds: null,
  lastActivityAt: null,
  sessionsAvailable: false,
} as const;

/**
 * Exact workspace-scoped agent metrics from `listAgentCatalog` item.sessions.
 * Never derive Failed/Runtime/Last activity from loaded session pages.
 */
export function useAgentCatalogMetrics(
  workspaceId: string | null,
  agentName: string | undefined,
  options?: UseAgentCatalogMetricsOptions
): AgentCatalogMetrics {
  const enabled = (options?.enabled ?? true) && Boolean(workspaceId) && Boolean(agentName);
  const catalogQuery = useAgentCatalog(
    workspaceId ?? "",
    { name: agentName, limit: 1 },
    { enabled }
  );
  // Exact name is the contract — never map another agent's metrics onto this view.
  const catalogItem = catalogQuery.agents.find(item => item.agent.name === agentName);
  const sessions = catalogItem?.sessions;

  if (!catalogQuery.sessionsAvailable || !sessions) {
    return {
      ...EMPTY_METRICS,
      isLoading: enabled && catalogQuery.isLoading,
      isError: catalogQuery.isError,
    };
  }

  return {
    total: sessions.total,
    active: sessions.active,
    failed: sessions.failed,
    runtimeSeconds: sessions.runtime_seconds,
    lastActivityAt: sessions.last_activity_at ?? null,
    sessionsAvailable: true,
    isLoading: enabled && catalogQuery.isLoading,
    isError: catalogQuery.isError,
  };
}
