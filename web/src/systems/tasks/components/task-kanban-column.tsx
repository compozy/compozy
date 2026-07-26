import { Plus } from "lucide-react";
import * as React from "react";

import { Button, StatusDot, type StatusDotTone } from "@agh/ui";

import { cn } from "@/lib/utils";

import type { TaskKanbanColumn as TaskKanbanColumnDef } from "../lib/task-grouping";

import type { PillTone } from "@agh/ui";

export interface TaskKanbanColumnProps {
  column: TaskKanbanColumnDef;
  count: number;
  totalCount?: number;
  tone: PillTone;
  onAdd?: () => void;
  emptyState?: React.ReactNode;
  children?: React.ReactNode;
  className?: string;
}

/**
 * Maps the kanban header pill tone onto a `<StatusDot>` tone. Only
 * attention-demanding columns emit a dot; neutral and `info` columns keep the
 * header rhythm flat so the accent budget stays reserved for the active CTA.
 */
function headerDotTone(tone: PillTone): StatusDotTone | null {
  switch (tone) {
    case "warning":
      return "warning";
    case "danger":
      return "danger";
    default:
      return null;
  }
}

export function TaskKanbanColumn({
  column,
  count,
  totalCount,
  tone,
  onAdd,
  emptyState,
  children,
  className,
}: TaskKanbanColumnProps) {
  const isEmpty = React.Children.count(children) === 0;
  const dotTone = headerDotTone(tone);
  const countLabel =
    totalCount === undefined
      ? undefined
      : count === totalCount
        ? `${count}`
        : `${count} of ${totalCount}`;

  return (
    <li
      className={cn(
        "flex min-w-0 flex-col overflow-hidden rounded-lg bg-canvas-soft",
        "min-h-115 max-h-[calc(100vh-var(--space-kanban-col-offset))]",
        className
      )}
      data-testid={`tasks-kanban-column-${column.id}`}
    >
      <header className="flex shrink-0 items-center gap-2 px-3 pt-3 pb-2">
        {dotTone === null ? null : <StatusDot tone={dotTone} size="default" label={column.label} />}
        <h2 className="text-small-body font-medium text-fg-strong">{column.label}</h2>
        {countLabel ? (
          <span
            className="font-mono text-badge tabular-nums text-faint"
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
            variant="ghost"
          >
            <Plus />
          </Button>
        ) : null}
      </header>

      <div
        className="flex min-h-0 flex-1 flex-col gap-1.5 overflow-y-auto px-2 pt-1 pb-3"
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
