import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import {
  deleteAgentSoul,
  putAgentSoul,
  rollbackAgentSoul,
  validateAgentSoul,
} from "../adapters/agent-soul-api";
import { agentSoulHistoryOptions, agentSoulOptions } from "../lib/query-options";
import { agentKeys } from "../lib/query-keys";
import type {
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
  return useQuery({
    ...agentSoulOptions(name, workspace),
    enabled: (options.enabled ?? true) && !!name,
  });
}

export function useAgentSoulHistory(
  name: string,
  workspace?: string | null,
  options: UseAgentSoulOptions = {}
) {
  return useQuery({
    ...agentSoulHistoryOptions(name, workspace),
    enabled: (options.enabled ?? true) && !!name,
  });
}

function invalidateSoulQueries(
  queryClient: ReturnType<typeof useQueryClient>,
  name: string,
  workspace?: string | null
) {
  return Promise.all([
    queryClient.invalidateQueries({ queryKey: agentKeys.soul(name, workspace) }),
    queryClient.invalidateQueries({ queryKey: agentKeys.soulHistory(name, workspace) }),
  ]);
}

export function useValidateAgentSoul(name: string) {
  return useMutation({
    mutationFn: (params: ValidateAgentSoulParams) => validateAgentSoul(name, params),
  });
}

export function usePutAgentSoul(name: string, workspace?: string | null) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (params: PutAgentSoulParams) => putAgentSoul(name, params),
    onSuccess: data => {
      queryClient.setQueryData(agentKeys.soul(name, workspace), data.soul);
    },
    onSettled: () => invalidateSoulQueries(queryClient, name, workspace),
  });
}

export function useDeleteAgentSoul(name: string, workspace?: string | null) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (params: DeleteAgentSoulParams) => deleteAgentSoul(name, params),
    onSettled: () => invalidateSoulQueries(queryClient, name, workspace),
  });
}

export function useRollbackAgentSoul(name: string, workspace?: string | null) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (params: RollbackAgentSoulParams) => rollbackAgentSoul(name, params),
    onSuccess: data => {
      queryClient.setQueryData(agentKeys.soul(name, workspace), data.soul);
    },
    onSettled: () => invalidateSoulQueries(queryClient, name, workspace),
  });
}
