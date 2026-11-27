import type { QueryClient } from "@tanstack/react-query";
import { createStore } from "@xstate/store";

import {
  isRemoteComputerId,
  normalizeComputerId,
  onAccountScopeEvicted,
  type ComputerId,
} from "@/lib/computer-scope";
import type { SessionState } from "../types";
import { sessionKeys } from "./query-keys";

/** Fallback cadence for a live session whose transcript stream is not on its live edge. */
export const SESSION_LIVE_REFETCH_INTERVAL_MS = 5_000;
/** A starting session settles quickly; its detail read follows it closely. */
export const SESSION_STARTING_REFETCH_INTERVAL_MS = 500;
/**
 * Safety cadence while the session stream is live. Frames, `session_usage_changed`, and
 * queue markers invalidate these reads as they happen; the safety read only bounds a
 * change the stream does not carry (a queued entry replaced from another client).
 */
export const SESSION_STREAM_SAFETY_REFETCH_INTERVAL_MS = 30_000;

/**
 * Live-edge stream owners per computer and session. Session ids are daemon-unique, so the workspace-
 * agnostic by-id reads share one entry with the workspace-scoped reads. A count lets
 * several surfaces hold the same session's stream without one release ending another.
 */
export const sessionStreamLivenessStore = createStore({
  context: { live: {} as Record<string, number> },
  on: {
    streamLive: (context, { key }: { key: string }) => ({
      live: { ...context.live, [key]: (context.live[key] ?? 0) + 1 },
    }),
    streamReleased: (context, { key }: { key: string }) => {
      const owners = context.live[key] ?? 0;
      if (owners === 0) return;
      if (owners > 1) return { live: { ...context.live, [key]: owners - 1 } };
      const { [key]: _released, ...live } = context.live;
      return { live };
    },
    remoteStreamsCleared: context => {
      let changed = false;
      const live: Record<string, number> = {};
      for (const [key, count] of Object.entries(context.live)) {
        if (isRemoteStreamKey(key)) {
          changed = true;
          continue;
        }
        live[key] = count;
      }
      return changed ? { live } : undefined;
    },
  },
});

function isRemoteStreamKey(key: string): boolean {
  try {
    const parsed: unknown = JSON.parse(key);
    return Array.isArray(parsed) && typeof parsed[0] === "string" && isRemoteComputerId(parsed[0]);
  } catch {
    return false;
  }
}

onAccountScopeEvicted(() => {
  sessionStreamLivenessStore.trigger.remoteStreamsCleared({});
});

function streamKey(sessionId: string, computerId?: ComputerId): string {
  return JSON.stringify([normalizeComputerId(computerId), sessionId]);
}

/** Holds the session's live-edge mark until the returned release runs. */
export function retainSessionStreamLiveness(
  sessionId: string,
  computerId?: ComputerId
): () => void {
  const key = streamKey(sessionId, computerId);
  sessionStreamLivenessStore.trigger.streamLive({ key });
  let released = false;
  return () => {
    if (released) return;
    released = true;
    sessionStreamLivenessStore.trigger.streamReleased({ key });
  };
}

export function isSessionStreamLive(sessionId: string, computerId?: ComputerId): boolean {
  return (
    (sessionStreamLivenessStore.getSnapshot().context.live[streamKey(sessionId, computerId)] ?? 0) >
    0
  );
}

/** Live sessions poll; stopped sessions are terminal and never refetch. */
export function isLiveSessionState(state: SessionState | null | undefined): boolean {
  return state === "active" || state === "starting" || state === "stopping";
}

/**
 * Cadence of a stream-backed session read: none once the session stopped, the safety
 * cadence while its stream is live, and the fallback cadence while it is not.
 * TanStack re-evaluates it after every result, so a stream invalidation re-arms it.
 */
export function sessionStreamBackedRefetchInterval(
  sessionId: string,
  state: SessionState | null | undefined,
  computerId?: ComputerId
): number | false {
  if (!isLiveSessionState(state)) return false;
  if (isSessionStreamLive(sessionId, computerId)) return SESSION_STREAM_SAFETY_REFETCH_INTERVAL_MS;
  return SESSION_LIVE_REFETCH_INTERVAL_MS;
}

/**
 * Re-arms the session's mounted stream-backed reads after its stream ownership changed.
 * An observer re-evaluates its cadence only after a result, so without this a read armed
 * for the safety cadence would wait out that timer before falling back. Reapplying the
 * observer's own options recomputes the interval without fetching a fresh query.
 */
export function rescheduleSessionStreamBackedReads(
  queryClient: QueryClient,
  workspaceId: string,
  sessionId: string,
  computerId?: ComputerId
): void {
  const keys = sessionKeys.forComputer(computerId);
  const cache = queryClient.getQueryCache();
  const queries = [
    ...[
      keys.detail(workspaceId, sessionId),
      keys.inputQueue(workspaceId, sessionId),
      keys.usage(workspaceId, sessionId),
      keys.usageTurns(workspaceId, sessionId),
    ].flatMap(queryKey => {
      const query = cache.get(queryClient.defaultQueryOptions({ queryKey }).queryHash);
      return query ? [query] : [];
    }),
    ...cache.findAll({ queryKey: keys.byIdRoot(sessionId) }),
  ];
  for (const query of queries) {
    for (const observer of query.observers) observer.setOptions(observer.options);
  }
}
