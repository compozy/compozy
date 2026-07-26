import { AlertCircle, ListChecks, Search } from "lucide-react";

import { Button, Empty, ListingPage, Skeleton, Spinner } from "@agh/ui";

import { groupTasksForList, taskStatusFacetTotal } from "../lib/task-grouping";
import type { TaskListItem, TaskStatus } from "../types";
import { TaskCard } from "./task-card";
import { TaskGroup } from "./task-group";

const TASK_LIST_SKELETON_IDS = [
  "task-list-skeleton-1",
  "task-list-skeleton-2",
  "task-list-skeleton-3",
  "task-list-skeleton-4",
  "task-list-skeleton-5",
];

export interface TasksListSurfaceProps {
  tasks: TaskListItem[];
  statusCounts: Record<TaskStatus, number>;
  isLoading?: boolean;
  errorMessage?: string | null;
  filterState?: "active" | "inactive";
  searchQuery: string;
  hasMore?: boolean;
  isLoadingMore?: boolean;
  onLoadMore?: () => void;
  onRetryLoad?: () => void;
}

export function TasksListSurface({
  tasks,
  statusCounts,
  isLoading = false,
  errorMessage = null,
  filterState = "inactive",
  searchQuery,
  hasMore = false,
  isLoadingMore = false,
  onLoadMore,
  onRetryLoad,
}: TasksListSurfaceProps) {
  const buckets = groupTasksForList(tasks).filter(
    bucket =>
      bucket.tasks.length > 0 || taskStatusFacetTotal(bucket.group.statuses, statusCounts) > 0
  );

  const visibleCount = tasks.length;
  const hasFilters = filterState === "active" || searchQuery.trim() !== "";

  return (
    <ListingPage data-testid="tasks-list-surface">
      <div className="flex flex-col gap-5" data-testid="tasks-list-surface-body">
        {isLoading && visibleCount === 0 ? (
          <div
            className="overflow-hidden rounded-lg border border-line bg-canvas-soft"
            data-testid="tasks-list-surface-loading"
          >
            {TASK_LIST_SKELETON_IDS.map(id => (
              <div
                className="flex items-center gap-3.5 border-b border-line-soft px-4 py-3 last:border-b-0"
                key={id}
              >
                <Skeleton className="size-[34px] shrink-0 rounded-md" />
                <div className="flex min-w-0 flex-1 flex-col gap-1.5">
                  <Skeleton className="h-3 w-3/5 rounded-xs" />
                  <Skeleton className="h-2.5 w-2/5 rounded-xs" />
                </div>
              </div>
            ))}
          </div>
        ) : errorMessage && visibleCount === 0 ? (
          <div className="flex min-h-0 flex-1 flex-col items-center justify-center gap-3">
            <Empty
              data-testid="tasks-list-surface-error"
              description={errorMessage}
              icon={AlertCircle}
              title="Unable to load tasks"
            />
            {onRetryLoad ? (
              <Button onClick={onRetryLoad} size="sm" type="button" variant="ghost">
                Retry loading tasks
              </Button>
            ) : null}
          </div>
        ) : visibleCount === 0 ? (
          <Empty
            data-testid="tasks-list-surface-empty"
            description={
              hasFilters
                ? "Clear filters to see other tasks in this workspace."
                : "Open a new task contract from the topbar to populate this list."
            }
            icon={hasFilters ? Search : ListChecks}
            title={hasFilters ? "No tasks match the current filters" : "No tasks yet"}
          />
        ) : (
          buckets.map(bucket => (
            <TaskGroup
              count={bucket.tasks.length}
              id={bucket.group.id}
              key={bucket.group.id}
              label={bucket.group.label}
              totalCount={taskStatusFacetTotal(bucket.group.statuses, statusCounts)}
            >
              {bucket.tasks.map(task => (
                <TaskCard key={task.id} task={task} />
              ))}
            </TaskGroup>
          ))
        )}
        {errorMessage && visibleCount > 0 ? (
          <div
            className="flex items-center justify-between gap-3 border-t border-line-soft pt-3 text-caption text-danger"
            data-testid="tasks-list-surface-pagination-error"
            role="alert"
          >
            <span>{errorMessage}</span>
            {onRetryLoad ? (
              <Button onClick={onRetryLoad} size="sm" type="button" variant="ghost">
                Retry loading tasks
              </Button>
            ) : null}
          </div>
        ) : null}
        {hasMore && onLoadMore && !errorMessage ? (
          <div className="flex items-center justify-center border-t border-line-soft pt-3">
            <Button
              aria-busy={isLoadingMore}
              aria-label={isLoadingMore ? "Loading more tasks" : "Load more tasks"}
              data-testid="tasks-list-load-more"
              disabled={isLoadingMore}
              onClick={onLoadMore}
              size="sm"
              type="button"
              variant="ghost"
            >
              {isLoadingMore ? <Spinner aria-hidden="true" className="size-3" /> : null}
              {isLoadingMore ? "Loading more" : "Load more"}
            </Button>
          </div>
        ) : null}
      </div>
    </ListingPage>
  );
}
