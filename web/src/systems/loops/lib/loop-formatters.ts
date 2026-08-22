import type { PillTone } from "@compozy/ui";

import { LOOP_RUN_STATUSES, LOOP_RUN_TERMINAL_STATUSES } from "@/generated/loop-enums";

import type { LoopRunStatus } from "../types";

export interface LoopStatusSignal {
  tone: PillTone;
  pulse: boolean;
}

const LOOP_STATUS_TONE = {
  queued: "neutral",
  running: "accent",
  watching: "info",
  "needs-approval": "info",
  paused: "neutral",
  done: "success",
  "no-op": "neutral",
  blocked: "warning",
  failed: "danger",
  exhausted: "warning",
  stalled: "neutral",
  canceled: "neutral",
} as const satisfies Record<LoopRunStatus, PillTone>;

/** Live states whose pill pulses (gated by `prefers-reduced-motion` at render). */
const LOOP_STATUS_PULSE = {
  running: true,
  watching: true,
} as const satisfies Partial<Record<LoopRunStatus, true>>;

const LOOP_STATUS_LABELS = {
  queued: "Queued",
  running: "Running",
  watching: "Watching",
  "needs-approval": "Needs Approval",
  paused: "Paused",
  done: "Done",
  "no-op": "No-op",
  blocked: "Blocked",
  failed: "Failed",
  exhausted: "Exhausted",
  stalled: "Stalled",
  canceled: "Canceled",
} as const satisfies Record<LoopRunStatus, string>;

const LOOP_STATUS_SET = new Set<string>(LOOP_RUN_STATUSES);
const LOOP_TERMINAL_STATUS_SET = new Set<LoopRunStatus>(
  LOOP_RUN_TERMINAL_STATUSES satisfies readonly LoopRunStatus[]
);

export function isLoopRunStatus(value: unknown): value is LoopRunStatus {
  // `Object.hasOwn`, not `in`: the latter matches inherited prototype keys
  // (`"toString"`, `"constructor"`), which would misclassify as valid statuses.
  return typeof value === "string" && LOOP_STATUS_SET.has(value);
}

export function isTerminalLoopStatus(status?: string | null): boolean {
  return isLoopRunStatus(status) && LOOP_TERMINAL_STATUS_SET.has(status);
}

export function loopStatusTone(status?: string | null): PillTone {
  return isLoopRunStatus(status) ? LOOP_STATUS_TONE[status] : "neutral";
}

export function loopStatusPulse(status?: string | null): boolean {
  return isLoopRunStatus(status)
    ? Boolean((LOOP_STATUS_PULSE as Record<string, true>)[status])
    : false;
}

/** Combined tone + pulse for a `StatePill` / `Pill.Dot`; unknown -> neutral, no pulse. */
export function loopStatusSignal(status?: string | null): LoopStatusSignal {
  return { tone: loopStatusTone(status), pulse: loopStatusPulse(status) };
}

export function loopStatusLabel(status?: string | null): string {
  if (isLoopRunStatus(status)) {
    return LOOP_STATUS_LABELS[status];
  }
  const trimmed = typeof status === "string" ? status.trim() : "";
  return trimmed === "" ? "Unknown" : trimmed;
}

export { LOOP_STATUS_LABELS, LOOP_STATUS_TONE };

/**
 * The terminal outcome, named for a run that has stopped.
 *
 * Same labels as the status pill, with one deliberate difference: a `no-op` run
 * is described by what happened ("Nothing to do") rather than by the wire word,
 * because the outcome line is a sentence about the run and "No-op" is a term of
 * art. Keeping both readings here stops the pill and the outcome from drifting
 * into two names for the same status.
 */
const LOOP_OUTCOME_LABEL_OVERRIDES: Partial<Record<LoopRunStatus, string>> = {
  "no-op": "Nothing to do",
};

export function loopOutcomeLabel(status: string): string {
  if (!isLoopRunStatus(status)) return "Finished";
  return LOOP_OUTCOME_LABEL_OVERRIDES[status] ?? LOOP_STATUS_LABELS[status];
}
