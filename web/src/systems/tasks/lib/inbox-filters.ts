import type { Filter, FilterFieldsConfig } from "@agh/ui";

import type { TaskPriority, TaskStatus } from "../types";
import { INBOX_UI_LANES, type InboxLaneFilterId, type InboxUiLane } from "./inbox-grouping";
import { taskPriorityLabel, taskStatusLabel } from "./task-formatters";

export type InboxFilterFieldKey = "lane" | "status" | "priority";

export interface InboxLaneCount {
  count: number;
  unread: number;
}

export interface InboxFilterState {
  laneFilter: InboxLaneFilterId;
  statusFilter: TaskStatus | null;
  priorityFilter: TaskPriority | null;
}

export interface InboxFilterHandlers {
  onLaneChange: (next: InboxLaneFilterId) => void;
  onStatusChange: (next: TaskStatus | null) => void;
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

function formatLaneOptionLabel(label: string, counts?: InboxLaneCount): string {
  if (!counts || counts.count === 0) {
    return label;
  }
  return `${label} · ${counts.count}`;
}

/**
 * Build the `FilterFieldsConfig` consumed by `<Filters>` — three single-select
 * chip fields covering lane, status, and priority. Lane labels embed live counts
 * so the dropdown preserves the per-lane signal the deprecated tab row exposed.
 * Icons stay at the component layer (this lib is JSX-free); pass them through
 * the optional descriptor map if a caller wants to render them on the trigger.
 */
export function buildInboxFilterFields(
  laneCounts: Map<InboxUiLane, InboxLaneCount>
): FilterFieldsConfig<string> {
  return [
    {
      key: "lane",
      label: "Lane",
      type: "select",
      options: INBOX_UI_LANES.map(lane => ({
        value: lane.id,
        label: formatLaneOptionLabel(lane.label, laneCounts.get(lane.id)),
      })),
    },
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
  ];
}

function buildChip(field: InboxFilterFieldKey, value: string): Filter<string> {
  return {
    id: `inbox-filter-${field}`,
    field,
    operator: "is",
    values: [value],
  };
}

/**
 * Project the typed inbox filter state held by `useTasksPage` onto the
 * `<Filters>` chip array. Chip ids are derived from `{field}` so the same
 * logical filter keeps a stable identity across renders.
 */
export function inboxFiltersToChips(state: InboxFilterState): Filter<string>[] {
  const chips: Filter<string>[] = [];
  if (state.laneFilter !== "all") {
    chips.push(buildChip("lane", state.laneFilter));
  }
  if (state.statusFilter) {
    chips.push(buildChip("status", state.statusFilter));
  }
  if (state.priorityFilter) {
    chips.push(buildChip("priority", state.priorityFilter));
  }
  return chips;
}

/**
 * Decode the `<Filters>` chip array back into the typed setters owned by
 * `useTasksPage`. Filters that disappear from the array reset their slot to
 * the default (`"all"` for lane, `null` for status/priority) so removing a chip
 * restores the unfiltered view.
 */
export function applyInboxFilterChips(
  chips: Filter<string>[],
  handlers: InboxFilterHandlers
): void {
  const lookup = new Map<string, string | undefined>();
  for (const chip of chips) {
    lookup.set(chip.field, chip.values[0]);
  }
  handlers.onLaneChange(asLaneFilter(lookup.get("lane")));
  handlers.onStatusChange(asTaskStatus(lookup.get("status")));
  handlers.onPriorityChange(asTaskPriority(lookup.get("priority")));
}

function asLaneFilter(value: string | undefined): InboxLaneFilterId {
  if (!value) {
    return "all";
  }
  return INBOX_UI_LANES.some(lane => lane.id === value) ? (value as InboxLaneFilterId) : "all";
}

function asTaskStatus(value: string | undefined): TaskStatus | null {
  if (!value) return null;
  return (STATUS_OPTIONS as readonly string[]).includes(value) ? (value as TaskStatus) : null;
}

function asTaskPriority(value: string | undefined): TaskPriority | null {
  if (!value) return null;
  return (PRIORITY_OPTIONS as readonly string[]).includes(value) ? (value as TaskPriority) : null;
}
