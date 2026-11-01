import { AlertCircle } from "@compozy/ui/icons";

import { Button, Empty, SkeletonRegion, Spinner } from "@compozy/ui";

import { TaskKanbanCard } from "./task-kanban-card";
import { TaskKanbanCardSkeleton } from "./task-kanban-card-skeleton";
import { TaskKanbanColumn } from "./task-kanban-column";
import { KANBAN_BOARD_CLASS, KANBAN_LOADING_CARDS, kanbanBoardStyle } from "./tasks-kanban-grid";
import { taskStatusFacetTotal, type KanbanColumnGroup } from "../lib/task-grouping";
import type { TaskListItem, TaskStatus } from "../types";
import { type ProfileListingScope } from "@/systems/profiles";
import type { WorkspaceBadgeIdentity } from "@/systems/workspace";

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

  const board = (
    <ul
      className={KANBAN_BOARD_CLASS}
      data-testid="tasks-kanban-board"
      style={kanbanBoardStyle(columns.length)}
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
            ? (KANBAN_LOADING_CARDS[columnIndex % KANBAN_LOADING_CARDS.length] ?? []).map(card => (
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
      className="flex shrink-0 items-center justify-between gap-3 border-t border-line-soft px-4 py-3 text-caption text-destructive"
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
