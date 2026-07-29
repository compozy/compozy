// Per-message size estimates for the transcript virtualizer. Replaces the flat
// `144` that was duplicated across `session-thread.tsx` and the virtualizer hook
// with one shared fallback plus a per-row-kind estimate derived from the task-26
// row model: a message's height is approximated by folding its parts into rows
// (the same collapsed shape the reader sees for a settled turn) and summing each
// kind's typical rendered height. The virtualizer still measures real DOM heights
// once a row is on screen — these estimates only seed off-screen rows, so a closer
// guess reduces scroll drift on long, heterogeneous threads without pixel accuracy.

import { deriveSessionRows, type SessionRow } from "./session-timeline.logic";
import { CHANGED_FILES_VISIBLE_CAP } from "./session-timeline-changed-files";
import { toTimelineParts } from "./timeline-message-parts";

// Single source of truth for the virtualizer's fallback row estimate.
export const VIRTUAL_MESSAGE_ESTIMATE = 144;

// A right-aligned user prompt is typically one or two lines.
const USER_MESSAGE_ESTIMATE = 64;

// Vertical rhythm around an assistant message (`pt-1` + content-aware `pb-2`/`pb-4`).
const MESSAGE_VERTICAL_PADDING = 28;

// Typical rendered height (px) per SessionRow kind under the calm-surface
// grammar: a `.trow`/summary line ≈ 26px, a `.marker` ≈ 24px, the `.tmore`
// toggle and `.working` line ≈ 22px, the turn-fold rule ≈ 34px with its border
// gap. `work` is per visible tool row.
const ROW_KIND_ESTIMATE: Record<SessionRow["kind"], number> = {
  text: 88,
  reasoning: 26,
  data: 24,
  working: 22,
  work: 26,
  "work-toggle": 22,
  "turn-fold": 34,
  "changed-files": 26,
};

const estimateCache = new WeakMap<object, number>();

function rowEstimate(row: SessionRow): number {
  if (row.kind === "work") {
    // A collapsed settled run is a single summary line; open runs pay per row.
    if (row.summary && !row.expanded) {
      return ROW_KIND_ESTIMATE.work;
    }
    const visible = row.summary ? row.entries.length : Math.max(1, row.visibleCount);
    return ROW_KIND_ESTIMATE.work * visible + (row.summary ? ROW_KIND_ESTIMATE.work : 0);
  }
  // The collapsed roll-up is one line; each revealed file adds a bare mono line.
  if (row.kind === "changed-files") {
    const visibleLineCount =
      Math.min(row.files.length, CHANGED_FILES_VISIBLE_CAP) +
      (row.files.length > CHANGED_FILES_VISIBLE_CAP ? 1 : 0);
    return ROW_KIND_ESTIMATE["changed-files"] + (row.expanded ? visibleLineCount * 22 : 0);
  }
  return ROW_KIND_ESTIMATE[row.kind];
}

function computeMessageEstimate(message: {
  role?: unknown;
  id?: string;
  content?: unknown;
}): number {
  if (message.role === "user") {
    return USER_MESSAGE_ESTIMATE;
  }
  const parts = toTimelineParts(message);
  if (parts.length === 0) {
    return VIRTUAL_MESSAGE_ESTIMATE;
  }
  // Fold as settled (no active turn) so the estimate matches the collapsed shape a
  // settled assistant turn renders; the live turn is measured on screen anyway.
  const rows = deriveSessionRows(parts, { foldSettledTurns: true });
  const total = rows.reduce((sum, row) => sum + rowEstimate(row), MESSAGE_VERTICAL_PADDING);
  return Math.max(USER_MESSAGE_ESTIMATE, total);
}

export function estimateMessageSize(message: unknown): number {
  if (message === null || typeof message !== "object") {
    return VIRTUAL_MESSAGE_ESTIMATE;
  }
  const cached = estimateCache.get(message);
  if (cached !== undefined) {
    return cached;
  }
  const size = computeMessageEstimate(
    message as { role?: unknown; id?: string; content?: unknown }
  );
  estimateCache.set(message, size);
  return size;
}
