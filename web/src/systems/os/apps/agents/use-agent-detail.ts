import { useNavigate } from "@tanstack/react-router";

import {
  resolveAgentDetailSearch,
  useAgent,
  useAgentCatalogMetrics,
  useAgentCreateHost,
  useAgentDeleteFlow,
  useAgentSessions,
  type AgentPayload,
  type AgentDetailSearch,
  type AgentDetailTab,
  type AgentInstructionFile,
  type AgentSessionFilter,
  type AgentSettingsSection,
  type ResolvedAgentDetailSearch,
} from "@/systems/agent";
import { useSessionCreate, type SessionPayload } from "@/systems/session";
import { useActiveWorkspace } from "@/systems/workspace";

export interface UseAgentDetailResult {
  agent: AgentPayload | undefined;
  agentLoading: boolean;
  agentError: Error | null;
  sessions: SessionPayload[];
  sessionsTotal: number;
  activeSessionsTotal: number;
  failedSessionsTotal: number | null;
  runtimeSeconds: number | null;
  /** True when catalog did not return exact aggregates (or the metrics query failed). */
  metricsUnavailable: boolean;
  metricsLoading: boolean;
  metricsError: boolean;
  lastSessionActivityAt: string | null;
  hasMoreSessions: boolean;
  isLoadingMoreSessions: boolean;
  onLoadMoreSessions: () => void;
  /** Row-page loading only — never OR'd with the metrics query. */
  sessionsLoading: boolean;
  /** Row-page error only — never OR'd with the metrics query. */
  sessionsError: boolean;
  search: ResolvedAgentDetailSearch;
  setTab: (tab: AgentDetailTab) => void;
  setFile: (file: AgentInstructionFile) => void;
  setFilter: (filter: AgentSessionFilter) => void;
  isCreatingForAgent: boolean;
  newSessionDisabled: boolean;
  onNewSession: () => void;
  onEditSettings: (section?: AgentSettingsSection) => void;
  onDuplicate: () => void;
  onDelete: () => void;
  onBackToAgents: () => void;
  deleteDialog: ReturnType<typeof useAgentDeleteFlow>["confirmDialog"];
}

export function useAgentDetail(name: string, rawSearch: AgentDetailSearch): UseAgentDetailResult {
  const navigate = useNavigate();
  const { activeWorkspaceId } = useActiveWorkspace();
  const sessionCreate = useSessionCreate();
  const createHost = useAgentCreateHost();
  const search = resolveAgentDetailSearch(rawSearch);
  const {
    data: agent,
    isLoading: agentLoading,
    error: agentError,
  } = useAgent(name, activeWorkspaceId);
  const metrics = useAgentCatalogMetrics(activeWorkspaceId, name);
  const {
    sessions,
    hasMore: hasMoreSessions,
    isLoadingMore: isLoadingMoreSessions,
    loadMore: onLoadMoreSessions,
    isLoading: sessionsLoading,
    isError: sessionsError,
  } = useAgentSessions(activeWorkspaceId, name);

  const deleteFlow = useAgentDeleteFlow({
    agent,
    workspaceId: activeWorkspaceId,
  });

  const setTab = (tab: AgentDetailTab) => {
    void navigate({
      to: "/agents/$name",
      params: { name },
      search: { tab, file: search.file, filter: search.filter },
      replace: true,
    });
  };

  const setFile = (file: AgentInstructionFile) => {
    void navigate({
      to: "/agents/$name",
      params: { name },
      search: { tab: "instructions", file, filter: search.filter },
      replace: true,
    });
  };

  const setFilter = (filter: AgentSessionFilter) => {
    void navigate({
      to: "/agents/$name",
      params: { name },
      search: { tab: "sessions", file: search.file, filter },
      replace: true,
    });
  };

  const onNewSession = () => {
    sessionCreate.openForAgent(name);
  };

  const onEditSettings = (section?: AgentSettingsSection) => {
    void navigate({
      to: "/agents/$name/settings",
      params: { name },
      search: section ? { section } : {},
    });
  };

  const onDuplicate = () => {
    if (!agent) return;
    createHost.openForDuplicate(agent);
  };

  const onBackToAgents = () => {
    void navigate({ to: "/agents" });
  };

  return {
    agent,
    agentLoading,
    agentError: (agentError as Error | null) ?? null,
    sessions,
    sessionsTotal: metrics.total,
    activeSessionsTotal: metrics.active,
    failedSessionsTotal: metrics.failed,
    runtimeSeconds: metrics.runtimeSeconds,
    metricsUnavailable: metrics.isError || !metrics.sessionsAvailable,
    metricsLoading: metrics.isLoading,
    metricsError: metrics.isError,
    lastSessionActivityAt: metrics.lastActivityAt,
    hasMoreSessions,
    isLoadingMoreSessions,
    onLoadMoreSessions,
    sessionsLoading,
    sessionsError,
    search,
    setTab,
    setFile,
    setFilter,
    isCreatingForAgent: sessionCreate.isCreating && sessionCreate.pendingAgentName === name,
    newSessionDisabled: !sessionCreate.hasActiveWorkspace,
    onNewSession,
    onEditSettings,
    onDuplicate,
    onDelete: deleteFlow.openDialog,
    onBackToAgents,
    deleteDialog: deleteFlow.confirmDialog,
  };
}

export const useAgentDetailPage = useAgentDetail;
