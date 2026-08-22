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
    // The profile-enforced by-id entries hold the same session under whichever
    // lens read it. A mutation knows the session, not the lens, so it sweeps the
    // whole family rather than guessing which one is on screen.
    queryClient.invalidateQueries({ queryKey: sessionKeys.byIdRoot(sessionId) }),
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
      queryKey: sessionKeys.inputQueue(workspaceId, sessionId),
      exact: true,
    }),
    queryClient.invalidateQueries({ queryKey: sessionKeys.byIdRoot(sessionId) }),
    invalidateWorkspaceSessionCatalog(queryClient, workspaceId),
  ]);
}
