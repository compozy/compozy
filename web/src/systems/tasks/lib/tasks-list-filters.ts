import type { Filter, FilterFieldsConfig } from "@compozy/ui";

import type { TaskListSortKey, TaskOwnerKind, TaskPriority, TaskStatus } from "../types";
import { taskOwnerKindLabel, taskPriorityLabel, taskStatusLabel } from "./task-formatters";

export type TaskFilterFieldKey = "status" | "owner" | "priority";

export interface TaskFilterOwnerOption {
  ref: string;
  kind: TaskOwnerKind;
}

export interface TaskFilterState {
  statusFilter: TaskStatus | null;
  ownerFilter: TaskFilterOwnerOption | null;
  priorityFilter: TaskPriority | null;
}

export interface TaskFilterHandlers {
  onStatusChange: (next: TaskStatus | null) => void;
  onOwnerChange: (next: TaskFilterOwnerOption | null) => void;
  onPriorityChange: (next: TaskPriority | null) => void;
}

export const TASK_STATUS_OPTIONS = [
  "in_progress",
  "ready",
  "blocked",
  "needs_attention",
  "pending",
  "draft",
  "completed",
  "failed",
  "canceled",
] as const satisfies readonly TaskStatus[];

export const TASK_PRIORITY_OPTIONS = [
  "urgent",
  "high",
  "medium",
  "low",
] as const satisfies readonly TaskPriority[];
export const TASK_LIST_SORT_OPTIONS = [
  "recent",
  "priority",
] as const satisfies readonly TaskListSortKey[];
const OWNER_KINDS: TaskOwnerKind[] = ["agent_session", "automation", "extension", "human", "pool"];

export function taskOwnerFilterValue(owner: TaskFilterOwnerOption): string {
  return `${encodeURIComponent(owner.kind)}:${encodeURIComponent(owner.ref)}`;
}

export function taskOwnerFilterFromValue(value: string | undefined): TaskFilterOwnerOption | null {
  if (!value) return null;
  const separator = value.indexOf(":");
  if (separator < 1) return null;
  try {
    const rawKind = decodeURIComponent(value.slice(0, separator));
    const ref = decodeURIComponent(value.slice(separator + 1));
    const kind = OWNER_KINDS.find(candidate => candidate === rawKind);
    return kind && ref ? { kind, ref } : null;
  } catch {
    return null;
  }
}

/**
 * Build the `FilterFieldsConfig` consumed by `<Filters>` — three single-select
 * chip fields covering status, owner, and priority. Owner options come
 * from the live task list (`ownerOptions` in `useTasksPage`) so the menu only
 * surfaces owners that actually exist in the active workspace.
 */
export function buildTaskFilterFields(
  ownerOptions: TaskFilterOwnerOption[]
): FilterFieldsConfig<string> {
  const refCounts = new Map<string, number>();
  for (const owner of ownerOptions) {
    refCounts.set(owner.ref, (refCounts.get(owner.ref) ?? 0) + 1);
  }
  return [
    {
      key: "status",
      label: "Status",
      type: "select",
      options: TASK_STATUS_OPTIONS.map(value => ({ value, label: taskStatusLabel(value) })),
    },
    {
      key: "priority",
      label: "Priority",
      type: "select",
      options: TASK_PRIORITY_OPTIONS.map(value => ({ value, label: taskPriorityLabel(value) })),
    },
    {
      key: "owner",
      label: "Owner",
      type: "select",
      searchable: true,
      options: ownerOptions.map(owner => ({
        value: taskOwnerFilterValue(owner),
        label:
          (refCounts.get(owner.ref) ?? 0) > 1
            ? `${owner.ref} · ${taskOwnerKindLabel(owner.kind)}`
            : owner.ref,
      })),
    },
  ];
}

/**
 * Project the typed filter state held by `useTasksPage` onto the `<Filters>`
 * chip array. Chip ids are derived from `{field, value}` so the same logical
 * filter keeps a stable identity across renders without an intermediate cache.
 */
export function taskFiltersToChips(state: TaskFilterState): Filter<string>[] {
  const chips: Filter<string>[] = [];
  if (state.statusFilter) {
    chips.push(buildChip("status", state.statusFilter));
  }
  if (state.priorityFilter) {
    chips.push(buildChip("priority", state.priorityFilter));
  }
  if (state.ownerFilter) {
    chips.push(buildChip("owner", taskOwnerFilterValue(state.ownerFilter)));
  }
  return chips;
}

function buildChip(field: TaskFilterFieldKey, value: string): Filter<string> {
  return {
    id: `task-filter-${field}`,
    field,
    operator: "is",
    values: [value],
  };
}

/**
 * Decode the `<Filters>` chip array back into the typed setters owned by
 * `useTasksPage`. Filters that disappear from the array reset their slot to
 * `null` so removing a chip restores the default.
 */
export function applyTaskFilterChips(chips: Filter<string>[], handlers: TaskFilterHandlers): void {
  const lookup = new Map<string, string | undefined>();
  for (const chip of chips) {
    lookup.set(chip.field, chip.values[0]);
  }

  handlers.onStatusChange(asTaskStatus(lookup.get("status")));
  handlers.onPriorityChange(asTaskPriority(lookup.get("priority")));
  handlers.onOwnerChange(taskOwnerFilterFromValue(lookup.get("owner")));
}

function asTaskStatus(value: string | undefined): TaskStatus | null {
  if (!value) return null;
  return (TASK_STATUS_OPTIONS as readonly string[]).includes(value) ? (value as TaskStatus) : null;
}

function asTaskPriority(value: string | undefined): TaskPriority | null {
  if (!value) return null;
  return (TASK_PRIORITY_OPTIONS as readonly string[]).includes(value)
    ? (value as TaskPriority)
    : null;
}

/** Quick status shortcuts on the Tasks strip; each writes the same single `status` filter. */
export type TaskQuickStatus = "all" | "in_progress" | "needs_attention";

export interface TaskQuickStatusItem {
  value: TaskQuickStatus;
  label: string;
  /** Facet count, only when it is exact for the pill (see {@link taskQuickStatusItems}). */
  count?: number;
}

const TASK_QUICK_STATUSES: ReadonlyArray<{ value: TaskQuickStatus; label: string }> = [
  { value: "all", label: "All" },
  { value: "in_progress", label: "In progress" },
  { value: "needs_attention", label: "Needs attention" },
];

/** The active quick pill for the current status filter; `null` when the filter names another status. */
export function taskQuickStatusFor(statusFilter: TaskStatus | null): TaskQuickStatus | null {
  if (statusFilter === null) return "all";
  if (statusFilter === "in_progress" || statusFilter === "needs_attention") return statusFilter;
  return null;
}

/**
 * Builds the quick status pills. The daemon computes status facets after the
 * status filter, so the facet counts are only exact while no status is selected;
 * with one selected, only that pill keeps its (exact) count.
 */
export function taskQuickStatusItems(
  statusFilter: TaskStatus | null,
  statusCounts: Record<TaskStatus, number>
): TaskQuickStatusItem[] {
  const total = Object.values(statusCounts).reduce((sum, count) => sum + count, 0);
  return TASK_QUICK_STATUSES.map(({ value, label }) => {
    const count = value === "all" ? total : statusCounts[value];
    const exact = statusFilter === null || statusFilter === value;
    return exact ? { value, label, count } : { value, label };
  });
}
