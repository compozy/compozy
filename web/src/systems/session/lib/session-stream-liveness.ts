import { createStore } from "@xstate/store";

import type { SessionState } from "../types";

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
 * Live-edge stream owners per session. Session ids are daemon-unique, so the workspace-
 * agnostic by-id reads share one entry with the workspace-scoped reads. A count lets
 * several surfaces hold the same session's stream without one release ending another.
 */
export const sessionStreamLivenessStore = createStore({
  context: { live: {} as Record<string, number> },
  on: {
    streamLive: (context, event: { sessionId: string }) => ({
      live: { ...context.live, [event.sessionId]: (context.live[event.sessionId] ?? 0) + 1 },
    }),
    streamReleased: (context, event: { sessionId: string }) => {
      const owners = context.live[event.sessionId] ?? 0;
      if (owners === 0) return;
      if (owners > 1) return { live: { ...context.live, [event.sessionId]: owners - 1 } };
      const { [event.sessionId]: _released, ...live } = context.live;
      return { live };
    },
  },
});

/** Holds the session's live-edge mark until the returned release runs. */
export function retainSessionStreamLiveness(sessionId: string): () => void {
  sessionStreamLivenessStore.trigger.streamLive({ sessionId });
  let released = false;
  return () => {
    if (released) return;
    released = true;
    sessionStreamLivenessStore.trigger.streamReleased({ sessionId });
  };
}

export function isSessionStreamLive(sessionId: string): boolean {
  return (sessionStreamLivenessStore.getSnapshot().context.live[sessionId] ?? 0) > 0;
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
  fallbackMs = SESSION_LIVE_REFETCH_INTERVAL_MS
): number | false {
  if (!isLiveSessionState(state)) return false;
  if (isSessionStreamLive(sessionId)) return SESSION_STREAM_SAFETY_REFETCH_INTERVAL_MS;
  return fallbackMs;
}
