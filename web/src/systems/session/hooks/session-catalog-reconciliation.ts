import type { QueryClient } from "@tanstack/react-query";

import { sessionKeys } from "../lib/query-keys";
import { queryKeyHasScope } from "../lib/session-query-policy";
import { isLiveSessionState } from "../lib/session-stream-liveness";
import type { SessionCatalogEventPayload, SessionPayload } from "../types";

/**
 * Re-reads the session detail, then wakes a stopped session's mounted transcript.
 *
 * A catalog upsert is the only signal for a transcript entry appended after the
 * session stopped (the post-stop discard marker): the transcript stream has already
 * ended on the terminal frame and a stopped session never polls. The decision is
 * taken from the authoritative detail read, never from the cached state — the wake
 * can arrive while the cache still says active/stopping although the daemon has
 * already stopped — so the transcript is re-read only after the detail invalidation
 * settles as non-live. Live sessions keep the live tail as the transcript's sole
 * owner, and an unknown state is left to the live tail's own detail read.
 */
async function refreshSessionDetail(
  queryClient: QueryClient,
  payload: SessionCatalogEventPayload,
  isOpen: () => boolean
): Promise<void> {
  const detailKey = sessionKeys.detail(payload.workspace_id, payload.session_id);
  const before = queryClient.getQueryData<SessionPayload>(detailKey);
  await queryClient.invalidateQueries(
    {
      predicate: ({ queryKey }) =>
        queryKey.length === detailKey.length && queryKeyHasScope(queryKey, detailKey),
    },
    { cancelRefetch: false }
  );
  if (!isOpen() || payload.kind !== "upserted") return;
  // Catalog changes can move the same session to another checkout, including
  // workspace-root changes with no worktree id. File results belong to that CWD.
  const after = queryClient.getQueryData<SessionPayload>(detailKey);
  if (
    before?.worktree_id !== after?.worktree_id ||
    before?.workspace_path !== after?.workspace_path
  ) {
    await queryClient.invalidateQueries({
      queryKey: sessionKeys.fileSearchRoot(payload.workspace_id, payload.session_id),
    });
  }
  const state = after?.state;
  if (state === undefined || isLiveSessionState(state)) return;
  const transcriptKey = sessionKeys.transcript(payload.workspace_id, payload.session_id);
  await queryClient.invalidateQueries({
    predicate: ({ queryKey }) =>
      queryKey.length === transcriptKey.length && queryKeyHasScope(queryKey, transcriptKey),
  });
}

/** Let a session read finish, then reconcile any wake that arrived during it. */
export function createSessionReadReconciler(queryClient: QueryClient) {
  const pending = new Map<string, SessionCatalogEventPayload | undefined>();
  const running = new Set<string>();
  let closed = false;
  const flush = async (sessionId: string) => {
    try {
      while (!closed && pending.has(sessionId)) {
        const payload = pending.get(sessionId);
        pending.delete(sessionId);
        const byIdKey = sessionKeys.byIdRoot(sessionId);
        const detailKey = payload ? sessionKeys.detail(payload.workspace_id, sessionId) : undefined;
        // A read already in progress can predate this wake. Joining it is safe
        // only if one trailing read observes the change after that read settles.
        const cache = queryClient.getQueryCache();
        const reads = cache
          .findAll({
            predicate: ({ queryKey }) =>
              queryKeyHasScope(queryKey, byIdKey) ||
              (detailKey !== undefined &&
                queryKey.length === detailKey.length &&
                queryKeyHasScope(queryKey, detailKey)),
          })
          .flatMap(query =>
            query.state.fetchStatus === "fetching" && query.promise ? [query.promise] : []
          );
        if (reads.length > 0) {
          pending.set(sessionId, payload);
          await Promise.allSettled(reads);
          continue;
        }
        await Promise.all([
          ...(payload ? [refreshSessionDetail(queryClient, payload, () => !closed)] : []),
          queryClient.invalidateQueries({ queryKey: byIdKey }, { cancelRefetch: false }),
        ]);
      }
    } finally {
      running.delete(sessionId);
    }
  };
  return {
    wake(sessionId: string, payload?: SessionCatalogEventPayload) {
      if (closed) return;
      // Upserts carry a transcript obligation that later layout/attention wakes cannot erase.
      const queued = pending.get(sessionId);
      if (queued?.kind !== "upserted" && (payload || !pending.has(sessionId))) {
        pending.set(sessionId, payload);
      }
      if (running.has(sessionId)) return;
      running.add(sessionId);
      void flush(sessionId);
    },
    close() {
      closed = true;
      pending.clear();
    },
  };
}
