import type { Query, QueryClient, QueryKey } from "@tanstack/react-query";

import { PROFILE_AGGREGATE } from "@/systems/profiles";

import { isSessionUnavailable } from "../adapters/session-api-errors";
import { sessionKeys } from "./query-keys";
import { queryKeyHasScope } from "./session-query-policy";

/**
 * A profile-scoped by-id read that answered "not in this profile" for a session the aggregate
 * read found in another profile. Events about that session cannot change this lens's answer, so
 * rereading it on each one only repeats the 404 for as long as the session is active (#1184).
 * A session missing from every profile is not settled here: its deletion reconciliation needs
 * the reread.
 */
function isSettledForeignByIdRead(queryClient: QueryClient, query: Query): boolean {
  const key = query.queryKey;
  const id = key[sessionKeys.all.length + 1];
  if (typeof id !== "string") return false;
  const root = sessionKeys.byIdRoot(id);
  if (key.length !== root.length + 1 || !queryKeyHasScope(key, root)) return false;
  if (key[root.length] === PROFILE_AGGREGATE) return false;
  if (query.state.status !== "error" || !isSessionUnavailable(query.state.error)) return false;
  return queryClient.getQueryData(sessionKeys.byId(id, PROFILE_AGGREGATE)) !== undefined;
}

/** Rereads every lens of one session's by-id family, except lenses already settled as foreign. */
export function invalidateSessionByIdReads(
  queryClient: QueryClient,
  sessionId: string,
  options?: { cancelRefetch?: boolean }
): Promise<void> {
  return queryClient.invalidateQueries(
    {
      queryKey: sessionKeys.byIdRoot(sessionId),
      predicate: query => !isSettledForeignByIdRead(queryClient, query),
    },
    options
  );
}

/**
 * Every catalog family that can hold a row of this workspace: its own lists and
 * the All projects aggregate, which keys under the empty workspace segment. A
 * session-scoped reread that skipped the aggregate left those rows to the
 * throttled catalog-stream wake alone.
 */
function sessionCatalogScopes(workspaceId: string): QueryKey[] {
  const aggregate = sessionKeys.workspaceLists("");
  return workspaceId.trim() === ""
    ? [aggregate]
    : [sessionKeys.workspaceLists(workspaceId), aggregate];
}

export function invalidateWorkspaceSessionCatalog(
  queryClient: QueryClient,
  workspaceId: string
): Promise<void> {
  return Promise.all(
    sessionCatalogScopes(workspaceId).map(queryKey => queryClient.invalidateQueries({ queryKey }))
  ).then(() => undefined);
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
    ...sessionCatalogScopes(workspaceId),
  ]);
}

export function invalidateSessionLiveQueries(
  queryClient: QueryClient,
  workspaceId: string,
  sessionId: string
): Promise<void> {
  return invalidateSessionReads(
    queryClient,
    [sessionKeys.byIdRoot(sessionId), ...sessionCatalogScopes(workspaceId)],
    [
      sessionKeys.detail(workspaceId, sessionId),
      sessionKeys.history(workspaceId, sessionId),
      sessionKeys.inputQueue(workspaceId, sessionId),
    ]
  );
}

/** The session's own live reads, without the catalogs that list it. */
export function invalidateSessionOwnLiveQueries(
  queryClient: QueryClient,
  workspaceId: string,
  sessionId: string
): Promise<void> {
  return invalidateSessionReads(
    queryClient,
    [sessionKeys.byIdRoot(sessionId)],
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

export function invalidateSessionMonitorsQueries(
  queryClient: QueryClient,
  workspaceId: string,
  sessionId: string
): Promise<void> {
  return invalidateSessionReads(queryClient, [], [sessionKeys.monitors(workspaceId, sessionId)]);
}

/** The session's process list and the operator's left-running aggregate it feeds. */
export function invalidateSessionProcessesQueries(
  queryClient: QueryClient,
  workspaceId: string,
  sessionId: string
): Promise<void> {
  return invalidateSessionReads(
    queryClient,
    [sessionKeys.processes(workspaceId, sessionId), sessionKeys.byIdRoot(sessionId)],
    [sessionKeys.processesLeftRunning()]
  );
}

function invalidateSessionReads(
  queryClient: QueryClient,
  prefixes: readonly QueryKey[],
  exact: readonly QueryKey[] = []
): Promise<void> {
  // Resource/session identities differ near the tail; avoid traversing the
  // shared workspace prefix for each unrelated retained read.
  return queryClient.invalidateQueries({
    predicate: query =>
      (exact.some(key => sameSessionReadKey(query.queryKey, key)) ||
        prefixes.some(key => queryKeyHasScope(query.queryKey, key))) &&
      !isSettledForeignByIdRead(queryClient, query),
  });
}

function sameSessionReadKey(left: QueryKey, right: QueryKey): boolean {
  if (left.length !== right.length) return false;
  for (let index = left.length - 1; index >= 0; index--) {
    if (left[index] !== right[index]) return false;
  }
  return true;
}
