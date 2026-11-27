import { useMutation, useQuery, useQueryClient, type QueryClient } from "@tanstack/react-query";
import { useEffect, useRef } from "react";

import { useCurrentComputerId } from "@/hooks/use-current-computer-id";
import { isRemoteComputerId, type ComputerId } from "@/lib/computer-scope";

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
  computerId?: ComputerId;
}

export function useWorktreeDetail(
  workspaceID: string,
  worktreeID: string,
  options: WorktreeScopeOptions = {}
) {
  const scopedComputerId = useCurrentComputerId(options.computerId);
  return useQuery(
    worktreeDetailOptions(workspaceID, worktreeID, options.enabled ?? true, scopedComputerId)
  );
}

export function useWorktreeStatus(
  workspaceID: string,
  worktreeID: string,
  options: { enabled?: boolean; refresh?: boolean; computerId?: ComputerId } = {}
) {
  const scopedComputerId = useCurrentComputerId(options.computerId);
  return useQuery(
    worktreeStatusOptions(workspaceID, worktreeID, { ...options, computerId: scopedComputerId })
  );
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
  const scopedComputerId = useCurrentComputerId(options.computerId);
  const query = useQuery(
    worktreeExitPlanOptions(workspaceID, worktreeID, options.enabled ?? true, scopedComputerId)
  );
  const settledAt = Math.max(query.dataUpdatedAt, query.errorUpdatedAt);
  const seenSettledAt = useRef(settledAt);
  useEffect(() => {
    if (settledAt === seenSettledAt.current) return;
    seenSettledAt.current = settledAt;
    void queryClient.invalidateQueries({
      queryKey: workspaceKeys.forComputer(scopedComputerId).worktreeStatus(workspaceID, worktreeID),
      exact: true,
    });
  }, [settledAt, queryClient, scopedComputerId, workspaceID, worktreeID]);
  return query;
}

export function usePrepareWorktreeExitPlan(
  workspaceID: string,
  worktreeID: string,
  computerId?: ComputerId
) {
  const queryClient = useQueryClient();
  const scopedComputerId = useCurrentComputerId(computerId);
  return useMutation({
    mutationFn: () =>
      isRemoteComputerId(scopedComputerId)
        ? prepareWorktreeExitPlan(workspaceID, worktreeID, undefined, scopedComputerId)
        : prepareWorktreeExitPlan(workspaceID, worktreeID),
    onSuccess: async plan => {
      const queryKey = workspaceKeys
        .forComputer(scopedComputerId)
        .worktreeExit(workspaceID, worktreeID);
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
  worktreeID: string,
  computerId?: ComputerId
): Promise<unknown> {
  const keys = workspaceKeys.forComputer(computerId);
  return Promise.all([
    queryClient.invalidateQueries({ queryKey: keys.worktrees(workspaceID), exact: true }),
    queryClient.invalidateQueries({
      queryKey: keys.worktreeDetail(workspaceID, worktreeID),
      exact: true,
    }),
    queryClient.invalidateQueries({
      queryKey: keys.worktreeStatus(workspaceID, worktreeID),
      exact: true,
    }),
    queryClient.invalidateQueries({
      queryKey: keys.worktreeExit(workspaceID, worktreeID),
      exact: true,
    }),
  ]);
}

/**
 * Resolves with the durable operation id. The action itself keeps running
 * daemon-side, so callers follow the per-worktree stream for progress and never
 * treat this resolution as completion.
 */
export function useRunWorktreeExitAction(
  workspaceID: string,
  worktreeID: string,
  computerId?: ComputerId
) {
  const queryClient = useQueryClient();
  const scopedComputerId = useCurrentComputerId(computerId);

  return useMutation({
    mutationFn: (params: RunWorktreeExitActionParams) =>
      isRemoteComputerId(scopedComputerId)
        ? runWorktreeExitAction(workspaceID, worktreeID, params, undefined, scopedComputerId)
        : runWorktreeExitAction(workspaceID, worktreeID, params),
    // A refused action emits no stream frame, so its stale plan is reread here too.
    onSettled: () =>
      invalidateWorktreeExitScope(queryClient, workspaceID, worktreeID, scopedComputerId),
  });
}

export function useCancelWorktreeExitAction(
  workspaceID: string,
  worktreeID: string,
  computerId?: ComputerId
) {
  const queryClient = useQueryClient();
  const scopedComputerId = useCurrentComputerId(computerId);

  return useMutation({
    mutationFn: (operationID: string) =>
      isRemoteComputerId(scopedComputerId)
        ? cancelWorktreeExitAction(
            workspaceID,
            worktreeID,
            operationID,
            undefined,
            scopedComputerId
          )
        : cancelWorktreeExitAction(workspaceID, worktreeID, operationID),
    onSuccess: () =>
      invalidateWorktreeExitScope(queryClient, workspaceID, worktreeID, scopedComputerId),
  });
}

/**
 * Explicit operator refresh. `forge` is only ever true here — the forge tier is
 * never refreshed on mount, so one gesture costs at most one provider call.
 */
export function useRefreshWorktreeStatus(
  workspaceID: string,
  worktreeID: string,
  computerId?: ComputerId
) {
  const queryClient = useQueryClient();
  const scopedComputerId = useCurrentComputerId(computerId);

  return useMutation({
    mutationFn: ({ forge = false }: { forge?: boolean } = {}) =>
      isRemoteComputerId(scopedComputerId)
        ? fetchWorktreeStatus(
            workspaceID,
            worktreeID,
            { refresh: true, forge },
            undefined,
            scopedComputerId
          )
        : fetchWorktreeStatus(workspaceID, worktreeID, { refresh: true, forge }),
    onSuccess: status => {
      queryClient.setQueryData(
        workspaceKeys.forComputer(scopedComputerId).worktreeStatus(workspaceID, worktreeID),
        status
      );
      return invalidateWorktreeExitScope(queryClient, workspaceID, worktreeID, scopedComputerId);
    },
  });
}
