import type { StateGlyphState } from "@compozy/ui";

import type { TaskInboxItem, TaskInboxLane } from "../types";

export type InboxUiLane = TaskInboxLane;

export type InboxLaneFilterId = "all" | InboxUiLane;

export const TASK_INBOX_LANE_FILTER_IDS = [
  "all",
  "my_work",
  "approvals",
  "failed_runs",
  "blocked",
  "archived",
] as const satisfies readonly InboxLaneFilterId[];

export interface InboxLaneDefinition {
  id: InboxUiLane;
  label: string;
}

export const INBOX_UI_LANES: InboxLaneDefinition[] = [
  { id: "my_work", label: "My work" },
  { id: "approvals", label: "Approvals" },
  { id: "failed_runs", label: "Failed runs" },
  { id: "blocked", label: "Blocked" },
  { id: "archived", label: "Archived" },
];

/**
 * UI-only inbox group vocabulary, each with its canonical `StateGlyph` state:
 * the two groups that wait on a person carry attention, the feed stays idle.
 * Group membership is derived from backend item shape via `resolveInboxGroupId`.
 */
export type InboxGroupId = "needs_review" | "blocked" | "updates";

export interface InboxGroupDefinition {
  id: InboxGroupId;
  label: string;
  glyph: StateGlyphState;
}

export const INBOX_GROUPS: InboxGroupDefinition[] = [
  { id: "needs_review", label: "Needs review", glyph: "attention" },
  { id: "blocked", label: "Blocked", glyph: "attention" },
  { id: "updates", label: "Updates", glyph: "idle" },
];

/**
 * Routes an inbox item into the three groups supported by backend signals.
 *
 * - Approval pending + `approvals` lane → `needs_review`.
 * - `blocked` lane OR task status `blocked` → `blocked`.
 * - Backend `archived` items + read state → `updates` (low-priority feed).
 * - `failed_runs` lane → `needs_review` (failures require operator action).
 * - Anything else with `my_work` lane → `updates` (informational).
 *
 */
export function resolveInboxGroupId(item: TaskInboxItem): InboxGroupId {
  if (item.lane === "approvals") {
    return "needs_review";
  }
  if (item.lane === "blocked" || item.task.status === "blocked") {
    return "blocked";
  }
  if (item.lane === "failed_runs") {
    return "needs_review";
  }
  return "updates";
}

export function resolveInboxLaneGroupId(lane: TaskInboxLane): InboxGroupId {
  if (lane === "approvals" || lane === "failed_runs") return "needs_review";
  if (lane === "blocked") return "blocked";
  return "updates";
}

export function backendLaneToUiLane(lane: TaskInboxLane): InboxUiLane {
  return lane;
}

/**
 * Plain words for the daemon's closed inbox `blocking_reason` codes. An
 * unrecognised code yields `null` so a raw wire token never reaches the row.
 */
const INBOX_BLOCKING_REASON_LABELS: Record<string, string> = {
  awaiting_approval: "Waiting for your approval",
  approval_rejected: "Approval rejected",
  awaiting_dependencies: "Waiting on other tasks",
  latest_run_failed: "Latest run failed",
};

export function inboxBlockingReasonLabel(code?: string | null): string | null {
  if (!code) return null;
  return INBOX_BLOCKING_REASON_LABELS[code] ?? null;
}
