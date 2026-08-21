import type { PillTone } from "@compozy/ui";
import type { LucideIcon } from "lucide-react";
import {
  Ban,
  Check,
  CircleAlert,
  CircleDashed,
  GitBranch,
  Hourglass,
  Pause,
  RotateCcw,
  TriangleAlert,
} from "lucide-react";

/**
 * The roster's closed projection vocabulary and the copy every chip renders.
 *
 * The daemon types these as bare strings on the wire, so the closed set is
 * authored here from the roster contract (`_spec.md` "Roster projection
 * contract": 14 persisted output states plus derived `not_taken`). Nothing else
 * in the web may spell a roster state — a raw enum reaching a component is the
 * leak UT-044 forbids.
 *
 * Colour never travels alone: every entry pairs a tone with a glyph and a
 * literal state word, which is the accessibility floor E2E-019 asserts.
 */
export const LOOP_ROSTER_STATES = [
  "pending",
  "queued",
  "running",
  "retrying",
  "waiting",
  "paused",
  "awaiting_child",
  "control_pending",
  "awaiting_goal",
  "succeeded",
  "partial",
  "failed",
  "canceled",
  "quarantined",
  "not_taken",
] as const;

export type LoopRosterState = (typeof LOOP_ROSTER_STATES)[number];

/**
 * The `state` values the roster route accepts as a FILTER — wire vocabulary, so
 * it is declared once in the adapter layer and re-exported here for the UI.
 */
export {
  LOOP_ROSTER_STATE_FILTERS,
  type LoopRosterStateFilter,
} from "../adapters/loop-roster-filters";

/**
 * How a state occupies its chip.
 *
 * `solid` is the ordinary tinted chip. `hollow` is reachable-but-unstarted —
 * an outline that reads as "nothing has happened here yet". `absent` is durable
 * route evidence: the run provably went elsewhere. Pending and not-taken must
 * stay distinguishable at a glance (Safety Invariant 14), and neither uses
 * opacity, which would drop the node name under the contrast floor.
 */
export type LoopStateChipForm = "solid" | "hollow" | "absent";

export interface LoopStateChip {
  state: LoopRosterState;
  /** The literal state word. Never a raw enum — no underscores reach the DOM. */
  label: string;
  tone: PillTone;
  icon: LucideIcon | null;
  form: LoopStateChipForm;
  /** The single live accent; the renderer unmounts it under reduced motion. */
  pulse: boolean;
}

const LOOP_ROSTER_STATE_LABELS = {
  pending: "pending",
  queued: "queued",
  running: "running",
  retrying: "retrying",
  waiting: "waiting",
  paused: "paused",
  awaiting_child: "awaiting child",
  // A gate holding for a human answer. The literal word is "pending"; warning
  // tone plus the alert glyph is what separates it from a calm roster `pending`.
  control_pending: "pending",
  awaiting_goal: "awaiting goal",
  succeeded: "succeeded",
  partial: "partial",
  failed: "failed",
  canceled: "canceled",
  quarantined: "quarantined",
  not_taken: "not taken",
} as const satisfies Record<LoopRosterState, string>;

const LOOP_ROSTER_STATE_TONES = {
  // Absence is calm: nothing reachable-but-unstarted, canceled, or never taken
  // is allowed an alarm colour.
  pending: "neutral",
  queued: "neutral",
  canceled: "neutral",
  not_taken: "neutral",
  // The single live accent on the page.
  running: "accent",
  // Parked on something — warning family, each carrying its own word.
  retrying: "warning",
  waiting: "warning",
  paused: "warning",
  awaiting_child: "warning",
  control_pending: "warning",
  awaiting_goal: "warning",
  partial: "warning",
  succeeded: "success",
  // Danger is reserved for these two on the page; the bell owns the rest.
  failed: "danger",
  quarantined: "danger",
} as const satisfies Record<LoopRosterState, PillTone>;

const LOOP_ROSTER_STATE_ICONS = {
  pending: CircleDashed,
  queued: CircleDashed,
  // The live state carries a pulsing dot instead of a glyph, so the accent is
  // the only motion on the row. Its literal word still carries the meaning.
  running: null,
  retrying: RotateCcw,
  waiting: Hourglass,
  paused: Pause,
  awaiting_child: Hourglass,
  control_pending: TriangleAlert,
  awaiting_goal: Hourglass,
  succeeded: Check,
  partial: TriangleAlert,
  failed: CircleAlert,
  canceled: Ban,
  quarantined: CircleAlert,
  not_taken: GitBranch,
} as const satisfies Record<LoopRosterState, LucideIcon | null>;

const LOOP_ROSTER_STATE_FORMS = {
  pending: "hollow",
  queued: "hollow",
  not_taken: "absent",
} as const satisfies Partial<Record<LoopRosterState, LoopStateChipForm>>;

const LOOP_ROSTER_STATE_SET = new Set<string>(LOOP_ROSTER_STATES);

export function isLoopRosterState(value: unknown): value is LoopRosterState {
  return typeof value === "string" && LOOP_ROSTER_STATE_SET.has(value);
}

/**
 * The one place a roster state becomes something renderable. An unrecognised
 * state degrades to a neutral `unknown` chip rather than printing the raw wire
 * value — truthful about not knowing, without leaking mechanics.
 */
export function loopRosterStateChip(state: string): LoopStateChip {
  if (!isLoopRosterState(state)) {
    return {
      state: "pending",
      label: "unknown",
      tone: "neutral",
      icon: CircleDashed,
      form: "hollow",
      pulse: false,
    };
  }
  return {
    state,
    label: LOOP_ROSTER_STATE_LABELS[state],
    tone: LOOP_ROSTER_STATE_TONES[state],
    icon: LOOP_ROSTER_STATE_ICONS[state],
    form:
      (LOOP_ROSTER_STATE_FORMS as Partial<Record<LoopRosterState, LoopStateChipForm>>)[state] ??
      "solid",
    pulse: state === "running",
  };
}

/** States that have settled for this round — the numerator's population. */
const LOOP_SETTLED_STATES = new Set<LoopRosterState>([
  "succeeded",
  "partial",
  "failed",
  "canceled",
  "quarantined",
]);

export function isSettledRosterState(state: string): boolean {
  return isLoopRosterState(state) && LOOP_SETTLED_STATES.has(state);
}

/**
 * Still owed work this round: neither settled nor durable route evidence.
 *
 * `not_taken` is deliberately excluded. A branch the run provably declined is
 * not unfinished business — it is a decision, and counting it as outstanding
 * would keep a completed round reading as in flight.
 */
export function isUnsettledRosterState(state: string): boolean {
  return isLoopRosterState(state) && !LOOP_SETTLED_STATES.has(state) && state !== "not_taken";
}

/** Settled, but not cleanly: the states a round has to answer for. */
const LOOP_FAILED_STATES = new Set<LoopRosterState>([
  "failed",
  "canceled",
  "quarantined",
  "partial",
]);

export function isFailedRosterState(state: string): boolean {
  return isLoopRosterState(state) && LOOP_FAILED_STATES.has(state);
}

/**
 * States in which a node provably never ran.
 *
 * There is no session, no execution record and no timing behind one of these, so
 * nothing downstream is safe to link and no verb is safe to offer. Both the node
 * panel and the verb bridge read this, and they must never disagree about
 * whether a cell exists.
 */
const LOOP_NEVER_MATERIALIZED_STATES = new Set<LoopRosterState>(["not_taken", "pending"]);

export function isNeverMaterializedRosterState(state: string): boolean {
  return isLoopRosterState(state) && LOOP_NEVER_MATERIALIZED_STATES.has(state);
}

/** Parked on something a human or another node owes it. */
export const LOOP_PARKED_STATES = [
  "retrying",
  "waiting",
  "paused",
  "awaiting_child",
  "control_pending",
  "awaiting_goal",
] as const satisfies readonly LoopRosterState[];

export type LoopParkedState = (typeof LOOP_PARKED_STATES)[number];

const LOOP_PARKED_STATE_SET = new Set<string>(LOOP_PARKED_STATES);

export function isParkedRosterState(state: string): state is LoopParkedState {
  return LOOP_PARKED_STATE_SET.has(state);
}

/**
 * Why a round is parked, in plain words. The progress label states the dominant
 * reason rather than freezing a percentage that has stopped meaning anything.
 *
 * Keyed by the parked union, so a new parked state cannot reach the page without
 * its explanation — the compiler asks for the sentence before the state ships.
 */
const LOOP_PARK_REASONS: Record<LoopParkedState, string> = {
  retrying: "waiting to retry",
  waiting: "waiting on something",
  paused: "paused",
  awaiting_child: "waiting on a child run",
  control_pending: "waiting for your decision",
  awaiting_goal: "waiting on its goal",
};

export function loopParkReason(state: string): string | null {
  return isParkedRosterState(state) ? LOOP_PARK_REASONS[state] : null;
}

export { LOOP_ROSTER_STATE_LABELS, LOOP_ROSTER_STATE_TONES };
