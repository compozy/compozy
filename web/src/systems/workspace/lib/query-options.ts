import { queryOptions } from "@tanstack/react-query";

import { fetchWorkspace, fetchWorkspaces } from "../adapters/workspace-api";
import { fetchWorktrees } from "../adapters/worktree-api";
import {
  fetchWorktreeExitPlan,
  fetchWorktreeStatus,
  inspectWorktree,
} from "../adapters/worktree-exit-api";
import { workspaceKeys } from "./query-keys";

export const WORKSPACE_REFETCH_INTERVAL = 10_000;

export function workspacesListOptions(enabled = true) {
  return queryOptions({
    queryKey: workspaceKeys.list(),
    queryFn: ({ signal }) => fetchWorkspaces(signal),
    enabled,
    staleTime: 60_000,
    refetchInterval: WORKSPACE_REFETCH_INTERVAL,
  });
}

export function workspaceDetailOptions(workspaceID: string) {
  return queryOptions({
    queryKey: workspaceKeys.detail(workspaceID),
    queryFn: ({ signal }) => fetchWorkspace(workspaceID, signal),
    staleTime: 60_000,
    refetchInterval: WORKSPACE_REFETCH_INTERVAL,
  });
}

/**
 * Server-scoped per workspace — worktree lists are never client-filtered across
 * workspaces. No polling interval: the `worktree_catalog_changed` stream owns
 * freshness, and the list is refetched on invalidation only.
 */
export function worktreesListOptions(workspaceID: string, enabled = true) {
  return queryOptions({
    queryKey: workspaceKeys.worktrees(workspaceID),
    queryFn: ({ signal }) => fetchWorktrees(workspaceID, {}, signal),
    enabled: enabled && workspaceID !== "",
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
