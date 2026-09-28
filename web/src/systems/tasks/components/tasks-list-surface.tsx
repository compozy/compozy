import { AlertCircle, GitBranch, ListChecks, Search } from "lucide-react";

import { Button, Empty, ListingPage, Spinner } from "@compozy/ui";

import { groupTasksForList, taskStatusFacetTotal } from "../lib/task-grouping";
import type { TaskListItem, TaskRecordsFilter, TaskStatus } from "../types";
import { TaskCard } from "./task-card";
import { TaskGroup } from "./task-group";
import { TaskRowsLoadingSkeleton } from "./task-loading-skeletons";
import { emptyForScope, type ProfileListingScope } from "@/systems/profiles";

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
          <TaskRowsLoadingSkeleton
            label="Loading tasks"
            rows={4}
            testId="tasks-list-surface-loading"
          />
        ) : errorMessage && visibleCount === 0 ? (
          <Empty
            action={
              onRetryLoad ? (
                <Button onClick={onRetryLoad} size="sm" type="button" variant="ghost">
                  Retry loading tasks
                </Button>
              ) : null
            }
            data-testid="tasks-list-surface-error"
            description={errorMessage}
            icon={AlertCircle}
            title="Couldn't load tasks"
          />
        ) : visibleCount === 0 && isRevealEmpty ? (
          <Empty
            action={
              onShowWorkItems ? (
                <Button onClick={onShowWorkItems} size="sm" type="button" variant="neutral">
                  Show tasks
                </Button>
              ) : null
            }
            data-testid="tasks-list-surface-loop-empty"
            description="Switch back to Tasks to see your work."
            icon={GitBranch}
            title="No loop steps in this project"
          />
        ) : visibleCount === 0 ? (
          <Empty
            data-testid="tasks-list-surface-empty"
            description={
              hasFilters
                ? "Clear filters to see other tasks in this project."
                : "Create one with New task above."
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
