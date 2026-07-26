import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

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
  return useQuery({
    ...agentHeartbeatOptions(name, workspace),
    enabled: (options.enabled ?? true) && !!name,
  });
}

export function useAgentHeartbeatHistory(
  name: string,
  workspace?: string | null,
  options: UseAgentHeartbeatOptions = {}
) {
  return useQuery({
    ...agentHeartbeatHistoryOptions(name, workspace),
    enabled: (options.enabled ?? true) && !!name,
  });
}

export function useAgentHeartbeatStatus(
  name: string,
  statusOptions: FetchAgentHeartbeatStatusParams = {},
  options: UseAgentHeartbeatOptions = {}
) {
  return useQuery({
    ...agentHeartbeatStatusOptions(name, statusOptions),
    enabled: (options.enabled ?? true) && !!name,
  });
}

function invalidateHeartbeatQueries(
  queryClient: ReturnType<typeof useQueryClient>,
  name: string,
  workspace?: string | null
) {
  return Promise.all([
    queryClient.invalidateQueries({ queryKey: agentKeys.heartbeat(name, workspace) }),
    queryClient.invalidateQueries({ queryKey: agentKeys.heartbeatHistory(name, workspace) }),
    queryClient.invalidateQueries({ queryKey: agentKeys.heartbeatStatuses(name, workspace) }),
  ]);
}

export function useValidateAgentHeartbeat(name: string) {
  return useMutation({
    mutationFn: (params: ValidateAgentHeartbeatParams) => validateAgentHeartbeat(name, params),
  });
}

export function usePutAgentHeartbeat(name: string, workspace?: string | null) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (params: PutAgentHeartbeatParams) => putAgentHeartbeat(name, params),
    onSuccess: data => {
      queryClient.setQueryData(agentKeys.heartbeat(name, workspace), data.heartbeat);
    },
    onSettled: () => invalidateHeartbeatQueries(queryClient, name, workspace),
  });
}

export function useDeleteAgentHeartbeat(name: string, workspace?: string | null) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (params: DeleteAgentHeartbeatParams) => deleteAgentHeartbeat(name, params),
    onSettled: () => invalidateHeartbeatQueries(queryClient, name, workspace),
  });
}

export function useRollbackAgentHeartbeat(name: string, workspace?: string | null) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (params: RollbackAgentHeartbeatParams) => rollbackAgentHeartbeat(name, params),
    onSuccess: data => {
      queryClient.setQueryData(agentKeys.heartbeat(name, workspace), data.heartbeat);
    },
    onSettled: () => invalidateHeartbeatQueries(queryClient, name, workspace),
  });
}

export function useWakeAgentHeartbeat(name: string, workspace?: string | null) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (params: WakeAgentHeartbeatParams) => wakeAgentHeartbeat(name, params),
    onSettled: () => {
      void queryClient.invalidateQueries({
        queryKey: agentKeys.heartbeatStatuses(name, workspace),
      });
      void queryClient.invalidateQueries({
        queryKey: agentKeys.heartbeat(name, workspace),
      });
    },
  });
}
