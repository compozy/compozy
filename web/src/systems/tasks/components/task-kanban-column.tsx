import { Plus } from "lucide-react";
import * as React from "react";

import { Button, StateGlyph } from "@compozy/ui";

import { cn } from "@/lib/utils";

import type { TaskKanbanColumn as TaskKanbanColumnDef } from "../lib/task-grouping";

export interface TaskKanbanColumnProps {
  column: TaskKanbanColumnDef;
  count: number;
  totalCount?: number;
  onAdd?: () => void;
  emptyState?: React.ReactNode;
  children?: React.ReactNode;
  className?: string;
}

export function TaskKanbanColumn({
  column,
  count,
  totalCount,
  onAdd,
  emptyState,
  children,
  className,
}: TaskKanbanColumnProps) {
  const isEmpty = React.Children.count(children) === 0;
  const countLabel =
    totalCount === undefined
      ? undefined
      : count === totalCount
        ? `${count}`
        : `${count} of ${totalCount}`;

  return (
    <li
      className={cn(
        "flex min-w-0 flex-col overflow-hidden rounded-lg bg-sunken",
        "min-h-115 max-h-[calc(100vh-var(--space-kanban-col-offset))]",
        className
      )}
      data-testid={`tasks-kanban-column-${column.id}`}
    >
      <header className="flex shrink-0 items-center gap-2 px-3 pt-3 pb-2">
        <StateGlyph data-testid={`tasks-kanban-column-glyph-${column.id}`} state={column.glyph} />
        <h2 className="text-item-title font-medium text-fg">{column.label}</h2>
        {countLabel ? (
          <span
            className="text-small-body tabular-nums text-subtle"
            data-testid={`tasks-kanban-column-count-${column.id}`}
          >
            {countLabel}
          </span>
        ) : null}
        {onAdd ? (
          <Button
            aria-label="Create task"
            className="ml-auto"
            data-testid={`tasks-kanban-column-add-${column.id}`}
            onClick={() => onAdd()}
            size="icon-xs"
            type="button"
            variant="quiet"
          >
            <Plus />
          </Button>
        ) : null}
      </header>

      <div
        className="flex min-h-0 flex-1 flex-col gap-2 overflow-y-auto px-2 pt-1 pb-3"
        data-testid={`tasks-kanban-column-body-${column.id}`}
      >
        {isEmpty
          ? (emptyState ?? (
              <div
                className="flex flex-1 items-center justify-center px-3 py-8 text-center text-small-body text-subtle"
                data-testid={`tasks-kanban-column-empty-${column.id}`}
              >
                No tasks
              </div>
            ))
          : children}
      </div>
    </li>
  );
}
