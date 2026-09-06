/**
 * The live-view transport as the screen reads it (S4, US-018). Pure
 * derivations over the live-tail store's transport facts: the chip word, the
 * grace that hides brief blips, and the "disconnected" answer the composer
 * gives a send. No timers here — callers pass the clock.
 */

/** Store phases the transport can be in; `failed` is retries exhausted, `terminal` is the session ending. */
export type SessionTransportPhase =
  | "disabled"
  | "connecting"
  | "live"
  | "waiting-reconnect"
  | "failed"
  | "terminal";

export interface SessionTransportFailure {
  /** How many reconnects were tried before giving up. */
  attempts: number;
  /** When the last attempt failed. */
  at: number;
}

/** The daemon's `transcript_snapshot.reason` vocabulary; unknown strings are kept verbatim. */
export type SessionTranscriptResetReason =
  | "fence_missing"
  | "epoch_mismatch"
  | "generation_mismatch"
  | "sequence_reset"
  | "cursor_expired";

export interface SessionTransportHistoryReset {
  /** The generation the daemon reset the view to. */
  generation: number;
  /** Why the daemon reset the view; `null` when the snapshot named none. */
  reason: string | null;
  at: number;
}

/**
 * What the reset means to the reader (US-017.AC-3 / EC-1). Only `cursor_expired`
 * names its cause — retention retired the history the view had — every other
 * reason states that the history changed without inventing rewind or compaction.
 */
export function describeSessionHistoryReset(reset: SessionTransportHistoryReset): {
  lead: string;
  rest: string;
} {
  switch (reset.reason) {
    case "cursor_expired":
      return {
        lead: "Older history is no longer retained",
        rest: " — showing the history that is still saved.",
      };
    case "sequence_reset":
      return {
        lead: "The conversation history was reset while you were away",
        rest: " — showing the current history.",
      };
    default:
      return {
        lead: "The conversation history changed while you were away",
        rest: " — showing the current history.",
      };
  }
}

/** The transport facts the store owns, read by every surface that renders them. */
export interface SessionTransportSnapshot {
  phase: SessionTransportPhase;
  /** Consecutive reconnects since the last live frame. */
  reconnectAttempt: number;
  /** The reopened stream is still replaying the gap since the drop. */
  catchingUp: boolean;
  /** When the last frame applied; `null` before the first. */
  lastLiveAt: number | null;
  /** When the stream stopped being live (or started connecting); `null` while live. */
  degradedAt: number | null;
  failure: SessionTransportFailure | null;
  historyReset: SessionTransportHistoryReset | null;
}

/** A blip shorter than this never reaches the screen (US-018.EC-1). */
export const SESSION_TRANSPORT_GRACE_MS = 2_000;

export const SESSION_TRANSPORT_LIVE: SessionTransportSnapshot = {
  catchingUp: false,
  degradedAt: null,
  failure: null,
  historyReset: null,
  lastLiveAt: null,
  phase: "live",
  reconnectAttempt: 0,
};

/** What the chip says; `null` is the healthy stream — the chip slot stays empty. */
export type SessionTransportChipModel =
  | { kind: "connecting" }
  | { kind: "reconnecting"; attempt: number }
  | { kind: "catching-up" }
  | { kind: "paused"; sinceMs: number | null }
  | { kind: "disconnected"; attempts: number };

/** True once the degraded phase has outlasted the grace. */
export function transportGraceElapsed(degradedAt: number | null, nowMs: number): boolean {
  return degradedAt !== null && nowMs - degradedAt >= SESSION_TRANSPORT_GRACE_MS;
}

/**
 * Phase → chip (UT-102). A background window reads paused whatever the
 * stream does; a healthy stream reads nothing; the first connect of a fresh
 * window reads "Connecting" and anything after a drop reads "Reconnecting"
 * with the count; exhausted retries are the only danger word.
 */
export function sessionTransportChip(
  snapshot: SessionTransportSnapshot,
  options: { graceElapsed: boolean; windowLive: boolean }
): SessionTransportChipModel | null {
  if (!options.windowLive) {
    return { kind: "paused", sinceMs: snapshot.lastLiveAt };
  }
  switch (snapshot.phase) {
    case "live":
      return snapshot.catchingUp ? { kind: "catching-up" } : null;
    case "failed":
      return {
        kind: "disconnected",
        attempts: snapshot.failure?.attempts ?? snapshot.reconnectAttempt,
      };
    case "connecting":
    case "waiting-reconnect": {
      if (!options.graceElapsed) return null;
      if (snapshot.lastLiveAt === null && snapshot.reconnectAttempt === 0) {
        return { kind: "connecting" };
      }
      return { kind: "reconnecting", attempt: Math.max(1, snapshot.reconnectAttempt) };
    }
    case "disabled":
    case "terminal":
      return null;
  }
}

/**
 * Whether a send would leave into a dead stream: the transport is trying to
 * come back after a drop, or gave up. The first connect of a fresh window is
 * not a disconnect — nothing was lost yet — and a stopped session is not one
 * either (its own gate answers).
 */
export function isSessionTransportDisconnected(snapshot: SessionTransportSnapshot): boolean {
  switch (snapshot.phase) {
    case "failed":
    case "waiting-reconnect":
      return true;
    case "connecting":
      return snapshot.lastLiveAt !== null || snapshot.reconnectAttempt > 0;
    default:
      return false;
  }
}
