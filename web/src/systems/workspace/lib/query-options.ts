import { queryOptions } from "@tanstack/react-query";

import { fetchWorkspace, fetchWorkspaces } from "../adapters/workspace-api";
import { fetchWorktrees } from "../adapters/worktree-api";
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
