import { AlertCircle } from "lucide-react";

import { Button, Empty, SkeletonRegion, Spinner } from "@compozy/ui";

import { TaskKanbanCard, TaskKanbanCardSkeleton } from "./task-kanban-card";
import { TaskKanbanColumn } from "./task-kanban-column";
import { taskStatusFacetTotal, type KanbanColumnGroup } from "../lib/task-grouping";
import type { TaskListItem, TaskStatus } from "../types";
import { type ProfileListingScope } from "@/systems/profiles";
import type { WorkspaceBadgeIdentity } from "@/systems/workspace";

/**
 * Placeholder cards per column while the board loads, in column order: a backlog-heavy shape that
 * fills the first columns and thins out, with some two-line titles among the one-line ones. Ids
 * are static; the placeholders carry no identity of their own.
 */
const LOADING_CARDS = [
  [
    { id: "a", titleLines: 2 },
    { id: "b", titleLines: 1 },
    { id: "c", titleLines: 1 },
  ],
  [
    { id: "a", titleLines: 1 },
    { id: "b", titleLines: 2 },
  ],
  [{ id: "a", titleLines: 1 }],
  [{ id: "a", titleLines: 2 }],
  [
    { id: "a", titleLines: 1 },
    { id: "b", titleLines: 1 },
    { id: "c", titleLines: 2 },
  ],
] as const;

export interface TasksKanbanBoardProps {
  columns: KanbanColumnGroup[];
  selectedTaskId: string | null;
  onSelectTask: (taskId: string) => void;
  onCreate?: () => void;
  onRetryTask?: (runId: string) => void;
  isLoading?: boolean;
  errorMessage?: string | null;
  statusCounts: Record<TaskStatus, number>;
  hasMore?: boolean;
  isLoadingMore?: boolean;
  onLoadMore?: () => void;
  onRetryLoad?: () => void;
  profile?: ProfileListingScope;
  /** Each card's project under All projects; absent while one scope is listed. */
  projectBadgeOf?: (task: TaskListItem) => WorkspaceBadgeIdentity;
}

export function TasksKanbanBoard({
  columns,
  selectedTaskId,
  onSelectTask,
  onCreate,
  onRetryTask,
  isLoading = false,
  errorMessage = null,
  statusCounts,
  hasMore = false,
  isLoadingMore = false,
  onLoadMore,
  onRetryLoad,
  profile,
  projectBadgeOf,
}: TasksKanbanBoardProps) {
  const loadedTaskCount = columns.reduce((count, column) => count + column.tasks.length, 0);
  if (errorMessage && loadedTaskCount === 0) {
    return <KanbanLoadError errorMessage={errorMessage} onRetryLoad={onRetryLoad} />;
  }
  // Placeholders stand in for cards that are not here yet; once any card has loaded they never
  // replace it, however the flag moves.
  const showSkeleton = isLoading && loadedTaskCount === 0;

  // Derive the grid track count from the canonical column set so it can never
  // drift from `KANBAN_COLUMNS`: a new status column widens the grid instead of
  // wrapping the last column onto a broken second row.
  const board = (
    <ul
      className="grid min-h-0 flex-1 gap-3 overflow-y-auto px-4 pt-4 pb-6"
      data-testid="tasks-kanban-board"
      style={{
        gridTemplateColumns: `repeat(${Math.max(columns.length, 1)}, minmax(0, 1fr))`,
      }}
    >
      {columns.map((group, columnIndex) => (
        <TaskKanbanColumn
          column={group.column}
          count={group.tasks.length}
          key={group.column.id}
          onAdd={onCreate}
          totalCount={
            showSkeleton ? undefined : taskStatusFacetTotal(group.column.statuses, statusCounts)
          }
        >
          {showSkeleton
            ? (LOADING_CARDS[columnIndex % LOADING_CARDS.length] ?? []).map(card => (
                <TaskKanbanCardSkeleton
                  key={`${group.column.id}-skeleton-${card.id}`}
                  titleLines={card.titleLines}
                />
              ))
            : group.tasks.map(task => (
                <TaskKanbanCard
                  key={task.id}
                  onRetry={onRetryTask}
                  onSelect={onSelectTask}
                  profileOwner={profile?.aggregate ? profile.ownerOf(task) : undefined}
                  projectBadge={projectBadgeOf?.(task)}
                  selected={task.id === selectedTaskId}
                  task={task}
                />
              ))}
        </TaskKanbanColumn>
      ))}
    </ul>
  );

  return (
    <div className="flex min-h-0 flex-1 flex-col overflow-hidden">
      {showSkeleton ? (
        <SkeletonRegion
          className="flex min-h-0 flex-1 flex-col"
          data-testid="tasks-kanban-loading"
          label="Loading kanban board"
        >
          {board}
        </SkeletonRegion>
      ) : (
        board
      )}
      {errorMessage ? (
        <KanbanPaginationError errorMessage={errorMessage} onRetryLoad={onRetryLoad} />
      ) : null}
      {hasMore && onLoadMore && !errorMessage ? (
        <KanbanLoadMore isLoadingMore={isLoadingMore} onLoadMore={onLoadMore} />
      ) : null}
    </div>
  );
}

function KanbanLoadError({
  errorMessage,
  onRetryLoad,
}: {
  errorMessage: string;
  onRetryLoad?: () => void;
}) {
  return (
    <div
      className="flex flex-1 flex-col items-center justify-center gap-3"
      data-testid="tasks-kanban-error"
      role="alert"
    >
      <Empty description={errorMessage} icon={AlertCircle} title="Unable to load kanban" />
      {onRetryLoad ? (
        <Button onClick={onRetryLoad} size="sm" type="button" variant="ghost">
          Retry loading tasks
        </Button>
      ) : null}
    </div>
  );
}

function KanbanPaginationError({
  errorMessage,
  onRetryLoad,
}: {
  errorMessage: string;
  onRetryLoad?: () => void;
}) {
  return (
    <div
      className="flex shrink-0 items-center justify-between gap-3 border-t border-line-soft px-4 py-3 text-caption text-danger"
      data-testid="tasks-kanban-pagination-error"
      role="alert"
    >
      <span>{errorMessage}</span>
      {onRetryLoad ? (
        <Button onClick={onRetryLoad} size="sm" type="button" variant="ghost">
          Retry loading tasks
        </Button>
      ) : null}
    </div>
  );
}

function KanbanLoadMore({
  isLoadingMore,
  onLoadMore,
}: {
  isLoadingMore: boolean;
  onLoadMore: () => void;
}) {
  return (
    <div className="flex shrink-0 items-center justify-center border-t border-line-soft px-4 py-3">
      <Button
        aria-busy={isLoadingMore}
        aria-label={isLoadingMore ? "Loading more tasks" : "Load more tasks"}
        data-testid="tasks-kanban-load-more"
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
