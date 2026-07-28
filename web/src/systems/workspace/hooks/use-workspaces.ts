import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import {
  createWorkspace,
  deleteWorkspace,
  resolveWorkspace,
  type CreateWorkspaceParams,
  type ResolveWorkspaceParams,
} from "@/systems/workspace/adapters/workspace-api";
import { workspaceKeys } from "@/systems/workspace/lib/query-keys";
import {
  workspaceDetailOptions,
  workspacesListOptions,
} from "@/systems/workspace/lib/query-options";
import type { WorkspacePayload } from "@/systems/workspace/types";

import { reconcileWorkspaceList } from "../lib/workspace-list-reconciliation";

interface UseWorkspaceOptions {
  enabled?: boolean;
}

export function useWorkspaces() {
  return useQuery(workspacesListOptions());
}

export function useWorkspace(workspaceID: string, options?: UseWorkspaceOptions) {
  return useQuery({
    ...workspaceDetailOptions(workspaceID),
    enabled: options?.enabled ?? Boolean(workspaceID),
  });
}

export function useResolveWorkspace() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (params: ResolveWorkspaceParams) => resolveWorkspace(params),
    onSuccess: workspace => {
      queryClient.setQueryData<WorkspacePayload[]>(workspaceKeys.list(), current =>
        reconcileWorkspaceList(current, workspace)
      );
      queryClient.invalidateQueries({ queryKey: workspaceKeys.lists() });
    },
  });
}

export function useCreateWorkspace() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (params: CreateWorkspaceParams) => createWorkspace(params),
    onSuccess: workspace => {
      queryClient.setQueryData<WorkspacePayload[]>(workspaceKeys.list(), current =>
        reconcileWorkspaceList(current, workspace)
      );
      queryClient.invalidateQueries({ queryKey: workspaceKeys.lists() });
    },
  });
}

export function useDeleteWorkspace() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (workspaceID: string) => deleteWorkspace(workspaceID),
    onSuccess: (_data, workspaceID) => {
      queryClient.setQueryData<WorkspacePayload[]>(workspaceKeys.list(), current =>
        current?.filter(workspace => workspace.id !== workspaceID)
      );
      queryClient.removeQueries({ queryKey: workspaceKeys.detail(workspaceID) });
      return queryClient.invalidateQueries({ queryKey: workspaceKeys.lists() });
    },
  });
}
