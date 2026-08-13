import { useMutation, useQuery, useQueryClient, type QueryClient } from "@tanstack/react-query";

import {
  cancelWorktreeExitAction,
  fetchWorktreeStatus,
  runWorktreeExitAction,
} from "../adapters/worktree-exit-api";
import { workspaceKeys } from "../lib/query-keys";
import {
  worktreeDetailOptions,
  worktreeExitPlanOptions,
  worktreeStatusOptions,
} from "../lib/query-options";
import type { RunWorktreeExitActionParams } from "../types";

interface WorktreeScopeOptions {
  enabled?: boolean;
}

export function useWorktreeDetail(
  workspaceID: string,
  worktreeID: string,
  options: WorktreeScopeOptions = {}
) {
  return useQuery(worktreeDetailOptions(workspaceID, worktreeID, options.enabled ?? true));
}

export function useWorktreeStatus(
  workspaceID: string,
  worktreeID: string,
  options: { enabled?: boolean; refresh?: boolean } = {}
) {
  return useQuery(worktreeStatusOptions(workspaceID, worktreeID, options));
}

export function useWorktreeExitPlan(
  workspaceID: string,
  worktreeID: string,
  options: WorktreeScopeOptions = {}
) {
  return useQuery(worktreeExitPlanOptions(workspaceID, worktreeID, options.enabled ?? true));
}

/**
 * Every worktree-qualified read the exit surface owns. Used after an action
 * settles and by the per-worktree stream — both must reread the same set.
 */
export function invalidateWorktreeExitScope(
  queryClient: QueryClient,
  workspaceID: string,
  worktreeID: string
): Promise<unknown> {
  return Promise.all([
    queryClient.invalidateQueries({ queryKey: workspaceKeys.worktrees(workspaceID) }),
    queryClient.invalidateQueries({
      queryKey: workspaceKeys.worktreeDetail(workspaceID, worktreeID),
      exact: true,
    }),
    queryClient.invalidateQueries({
      queryKey: workspaceKeys.worktreeStatus(workspaceID, worktreeID),
      exact: true,
    }),
    queryClient.invalidateQueries({
      queryKey: workspaceKeys.worktreeExit(workspaceID, worktreeID),
      exact: true,
    }),
  ]);
}

/**
 * Resolves with the durable operation id. The action itself keeps running
 * daemon-side, so callers follow the per-worktree stream for progress and never
 * treat this resolution as completion.
 */
export function useRunWorktreeExitAction(workspaceID: string, worktreeID: string) {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (params: RunWorktreeExitActionParams) =>
      runWorktreeExitAction(workspaceID, worktreeID, params),
    onSuccess: () => invalidateWorktreeExitScope(queryClient, workspaceID, worktreeID),
  });
}

export function useCancelWorktreeExitAction(workspaceID: string, worktreeID: string) {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (operationID: string) =>
      cancelWorktreeExitAction(workspaceID, worktreeID, operationID),
    onSuccess: () => invalidateWorktreeExitScope(queryClient, workspaceID, worktreeID),
  });
}

/**
 * Explicit operator refresh. `forge` is only ever true here — the forge tier is
 * never refreshed on mount, so one gesture costs at most one provider call.
 */
export function useRefreshWorktreeStatus(workspaceID: string, worktreeID: string) {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({ forge = false }: { forge?: boolean } = {}) =>
      fetchWorktreeStatus(workspaceID, worktreeID, { refresh: true, forge }),
    onSuccess: status => {
      queryClient.setQueryData(workspaceKeys.worktreeStatus(workspaceID, worktreeID), status);
      return invalidateWorktreeExitScope(queryClient, workspaceID, worktreeID);
    },
  });
}
