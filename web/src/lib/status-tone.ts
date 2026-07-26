/**
 * Status-to-tone dictionaries for the core status-tone vocabulary.
 *
 * Three exhaustive `Record<...>` maps consumed by Tasks, Bridges, Knowledge,
 * Automation, and other system formatters. `as const satisfies Record<Key, PillTone>`
 * gives compile-time exhaustiveness against the local key types: if a new task
 * status surfaces in the UI, `make bun-typecheck` fails on the dictionary until
 * the entry is added.
 *
 * The local `TaskStatus` union deliberately scopes to the UI-renderable
 * states from the backend `Status` enum in `internal/task/types.go`. Task lanes
 * do not render canceled tasks today; downstream consumers map `canceled` to a
 * neutral fallback at the call site or route through `RUN_STATUS_TONE.canceled`.
 * `needs_attention` shares the warning tone used by run-level escalation.
 * This also defers the `stuck` UI tone (the dashboard exposes a separate
 * `stuck: bool` field, not a Status value), and `queued` is not a Status value.
 *
 * `TaskLane` is a UI vocabulary, not backend-bound (per N-004): it covers the
 * sidebar/topbar lane names, none of which exist in `internal/task/types.go`.
 */
import type { PillTone } from "@agh/ui";

export type TaskStatus =
  | "draft"
  | "pending"
  | "blocked"
  | "needs_attention"
  | "ready"
  | "in_progress"
  | "completed"
  | "failed";

export type TaskRunStatus =
  | "pending"
  | "in_progress"
  | "needs_attention"
  | "completed"
  | "failed"
  | "canceled";

/** UI lane vocabulary, not backend-bound. */
export type TaskLane =
  | "active"
  | "blocked"
  | "recent"
  | "my_work"
  | "mentions"
  | "failed_runs"
  | "updates"
  | "approvals";

export const TASK_STATUS_TONE = {
  draft: "neutral",
  pending: "neutral",
  blocked: "danger",
  needs_attention: "warning",
  ready: "neutral",
  in_progress: "info",
  completed: "success",
  failed: "danger",
} as const satisfies Record<TaskStatus, PillTone>;

export const RUN_STATUS_TONE = {
  pending: "neutral",
  in_progress: "info",
  needs_attention: "warning",
  completed: "success",
  failed: "danger",
  canceled: "neutral",
} as const satisfies Record<TaskRunStatus, PillTone>;

/** UI lane vocabulary, not backend-bound. */
export const TASK_LANE_TONE = {
  active: "neutral",
  blocked: "danger",
  recent: "neutral",
  my_work: "neutral",
  mentions: "accent",
  failed_runs: "danger",
  updates: "neutral",
  approvals: "info",
} as const satisfies Record<TaskLane, PillTone>;
