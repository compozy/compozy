import {
  projectFilterChip,
  projectFilterValuesFromChips,
  type ProjectFilterModel,
} from "@/systems/workspace";

import {
  type InboxLaneCount,
  applyInboxFilterChips,
  buildInboxFilterFields,
  inboxFiltersToChips,
} from "../lib/inbox-filters";
import {
  INBOX_GROUPS,
  INBOX_UI_LANES,
  type InboxGroupId,
  type InboxLaneFilterId,
  type InboxUiLane,
  backendLaneToUiLane,
  resolveInboxGroupId,
  resolveInboxLaneGroupId,
} from "../lib/inbox-grouping";
import type {
  TaskInboxGroup,
  TaskInboxItem,
  TaskInboxView,
  TaskPriority,
  TaskStatus,
} from "../types";

type LaneCount = InboxLaneCount;

interface UseTasksInboxViewArgs {
  inbox: TaskInboxView | null;
  laneFilter: InboxLaneFilterId;
  onLaneChange: (lane: InboxLaneFilterId) => void;
  statusFilter: TaskStatus | null;
  onStatusChange: (next: TaskStatus | null) => void;
  priorityFilter: TaskPriority | null;
  onPriorityChange: (next: TaskPriority | null) => void;
  /** The shared Project filter; its field is present only under All projects. */
  projectFilter?: Pick<ProjectFilterModel, "field" | "values" | "setValues">;
}

function flattenItems(inbox: TaskInboxView | null): TaskInboxItem[] {
  if (!inbox?.groups) {
    return [];
  }
  return inbox.groups.flatMap(group => group.items ?? []);
}

function computeLaneCounts(groups: TaskInboxGroup[] | undefined): Map<InboxUiLane, LaneCount> {
  const counts = new Map<InboxUiLane, LaneCount>();
  for (const lane of INBOX_UI_LANES) {
    counts.set(lane.id, { count: 0, unread: 0 });
  }
  for (const group of groups ?? []) {
    const entry = counts.get(backendLaneToUiLane(group.lane));
    if (!entry) continue;
    entry.count = group.count;
    entry.unread = group.unread_count;
  }
  return counts;
}

function partitionByGroup(items: TaskInboxItem[]): Map<InboxGroupId, TaskInboxItem[]> {
  const buckets = new Map<InboxGroupId, TaskInboxItem[]>();
  for (const group of INBOX_GROUPS) {
    buckets.set(group.id, []);
  }
  for (const item of items) {
    const groupId = resolveInboxGroupId(item);
    buckets.get(groupId)?.push(item);
  }
  return buckets;
}

function countByDisplayGroup(groups: TaskInboxGroup[] | undefined): Record<InboxGroupId, number> {
  const counts: Record<InboxGroupId, number> = {
    needs_review: 0,
    blocked: 0,
    updates: 0,
  };
  for (const group of groups ?? []) {
    const groupId = resolveInboxLaneGroupId(group.lane);
    counts[groupId] += group.count;
  }
  return counts;
}

export function useTasksInboxView({
  inbox,
  laneFilter,
  onLaneChange,
  statusFilter,
  onStatusChange,
  priorityFilter,
  onPriorityChange,
  projectFilter,
}: UseTasksInboxViewArgs) {
  const allItems = flattenItems(inbox);
  const laneCounts = computeLaneCounts(inbox?.groups);
  const projectField = projectFilter?.field ?? null;
  // Project leads: it decides which projects the other filters search.
  const filterFields = [
    ...(projectField ? [projectField] : []),
    ...buildInboxFilterFields(laneCounts),
  ];
  const projectChip = projectField ? projectFilterChip(projectFilter?.values ?? []) : null;
  const filterChips = [
    ...(projectChip ? [projectChip] : []),
    ...inboxFiltersToChips({ laneFilter, statusFilter, priorityFilter }),
  ];
  const handleFiltersChange = (chips: Parameters<typeof applyInboxFilterChips>[0]) => {
    applyInboxFilterChips(chips, { onLaneChange, onStatusChange, onPriorityChange });
    if (projectField) projectFilter?.setValues(projectFilterValuesFromChips(chips));
  };
  const groups = partitionByGroup(allItems);
  const groupTotals = countByDisplayGroup(inbox?.groups);

  return {
    filterChips,
    filterFields,
    groups,
    groupTotals,
    handleFiltersChange,
    hasItems: allItems.length > 0,
  };
}
