import { ListingToolbar } from "@compozy/ui";

import type { TaskFilterOwnerOption } from "../lib/tasks-list-filters";
import type { TaskListSortKey, TaskPriority, TaskRecordsFilter, TaskStatus } from "../types";
import { TasksListFilters } from "./tasks-list-filters";
import { TasksListQuickStatus } from "./tasks-list-quick-status";
import { TasksListRecordsFilter } from "./tasks-list-records-filter";
import { TasksListSort } from "./tasks-list-sort";

export interface TasksListToolbarProps {
  statusFilter: TaskStatus | null;
  statusCounts: Record<TaskStatus, number>;
  ownerFilter: TaskFilterOwnerOption | null;
  priorityFilter: TaskPriority | null;
  ownerOptions: TaskFilterOwnerOption[];
  sortBy: TaskListSortKey;
  searchQuery: string;
  recordsFilter: TaskRecordsFilter;
  onStatusChange: (next: TaskStatus | null) => void;
  onOwnerChange: (next: TaskFilterOwnerOption | null) => void;
  onPriorityChange: (next: TaskPriority | null) => void;
  onSortChange: (next: TaskListSortKey) => void;
  onSearchQueryChange: (next: string) => void;
  onRecordsFilterChange: (next: TaskRecordsFilter) => void;
}

/** Window-local tools for the Tasks list context strip. */
export function TasksListToolbar({
  statusFilter,
  statusCounts,
  ownerFilter,
  priorityFilter,
  ownerOptions,
  sortBy,
  searchQuery,
  recordsFilter,
  onStatusChange,
  onOwnerChange,
  onPriorityChange,
  onSortChange,
  onSearchQueryChange,
  onRecordsFilterChange,
}: TasksListToolbarProps) {
  return (
    // The strip collapses by priority as the pane narrows (a size container, so
    // it measures its own share of the strip): the quick status pills leave
    // first — status stays under Filter — then Filter, the reveal and the sort
    // drop to their icons, keeping their accessible names.
    <ListingToolbar className="@container/tasks-strip w-full min-w-0">
      <ListingToolbar.Leading className="flex-nowrap">
        <TasksListQuickStatus
          className="hidden @2xl/tasks-strip:inline-flex"
          onStatusChange={onStatusChange}
          statusCounts={statusCounts}
          statusFilter={statusFilter}
        />
        <ListingToolbar.Search
          aria-label="Search tasks"
          containerClassName="min-w-20 max-w-48 flex-1"
          data-testid="tasks-list-search-input"
          onChange={onSearchQueryChange}
          placeholder="Search tasks"
          value={searchQuery}
        />
        <ListingToolbar.Filters>
          <TasksListFilters
            onOwnerChange={onOwnerChange}
            onPriorityChange={onPriorityChange}
            onStatusChange={onStatusChange}
            ownerFilter={ownerFilter}
            ownerOptions={ownerOptions}
            priorityFilter={priorityFilter}
            statusFilter={statusFilter}
          />
        </ListingToolbar.Filters>
      </ListingToolbar.Leading>
      <ListingToolbar.Trailing className="gap-2.5">
        {/* Which records, then how they are ordered: the population question is
            upstream of the ordering question. */}
        <TasksListRecordsFilter onChange={onRecordsFilterChange} value={recordsFilter} />
        <TasksListSort onSortChange={onSortChange} sortBy={sortBy} />
      </ListingToolbar.Trailing>
    </ListingToolbar>
  );
}
