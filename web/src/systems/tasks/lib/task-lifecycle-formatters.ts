import type { PillTone } from "@agh/ui";

import type { TaskListItem, TaskRecord, TaskRunStatus, TaskStatus } from "../types";

export function countTasksByStatus(tasks: TaskListItem[]): Record<TaskStatus, number> {
  const counts: Record<TaskStatus, number> = {
    draft: 0,
    pending: 0,
    blocked: 0,
    needs_attention: 0,
    ready: 0,
    in_progress: 0,
    completed: 0,
    failed: 0,
    canceled: 0,
  };

  for (const task of tasks) {
    counts[task.status] = (counts[task.status] ?? 0) + 1;
  }

  return counts;
}

export type TaskLifecyclePhase =
  | "saved_intent"
  | "awaiting_approval"
  | "ready_to_start"
  | "queued"
  | "running"
  | "needs_attention"
  | "completed"
  | "failed"
  | "canceled"
  | "blocked";

type TaskLifecycleInput = Pick<TaskListItem, "status" | "approval_state" | "draft"> & {
  active_run?: TaskListItem["active_run"] | null;
};

const ACTIVE_RUN_STATUSES = new Set<TaskRunStatus>(["running", "starting", "claimed"]);

/**
 * Resolves the manual-first lifecycle phase for a task.
 *
 * The lifecycle is a UI-only narrative built from the canonical task status,
 * approval state, and the task list `active_run` summary so creation
 * (saved_intent) reads as separate from publish/start/approval handoff
 * (queued/running). Channel availability is a separate concern — see
 * `runIsCoordinated` and `runCoordinationChannelLabel`.
 */
export function taskLifecyclePhase(task: TaskLifecycleInput): TaskLifecyclePhase {
  if (task.status === "completed") {
    return "completed";
  }

  if (task.status === "failed") {
    return "failed";
  }

  if (task.status === "canceled") {
    return "canceled";
  }

  if (task.draft === true || task.status === "draft") {
    return "saved_intent";
  }

  if (task.approval_state === "pending") {
    return "awaiting_approval";
  }

  const activeRun = task.active_run;
  if (activeRun) {
    if (activeRun.status === "needs_attention") return "needs_attention";
    if (activeRun.status && ACTIVE_RUN_STATUSES.has(activeRun.status)) {
      return "running";
    }

    if (activeRun.status === "queued") {
      return "queued";
    }
  }
  if (task.status === "blocked") {
    return "blocked";
  }

  if (task.status === "needs_attention") {
    return "needs_attention";
  }

  if (task.status === "in_progress") {
    return "running";
  }

  return "ready_to_start";
}

const TASK_LIFECYCLE_PHASE_LABELS: Record<TaskLifecyclePhase, string> = {
  saved_intent: "Saved intent",
  awaiting_approval: "Awaiting approval",
  ready_to_start: "Ready to start",
  queued: "Coordinator handoff",
  running: "Running",
  needs_attention: "Needs attention",
  completed: "Completed",
  failed: "Failed",
  canceled: "Canceled",
  blocked: "Blocked",
};

export function taskLifecyclePhaseLabel(phase: TaskLifecyclePhase): string {
  return TASK_LIFECYCLE_PHASE_LABELS[phase];
}

const TASK_LIFECYCLE_PHASE_DESCRIPTIONS: Record<TaskLifecyclePhase, string> = {
  saved_intent:
    "Task is saved intent. Publish or start to enqueue an executable run for the coordinator.",
  awaiting_approval:
    "Approval gates execution. Approving enqueues an executable run for the coordinator.",
  ready_to_start:
    "Task is ready. Start enqueues a coordinator-handoff run; manual workers may also claim it.",
  queued: "Coordinator handoff is in flight. A worker session will claim this queued run.",
  running:
    "A worker session is executing the active run. Channel messages support coordination only.",
  needs_attention:
    "Task is escalated for operator recovery. Recover it before any new run can be enqueued or claimed.",
  completed: "The latest run completed. Task ownership and terminal status are durable.",
  failed: "The latest run failed. Retry, cancel, or follow up — channel chatter never owns status.",
  canceled: "The task or its run was canceled.",
  blocked: "Blocked by a dependency or policy. Resolve the blocker before the run can be enqueued.",
};

export function taskLifecyclePhaseDescription(phase: TaskLifecyclePhase): string {
  return TASK_LIFECYCLE_PHASE_DESCRIPTIONS[phase];
}

const TASK_LIFECYCLE_PHASE_TONES: Record<TaskLifecyclePhase, PillTone> = {
  saved_intent: "neutral",
  awaiting_approval: "info",
  ready_to_start: "neutral",
  queued: "neutral",
  running: "accent",
  needs_attention: "warning",
  completed: "neutral",
  failed: "danger",
  canceled: "danger",
  blocked: "danger",
};

export function taskLifecyclePhaseTone(phase: TaskLifecyclePhase): PillTone {
  return TASK_LIFECYCLE_PHASE_TONES[phase];
}

export interface TaskHandoffActionLabel {
  label: string;
  tooltip: string;
}

const TASK_HANDOFF_ACTION_COPY = {
  publish: {
    label: "Publish",
    tooltip:
      "Publish marks the saved intent as ready and enqueues an executable run for coordinator handoff.",
  },
  approve: {
    label: "Approve",
    tooltip:
      "Approve enqueues an executable run for coordinator handoff. Rejecting blocks execution instead.",
  },
  reject: {
    label: "Reject",
    tooltip: "Reject the task. No run is enqueued and the task moves to blocked.",
  },
  start: {
    label: "Start run",
    tooltip:
      "Start enqueues an executable run for coordinator handoff. Manual workers may also claim it.",
  },
  cancel: {
    label: "Cancel",
    tooltip: "Cancel the task. Active runs are released; coordinator stops orchestrating it.",
  },
  retry: {
    label: "Retry",
    tooltip: "Re-enqueue this task as a coordinator-handoff run.",
  },
  edit: {
    label: "Edit",
    tooltip: "Open the editor. Editing keeps the task in saved intent until you publish or start.",
  },
} satisfies Record<string, TaskHandoffActionLabel>;

export function taskHandoffActionCopy(
  action: keyof typeof TASK_HANDOFF_ACTION_COPY
): TaskHandoffActionLabel {
  return TASK_HANDOFF_ACTION_COPY[action];
}

type CoordinationCarrier = {
  resolved_network_participation?: {
    mode?: string | null;
    channel_id?: string | null;
    source?: string | null;
  } | null;
  coordination_channel?: {
    id?: string | null;
    display_name?: string | null;
    purpose?: string | null;
  } | null;
};

/**
 * Returns true when the run participates live or carries a coordination conversation
 * binding. Channel presence supports operator/agent conversation; it never replaces
 * task-run ownership or terminal status.
 */
export function runIsCoordinated<T extends CoordinationCarrier | null | undefined>(
  run: T
): boolean {
  if (!run) {
    return false;
  }

  const participation = run.resolved_network_participation;
  if (participation?.mode === "live") {
    return true;
  }
  if (typeof participation?.channel_id === "string" && participation.channel_id.trim() !== "") {
    return true;
  }

  return Boolean(run.coordination_channel?.id);
}

/**
 * Resolves a short human label for a coordination / live participation channel.
 * Prefers the embedded conversation display name, then resolved channel id.
 */
export function runCoordinationChannelLabel<T extends CoordinationCarrier | null | undefined>(
  run: T
): string {
  if (!run) {
    return "";
  }

  const display = run.coordination_channel?.display_name?.trim();
  if (display) {
    return display;
  }

  const embeddedId = run.coordination_channel?.id?.trim();
  if (embeddedId) {
    return embeddedId;
  }

  const channelId = run.resolved_network_participation?.channel_id?.trim();
  if (channelId) {
    return channelId;
  }

  return "Coordination channel";
}

/**
 * Compatibility helpers for callers that have only the `TaskRecord` (without an
 * `active_run`). These let the detail header keep a single source of truth for
 * lifecycle copy without forcing every caller to construct a full `TaskListItem`.
 */
export function taskLifecyclePhaseFromRecord(
  task: Pick<TaskRecord, "status" | "approval_state" | "draft">,
  activeRun?: TaskListItem["active_run"] | null | undefined
): TaskLifecyclePhase {
  return taskLifecyclePhase({
    status: task.status,
    approval_state: task.approval_state,
    draft: task.draft,
    active_run: activeRun ?? null,
  });
}
