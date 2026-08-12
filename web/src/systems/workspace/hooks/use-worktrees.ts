import {
  useMutation,
  useQueries,
  useQuery,
  useQueryClient,
  type QueryClient,
} from "@tanstack/react-query";

import {
  adoptWorktree,
  cancelWorktreeCreate,
  createWorktree,
  dismissWorktree,
  removeWorktree,
  type AdoptWorktreeParams,
  type CreateWorktreeParams,
} from "../adapters/worktree-api";
import { workspaceKeys } from "../lib/query-keys";
import { worktreesListOptions } from "../lib/query-options";
import { reconcileWorktreeList, removeWorktreeFromList } from "../lib/worktree-list-reconciliation";
import type { WorktreeListingByWorkspace } from "../lib/workspace-tree";
import type { WorktreePayload, WorktreesResponse } from "../types";

interface UseWorktreesOptions {
  enabled?: boolean;
}

export function useWorktrees(workspaceID: string | null, options: UseWorktreesOptions = {}) {
  return useQuery(worktreesListOptions(workspaceID ?? "", options.enabled ?? true));
}

/**
 * Server-scoped listings for every authorized workspace, keyed by workspace id.
 * Each entry is its own query under the canonical `workspaceKeys.worktrees(id)`,
 * so a catalog frame invalidates exactly one workspace and nothing is ever
 * filtered across workspaces client-side.
 *
 * The nav surfaces render all workspaces, so loading only the active one would
 * silently drop every other workspace's worktrees from the tree.
 */
export function useWorktreeListings(
  workspaces: ReadonlyArray<{ id: string }>,
  options: UseWorktreesOptions = {}
): WorktreeListingByWorkspace {
  const enabled = options.enabled ?? true;
  const workspaceIds: string[] = [];
  for (const workspace of workspaces) {
    if (workspace.id.trim() !== "") workspaceIds.push(workspace.id);
  }

  return useQueries({
    queries: workspaceIds.map(id => worktreesListOptions(id, enabled)),
    combine: results => {
      const listings: Record<string, WorktreesResponse | undefined> = {};
      workspaceIds.forEach((id, index) => {
        listings[id] = results[index]?.data;
      });
      return listings;
    },
  });
}

/** Reconcile the known row first, then reread — the owner's invalidation contract. */
function reconcileThenInvalidate(
  queryClient: QueryClient,
  workspaceID: string,
  worktree: WorktreePayload
): Promise<void> {
  queryClient.setQueryData<WorktreesResponse>(workspaceKeys.worktrees(workspaceID), current =>
    reconcileWorktreeList(current, worktree)
  );
  return queryClient.invalidateQueries({ queryKey: workspaceKeys.worktrees(workspaceID) });
}

export function useCreateWorktree(workspaceID: string) {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (params: CreateWorktreeParams) => createWorktree(workspaceID, params),
    // The 202 payload is the pending row; it appears in every list immediately
    // and materializes daemon-side.
    onSuccess: worktree => reconcileThenInvalidate(queryClient, workspaceID, worktree),
  });
}

export function useCancelWorktreeCreate(workspaceID: string) {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (worktreeID: string) => cancelWorktreeCreate(workspaceID, worktreeID),
    onSuccess: (_data, worktreeID) => {
      queryClient.setQueryData<WorktreesResponse>(workspaceKeys.worktrees(workspaceID), current =>
        removeWorktreeFromList(current, worktreeID)
      );
      return queryClient.invalidateQueries({ queryKey: workspaceKeys.worktrees(workspaceID) });
    },
  });
}

/**
 * Also the "It's back" path: adopting a registered `missing` path revalidates
 * Git identity and restores that record to `ready`.
 */
export function useAdoptWorktree(workspaceID: string) {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (params: AdoptWorktreeParams) => adoptWorktree(workspaceID, params),
    onSuccess: worktree => reconcileThenInvalidate(queryClient, workspaceID, worktree),
  });
}

export function useRemoveWorktree(workspaceID: string) {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({ worktreeID, force }: { worktreeID: string; force?: boolean }) =>
      removeWorktree(workspaceID, worktreeID, { force }),
    onSuccess: (_data, variables) => {
      queryClient.setQueryData<WorktreesResponse>(workspaceKeys.worktrees(workspaceID), current =>
        removeWorktreeFromList(current, variables.worktreeID)
      );
      queryClient.removeQueries({
        queryKey: workspaceKeys.worktreeDetail(workspaceID, variables.worktreeID),
      });
      return queryClient.invalidateQueries({ queryKey: workspaceKeys.worktrees(workspaceID) });
    },
  });
}

export function useDismissWorktree(workspaceID: string) {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (worktreeID: string) => dismissWorktree(workspaceID, worktreeID),
    onSuccess: (_data, worktreeID) => {
      queryClient.setQueryData<WorktreesResponse>(workspaceKeys.worktrees(workspaceID), current =>
        removeWorktreeFromList(current, worktreeID)
      );
      return queryClient.invalidateQueries({ queryKey: workspaceKeys.worktrees(workspaceID) });
    },
  });
}
