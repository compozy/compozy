import { AlertCircle, GitBranch, ListChecks, Search } from "lucide-react";

import { Button, Empty, ListingPage, Skeleton, Spinner } from "@compozy/ui";

import { groupTasksForList, taskStatusFacetTotal } from "../lib/task-grouping";
import type { TaskListItem, TaskRecordsFilter, TaskStatus } from "../types";
import { TaskCard } from "./task-card";
import { TaskGroup } from "./task-group";
import { emptyForScope, type ProfileListingScope } from "@/systems/profiles";

const TASK_LIST_SKELETON_IDS = [
  "task-list-skeleton-1",
  "task-list-skeleton-2",
  "task-list-skeleton-3",
  "task-list-skeleton-4",
  "task-list-skeleton-5",
];

export interface TasksListSurfaceProps {
  tasks: TaskListItem[];
  /** Owner tags in aggregate mode; names the profile in the empty state. */
  profile: Pick<ProfileListingScope, "aggregate" | "ownerOf" | "scopeLabel">;
  statusCounts: Record<TaskStatus, number>;
  isLoading?: boolean;
  errorMessage?: string | null;
  filterState?: "active" | "inactive";
  searchQuery: string;
  /** Which population the server returned — work items only, or with Loop records revealed. */
  recordsFilter?: TaskRecordsFilter;
  onShowWorkItems?: () => void;
  onOpenLoopRun?: () => void;
  hasMore?: boolean;
  isLoadingMore?: boolean;
  onLoadMore?: () => void;
  onRetryLoad?: () => void;
}

export function TasksListSurface({
  tasks,
  profile,
  statusCounts,
  isLoading = false,
  errorMessage = null,
  filterState = "inactive",
  searchQuery,
  recordsFilter = "work",
  onShowWorkItems,
  onOpenLoopRun,
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
  // The reveal is what emptied this list, so the message has to name it and say
  // how to leave it — the generic empty would read as "you have no work"
  // (US-002.EC-1). A narrower filter on top of the reveal is the better story,
  // so it keeps its own message.
  const isRevealEmpty = recordsFilter === "loop" && !hasFilters;

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
        ) : visibleCount === 0 && isRevealEmpty ? (
          <Empty
            action={
              onShowWorkItems ? (
                <Button onClick={onShowWorkItems} size="sm" type="button" variant="neutral">
                  Show work items
                </Button>
              ) : null
            }
            data-testid="tasks-list-surface-loop-empty"
            description="Turn the filter back to work items to see your tasks."
            icon={GitBranch}
            title="No loop records in this workspace"
          />
        ) : visibleCount === 0 ? (
          <Empty
            data-testid="tasks-list-surface-empty"
            description={
              hasFilters
                ? "Clear filters to see other tasks in this workspace."
                : "Open a new task contract from the topbar to populate this list."
            }
            icon={hasFilters ? Search : ListChecks}
            title={
              hasFilters
                ? "No tasks match the current filters"
                : emptyForScope("tasks", profile.scopeLabel)
            }
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
                <TaskCard
                  key={task.id}
                  onOpenLoopRun={onOpenLoopRun}
                  profileOwner={profile.aggregate ? profile.ownerOf(task) : undefined}
                  task={task}
                />
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
