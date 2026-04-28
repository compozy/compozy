import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import {
  listWorkspaces,
  resolveWorkspace,
  syncWorkspaces,
  type ResolveWorkspaceParams,
} from "../adapters/workspaces-api";
import { workspaceKeys } from "../lib/query-keys";
import type { Workspace, WorkspaceSyncResult } from "../types";

export function useWorkspaces() {
  return useQuery<Workspace[]>({
    queryKey: workspaceKeys.list(),
    queryFn: listWorkspaces,
  });
}

export function useResolveWorkspace() {
  const queryClient = useQueryClient();
  return useMutation<Workspace, Error, ResolveWorkspaceParams>({
    mutationFn: params => resolveWorkspace(params),
    onSuccess: workspace => {
      queryClient.setQueryData<Workspace[]>(workspaceKeys.list(), current => {
        const base = current ?? [];
        const rest = base.filter(entry => entry.id !== workspace.id);
        return [workspace, ...rest];
      });
    },
  });
}

export function useSyncWorkspaces() {
  const queryClient = useQueryClient();
  return useMutation<WorkspaceSyncResult, Error>({
    mutationFn: syncWorkspaces,
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: workspaceKeys.list() });
    },
  });
}
