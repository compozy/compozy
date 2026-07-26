import { formatDuration, type OwnerAvatarProps, type PillTone, type RunCardStatus } from "@agh/ui";

import {
  RUN_STATUS_TONE,
  TASK_LANE_TONE,
  TASK_STATUS_TONE,
  type TaskLane,
  type TaskStatus as UiTaskStatus,
} from "@/lib/status-tone";
import type {
  TaskApprovalState,
  TaskBlockedReason,
  TaskBlockedReasonSource,
  TaskBlockKind,
  TaskInboxLane,
  TaskListItem,
  TaskOwnerKind,
  TaskPriority,
  TaskRecord,
  TaskRunStatus,
  TaskStatus,
} from "../types";

export interface TaskStatusSignal {
  tone: PillTone;
  pulse?: boolean;
}

/**
 * Maps a task status (production vocabulary OR the `docs/design/web-inspiration/`
 * shorthand of `done | running | pending | blocked | failed`) to the DESIGN.md §4
 * `StatusDot` tone and pulse. Used by `tasks-list-row`, detail header, kanban cards,
 * inbox rows and table cells so that the visual signal stays consistent.
 *
 * Color is signal, never decoration: terminal / normal states render neutral,
 * only attention-demanding states carry a semantic tone. The `accent` + `pulse`
 * combination is reserved for genuinely running work.
 */
export function taskStatusSignal(status?: TaskStatus | string | null): TaskStatusSignal {
  switch (status) {
    case "in_progress":
    case "running":
      return { tone: "accent", pulse: true };
    case "blocked":
      // Matches TASK_STATUS_TONE so the dot and the status pill agree, and stays
      // distinct from the needs_attention escalation dot (warning) — no coercion.
      return { tone: "danger" };
    case "needs_attention":
      return { tone: "warning" };
    case "failed":
    case "canceled":
      return { tone: "danger" };
    case "completed":
    case "done":
    case "ready":
    case "pending":
    case "draft":
    default:
      return { tone: "neutral" };
  }
}

/** Convenience: short 7-char identifier for `MonoBadge` id chips in list rows. */
export function taskShortId(task: { id: string; identifier?: string | null }): string {
  if (task.identifier) return task.identifier;
  return task.id.length > 7 ? task.id.slice(0, 7) : task.id;
}

const TASK_STATUS_LABELS: Record<TaskStatus, string> = {
  draft: "Draft",
  pending: "Not started",
  blocked: "Blocked",
  needs_attention: "Needs attention",
  ready: "Ready",
  in_progress: "In progress",
  completed: "Completed",
  failed: "Failed",
  canceled: "Canceled",
};

const TASK_PRIORITY_LABELS: Record<TaskPriority, string> = {
  low: "Low",
  medium: "Medium",
  high: "High",
  urgent: "Urgent",
};

const TASK_INBOX_LANE_LABELS: Record<TaskInboxLane, string> = {
  my_work: "My work",
  approvals: "Approvals",
  failed_runs: "Failed runs",
  blocked: "Blocked",
  archived: "Archived",
};

const TASK_APPROVAL_STATE_LABELS: Record<TaskApprovalState, string> = {
  not_required: "Not required",
  pending: "Approval pending",
  approved: "Approved",
  rejected: "Rejected",
};

export function taskStatusLabel(status?: TaskStatus | null): string {
  if (!status) {
    return "Unknown";
  }

  return TASK_STATUS_LABELS[status] ?? status;
}

export function taskPriorityLabel(priority?: TaskPriority | null): string {
  if (!priority) {
    return "Unset";
  }

  return TASK_PRIORITY_LABELS[priority] ?? priority;
}

export function taskInboxLaneLabel(lane: TaskInboxLane): string {
  return TASK_INBOX_LANE_LABELS[lane] ?? lane;
}

export function taskApprovalStateLabel(state?: TaskApprovalState | null): string {
  if (!state) {
    return "Not required";
  }

  return TASK_APPROVAL_STATE_LABELS[state] ?? state;
}

/**
 * Maps a task status to its `PillTone` via the central `TASK_STATUS_TONE`
 * dictionary. The dictionary keys cover the seven UI-renderable
 * statuses; `canceled` and unknown values fall back to `neutral` per
 * `web/src/lib/status-tone.ts` documentation.
 */
export function taskStatusTone(status?: TaskStatus | null): PillTone {
  if (!status) return "neutral";
  if (status in TASK_STATUS_TONE) {
    return TASK_STATUS_TONE[status as UiTaskStatus];
  }
  return "neutral";
}

export function taskPriorityTone(_priority?: TaskPriority | null): PillTone {
  // Priority never colorizes — hierarchy is expressed via weight and position,
  // not hue. Stacking signal tones per row is decoration, not signal.
  return "neutral";
}

const TASK_BLOCKED_SOURCE_LABELS: Record<TaskBlockedReasonSource, string> = {
  dependency: "Dependency",
  approval: "Approval",
  paused: "Paused",
  block: "Block",
};

const TASK_BLOCK_KIND_LABELS: Record<TaskBlockKind, string> = {
  needs_input: "Needs input",
  capability: "Capability",
  transient: "Transient",
};

/**
 * Blocking-reason tones map each source to its signal token. Machine-resolvable
 * waits (`dependency`) read neutral; the informational `approval` wait reads
 * `info`; operator/runtime-declared holds (`paused`, `block`) read `warning`.
 * Tone is signal, never decoration — no source stacks a danger tone.
 */
const TASK_BLOCKED_SOURCE_TONE: Record<TaskBlockedReasonSource, PillTone> = {
  dependency: "neutral",
  approval: "info",
  paused: "warning",
  block: "warning",
};

export function taskBlockedSourceLabel(source: TaskBlockedReasonSource): string {
  return TASK_BLOCKED_SOURCE_LABELS[source] ?? source;
}

export function taskBlockKindLabel(kind?: TaskBlockKind | null): string | undefined {
  if (!kind) {
    return undefined;
  }
  return TASK_BLOCK_KIND_LABELS[kind] ?? kind;
}

export interface BlockedReasonChip {
  key: string;
  source: TaskBlockedReasonSource;
  sourceLabel: string;
  tone: PillTone;
  kind?: TaskBlockKind;
  kindLabel?: string;
  reason?: string;
  dependsOnTaskIds?: string[];
}

/**
 * Projects the read-only `blocked_reasons` array into one chip descriptor per
 * entry, preserving order and carrying only payload data (truthful UI). Returns
 * an empty array when the task carries no blocking causes, so the caller renders
 * nothing rather than an empty container.
 */
export function projectBlockedReasonChips(
  reasons?: TaskBlockedReason[] | null
): BlockedReasonChip[] {
  if (!reasons || reasons.length === 0) {
    return [];
  }
  return reasons.map((reason, index) => {
    const trimmedReason = reason.reason?.trim();
    const dependsOnTaskIds = reason.depends_on_task_ids?.filter(id => id.trim() !== "") ?? [];
    return {
      key: reason.block_id?.trim() || `${reason.source}-${index}`,
      source: reason.source,
      sourceLabel: taskBlockedSourceLabel(reason.source),
      tone: TASK_BLOCKED_SOURCE_TONE[reason.source] ?? "neutral",
      kind: reason.kind ?? undefined,
      kindLabel: taskBlockKindLabel(reason.kind),
      reason: trimmedReason || undefined,
      dependsOnTaskIds: dependsOnTaskIds.length > 0 ? dependsOnTaskIds : undefined,
    };
  });
}

/**
 * Maps a backend run status to its `PillTone` via `RUN_STATUS_TONE`. The wire enum carries eight
 * values (queued / claimed / starting / running / needs_attention / completed / failed / canceled);
 * `toRunCardStatus` collapses them to the frontend run-card enum, which is the key shape of
 * `RUN_STATUS_TONE`.
 */
export function taskRunStatusTone(status?: TaskRunStatus | null): PillTone {
  if (!status) return "neutral";
  return RUN_STATUS_TONE[toRunCardStatus(status)];
}

const TASK_RUN_STATUS_LABELS: Record<TaskRunStatus, string> = {
  queued: "Queued",
  claimed: "Assigned",
  starting: "Starting",
  running: "Running",
  completed: "Completed",
  failed: "Failed",
  canceled: "Canceled",
  needs_attention: "Needs attention",
};

export function taskRunStatusLabel(status?: TaskRunStatus | null): string {
  if (!status) {
    return "Unknown";
  }
  return TASK_RUN_STATUS_LABELS[status];
}

/**
 * Maps the wire `TaskRunStatus` enum (queued/claimed/starting/running/needs_attention/completed/
 * failed/canceled) to the `<RunCard>` `RunCardStatus` enum. `needs_attention` keeps its own lane so
 * the run-status chip carries the warning signal tone.
 */
export function toRunCardStatus(status: TaskRunStatus): RunCardStatus {
  switch (status) {
    case "queued":
      return "pending";
    case "claimed":
    case "starting":
    case "running":
      return "in_progress";
    case "needs_attention":
      return "needs_attention";
    case "completed":
      return "completed";
    case "failed":
      return "failed";
    case "canceled":
      return "canceled";
    default:
      return "pending";
  }
}

interface ElapsedRunInput {
  started_at?: string | null;
  ended_at?: string | null;
}

/**
 * Computes a pre-formatted elapsed string for a run via `started_at - ended_at`.
 * For live runs without `ended_at`, falls back to `Date.now()`. Returns
 * `undefined` when the input lacks a parseable `started_at`.
 */
export function computeElapsed(run: ElapsedRunInput): string | undefined {
  if (!run.started_at) return undefined;
  const startedAt = Date.parse(run.started_at);
  if (Number.isNaN(startedAt)) return undefined;
  const endedAt = run.ended_at ? Date.parse(run.ended_at) : Date.now();
  if (Number.isNaN(endedAt)) return undefined;
  const delta = Math.max(0, endedAt - startedAt);
  return formatDuration(delta);
}

const TASK_INBOX_LANE_TONE_KEY: Record<TaskInboxLane, TaskLane | null> = {
  my_work: "my_work",
  approvals: "approvals",
  failed_runs: "failed_runs",
  blocked: "blocked",
  // `archived` is a backend-only lane with no UI tone counterpart per N-004.
  archived: null,
};

/**
 * Maps a backend inbox lane to its `PillTone` via the central `TASK_LANE_TONE`
 * dictionary (DESIGN.md §5). `approvals` resolves to `info`;
 * `archived` has no UI lane counterpart and collapses to `neutral`.
 */
export function taskLaneTone(lane: TaskInboxLane): PillTone {
  const key = TASK_INBOX_LANE_TONE_KEY[lane];
  return key ? TASK_LANE_TONE[key] : "neutral";
}

export function taskHasApprovalPending(task: Pick<TaskListItem, "approval_state">): boolean {
  return task.approval_state === "pending";
}

export function taskIsDraft(task: Pick<TaskListItem, "draft" | "status">): boolean {
  return task.draft === true || task.status === "draft";
}

export function taskIsBlocked(task: Pick<TaskListItem, "status">): boolean {
  return task.status === "blocked";
}

/**
 * A task is recoverable only when its derived status is `needs_attention`. Gating
 * on the derived status (not the raw `needs_attention_at` boolean) respects the
 * precedence `terminal > needs_attention` (ADR-003): a task force-completed or
 * canceled while `needs_attention_at` is still set reads as terminal, so the
 * Recover control never appears on a task the runtime would reject (truthful UI).
 */
export function taskCanRecover(task: Pick<TaskRecord, "status">): boolean {
  return task.status === "needs_attention";
}

/**
 * The wake indicator is meaningful only for tasks created by an agent session —
 * `wake_creator` never fires for human/automation/daemon creators (ADR-004), so
 * the indicator is suppressed rather than rendering a control with no effect.
 */
export function taskWakeIndicatorApplies(task: Pick<TaskRecord, "created_by">): boolean {
  return task.created_by?.kind === "agent_session";
}

export function matchesTaskQuery(
  task: Pick<TaskListItem, "title" | "identifier">,
  query: string
): boolean {
  const normalized = query.trim().toLowerCase();
  if (normalized === "") {
    return true;
  }

  const title = task.title?.toLowerCase() ?? "";
  const identifier = task.identifier?.toLowerCase() ?? "";

  return title.includes(normalized) || identifier.includes(normalized);
}

/**
 * Maps the backend owner kind onto the `<OwnerAvatar>` palette tier
 * in DESIGN.md §3.5. Agent sessions, automation runs, extensions, network peers,
 * and worker pools all read as `agent` for color selection; humans get the
 * `human` slot ladder; unassigned tasks fall back to the system palette.
 */
export function ownerAvatarKindFor(
  kind?: TaskOwnerKind | (string & {}) | null
): OwnerAvatarProps["ownerKind"] {
  switch (kind) {
    case "human":
      return "human";
    case "agent_session":
    case "automation":
    case "extension":
    case "network_peer":
    case "pool":
      return "agent";
    default:
      return "system";
  }
}

const TASK_OWNER_KIND_LABELS: Record<TaskOwnerKind, string> = {
  human: "Human",
  agent_session: "Agent",
  automation: "Automation",
  extension: "Extension",
  network_peer: "Peer",
  pool: "Pool",
};

export function taskOwnerKindLabel(kind?: TaskOwnerKind | null): string {
  if (!kind) {
    return "Unassigned";
  }

  return TASK_OWNER_KIND_LABELS[kind] ?? kind;
}

export function taskOwnerLabel(
  owner?: Pick<NonNullable<TaskListItem["owner"]>, "kind" | "ref"> | null
): string {
  if (!owner) {
    return "Unassigned";
  }

  return owner.ref || taskOwnerKindLabel(owner.kind);
}

export {
  formatAttemptLabel,
  formatDurationMs,
  formatPercent,
  formatRelativeTime,
} from "./task-time-formatters";

export {
  countTasksByStatus,
  runCoordinationChannelLabel,
  runIsCoordinated,
  taskHandoffActionCopy,
  taskLifecyclePhase,
  taskLifecyclePhaseDescription,
  taskLifecyclePhaseFromRecord,
  taskLifecyclePhaseLabel,
  taskLifecyclePhaseTone,
  type TaskHandoffActionLabel,
  type TaskLifecyclePhase,
} from "./task-lifecycle-formatters";

/** Head-status text tone map (`.w2-status`: bare tone-colored text, never a filled pill). */
export const HEAD_STATUS_TONE_TEXT: Record<PillTone, string> = {
  neutral: "text-muted",
  accent: "text-accent",
  success: "text-success",
  warning: "text-warning",
  danger: "text-danger",
  info: "text-info",
};
