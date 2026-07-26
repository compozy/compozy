import { queryOptions } from "@tanstack/react-query";

import { fetchWorkspace, fetchWorkspaces } from "../adapters/workspace-api";
import { workspaceKeys } from "./query-keys";

export const WORKSPACE_REFETCH_INTERVAL = 10_000;

export function workspacesListOptions() {
  return queryOptions({
    queryKey: workspaceKeys.list(),
    queryFn: ({ signal }) => fetchWorkspaces(signal),
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
