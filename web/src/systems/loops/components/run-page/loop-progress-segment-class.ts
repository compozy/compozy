import type { LoopProgressSegment } from "../../lib/loop-run-fanout-band";

/**
 * The one place a progress segment becomes ink.
 *
 * Shared by the round bar and the fan-out lanes so the two never drift into
 * separate vocabularies for the same fact. The bar is a magnitude channel: a
 * parked segment keeps the informational tint rather than borrowing warning,
 * because the alarm belongs on the step's chip, one line below.
 */
export const LOOP_PROGRESS_SEGMENT_CLASS: Record<LoopProgressSegment, string> = {
  clean: "bg-success",
  active: "bg-accent-dim",
  parked: "bg-info-tint ring-1 ring-inset ring-info",
  failed: "bg-danger",
  canceled: "bg-badge-fill",
  // A branch a filter skipped: it left the denominator and reads as absence.
  never: "bg-canvas-tint ring-1 ring-inset ring-line",
  pending: "bg-badge-fill",
};
