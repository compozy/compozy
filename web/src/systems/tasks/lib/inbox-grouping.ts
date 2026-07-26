import type { StatusDotTone, StatusDotVariant } from "@agh/ui";

import type { TaskInboxItem, TaskInboxLane } from "../types";

export type InboxUiLane = TaskInboxLane;

export type InboxLaneFilterId = "all" | InboxUiLane;

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
 * UI-only inbox group vocabulary — five groups with a dot tone
 * each (warning solid / danger solid / warning ring / accent solid / faint
 * ring). Group membership is derived from backend item shape via
 * `resolveInboxGroupId`.
 */
export type InboxGroupId = "needs_review" | "blocked" | "updates";

export interface InboxGroupDefinition {
  id: InboxGroupId;
  label: string;
  dotTone: StatusDotTone;
  dotVariant: StatusDotVariant;
}

export const INBOX_GROUPS: InboxGroupDefinition[] = [
  { id: "needs_review", label: "Needs review", dotTone: "warning", dotVariant: "solid" },
  { id: "blocked", label: "Blocked", dotTone: "danger", dotVariant: "solid" },
  { id: "updates", label: "Updates", dotTone: "faint", dotVariant: "ring" },
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
