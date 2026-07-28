import { useInfiniteQuery, useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import { getNavCountsStore } from "@/systems/runtime";
import { workspaceKeys } from "@/systems/workspace";

import { createAgent, deleteAgent, duplicateAgent, updateAgent } from "../adapters/agent-api";
import { agentCatalogOptions, agentsListOptions, agentDetailOptions } from "../lib/query-options";
import { agentKeys } from "../lib/query-keys";
import { agentCatalogPage, flattenAgentCatalogPages } from "../lib/agent-catalog-query";
import type {
  AgentCatalogStableFilter,
  AgentPayload,
  CreateAgentParams,
  DeleteAgentResponse,
  DuplicateAgentParams,
  UpdateAgentParams,
} from "../types";

interface UseAgentsOptions {
  enabled?: boolean;
}

function invalidateAgentCollectionQueries(
  queryClient: ReturnType<typeof useQueryClient>,
  settledError: unknown,
  workspace?: string | null
): void {
  // Callers attach onError for UX; settlement still refreshes lists after any outcome.
  queryClient.invalidateQueries({ queryKey: agentKeys.lists() });
  queryClient.invalidateQueries({ queryKey: agentKeys.catalogs() });
  if (workspace) {
    queryClient.invalidateQueries({ queryKey: workspaceKeys.detail(workspace) });
  }
  if (settledError) return;
}

export function useAgents(workspace?: string | null, options: UseAgentsOptions = {}) {
  return useQuery({
    ...agentsListOptions(workspace),
    enabled: options.enabled ?? true,
  });
}

export function useAgentCatalog(
  workspace: string,
  filters: AgentCatalogStableFilter = {},
  options: UseAgentsOptions = {}
) {
  const query = useInfiniteQuery({
    ...agentCatalogOptions(workspace, filters),
    enabled: Boolean(workspace) && (options.enabled ?? true),
  });
  const firstPage = agentCatalogPage(query.data);
  return {
    ...query,
    agents: flattenAgentCatalogPages(query.data),
    facets: firstPage?.facets,
    sessionsAvailable: firstPage?.sessions_available ?? false,
    total: firstPage?.page.total ?? 0,
  };
}

export function useAgent(name: string, workspace?: string | null) {
  return useQuery(agentDetailOptions(name, workspace));
}

export function useCreateAgent() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (params: CreateAgentParams) => createAgent(params),
    onSuccess: (agent, params) => {
      const workspace = params.scope === "workspace" ? params.workspace : null;
      queryClient.setQueryData<AgentPayload>(agentKeys.detail(agent.name, workspace), agent);
    },
    onSettled: (_agent, error, params) => {
      invalidateAgentCollectionQueries(
        queryClient,
        error,
        params?.scope === "workspace" ? params.workspace : null
      );
    },
  });
}

export interface UpdateAgentVariables {
  name: string;
  params: UpdateAgentParams;
  /**
   * Workspace key used by `useAgent` for this view.
   * Must match the read key even when the winning agent is global
   * (`params.workspace` omitted/null) so success writes do not miss the cache.
   */
  cacheWorkspace: string | null;
}

export function useUpdateAgent() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({ name, params }: UpdateAgentVariables) => updateAgent(name, params),
    onSuccess: (agent, variables) => {
      // Write only the exact detail key for this view — never broadcast to all
      // same-name workspace caches, and never fall back to params.workspace
      // (null for global winners viewed under an active workspace).
      queryClient.setQueryData<AgentPayload>(
        agentKeys.detail(agent.name, variables.cacheWorkspace),
        agent
      );
    },
    onSettled: (_agent, error, variables) => {
      invalidateAgentCollectionQueries(queryClient, error, variables?.cacheWorkspace);
    },
  });
}

export interface DeleteAgentVariables {
  name: string;
  workspace?: string | null;
}

export function useDeleteAgent() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({ name, workspace }: DeleteAgentVariables) => deleteAgent(name, workspace),
    onSuccess: (_result: DeleteAgentResponse, variables) => {
      const workspace = variables.workspace ?? null;
      queryClient.removeQueries({ queryKey: agentKeys.detail(variables.name, workspace) });
      getNavCountsStore(workspace).getState().refresh();
    },
    onSettled: (_result, error, variables) => {
      invalidateAgentCollectionQueries(queryClient, error, variables?.workspace);
    },
  });
}

export interface DuplicateAgentVariables {
  sourceName: string;
  params: DuplicateAgentParams;
}

export function useDuplicateAgent() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({ sourceName, params }: DuplicateAgentVariables) =>
      duplicateAgent(sourceName, params),
    onSuccess: (agent, variables) => {
      const workspace =
        variables.params.scope === "workspace" ? (variables.params.workspace ?? null) : null;
      queryClient.setQueryData<AgentPayload>(agentKeys.detail(agent.name, workspace), agent);
    },
    onSettled: (_agent, error, variables) => {
      invalidateAgentCollectionQueries(
        queryClient,
        error,
        variables?.params.scope === "workspace" ? variables.params.workspace : null
      );
    },
  });
}
