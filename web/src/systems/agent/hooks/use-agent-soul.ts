import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import { useProfileReadScope } from "@/systems/profiles";

import {
  deleteAgentSoul,
  putAgentSoul,
  rollbackAgentSoul,
  validateAgentSoul,
} from "../adapters/agent-soul-api";
import { agentSoulHistoryOptions, agentSoulOptions } from "../lib/query-options";
import { agentKeys } from "../lib/query-keys";
import type {
  AgentAuthoredMutationVariables,
  DeleteAgentSoulParams,
  PutAgentSoulParams,
  RollbackAgentSoulParams,
  ValidateAgentSoulParams,
} from "../types";

interface UseAgentSoulOptions {
  enabled?: boolean;
}

export function useAgentSoul(
  name: string,
  workspace?: string | null,
  options: UseAgentSoulOptions = {}
) {
  const { destination } = useProfileReadScope();
  return useQuery({
    ...agentSoulOptions(name, workspace, destination),
    enabled: (options.enabled ?? true) && !!name,
  });
}

export function useAgentSoulHistory(
  name: string,
  workspace?: string | null,
  options: UseAgentSoulOptions = {}
) {
  const { destination } = useProfileReadScope();
  return useQuery({
    ...agentSoulHistoryOptions(name, workspace, destination),
    enabled: (options.enabled ?? true) && !!name,
  });
}

function invalidateSoulQueries(
  queryClient: ReturnType<typeof useQueryClient>,
  name: string,
  workspace: string | null,
  profile: string
) {
  return Promise.all([
    queryClient.invalidateQueries({ queryKey: agentKeys.soul(name, workspace, profile) }),
    queryClient.invalidateQueries({ queryKey: agentKeys.soulHistory(name, workspace, profile) }),
  ]);
}

export function useValidateAgentSoul() {
  return useMutation({
    mutationFn: ({
      name,
      params,
      profile,
    }: AgentAuthoredMutationVariables<ValidateAgentSoulParams>) =>
      validateAgentSoul(name, params, undefined, profile),
  });
}

export function usePutAgentSoul() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ name, params, profile }: AgentAuthoredMutationVariables<PutAgentSoulParams>) =>
      putAgentSoul(name, params, undefined, profile),
    onSuccess: (data, { name, cacheWorkspace, profile }) => {
      queryClient.setQueryData(agentKeys.soul(name, cacheWorkspace, profile), data.soul);
    },
    onSettled: (_data, _error, { name, cacheWorkspace, profile }) =>
      invalidateSoulQueries(queryClient, name, cacheWorkspace, profile),
  });
}

export function useDeleteAgentSoul() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({
      name,
      params,
      profile,
    }: AgentAuthoredMutationVariables<DeleteAgentSoulParams>) =>
      deleteAgentSoul(name, params, undefined, profile),
    onSettled: (_data, _error, { name, cacheWorkspace, profile }) =>
      invalidateSoulQueries(queryClient, name, cacheWorkspace, profile),
  });
}

export function useRollbackAgentSoul() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({
      name,
      params,
      profile,
    }: AgentAuthoredMutationVariables<RollbackAgentSoulParams>) =>
      rollbackAgentSoul(name, params, undefined, profile),
    onSuccess: (data, { name, cacheWorkspace, profile }) => {
      queryClient.setQueryData(agentKeys.soul(name, cacheWorkspace, profile), data.soul);
    },
    onSettled: (_data, _error, { name, cacheWorkspace, profile }) =>
      invalidateSoulQueries(queryClient, name, cacheWorkspace, profile),
  });
}
