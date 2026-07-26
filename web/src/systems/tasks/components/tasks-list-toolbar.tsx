import { ListingToolbar } from "@agh/ui";

import type { TaskFilterOwnerOption } from "../lib/tasks-list-filters";
import type { TaskListSortKey, TaskPriority, TaskStatus } from "../types";
import { TasksListFilters } from "./tasks-list-filters";
import { TasksListSort } from "./tasks-list-sort";

export interface TasksListToolbarProps {
  statusFilter: TaskStatus | null;
  ownerFilter: TaskFilterOwnerOption | null;
  priorityFilter: TaskPriority | null;
  ownerOptions: TaskFilterOwnerOption[];
  sortBy: TaskListSortKey;
  searchQuery: string;
  onStatusChange: (next: TaskStatus | null) => void;
  onOwnerChange: (next: TaskFilterOwnerOption | null) => void;
  onPriorityChange: (next: TaskPriority | null) => void;
  onSortChange: (next: TaskListSortKey) => void;
  onSearchQueryChange: (next: string) => void;
}

/** Window-local tools for the Tasks list context strip. */
export function TasksListToolbar({
  statusFilter,
  ownerFilter,
  priorityFilter,
  ownerOptions,
  sortBy,
  searchQuery,
  onStatusChange,
  onOwnerChange,
  onPriorityChange,
  onSortChange,
  onSearchQueryChange,
}: TasksListToolbarProps) {
  return (
    <ListingToolbar className="w-full">
      <ListingToolbar.Leading className="flex-nowrap">
        <ListingToolbar.Search
          aria-label="Search tasks"
          containerClassName="min-w-28 max-w-48 flex-1"
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
        <TasksListSort onSortChange={onSortChange} sortBy={sortBy} />
      </ListingToolbar.Trailing>
    </ListingToolbar>
  );
}
