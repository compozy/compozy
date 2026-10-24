import { PillGroup, type PillGroupItem, SkeletonText } from "@compozy/ui";

import {
  type TaskQuickStatus,
  taskQuickStatusFor,
  taskQuickStatusItems,
} from "../lib/tasks-list-filters";
import type { TaskStatus } from "../types";

export interface TasksListQuickStatusProps {
  statusFilter: TaskStatus | null;
  statusCounts: Record<TaskStatus, number>;
  /** The facet counts are still being read: each pill that will carry one holds its place. */
  countsLoading?: boolean;
  onStatusChange: (next: TaskStatus | null) => void;
}

/** Leading status shortcuts on the Tasks strip (All · In progress · Needs attention). */
export function TasksListQuickStatus({
  statusFilter,
  statusCounts,
  countsLoading = false,
  onStatusChange,
}: TasksListQuickStatusProps) {
  const items: PillGroupItem<TaskQuickStatus | "other">[] = taskQuickStatusItems(
    statusFilter,
    statusCounts
  ).map(item => ({
    value: item.value,
    // A pill that will carry a count holds its place while the counts load, so the strip's other
    // controls do not shift when they arrive.
    label:
      countsLoading && item.count !== undefined ? (
        <>
          {item.label}
          <SkeletonText className="w-2.5" />
        </>
      ) : (
        item.label
      ),
    badge: item.count,
    testId: `tasks-quick-status-${item.value}`,
  }));
  return (
    <PillGroup
      aria-label="Status"
      data-testid="tasks-quick-status"
      items={items}
      onChange={next => onStatusChange(next === "all" || next === "other" ? null : next)}
      value={taskQuickStatusFor(statusFilter) ?? "other"}
    />
  );
}
