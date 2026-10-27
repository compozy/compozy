import type { QueryClient, QueryKey } from "@tanstack/react-query";

import { sessionKeys } from "./query-keys";
import { queryKeyHasScope } from "./session-query-policy";

export function invalidateWorkspaceSessionCatalog(
  queryClient: QueryClient,
  workspaceId: string
): Promise<void> {
  return queryClient.invalidateQueries({
    queryKey: sessionKeys.workspaceLists(workspaceId),
  });
}

export function invalidateSessionMutationQueries(
  queryClient: QueryClient,
  workspaceId: string,
  sessionId: string
): Promise<void> {
  return invalidateSessionReads(queryClient, [
    sessionKeys.detail(workspaceId, sessionId),
    // The profile-enforced by-id entries hold the same session under whichever
    // lens read it. A mutation knows the session, not the lens, so it sweeps the
    // whole family rather than guessing which one is on screen.
    sessionKeys.byIdRoot(sessionId),
    sessionKeys.workspaceLists(workspaceId),
  ]);
}

export function invalidateSessionLiveQueries(
  queryClient: QueryClient,
  workspaceId: string,
  sessionId: string
): Promise<void> {
  return invalidateSessionReads(
    queryClient,
    [sessionKeys.byIdRoot(sessionId), sessionKeys.workspaceLists(workspaceId)],
    [
      sessionKeys.detail(workspaceId, sessionId),
      sessionKeys.history(workspaceId, sessionId),
      sessionKeys.inputQueue(workspaceId, sessionId),
    ]
  );
}

/** The context rail's reads: the usage reading and its per-turn compaction markers. */
export function invalidateSessionUsageQueries(
  queryClient: QueryClient,
  workspaceId: string,
  sessionId: string
): Promise<void> {
  return invalidateSessionReads(
    queryClient,
    [],
    [sessionKeys.usage(workspaceId, sessionId), sessionKeys.usageTurns(workspaceId, sessionId)]
  );
}

function invalidateSessionReads(
  queryClient: QueryClient,
  prefixes: readonly QueryKey[],
  exact: readonly QueryKey[] = []
): Promise<void> {
  // Each invalidation scans the cache twice; match this wake's keys in one pass.
  return queryClient.invalidateQueries({
    predicate: ({ queryKey }) =>
      exact.some(key => key.length === queryKey.length && queryKeyHasScope(queryKey, key)) ||
      prefixes.some(key => queryKeyHasScope(queryKey, key)),
  });
}
