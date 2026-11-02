import { useMutation, useQuery, useQueryClient, type QueryClient } from "@tanstack/react-query";

import { useProfileReadScope } from "@/systems/profiles";

import {
  putAgentMemory,
  rollbackAgentMemory,
  validateAgentMemory,
} from "../adapters/agent-memory-api";
import { agentMemoryHistoryOptions, agentMemoryOptions } from "../lib/query-options";
import { agentKeys } from "../lib/query-keys";
import { useAgentWindowScope } from "./use-agent-window-scope";
import type {
  AgentAuthoredMutationVariables,
  AgentMemoryDocument,
  PutAgentMemoryParams,
  RollbackAgentMemoryParams,
  ValidateAgentMemoryParams,
} from "../types";

export function useAgentMemory(name: string, workspace?: string | null) {
  const { destination } = useProfileReadScope();
  return useQuery(agentMemoryOptions(name, workspace, destination));
}

/** The setup list's line: how many notes it holds, read only for agents with memory. */
export function useAgentMemorySummary(agent: { name: string; kind?: string | null }) {
  const { destination } = useProfileReadScope();
  const { workspace } = useAgentWindowScope();
  const { data } = useQuery({
    ...agentMemoryOptions(agent.name, workspace, destination),
    enabled: agent.kind === "stays_on",
  });
  if (!data) return "Nothing yet";
  const notes = data.content.split("\n").filter(line => line.trim() !== "").length;
  return notes === 0 ? "Nothing yet" : `${notes} ${notes === 1 ? "note" : "notes"}`;
}

export function useAgentMemoryHistory(name: string, workspace?: string | null) {
  const { destination } = useProfileReadScope();
  return useQuery(agentMemoryHistoryOptions(name, workspace, destination));
}

function adoptMemory(
  queryClient: QueryClient,
  document: AgentMemoryDocument,
  { name, cacheWorkspace, profile }: AgentAuthoredMutationVariables<unknown>
) {
  queryClient.setQueryData(agentKeys.memory(name, cacheWorkspace, profile), document);
}

function invalidateMemory(
  queryClient: QueryClient,
  { name, cacheWorkspace, profile }: AgentAuthoredMutationVariables<unknown>
) {
  return queryClient.invalidateQueries({
    queryKey: agentKeys.memory(name, cacheWorkspace, profile),
  });
}

export function usePutAgentMemory() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({
      name,
      params,
      cacheWorkspace,
      profile,
    }: AgentAuthoredMutationVariables<PutAgentMemoryParams>) =>
      putAgentMemory(name, params, cacheWorkspace, undefined, profile),
    onSuccess: (document, variables) => adoptMemory(queryClient, document, variables),
    onSettled: (_data, _error, variables) => invalidateMemory(queryClient, variables),
  });
}

export function useValidateAgentMemory() {
  return useMutation({
    mutationFn: ({
      name,
      params,
      cacheWorkspace,
      profile,
    }: AgentAuthoredMutationVariables<ValidateAgentMemoryParams>) =>
      validateAgentMemory(name, params, cacheWorkspace, undefined, profile),
  });
}

export function useRollbackAgentMemory() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({
      name,
      params,
      cacheWorkspace,
      profile,
    }: AgentAuthoredMutationVariables<RollbackAgentMemoryParams>) =>
      rollbackAgentMemory(name, params, cacheWorkspace, undefined, profile),
    onSuccess: (document, variables) => adoptMemory(queryClient, document, variables),
    onSettled: (_data, _error, variables) => invalidateMemory(queryClient, variables),
  });
}
