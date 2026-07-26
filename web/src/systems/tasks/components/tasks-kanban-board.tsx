import { AlertCircle } from "lucide-react";

import { Button, Empty, Skeleton, Spinner } from "@agh/ui";

import { TaskKanbanCard } from "./task-kanban-card";
import { TaskKanbanColumn } from "./task-kanban-column";
import {
  taskStatusFacetTotal,
  type KanbanColumnGroup,
  type TaskKanbanColumnId,
} from "../lib/task-grouping";
import type { TaskStatus } from "../types";

import type { PillTone } from "@agh/ui";

/**
 * Column header tone — `In progress` reads as `info` (live work without an
 * accent recolor), `Blocked` reads as `danger`, `Needs attention` reads as
 * `warning` (distinct escalation, no coercion), terminal `Done` and `Pending`
 * stay neutral.
 */
const COLUMN_HEADER_TONE: Record<TaskKanbanColumnId, PillTone> = {
  pending: "neutral",
  in_progress: "info",
  blocked: "danger",
  needs_attention: "warning",
  done: "neutral",
};

const KANBAN_SKELETON_KEYS = ["a", "b", "c"] as const;

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
}: TasksKanbanBoardProps) {
  const loadedTaskCount = columns.reduce((count, column) => count + column.tasks.length, 0);
  if (errorMessage && loadedTaskCount === 0) {
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

  // Derive the grid track count from the canonical column set so it can never
  // drift from `KANBAN_COLUMNS`: a new status column widens the grid instead of
  // wrapping the last column onto a broken second row.
  return (
    <div className="flex min-h-0 flex-1 flex-col overflow-hidden">
      {isLoading ? (
        <span aria-live="polite" className="sr-only" data-testid="tasks-kanban-loading">
          Loading kanban board
        </span>
      ) : null}
      <ul
        className="grid min-h-0 flex-1 gap-3 overflow-y-auto px-4 pt-4 pb-6"
        data-testid="tasks-kanban-board"
        style={{
          gridTemplateColumns: `repeat(${Math.max(columns.length, 1)}, minmax(0, 1fr))`,
        }}
      >
        {columns.map(group => (
          <TaskKanbanColumn
            column={group.column}
            count={group.tasks.length}
            key={group.column.id}
            onAdd={onCreate}
            tone={COLUMN_HEADER_TONE[group.column.id]}
            totalCount={
              isLoading ? undefined : taskStatusFacetTotal(group.column.statuses, statusCounts)
            }
          >
            {isLoading
              ? KANBAN_SKELETON_KEYS.map(slot => (
                  <KanbanCardSkeleton key={`${group.column.id}-skeleton-${slot}`} />
                ))
              : group.tasks.map(task => (
                  <TaskKanbanCard
                    key={task.id}
                    onRetry={onRetryTask}
                    onSelect={onSelectTask}
                    selected={task.id === selectedTaskId}
                    task={task}
                  />
                ))}
          </TaskKanbanColumn>
        ))}
      </ul>
      {errorMessage ? (
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
      ) : null}
      {hasMore && onLoadMore && !errorMessage ? (
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
            {isLoadingMore ? <Spinner aria-hidden="true" className="size-3" /> : null}
            {isLoadingMore ? "Loading more" : "Load more"}
          </Button>
        </div>
      ) : null}
    </div>
  );
}

function KanbanCardSkeleton() {
  return (
    <div
      aria-hidden="true"
      className="flex w-full min-w-0 flex-col gap-2 rounded-md bg-canvas-tint p-3"
      data-testid="tasks-kanban-card-skeleton"
    >
      <Skeleton className="h-3 w-4/5 rounded-xs" />
      <Skeleton className="h-2.5 w-12 rounded-xs" />
      <div className="flex items-center justify-between gap-2">
        <Skeleton className="h-2.5 w-24 rounded-xs" />
        <Skeleton className="h-2.5 w-8 rounded-xs" />
      </div>
    </div>
  );
}
