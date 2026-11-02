import { useMutation, useQuery, useQueryClient, type QueryClient } from "@tanstack/react-query";
import { useEffect, useRef } from "react";

import {
  cancelWorktreeExitAction,
  fetchWorktreeStatus,
  prepareWorktreeExitPlan,
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

/**
 * A plan read refreshes and persists the worktree status on the daemon without
 * announcing it on the stream, so the status strip is told here: each plan read
 * that settles, in success or failure, rereads the status key, which is a cached
 * read and emits nothing. A plan already cached at mount settled nothing new.
 */
export function useWorktreeExitPlan(
  workspaceID: string,
  worktreeID: string,
  options: WorktreeScopeOptions = {}
) {
  const queryClient = useQueryClient();
  const query = useQuery(worktreeExitPlanOptions(workspaceID, worktreeID, options.enabled ?? true));
  const settledAt = Math.max(query.dataUpdatedAt, query.errorUpdatedAt);
  const seenSettledAt = useRef(settledAt);
  useEffect(() => {
    if (settledAt === seenSettledAt.current) return;
    seenSettledAt.current = settledAt;
    void queryClient.invalidateQueries({
      queryKey: workspaceKeys.worktreeStatus(workspaceID, worktreeID),
      exact: true,
    });
  }, [settledAt, queryClient, workspaceID, worktreeID]);
  return query;
}

export function usePrepareWorktreeExitPlan(workspaceID: string, worktreeID: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: () => prepareWorktreeExitPlan(workspaceID, worktreeID),
    onSuccess: async plan => {
      const queryKey = workspaceKeys.worktreeExit(workspaceID, worktreeID);
      await queryClient.cancelQueries({ queryKey, exact: true });
      queryClient.setQueryData(queryKey, plan);
    },
  });
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
    queryClient.invalidateQueries({ queryKey: workspaceKeys.worktrees(workspaceID), exact: true }),
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
    // A refused action emits no stream frame, so its stale plan is reread here too.
    onSettled: () => invalidateWorktreeExitScope(queryClient, workspaceID, worktreeID),
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
