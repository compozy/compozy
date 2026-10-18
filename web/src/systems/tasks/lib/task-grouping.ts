import type { StateGlyphState } from "@compozy/ui";

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
  /** Canonical `StateGlyph` state for the column header. */
  glyph: StateGlyphState;
}

const KANBAN_COLUMNS: TaskKanbanColumn[] = [
  { id: "pending", label: "Pending", statuses: ["draft", "pending", "ready"], glyph: "queued" },
  { id: "in_progress", label: "In progress", statuses: ["in_progress"], glyph: "running" },
  { id: "blocked", label: "Blocked", statuses: ["blocked"], glyph: "attention" },
  {
    id: "needs_attention",
    label: "Needs attention",
    statuses: ["needs_attention"],
    glyph: "attention",
  },
  { id: "done", label: "Done", statuses: ["completed", "failed", "canceled"], glyph: "done" },
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
  /** Canonical `StateGlyph` state for the group header. */
  glyph: StateGlyphState;
}

const LIST_GROUPS: TaskListGroupDefinition[] = [
  {
    id: "active",
    label: "Active",
    statuses: ["in_progress"],
    glyph: "running",
  },
  {
    // Both escalation buckets wait on a person, so both carry the attention
    // glyph (taskStateGlyph); the labels keep them distinct (no coercion).
    id: "blocked",
    label: "Blocked",
    statuses: ["blocked"],
    glyph: "attention",
  },
  {
    id: "needs_attention",
    label: "Needs attention",
    statuses: ["needs_attention"],
    glyph: "attention",
  },
  {
    id: "queued",
    label: "Queued",
    statuses: ["ready", "pending", "draft"],
    glyph: "queued",
  },
  {
    // Canceled work is finished on purpose, so it reads as Done (like the
    // kanban Done column) rather than as a failure; its row still says Canceled.
    id: "done",
    label: "Done",
    statuses: ["completed", "canceled"],
    glyph: "done",
  },
  {
    id: "failed",
    label: "Failed",
    statuses: ["failed"],
    glyph: "failed",
  },
];

/** Group-header `StateGlyph` state for a list group id. */
export function listGroupGlyph(groupId: TaskListGroupId): StateGlyphState {
  return LIST_GROUPS.find(entry => entry.id === groupId)?.glyph ?? "idle";
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
