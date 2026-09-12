import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import { useProfileReadScope } from "@/systems/profiles";

import {
  deleteAgentHeartbeat,
  putAgentHeartbeat,
  rollbackAgentHeartbeat,
  validateAgentHeartbeat,
  wakeAgentHeartbeat,
  type FetchAgentHeartbeatStatusParams,
} from "../adapters/agent-heartbeat-api";
import {
  agentHeartbeatHistoryOptions,
  agentHeartbeatOptions,
  agentHeartbeatStatusOptions,
} from "../lib/query-options";
import { agentKeys } from "../lib/query-keys";
import type {
  AgentAuthoredMutationVariables,
  DeleteAgentHeartbeatParams,
  PutAgentHeartbeatParams,
  RollbackAgentHeartbeatParams,
  ValidateAgentHeartbeatParams,
  WakeAgentHeartbeatParams,
} from "../types";

interface UseAgentHeartbeatOptions {
  enabled?: boolean;
}

export function useAgentHeartbeat(
  name: string,
  workspace?: string | null,
  options: UseAgentHeartbeatOptions = {}
) {
  const { destination } = useProfileReadScope();
  return useQuery({
    ...agentHeartbeatOptions(name, workspace, destination),
    enabled: (options.enabled ?? true) && !!name,
  });
}

export function useAgentHeartbeatHistory(
  name: string,
  workspace?: string | null,
  options: UseAgentHeartbeatOptions = {}
) {
  const { destination } = useProfileReadScope();
  return useQuery({
    ...agentHeartbeatHistoryOptions(name, workspace, destination),
    enabled: (options.enabled ?? true) && !!name,
  });
}

export function useAgentHeartbeatStatus(
  name: string,
  statusOptions: FetchAgentHeartbeatStatusParams = {},
  options: UseAgentHeartbeatOptions = {}
) {
  const { destination } = useProfileReadScope();
  return useQuery({
    ...agentHeartbeatStatusOptions(name, { ...statusOptions, profile: destination }),
    enabled: (options.enabled ?? true) && !!name,
  });
}

function invalidateHeartbeatQueries(
  queryClient: ReturnType<typeof useQueryClient>,
  name: string,
  workspace: string | null,
  profile: string
) {
  return Promise.all([
    queryClient.invalidateQueries({ queryKey: agentKeys.heartbeat(name, workspace, profile) }),
    queryClient.invalidateQueries({
      queryKey: agentKeys.heartbeatHistory(name, workspace, profile),
    }),
    queryClient.invalidateQueries({
      queryKey: agentKeys.heartbeatStatuses(name, workspace, profile),
    }),
  ]);
}

export function useValidateAgentHeartbeat() {
  return useMutation({
    mutationFn: ({
      name,
      params,
      profile,
    }: AgentAuthoredMutationVariables<ValidateAgentHeartbeatParams>) =>
      validateAgentHeartbeat(name, params, undefined, profile),
  });
}

export function usePutAgentHeartbeat() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({
      name,
      params,
      profile,
    }: AgentAuthoredMutationVariables<PutAgentHeartbeatParams>) =>
      putAgentHeartbeat(name, params, undefined, profile),
    onSuccess: (data, { name, cacheWorkspace, profile }) => {
      queryClient.setQueryData(agentKeys.heartbeat(name, cacheWorkspace, profile), data.heartbeat);
    },
    onSettled: (_data, _error, { name, cacheWorkspace, profile }) =>
      invalidateHeartbeatQueries(queryClient, name, cacheWorkspace, profile),
  });
}

export function useDeleteAgentHeartbeat() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({
      name,
      params,
      profile,
    }: AgentAuthoredMutationVariables<DeleteAgentHeartbeatParams>) =>
      deleteAgentHeartbeat(name, params, undefined, profile),
    onSettled: (_data, _error, { name, cacheWorkspace, profile }) =>
      invalidateHeartbeatQueries(queryClient, name, cacheWorkspace, profile),
  });
}

export function useRollbackAgentHeartbeat() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({
      name,
      params,
      profile,
    }: AgentAuthoredMutationVariables<RollbackAgentHeartbeatParams>) =>
      rollbackAgentHeartbeat(name, params, undefined, profile),
    onSuccess: (data, { name, cacheWorkspace, profile }) => {
      queryClient.setQueryData(agentKeys.heartbeat(name, cacheWorkspace, profile), data.heartbeat);
    },
    onSettled: (_data, _error, { name, cacheWorkspace, profile }) =>
      invalidateHeartbeatQueries(queryClient, name, cacheWorkspace, profile),
  });
}

export function useWakeAgentHeartbeat() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({
      name,
      params,
      profile,
    }: AgentAuthoredMutationVariables<WakeAgentHeartbeatParams>) =>
      wakeAgentHeartbeat(name, params, undefined, profile),
    onSettled: (_data, _error, { name, cacheWorkspace, profile }) => {
      void queryClient.invalidateQueries({
        queryKey: agentKeys.heartbeatStatuses(name, cacheWorkspace, profile),
      });
      void queryClient.invalidateQueries({
        queryKey: agentKeys.heartbeat(name, cacheWorkspace, profile),
      });
    },
  });
}
