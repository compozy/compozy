import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import {
  createWorkspace,
  setWorkspaceArchived,
  deleteWorkspace,
  resolveWorkspace,
  type CreateWorkspaceParams,
  type ResolveWorkspaceParams,
} from "@/systems/workspace/adapters/workspace-api";
import { sessionKeys } from "@/systems/session";
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

export function useWorkspaces(options: UseWorkspaceOptions = {}) {
  return useQuery(workspacesListOptions(options.enabled ?? true));
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
      return Promise.all([
        queryClient.invalidateQueries({ queryKey: workspaceKeys.lists() }),
        queryClient.invalidateQueries({ queryKey: sessionKeys.lists() }),
      ]);
    },
  });
}

export function useArchivedWorkspaces() {
  return useQuery(workspacesListOptions(true, true));
}

export function useSetWorkspaceArchived() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ id, archived }: { id: string; archived: boolean }) =>
      setWorkspaceArchived(id, archived),
    onSuccess: async workspace => {
      queryClient.setQueryData<WorkspacePayload[]>(workspaceKeys.list(), current =>
        workspace.archived_at
          ? current?.filter(item => item.id !== workspace.id)
          : reconcileWorkspaceList(current, workspace)
      );
      await queryClient.invalidateQueries();
    },
  });
}
