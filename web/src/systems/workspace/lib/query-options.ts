import { queryOptions } from "@tanstack/react-query";

import { isRemoteComputerId, LOCAL_COMPUTER_ID, type ComputerId } from "@/lib/computer-scope";
import { fetchWorkspace, fetchWorkspaces } from "../adapters/workspace-api";
import {
  fetchWorktrees,
  fetchWorktreesAggregate,
  WorktreeApiError,
} from "../adapters/worktree-api";
import {
  fetchWorktreeExitPlan,
  fetchWorktreeStatus,
  inspectWorktree,
} from "../adapters/worktree-exit-api";
import { workspaceKeys } from "./query-keys";
import { USER_SCOPE_SELECTOR } from "./destination";

export const WORKSPACE_REFETCH_INTERVAL = 10_000;

export function workspacesListOptions(
  enabled = true,
  archived = false,
  computerId: ComputerId = LOCAL_COMPUTER_ID
) {
  const keys = workspaceKeys.forComputer(computerId);
  return queryOptions({
    queryKey: archived ? keys.archived() : keys.list(),
    queryFn: ({ signal }) =>
      isRemoteComputerId(computerId)
        ? fetchWorkspaces(signal, archived, computerId)
        : fetchWorkspaces(signal, archived),
    enabled,
    staleTime: 60_000,
    refetchInterval: WORKSPACE_REFETCH_INTERVAL,
  });
}

/**
 * Computer-scoped project list used by the `Run on` picker (`["computer", id, "workspaces"]`, UT-081).
 */
export function computerWorkspacesOptions(
  computerId: ComputerId = LOCAL_COMPUTER_ID,
  enabled = true
) {
  const keys = workspaceKeys.forComputer(computerId);
  return queryOptions({
    queryKey: keys.all,
    queryFn: ({ signal }) =>
      isRemoteComputerId(computerId)
        ? fetchWorkspaces(signal, false, computerId)
        : fetchWorkspaces(signal, false),
    enabled,
    staleTime: 30_000,
    retry: false,
  });
}

export function workspaceDetailOptions(
  workspaceID: string,
  computerId: ComputerId = LOCAL_COMPUTER_ID
) {
  const keys = workspaceKeys.forComputer(computerId);
  return queryOptions({
    queryKey: keys.detail(workspaceID),
    queryFn: ({ signal }) =>
      isRemoteComputerId(computerId)
        ? fetchWorkspace(workspaceID, signal, computerId)
        : fetchWorkspace(workspaceID, signal),
    staleTime: 60_000,
    refetchInterval: WORKSPACE_REFETCH_INTERVAL,
  });
}

/**
 * Server-scoped per workspace — worktree lists are never client-filtered across
 * workspaces. No polling interval: the `worktree_catalog_changed` stream owns
 * freshness, and the list is refetched on invalidation only.
 */
export function worktreesListOptions(
  workspaceID: string,
  enabled = true,
  availability?: { rootMissing: boolean; observedAt: number },
  computerId: ComputerId = LOCAL_COMPUTER_ID
) {
  const keys = workspaceKeys.forComputer(computerId);
  return queryOptions({
    queryKey: keys.worktrees(workspaceID),
    queryFn: ({ signal }) =>
      fetchWorktrees(workspaceID, isRemoteComputerId(computerId) ? { computerId } : {}, signal),
    // User-scope work has no worktrees (USW BR-4); the daemon rejects the read.
    enabled: query =>
      enabled &&
      workspaceID !== "" &&
      workspaceID !== USER_SCOPE_SELECTOR &&
      !availability?.rootMissing &&
      (!isWorktreeListTerminalError(query.state.error) ||
        (query.state.error.status === 410 &&
          availability !== undefined &&
          availability.observedAt > query.state.errorUpdatedAt)),
    retry: (count, error) => !isWorktreeListTerminalError(error) && count < 1,
    staleTime: 60_000,
  });
}

/** A listing parked on a gone root: only a later catalog observation re-enables it. */
export function isWorktreeListGone(error: unknown): boolean {
  return error instanceof WorktreeApiError && error.status === 410;
}

function isWorktreeListTerminalError(error: unknown): error is WorktreeApiError {
  return error instanceof WorktreeApiError && [400, 401, 403, 404, 410].includes(error.status);
}

/**
 * Every project's worktrees in one store read (All projects): adopted records
 * with cached status, no discovery and no git work. Catalog frames invalidate
 * it beside the frame's own workspace.
 */
export function worktreesAggregateOptions(workspaceIDs: readonly string[] = [], enabled = true) {
  return queryOptions({
    queryKey: workspaceKeys.worktreeAggregate(workspaceIDs),
    queryFn: ({ signal }) => fetchWorktreesAggregate(workspaceIDs, signal),
    enabled,
    staleTime: 60_000,
  });
}

function worktreeScopeEnabled(workspaceID: string, worktreeID: string, enabled: boolean) {
  return enabled && workspaceID !== "" && worktreeID !== "";
}

/**
 * One worktree record plus its last status, forge cache, and bindings summary.
 * The per-worktree stream owns freshness — no polling interval here either.
 */
export function worktreeDetailOptions(workspaceID: string, worktreeID: string, enabled = true) {
  return queryOptions({
    queryKey: workspaceKeys.worktreeDetail(workspaceID, worktreeID),
    queryFn: ({ signal }) => inspectWorktree(workspaceID, worktreeID, signal),
    enabled: worktreeScopeEnabled(workspaceID, worktreeID, enabled),
    staleTime: 30_000,
  });
}

/**
 * `refresh` is opt-in per caller: the strip reads the cached row on mount and
 * only forces a git scan when the operator asks for it or an action settles.
 */
export function worktreeStatusOptions(
  workspaceID: string,
  worktreeID: string,
  options: { enabled?: boolean; refresh?: boolean } = {}
) {
  const { enabled = true, refresh } = options;
  return queryOptions({
    queryKey: workspaceKeys.worktreeStatus(workspaceID, worktreeID),
    queryFn: ({ signal }) => fetchWorktreeStatus(workspaceID, worktreeID, { refresh }, signal),
    enabled: worktreeScopeEnabled(workspaceID, worktreeID, enabled),
    staleTime: 15_000,
  });
}

/**
 * The daemon recomputes the ladder on every read (it refreshes status itself),
 * so this key is invalidated after actions and stream frames rather than polled.
 */
export function worktreeExitPlanOptions(workspaceID: string, worktreeID: string, enabled = true) {
  return queryOptions({
    queryKey: workspaceKeys.worktreeExit(workspaceID, worktreeID),
    queryFn: ({ signal }) => fetchWorktreeExitPlan(workspaceID, worktreeID, signal),
    enabled: worktreeScopeEnabled(workspaceID, worktreeID, enabled),
    staleTime: 0,
  });
}

/** Cache-only handoff from the catalog stream to an active materialization owner. */
export function worktreeMaterializationFailureOptions(workspaceID: string, worktreeID: string) {
  return queryOptions({
    queryKey: workspaceKeys.worktreeMaterializationFailure(workspaceID, worktreeID),
    queryFn: async () => "",
    enabled: false,
    staleTime: Number.POSITIVE_INFINITY,
  });
}
