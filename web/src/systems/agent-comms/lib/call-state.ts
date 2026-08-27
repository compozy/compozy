/**
 * The one call-state → tone / word dictionary.
 *
 * Every surface reads from here. The wire already closes `state`, `verdict`, and
 * `delivery` on the generated contract; `to*` is the only narrowing seam for a
 * raw string that somehow missed that close. Glyphs live in `call-state-glyphs.ts`.
 *
 * Grammar is locked in `docs/design/opendesign/agent-comms/DESIGN-NOTES.md`:
 *
 * - **Colour never announces liveness; motion does.** `queued` and `running` are
 *   neutral. The typing dots carry aliveness.
 * - **The needs-you class shares one tone and differs by glyph.**
 * - **`canceled` and `timeout` are warning, not danger.**
 * - **The word in the chip is the runtime's exact term.**
 */
import type { PillTone } from "@compozy/ui";

import { type CallDelivery, type CallState, type CallVerdict, type ChildState } from "../types";

export type { CallDelivery, CallState, CallVerdict, ChildState };

/**
 * Which of the three needs-you causes a state carries, if any.
 *
 * Only `invalid-result` and `completed-without-result` are needs-you *by state*.
 * The third (a child blocked on a decision) lives on the child session.
 */
export type CallAttentionClass = "needs-you" | "finished" | "none";

export interface CallStateSignal {
  tone: PillTone;
  /** Exact CLI vocabulary — also the accessible label. */
  label: CallState;
  /** Whether the row shows typing dots. Motion, not colour, means alive. */
  live: boolean;
  /** No further transition is possible. */
  terminal: boolean;
  attention: CallAttentionClass;
}

export const CALL_STATE_SIGNAL = {
  queued: {
    tone: "neutral",
    label: "queued",
    live: false,
    terminal: false,
    attention: "none",
  },
  running: {
    tone: "neutral",
    label: "running",
    live: true,
    terminal: false,
    attention: "none",
  },
  completed: {
    tone: "success",
    label: "completed",
    live: false,
    terminal: true,
    attention: "finished",
  },
  "invalid-result": {
    tone: "danger",
    label: "invalid-result",
    live: false,
    terminal: true,
    attention: "needs-you",
  },
  "completed-without-result": {
    tone: "danger",
    label: "completed-without-result",
    live: false,
    terminal: true,
    attention: "needs-you",
  },
  failed: {
    tone: "danger",
    label: "failed",
    live: false,
    terminal: true,
    attention: "none",
  },
  canceled: {
    tone: "warning",
    label: "canceled",
    live: false,
    terminal: true,
    attention: "none",
  },
  timeout: {
    tone: "warning",
    label: "timeout",
    live: false,
    terminal: true,
    attention: "none",
  },
  expired: {
    tone: "danger",
    label: "expired",
    live: false,
    terminal: true,
    attention: "none",
  },
} as const satisfies Record<CallState, CallStateSignal>;

/**
 * Narrow a wire state, or return null.
 *
 * A state the web does not know is not guessed into a neighbour — callers render
 * the raw word with neutral tone rather than claiming a meaning the daemon did
 * not send.
 */
export function toCallState(raw: string | undefined): CallState | null {
  if (!raw) return null;
  return Object.hasOwn(CALL_STATE_SIGNAL, raw) ? (raw as CallState) : null;
}

export function callStateSignal(state: CallState): CallStateSignal {
  return CALL_STATE_SIGNAL[state];
}

/** Needs-you by state alone. The blocked-child cause is joined in elsewhere. */
export function isNeedsYouCallState(state: CallState): boolean {
  return CALL_STATE_SIGNAL[state].attention === "needs-you";
}

/** Completed work awaiting a look — the bell's Finished section, never counted. */
export function isFinishedCallState(state: CallState): boolean {
  return CALL_STATE_SIGNAL[state].attention === "finished";
}

export function isTerminalCallState(state: CallState): boolean {
  return CALL_STATE_SIGNAL[state].terminal;
}

// --- Verdict ----------------------------------------------------------------

export interface CallVerdictSignal {
  label: CallVerdict;
  /** Plain-language gloss for the timeline line, never for the chip. */
  description: string;
}

export const CALL_VERDICT_SIGNAL = {
  returned: { label: "returned", description: "answer accepted as sent" },
  extracted: {
    label: "extracted",
    description: "the answer was recovered from its last message and checked",
  },
  repaired: { label: "repaired", description: "the second answer matched" },
} as const satisfies Record<CallVerdict, CallVerdictSignal>;

export function toCallVerdict(raw: string | undefined): CallVerdict | null {
  if (!raw) return null;
  return Object.hasOwn(CALL_VERDICT_SIGNAL, raw) ? (raw as CallVerdict) : null;
}

// --- Delivery ---------------------------------------------------------------

export interface CallDeliverySignal {
  tone: PillTone;
  label: CallDelivery;
}

export const CALL_DELIVERY_SIGNAL = {
  "delivered-into-turn": { tone: "success", label: "delivered-into-turn" },
  woke: { tone: "success", label: "woke" },
  queued: { tone: "neutral", label: "queued" },
  failed: { tone: "danger", label: "failed" },
} as const satisfies Record<CallDelivery, CallDeliverySignal>;

export function toCallDelivery(raw: string | undefined): CallDelivery | null {
  if (!raw) return null;
  return Object.hasOwn(CALL_DELIVERY_SIGNAL, raw) ? (raw as CallDelivery) : null;
}

// --- Child session state ----------------------------------------------------

export interface ChildStateSignal {
  tone: PillTone;
  label: ChildState;
  live: boolean;
}

export const CHILD_STATE_SIGNAL = {
  running: { tone: "neutral", label: "running", live: true },
  parked: { tone: "neutral", label: "parked", live: false },
  gone: { tone: "neutral", label: "gone", live: false },
} as const satisfies Record<ChildState, ChildStateSignal>;

export function toChildState(raw: string | undefined): ChildState | null {
  if (!raw) return null;
  return Object.hasOwn(CHILD_STATE_SIGNAL, raw) ? (raw as ChildState) : null;
}
