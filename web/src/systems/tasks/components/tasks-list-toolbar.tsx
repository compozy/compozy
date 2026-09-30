import { ListingToolbar } from "@compozy/ui";

import { taskQuickStatusExpresses, type TaskFilterOwnerOption } from "../lib/tasks-list-filters";
import type { TaskListSortKey, TaskPriority, TaskRecordsFilter, TaskStatus } from "../types";
import { useInlineSize } from "../hooks/use-inline-size";
import { TasksListFilters } from "./tasks-list-filters";
import { TasksListQuickStatus } from "./tasks-list-quick-status";
import { TasksListRecordsFilter } from "./tasks-list-records-filter";
import { TasksListSort } from "./tasks-list-sort";

/** Strip width (px) at which the quick status pills fit beside search and Filter. */
const QUICK_STATUS_MIN_STRIP_WIDTH = 672;
/**
 * Strip width (px) below which even the icon-only strip leaves the search short
 * of its usable floor: the field folds to an icon button instead of clipping.
 * The field's 140px floor plus the icon-only controls and gaps measure 329px.
 */
const SEARCH_FIELD_MIN_STRIP_WIDTH = 336;

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
  // The quick pills need the strip's own width (a tiled pane, not the viewport).
  // One measurement decides both whether they show and whether the Filter strip
  // may leave their status to them — so a status is always visible exactly once.
  const [toolbarRef, stripWidth] = useInlineSize<HTMLDivElement>();
  const quickStatusFits = stripWidth === null || stripWidth >= QUICK_STATUS_MIN_STRIP_WIDTH;
  const searchCollapsed = stripWidth !== null && stripWidth < SEARCH_FIELD_MIN_STRIP_WIDTH;
  return (
    // The strip collapses by priority as the pane narrows (a size container, so
    // it measures its own share of the strip): the quick status pills leave
    // first — their status returns as a Filter chip — then Filter, the reveal and
    // the sort drop to their icons, keeping their accessible names; last the
    // search folds to an icon button rather than clip below its usable width.
    <ListingToolbar className="@container/tasks-strip w-full min-w-0" ref={toolbarRef}>
      <ListingToolbar.Leading className="flex-nowrap">
        {quickStatusFits ? (
          <TasksListQuickStatus
            onStatusChange={onStatusChange}
            statusCounts={statusCounts}
            statusFilter={statusFilter}
          />
        ) : null}
        <ListingToolbar.Search
          aria-label="Search tasks"
          collapsed={searchCollapsed}
          containerClassName="max-w-48 flex-1"
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
            statusShownElsewhere={quickStatusFits && taskQuickStatusExpresses(statusFilter)}
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
