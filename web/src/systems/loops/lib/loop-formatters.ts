import type { StateGlyphState } from "@compozy/ui";

import { LOOP_RUN_STATUSES, LOOP_RUN_TERMINAL_STATUSES } from "@/generated/loop-enums";

import type { LoopRun, LoopRunStatus } from "../types";

/**
 * Run status → canonical `StateGlyph` state: live runs (running, watching)
 * spin, a run waiting on a person reads as attention, finished runs are done,
 * failures (including exhausted and stalled runs) carry the failed mark, paused
 * and canceled runs read as stopped, and a run with nothing to do stays idle.
 */
const LOOP_STATUS_GLYPH = {
  queued: "queued",
  running: "running",
  watching: "running",
  "needs-approval": "attention",
  paused: "stopped",
  done: "done",
  "no-op": "idle",
  blocked: "attention",
  failed: "failed",
  exhausted: "failed",
  stalled: "failed",
  canceled: "stopped",
} as const satisfies Record<LoopRunStatus, StateGlyphState>;

const LOOP_STATUS_LABELS = {
  queued: "Queued",
  running: "Running",
  watching: "Watching",
  "needs-approval": "Needs approval",
  paused: "Paused",
  done: "Done",
  "no-op": "Nothing to do",
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

/** A daemon-owned run that can still change through runtime execution. */
export function isLiveLoopRun(run?: Pick<LoopRun, "historical" | "status"> | null): boolean {
  return Boolean(
    run && !run.historical && isLoopRunStatus(run.status) && !isTerminalLoopStatus(run.status)
  );
}

/** The run status glyph; unknown statuses stay idle. */
export function loopStatusGlyph(status?: string | null): StateGlyphState {
  return isLoopRunStatus(status) ? LOOP_STATUS_GLYPH[status] : "idle";
}

export function loopStatusLabel(status?: string | null): string {
  if (isLoopRunStatus(status)) {
    return LOOP_STATUS_LABELS[status];
  }
  const trimmed = typeof status === "string" ? status.trim() : "";
  return trimmed === "" ? "Unknown" : trimmed;
}

export { LOOP_STATUS_GLYPH, LOOP_STATUS_LABELS };

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
