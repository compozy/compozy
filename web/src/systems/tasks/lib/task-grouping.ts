import type { StatusDotProps, StatusDotTone, StatusDotVariant } from "@agh/ui";

import type { TaskListItem, TaskStatus } from "../types";

/**
 * Kanban column set: `Pending · In progress · Blocked · Needs attention · Done`.
 * `needs_attention` is its own column, distinct from `blocked` (no coercion) so
 * the unblock-loop breaker's escalation is visible and recoverable on the board.
 *
 * Terminal statuses (`completed`, `failed`, `canceled`) collapse into the
 * `done` column — the proposal's kanban surface treats every terminal state
 * as "off the board" and routes failure detail to the row card itself.
 */
export type TaskKanbanColumnId = "pending" | "in_progress" | "blocked" | "needs_attention" | "done";

export interface TaskKanbanColumn {
  id: TaskKanbanColumnId;
  label: string;
  statuses: TaskStatus[];
}

const KANBAN_COLUMNS: TaskKanbanColumn[] = [
  { id: "pending", label: "Pending", statuses: ["draft", "pending", "ready"] },
  { id: "in_progress", label: "In progress", statuses: ["in_progress"] },
  { id: "blocked", label: "Blocked", statuses: ["blocked"] },
  { id: "needs_attention", label: "Needs attention", statuses: ["needs_attention"] },
  { id: "done", label: "Done", statuses: ["completed", "failed", "canceled"] },
];

/**
 * List-view group buckets in order:
 * Active · Blocked · Needs attention · Queued · Done · Failed.
 *
 * `needs_attention` is its own bucket next to `blocked` so an escalated task is
 * visible and recoverable from the list (not folded into `blocked`).
 *
 */
export type TaskListGroupId =
  | "active"
  | "blocked"
  | "needs_attention"
  | "queued"
  | "done"
  | "failed";

export interface TaskListGroupDefinition {
  id: TaskListGroupId;
  label: string;
  statuses: TaskStatus[];
  dotTone: StatusDotTone;
  dotVariant: StatusDotVariant;
}

const LIST_GROUPS: TaskListGroupDefinition[] = [
  {
    id: "active",
    label: "Active",
    statuses: ["in_progress"],
    dotTone: "accent",
    dotVariant: "ring",
  },
  {
    // `blocked` reads `danger` and `needs_attention` reads `warning`, matching
    // TASK_STATUS_TONE and taskStatusSignal so the two escalation buckets stay
    // distinct at the group header too (no coercion).
    id: "blocked",
    label: "Blocked",
    statuses: ["blocked"],
    dotTone: "danger",
    dotVariant: "solid",
  },
  {
    id: "needs_attention",
    label: "Needs attention",
    statuses: ["needs_attention"],
    dotTone: "warning",
    dotVariant: "solid",
  },
  {
    id: "queued",
    label: "Queued",
    statuses: ["ready", "pending", "draft"],
    dotTone: "faint",
    dotVariant: "ring",
  },
  { id: "done", label: "Done", statuses: ["completed"], dotTone: "faint", dotVariant: "solid" },
  {
    id: "failed",
    label: "Failed",
    statuses: ["failed", "canceled"],
    dotTone: "danger",
    dotVariant: "solid",
  },
];

/** Convenience accessor for `<StatusDot>` props derived from a list group id. */
export function listGroupDotProps(
  groupId: TaskListGroupId
): Pick<StatusDotProps, "tone" | "variant"> {
  const definition = LIST_GROUPS.find(entry => entry.id === groupId);
  return {
    tone: definition?.dotTone ?? "faint",
    variant: definition?.dotVariant ?? "ring",
  };
}

export interface TaskListGroupBucket {
  group: TaskListGroupDefinition;
  tasks: TaskListItem[];
}

export function getTaskListGroups(): TaskListGroupDefinition[] {
  return LIST_GROUPS;
}

export function resolveTaskListGroupId(status: TaskStatus | string): TaskListGroupId | null {
  for (const group of LIST_GROUPS) {
    if ((group.statuses as readonly string[]).includes(status)) {
      return group.id;
    }
  }
  return null;
}

/** Partition the list into ordered group buckets; callers decide how to render empty buckets. */
export function groupTasksForList(tasks: TaskListItem[]): TaskListGroupBucket[] {
  const buckets = new Map<TaskListGroupId, TaskListItem[]>();
  for (const group of LIST_GROUPS) {
    buckets.set(group.id, []);
  }

  for (const task of tasks) {
    const groupId = resolveTaskListGroupId(task.status);
    if (!groupId) {
      continue;
    }
    buckets.get(groupId)?.push(task);
  }

  return LIST_GROUPS.map(group => ({
    group,
    tasks: buckets.get(group.id) ?? [],
  }));
}

export interface KanbanColumnGroup {
  column: TaskKanbanColumn;
  tasks: TaskListItem[];
}

export function getKanbanColumns(): TaskKanbanColumn[] {
  return KANBAN_COLUMNS;
}

export function taskStatusFacetTotal(
  statuses: readonly TaskStatus[],
  counts: Record<TaskStatus, number>
): number {
  return statuses.reduce((total, status) => total + counts[status], 0);
}

export function groupTasksForKanban(tasks: TaskListItem[]): KanbanColumnGroup[] {
  const buckets = new Map<TaskKanbanColumnId, TaskListItem[]>();
  for (const column of KANBAN_COLUMNS) {
    buckets.set(column.id, []);
  }

  for (const task of tasks) {
    const columnId = resolveKanbanColumnId(task.status);
    if (!columnId) {
      continue;
    }

    buckets.get(columnId)?.push(task);
  }

  return KANBAN_COLUMNS.map(column => ({
    column,
    tasks: buckets.get(column.id) ?? [],
  }));
}

export function resolveKanbanColumnId(status: TaskStatus | string): TaskKanbanColumnId | null {
  for (const column of KANBAN_COLUMNS) {
    if ((column.statuses as readonly string[]).includes(status)) {
      return column.id;
    }
  }

  return null;
}
