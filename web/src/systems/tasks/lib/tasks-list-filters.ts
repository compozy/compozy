import type { Filter, FilterFieldsConfig } from "@agh/ui";

import type { TaskOwnerKind, TaskPriority, TaskStatus } from "../types";
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

const STATUS_OPTIONS: TaskStatus[] = [
  "in_progress",
  "ready",
  "blocked",
  "needs_attention",
  "pending",
  "draft",
  "completed",
  "failed",
  "canceled",
];

const PRIORITY_OPTIONS: TaskPriority[] = ["urgent", "high", "medium", "low"];
const OWNER_KINDS: TaskOwnerKind[] = [
  "agent_session",
  "automation",
  "extension",
  "human",
  "network_peer",
  "pool",
];

export function taskOwnerFilterValue(owner: TaskFilterOwnerOption): string {
  return `${encodeURIComponent(owner.kind)}:${encodeURIComponent(owner.ref)}`;
}

function taskOwnerFilterFromValue(value: string | undefined): TaskFilterOwnerOption | null {
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
      options: STATUS_OPTIONS.map(value => ({ value, label: taskStatusLabel(value) })),
    },
    {
      key: "priority",
      label: "Priority",
      type: "select",
      options: PRIORITY_OPTIONS.map(value => ({ value, label: taskPriorityLabel(value) })),
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
  return (STATUS_OPTIONS as readonly string[]).includes(value) ? (value as TaskStatus) : null;
}

function asTaskPriority(value: string | undefined): TaskPriority | null {
  if (!value) return null;
  return (PRIORITY_OPTIONS as readonly string[]).includes(value) ? (value as TaskPriority) : null;
}
