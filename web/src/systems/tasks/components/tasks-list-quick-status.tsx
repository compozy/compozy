import { PillGroup, type PillGroupItem } from "@compozy/ui";

import {
  type TaskQuickStatus,
  taskQuickStatusFor,
  taskQuickStatusItems,
} from "../lib/tasks-list-filters";
import type { TaskStatus } from "../types";

export interface TasksListQuickStatusProps {
  className?: string;
  statusFilter: TaskStatus | null;
  statusCounts: Record<TaskStatus, number>;
  onStatusChange: (next: TaskStatus | null) => void;
}

/** Leading status shortcuts on the Tasks strip (All · In progress · Needs attention). */
export function TasksListQuickStatus({
  className,
  statusFilter,
  statusCounts,
  onStatusChange,
}: TasksListQuickStatusProps) {
  const items: PillGroupItem<TaskQuickStatus | "other">[] = taskQuickStatusItems(
    statusFilter,
    statusCounts
  ).map(item => ({
    value: item.value,
    label: item.label,
    badge: item.count,
    testId: `tasks-quick-status-${item.value}`,
  }));
  return (
    <PillGroup
      aria-label="Status"
      className={className}
      data-testid="tasks-quick-status"
      items={items}
      onChange={next => onStatusChange(next === "all" || next === "other" ? null : next)}
      value={taskQuickStatusFor(statusFilter) ?? "other"}
    />
  );
}
