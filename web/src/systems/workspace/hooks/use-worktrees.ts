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
import { activeWorkspaceStore } from "../stores/active-workspace-store";
import { workspaceKeys } from "../lib/query-keys";
import { worktreesAggregateOptions, worktreesListOptions } from "../lib/query-options";
import { reconcileWorktreeList, removeWorktreeFromList } from "../lib/worktree-list-reconciliation";
import type { WorktreeListingByWorkspace } from "../lib/workspace-tree";
import { describeBranchOutcome } from "../lib/worktree-branch-outcome";
import type { WorktreeBranchOutcome, WorktreePayload, WorktreesResponse } from "../types";
import { notifyUser } from "@/lib/user-feedback";
import { useWorkspaces } from "./use-workspaces";
import { createdInProfileToast, useProfileReadScope } from "@/systems/profiles";

interface UseWorktreesOptions {
  enabled?: boolean;
}

export function useWorktrees(workspaceID: string | null, options: UseWorktreesOptions = {}) {
  const catalog = useWorkspaces();
  const workspace = catalog.data?.find(item => item.id === workspaceID);
  return useQuery(
    worktreesListOptions(
      workspaceID ?? "",
      options.enabled ?? true,
      workspace
        ? { rootMissing: workspace.root_missing === true, observedAt: catalog.dataUpdatedAt }
        : undefined
    )
  );
}

/** Every project's worktrees in one store read (All projects), narrowed by the Project filter. */
export function useWorktreeAggregate(
  workspaceIDs: readonly string[] = [],
  options: UseWorktreesOptions = {}
) {
  return useQuery(worktreesAggregateOptions(workspaceIDs, options.enabled ?? true));
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
  const catalog = useWorkspaces();
  const enabled = options.enabled ?? true;
  const workspaceIds: string[] = [];
  for (const workspace of workspaces) {
    if (workspace.id.trim() !== "") workspaceIds.push(workspace.id);
  }

  return useQueries({
    queries: workspaceIds.map(id => {
      const workspace = catalog.data?.find(item => item.id === id);
      return worktreesListOptions(
        id,
        enabled,
        workspace
          ? { rootMissing: workspace.root_missing === true, observedAt: catalog.dataUpdatedAt }
          : undefined
      );
    }),
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
  // Stamped with the acting profile — `default` while the aggregate is on.
  const { aggregate, destination } = useProfileReadScope();

  return useMutation({
    mutationFn: (params: CreateWorktreeParams) => createWorktree(workspaceID, params, destination),
    // The 202 payload is the pending row; it appears in every list immediately
    // and materializes daemon-side.
    onSuccess: worktree => {
      // Only under the aggregate, and naming the owner the daemon returned
      // rather than the one we asked for — an echoed request could never
      // surface a misfile (US-012.AC-2).
      if (aggregate) {
        notifyUser({ message: createdInProfileToast(worktree.profile_name), tone: "success" });
      }
      return reconcileThenInvalidate(queryClient, workspaceID, worktree);
    },
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
export function useAdoptWorktree(workspaceID: string, profile?: string) {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (params: AdoptWorktreeParams) =>
      adoptWorktree(workspaceID, params, undefined, profile),
    onSuccess: worktree => reconcileThenInvalidate(queryClient, workspaceID, worktree),
  });
}

interface WorktreeCleanupVariables {
  worktreeID: string;
  force?: boolean;
  /** Opt-in: also delete the recorded local branch after cleanup. */
  deleteBranch?: boolean;
}

/**
 * Reports an opted-in branch deletion. An unmerged branch offers one forced
 * retry: cleanup is already done, so repeating it only re-runs the branch step.
 */
function reportBranchOutcome(
  outcome: WorktreeBranchOutcome | null,
  forced: boolean,
  forceDelete: () => Promise<WorktreeBranchOutcome | null>
): void {
  if (!outcome) return;
  const retry = !outcome.deleted && outcome.reason === "not_fully_merged" && !forced;
  notifyUser({
    message: describeBranchOutcome(outcome),
    tone: outcome.deleted ? "success" : "warning",
    action: retry
      ? {
          label: "Delete anyway",
          onClick: () => {
            void forceDelete().then(
              next => reportBranchOutcome(next, true, forceDelete),
              (error: unknown) =>
                notifyUser({
                  message: error instanceof Error ? error.message : "Branch deletion failed.",
                  tone: "error",
                })
            );
          },
        }
      : undefined,
  });
}

/** Removes one authorized checkout and clears only its matching UI scope after success. */
export function useRemoveWorktree(workspaceID: string, profile?: string) {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({ worktreeID, force, deleteBranch }: WorktreeCleanupVariables) =>
      removeWorktree(workspaceID, worktreeID, { force, deleteBranch, profile }),
    onSuccess: (outcome, variables) => {
      reportBranchOutcome(outcome, Boolean(variables.force), () =>
        removeWorktree(workspaceID, variables.worktreeID, {
          force: true,
          deleteBranch: true,
          profile,
        })
      );
      activeWorkspaceStore.trigger.worktreeRemoved({
        workspaceId: workspaceID,
        worktreeId: variables.worktreeID,
      });
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

/** Dismisses retained metadata and refreshes the catalog without deleting session/run history. */
export function useDismissWorktree(workspaceID: string, profile?: string) {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({ worktreeID, force, deleteBranch }: WorktreeCleanupVariables) =>
      dismissWorktree(workspaceID, worktreeID, { force, deleteBranch, profile }),
    onSuccess: (outcome, variables) => {
      reportBranchOutcome(outcome, Boolean(variables.force), () =>
        dismissWorktree(workspaceID, variables.worktreeID, {
          force: true,
          deleteBranch: true,
          profile,
        })
      );
      activeWorkspaceStore.trigger.worktreeRemoved({
        workspaceId: workspaceID,
        worktreeId: variables.worktreeID,
      });
      queryClient.setQueryData<WorktreesResponse>(workspaceKeys.worktrees(workspaceID), current =>
        removeWorktreeFromList(current, variables.worktreeID)
      );
      return queryClient.invalidateQueries({ queryKey: workspaceKeys.worktrees(workspaceID) });
    },
  });
}
