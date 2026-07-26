import type { QueryClient } from "@tanstack/react-query";

import { sessionKeys } from "./query-keys";

export function invalidateWorkspaceSessionCatalog(
  queryClient: QueryClient,
  workspaceId: string
): Promise<void> {
  return queryClient.invalidateQueries({
    queryKey: sessionKeys.workspaceLists(workspaceId),
  });
}

export async function invalidateSessionMutationQueries(
  queryClient: QueryClient,
  workspaceId: string,
  sessionId: string
): Promise<void> {
  await Promise.all([
    queryClient.invalidateQueries({
      queryKey: sessionKeys.detail(workspaceId, sessionId),
    }),
    queryClient.invalidateQueries({
      queryKey: sessionKeys.byId(sessionId),
      exact: true,
    }),
    invalidateWorkspaceSessionCatalog(queryClient, workspaceId),
  ]);
}

export async function invalidateSessionLiveQueries(
  queryClient: QueryClient,
  workspaceId: string,
  sessionId: string
): Promise<void> {
  await Promise.all([
    queryClient.invalidateQueries({
      queryKey: sessionKeys.detail(workspaceId, sessionId),
      exact: true,
    }),
    queryClient.invalidateQueries({
      queryKey: sessionKeys.history(workspaceId, sessionId),
      exact: true,
    }),
    queryClient.invalidateQueries({
      queryKey: sessionKeys.byId(sessionId),
      exact: true,
    }),
    invalidateWorkspaceSessionCatalog(queryClient, workspaceId),
  ]);
}
