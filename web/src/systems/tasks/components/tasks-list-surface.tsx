import { AlertCircle, GitBranch, ListChecks, Search } from "lucide-react";
import type { ReactNode } from "react";

import {
  Button,
  Empty,
  ListingPage,
  SkeletonRegion,
  Spinner,
  Table,
  TableHead,
  TableHeader,
  TableRow,
} from "@compozy/ui";
import { cn } from "@/lib/utils";

import { groupTasksForList, taskStatusFacetTotal } from "../lib/task-grouping";
import { TASKS_TABLE_COLUMN_CLASS } from "../lib/tasks-table-columns";
import type { TaskListItem, TaskRecordsFilter, TaskStatus } from "../types";
import { TaskCard } from "./task-card";
import { TaskGroup, TaskGroupSkeleton } from "./task-group";
import { TasksListRowSkeleton } from "./tasks-list-row";
import { emptyForScope, type ProfileListingScope } from "@/systems/profiles";
import type { WorkspaceBadgeIdentity } from "@/systems/workspace";

/**
 * The buckets and rows a loading list stands in with: two groups of varied rows, enough to fill a
 * window, with no bucket named because none is known yet. Ids are static; the placeholders carry
 * no identity of their own.
 */
const LOADING_GROUPS = [
  {
    id: "first",
    rows: [
      { id: "a", titleWidth: "w-56", metaWidth: "w-24" },
      { id: "b", titleWidth: "w-40", metaWidth: "w-16" },
      { id: "c", titleWidth: "w-64", metaWidth: "w-28" },
      { id: "d", titleWidth: "w-48", metaWidth: "w-20" },
    ],
  },
  {
    id: "second",
    rows: [
      { id: "a", titleWidth: "w-44", metaWidth: "w-20" },
      { id: "b", titleWidth: "w-60", metaWidth: "w-24" },
      { id: "c", titleWidth: "w-36", metaWidth: "w-16" },
      { id: "d", titleWidth: "w-52", metaWidth: "w-28" },
      { id: "e", titleWidth: "w-48", metaWidth: "w-20" },
      { id: "f", titleWidth: "w-40", metaWidth: "w-24" },
    ],
  },
] as const;

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
  /** Each row's project under All projects; absent while one scope is listed. */
  projectBadgeOf?: (task: TaskListItem) => WorkspaceBadgeIdentity;
  /** Clears every list filter, offered when the filters emptied the list. */
  onClearFilters?: () => void;
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
  projectBadgeOf,
  onClearFilters,
}: TasksListSurfaceProps) {
  const hasTasks = tasks.length > 0;
  const hasFilters = filterState === "active" || searchQuery.trim() !== "";

  // No rows yet and the read is still out: stand in with the table itself, in the container the
  // rows will fill, rather than a generic placeholder in the empty-state page.
  if (!hasTasks && isLoading) return <TasksListLoading />;

  if (!hasTasks) {
    return (
      <ListingPage data-testid="tasks-list-surface">
        <div className="flex flex-col gap-5" data-testid="tasks-list-surface-body">
          <TasksListEmptyBody
            errorMessage={errorMessage}
            hasFilters={hasFilters}
            onClearFilters={onClearFilters}
            onRetryLoad={onRetryLoad}
            onShowWorkItems={onShowWorkItems}
            recordsFilter={recordsFilter}
            scopeLabel={profile.scopeLabel}
          />
        </div>
      </ListingPage>
    );
  }

  // Rows run edge to edge on the pane surface (reference table grammar); the
  // `@container` lets columns drop by pane width rather than viewport width.
  return (
    <div
      className="@container flex min-h-0 flex-1 flex-col overflow-y-auto"
      data-slot="tasks-list-surface"
      data-testid="tasks-list-surface"
    >
      <div className="flex flex-col pb-4" data-testid="tasks-list-surface-body">
        <TasksListTable
          onOpenLoopRun={onOpenLoopRun}
          profile={profile}
          projectBadgeOf={projectBadgeOf}
          statusCounts={statusCounts}
          tasks={tasks}
        />
        {errorMessage ? (
          <div
            className="flex items-center justify-between gap-3 px-4 pt-3 text-small-body text-danger"
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
    </div>
  );
}

type TasksListTableProps = Pick<
  TasksListSurfaceProps,
  "tasks" | "profile" | "statusCounts" | "onOpenLoopRun" | "projectBadgeOf"
>;

function TasksListTable({
  tasks,
  profile,
  statusCounts,
  onOpenLoopRun,
  projectBadgeOf,
}: TasksListTableProps) {
  const total = Object.values(statusCounts).reduce((sum, count) => sum + count, 0);
  return (
    <TasksListTableFrame total={Math.max(total, tasks.length)}>
      <TasksListGroups
        onOpenLoopRun={onOpenLoopRun}
        profile={profile}
        projectBadgeOf={projectBadgeOf}
        statusCounts={statusCounts}
        tasks={tasks}
      />
    </TasksListTableFrame>
  );
}

/**
 * The table and its header, shared by the loaded list and its skeleton so the header renders
 * at once with real labels. `total` is the whole list's size; absent while it is not known yet.
 */
function TasksListTableFrame({ total, children }: { total?: number; children: ReactNode }) {
  return (
    <Table className="table-fixed" data-testid="tasks-list-table" overflowX="hidden">
      <TableHeader>
        <TableRow>
          <TableHead className="pl-4">
            Task
            {total === undefined ? null : (
              <>
                {" "}
                <span className="font-normal text-subtle tabular-nums" data-slot="tasks-list-total">
                  ({total})
                </span>
              </>
            )}
          </TableHead>
          <TableHead className={TASKS_TABLE_COLUMN_CLASS.id}>ID</TableHead>
          <TableHead className={TASKS_TABLE_COLUMN_CLASS.status}>Status</TableHead>
          <TableHead className={TASKS_TABLE_COLUMN_CLASS.priority}>Priority</TableHead>
          <TableHead className={cn("pr-4 @5xl:pr-3", TASKS_TABLE_COLUMN_CLASS.owner)}>
            <span className="sr-only">Owner</span>
          </TableHead>
          <TableHead className={cn("pr-4", TASKS_TABLE_COLUMN_CLASS.updated)}>Updated</TableHead>
        </TableRow>
      </TableHeader>
      {children}
    </Table>
  );
}

function TasksListLoading() {
  return (
    <div
      className="@container flex min-h-0 flex-1 flex-col overflow-hidden"
      data-slot="tasks-list-surface"
      data-testid="tasks-list-surface"
    >
      <SkeletonRegion
        className="flex flex-col pb-4"
        data-testid="tasks-list-surface-loading"
        label="Loading tasks"
      >
        <TasksListTableFrame>
          {LOADING_GROUPS.map(group => (
            <TaskGroupSkeleton key={group.id}>
              {group.rows.map(row => (
                <TasksListRowSkeleton
                  key={row.id}
                  metaWidth={row.metaWidth}
                  titleWidth={row.titleWidth}
                />
              ))}
            </TaskGroupSkeleton>
          ))}
        </TasksListTableFrame>
      </SkeletonRegion>
    </div>
  );
}

function TasksListGroups({
  tasks,
  profile,
  statusCounts,
  onOpenLoopRun,
  projectBadgeOf,
}: TasksListTableProps) {
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
          projectBadge={projectBadgeOf?.(task)}
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

/** Body shown when the listing has no rows: failed, filtered to nothing, or genuinely empty. */
function TasksListEmptyBody({
  errorMessage,
  hasFilters,
  recordsFilter,
  scopeLabel,
  onRetryLoad,
  onShowWorkItems,
  onClearFilters,
}: {
  errorMessage: string | null;
  hasFilters: boolean;
  recordsFilter: TaskRecordsFilter;
  scopeLabel: ProfileListingScope["scopeLabel"];
  onRetryLoad?: () => void;
  onShowWorkItems?: () => void;
  onClearFilters?: () => void;
}) {
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
        action={
          onClearFilters ? (
            <Button
              data-testid="tasks-list-clear-filters"
              onClick={onClearFilters}
              size="sm"
              type="button"
              variant="neutral"
            >
              Clear filters
            </Button>
          ) : null
        }
        data-testid="tasks-list-surface-empty"
        description="Clear filters to see other tasks."
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
    <div className="flex items-center justify-center pt-3">
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
        {isLoadingMore ? (
          <Spinner aria-hidden="true" className="size-3.5" data-icon="inline-start" />
        ) : null}
        {isLoadingMore ? "Loading more" : "Load more"}
      </Button>
    </div>
  );
}
