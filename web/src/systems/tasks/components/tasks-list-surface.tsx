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
  const hasTasks = tasks.length > 0;
  const hasFilters = filterState === "active" || searchQuery.trim() !== "";

  return (
    <ListingPage data-testid="tasks-list-surface">
      <div className="flex flex-col gap-5" data-testid="tasks-list-surface-body">
        {hasTasks ? (
          <TasksListGroups
            onOpenLoopRun={onOpenLoopRun}
            profile={profile}
            statusCounts={statusCounts}
            tasks={tasks}
          />
        ) : (
          <TasksListEmptyBody
            errorMessage={errorMessage}
            hasFilters={hasFilters}
            isLoading={isLoading}
            onRetryLoad={onRetryLoad}
            onShowWorkItems={onShowWorkItems}
            recordsFilter={recordsFilter}
            scopeLabel={profile.scopeLabel}
          />
        )}
        {errorMessage && hasTasks ? (
          <div
            className="flex items-center justify-between gap-3 border-t border-line-soft pt-3 text-caption text-danger"
            data-testid="tasks-list-surface-pagination-error"
            role="alert"
          >
            <span>{errorMessage}</span>
            {onRetryLoad ? <TasksListRetryButton onRetryLoad={onRetryLoad} /> : null}
          </div>
        ) : null}
        {hasMore && onLoadMore && !errorMessage ? (
          <TasksListLoadMore isLoadingMore={isLoadingMore} onLoadMore={onLoadMore} />
        ) : null}
      </div>
    </ListingPage>
  );
}

function TasksListGroups({
  tasks,
  profile,
  statusCounts,
  onOpenLoopRun,
}: Pick<TasksListSurfaceProps, "tasks" | "profile" | "statusCounts" | "onOpenLoopRun">) {
  const buckets = groupTasksForList(tasks).filter(
    bucket =>
      bucket.tasks.length > 0 || taskStatusFacetTotal(bucket.group.statuses, statusCounts) > 0
  );
  return buckets.map(bucket => (
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
  ));
}

function TasksListRetryButton({ onRetryLoad }: { onRetryLoad: () => void }) {
  return (
    <Button onClick={onRetryLoad} size="sm" type="button" variant="ghost">
      Retry loading tasks
    </Button>
  );
}

/** Body shown while the listing has no rows: loading, failed, or genuinely empty. */
function TasksListEmptyBody({
  isLoading,
  errorMessage,
  hasFilters,
  recordsFilter,
  scopeLabel,
  onRetryLoad,
  onShowWorkItems,
}: {
  isLoading: boolean;
  errorMessage: string | null;
  hasFilters: boolean;
  recordsFilter: TaskRecordsFilter;
  scopeLabel: ProfileListingScope["scopeLabel"];
  onRetryLoad?: () => void;
  onShowWorkItems?: () => void;
}) {
  if (isLoading) {
    return (
      <TaskRowsLoadingSkeleton label="Loading tasks" rows={4} testId="tasks-list-surface-loading" />
    );
  }
  if (errorMessage) {
    return (
      <Empty
        action={onRetryLoad ? <TasksListRetryButton onRetryLoad={onRetryLoad} /> : null}
        data-testid="tasks-list-surface-error"
        description={errorMessage}
        icon={AlertCircle}
        title="Couldn't load tasks"
      />
    );
  }
  // The reveal is what emptied this list, so the message has to name it and say
  // how to leave it — the generic empty would read as "you have no work"
  // (US-002.EC-1). A narrower filter on top of the reveal is the better story,
  // so it keeps its own message.
  if (recordsFilter === "loop" && !hasFilters) {
    return (
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
    );
  }
  if (hasFilters) {
    return (
      <Empty
        data-testid="tasks-list-surface-empty"
        description="Clear filters to see other tasks in this project."
        icon={Search}
        title="No tasks match the current filters"
      />
    );
  }
  return (
    <Empty
      data-testid="tasks-list-surface-empty"
      description="Create one with New task above."
      icon={ListChecks}
      title={emptyForScope("tasks", scopeLabel)}
    />
  );
}

function TasksListLoadMore({
  isLoadingMore,
  onLoadMore,
}: {
  isLoadingMore: boolean;
  onLoadMore: () => void;
}) {
  return (
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
  );
}
