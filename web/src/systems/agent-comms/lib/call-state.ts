/**
 * The one call-state → tone / glyph / word dictionary.
 *
 * Every surface reads from here: tree rows, call detail, the Calls inspector
 * panel, transcript cards, bell rows. The wire types `state`, `verdict`, and
 * `delivery` as open `string` (the daemon owns the vocabulary in
 * `internal/calls/types.go`), so the web declares the literal unions in
 * `../types` and funnels every raw value through the `to*` narrowers below.
 * `as const satisfies Record<…>` makes each dictionary exhaustive — a new token
 * fails typecheck until it has an entry.
 *
 * Grammar is locked in `docs/design/opendesign/agent-comms/DESIGN-NOTES.md`:
 *
 * - **Colour never announces liveness; motion does.** `queued` and `running` are
 *   neutral. A tree can hold dozens of running calls, so accent there would blow
 *   the accent budget into a control-room wash. The typing dots carry aliveness,
 *   and reduced motion holds them steady.
 * - **The needs-you class shares one tone and differs by glyph.** `invalid-result`,
 *   `completed-without-result`, `failed`, and `expired` all mean "expectations
 *   unmet" and all render danger — colour is never the only channel.
 * - **`canceled` and `timeout` are warning, not danger.** Both are deliberate
 *   outcomes, not faults.
 * - **The word in the chip is the runtime's exact term** and never leaves the
 *   chip, so screen and CLI cannot disagree. "pending", "done", and "error" are
 *   banned as state names.
 */
import {
  Ban,
  Check,
  Circle,
  CircleOff,
  CircleSlash,
  Clock,
  FileX,
  Hourglass,
  Moon,
  TimerOff,
  X,
  type LucideIcon,
} from "lucide-react";

import type { PillTone } from "@compozy/ui";

import { type CallDelivery, type CallState, type CallVerdict, type ChildState } from "../types";

// The dictionary module is where consumers reach for the vocabulary, so the
// unions travel with the signals that interpret them.
export type { CallDelivery, CallState, CallVerdict, ChildState };

/**
 * Which of the three needs-you causes a state carries, if any.
 *
 * Only `invalid-result` and `completed-without-result` are needs-you *by state*.
 * `failed` and `expired` share the danger tone because expectations went unmet,
 * but they are not operator-actionable in the bell — the spec names exactly
 * three causes and the third (a child blocked on a decision) lives on the child
 * session, not on the call.
 */
export type CallAttentionClass = "needs-you" | "finished" | "none";

export interface CallStateSignal {
  tone: PillTone;
  glyph: LucideIcon;
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
    glyph: Clock,
    label: "queued",
    live: false,
    terminal: false,
    attention: "none",
  },
  running: {
    tone: "neutral",
    glyph: Circle,
    label: "running",
    live: true,
    terminal: false,
    attention: "none",
  },
  completed: {
    tone: "success",
    glyph: Check,
    label: "completed",
    live: false,
    terminal: true,
    attention: "finished",
  },
  "invalid-result": {
    tone: "danger",
    glyph: FileX,
    label: "invalid-result",
    live: false,
    terminal: true,
    attention: "needs-you",
  },
  "completed-without-result": {
    tone: "danger",
    glyph: CircleSlash,
    label: "completed-without-result",
    live: false,
    terminal: true,
    attention: "needs-you",
  },
  failed: {
    tone: "danger",
    glyph: X,
    label: "failed",
    live: false,
    terminal: true,
    attention: "none",
  },
  canceled: {
    tone: "warning",
    glyph: Ban,
    label: "canceled",
    live: false,
    terminal: true,
    attention: "none",
  },
  timeout: {
    tone: "warning",
    glyph: TimerOff,
    label: "timeout",
    live: false,
    terminal: true,
    attention: "none",
  },
  expired: {
    tone: "danger",
    glyph: Hourglass,
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

/**
 * How the answer arrived. This rides beside the success chip as a neutral mono
 * word — provenance is admission truth, so `extracted` is never dressed up as
 * `returned`.
 */
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
  glyph: LucideIcon;
  label: CallDelivery;
}

/**
 * The four public receipts. A receipt is a confirmation, so the two delivered
 * forms are success; `queued` is neutral; `failed` carries the typed reason on
 * the row. No read/seen state exists in the runtime, so none renders.
 */
export const CALL_DELIVERY_SIGNAL = {
  "delivered-into-turn": {
    tone: "success",
    glyph: Check,
    label: "delivered-into-turn",
  },
  woke: { tone: "success", glyph: Check, label: "woke" },
  queued: { tone: "neutral", glyph: Clock, label: "queued" },
  failed: { tone: "danger", glyph: X, label: "failed" },
} as const satisfies Record<CallDelivery, CallDeliverySignal>;

export function toCallDelivery(raw: string | undefined): CallDelivery | null {
  if (!raw) return null;
  return Object.hasOwn(CALL_DELIVERY_SIGNAL, raw) ? (raw as CallDelivery) : null;
}

// --- Child session state ----------------------------------------------------

export interface ChildStateSignal {
  tone: PillTone;
  glyph: LucideIcon;
  label: ChildState;
  live: boolean;
}

/**
 * Exactly three. `parked` is still addressable — contacting it is what revives
 * it — so there is no Revive control anywhere; the affordances are call-again
 * and message. A `gone` child keeps its identity and loses its affordances
 * entirely, never greyed.
 */
export const CHILD_STATE_SIGNAL = {
  running: { tone: "neutral", glyph: Circle, label: "running", live: true },
  parked: { tone: "neutral", glyph: Moon, label: "parked", live: false },
  gone: { tone: "neutral", glyph: CircleOff, label: "gone", live: false },
} as const satisfies Record<ChildState, ChildStateSignal>;

export function toChildState(raw: string | undefined): ChildState | null {
  if (!raw) return null;
  return Object.hasOwn(CHILD_STATE_SIGNAL, raw) ? (raw as ChildState) : null;
}
