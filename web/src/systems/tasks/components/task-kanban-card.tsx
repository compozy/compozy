import { AlertCircle } from "lucide-react";
import * as React from "react";

import { Button, MonoId, OwnerAvatar, Pill } from "@agh/ui";

import { cn } from "@/lib/utils";

import {
  formatRelativeTime,
  ownerAvatarKindFor,
  taskOwnerLabel,
  taskShortId,
  taskStatusTone,
} from "../lib/task-formatters";
import type { TaskListItem } from "../types";

export interface TaskKanbanCardProps {
  task: TaskListItem;
  selected?: boolean;
  onSelect?: (taskId: string) => void;
  onRetry?: (runId: string) => void;
}

const STATUS_LABELS: Partial<Record<TaskListItem["status"], string>> = {
  pending: "Pending",
  ready: "Ready",
  in_progress: "In progress",
  blocked: "Blocked",
  needs_attention: "Needs attention",
  completed: "Done",
  failed: "Failed",
  canceled: "Canceled",
  draft: "Draft",
};

function statusLabel(status: TaskListItem["status"]): string {
  return STATUS_LABELS[status] ?? status;
}

export function TaskKanbanCard({ task, selected = false, onSelect, onRetry }: TaskKanbanCardProps) {
  const activeRun = task.active_run ?? null;
  const isFailed = task.status === "failed";
  const failedError = isFailed && activeRun?.error ? activeRun.error : null;
  const canRetry = isFailed && Boolean(activeRun?.id && onRetry);

  const identifier = taskShortId(task);
  const ownerLabel = taskOwnerLabel(task.owner);
  const ownerId = task.owner?.ref ?? task.owner?.kind ?? "unassigned";
  const ownerKind = ownerAvatarKindFor(task.owner?.kind);
  const lastActivity = task.last_activity_at ?? task.updated_at;
  const timestamp = formatRelativeTime(lastActivity);
  const statusTone = taskStatusTone(task.status);
  const showStatusPill = statusTone !== "neutral";

  const clickable = onSelect !== undefined;
  const selectTaskCard = clickable ? () => onSelect?.(task.id) : undefined;
  const handleKeyDown = clickable
    ? (event: React.KeyboardEvent<HTMLDivElement>) => {
        if (event.target !== event.currentTarget) return;
        if (event.key === "Enter" || event.key === " ") {
          event.preventDefault();
          onSelect?.(task.id);
        }
      }
    : undefined;

  return (
    <div
      role="button"
      aria-disabled={!clickable}
      tabIndex={clickable ? 0 : undefined}
      aria-pressed={clickable ? selected : undefined}
      data-selected={selected ? "true" : undefined}
      data-status={task.status}
      data-testid={`tasks-kanban-card-${task.id}`}
      onClick={selectTaskCard}
      onKeyDown={handleKeyDown}
      className={cn(
        "relative flex w-full min-w-0 flex-col gap-2 overflow-hidden rounded-md bg-canvas-tint p-3 text-left transition-colors duration-base ease-out",
        "shadow-hairline-inset",
        "hover:bg-elevated hover:inset-ring-1 hover:inset-ring-line",
        clickable && "cursor-pointer",
        clickable &&
          "focus-visible:shadow-focus-inset focus-visible:outline-none focus-visible:ring-0",
        selected && "bg-elevated inset-ring-1 inset-ring-line"
      )}
    >
      <div className="flex min-w-0 items-start justify-between gap-2">
        <h3 className="line-clamp-2 min-w-0 text-small-body font-medium leading-snug text-fg-strong">
          {task.title}
        </h3>
        {showStatusPill ? (
          <Pill size="xs" tone={statusTone}>
            {statusLabel(task.status)}
          </Pill>
        ) : null}
      </div>

      <MonoId value={identifier} size="sm" data-slot="k-card-id" />

      {failedError ? (
        <div
          className="flex items-start gap-1.5 rounded-xs bg-danger-tint px-2 py-1 font-mono text-mono-id text-danger"
          data-testid={`tasks-kanban-card-error-${task.id}`}
        >
          <AlertCircle aria-hidden="true" className="mt-px size-3 shrink-0" />
          <span className="min-w-0 wrap-break-word">{failedError}</span>
        </div>
      ) : null}

      <div className="flex min-w-0 items-center justify-between gap-2">
        <div
          className="flex min-w-0 items-center gap-1.5"
          data-testid={`tasks-kanban-card-owner-${task.id}`}
        >
          <OwnerAvatar
            data-testid={`tasks-kanban-card-avatar-${task.id}`}
            name={ownerLabel}
            ownerId={ownerId}
            ownerKind={ownerKind}
            size="sm"
          />
          <span className="min-w-0 truncate text-eyebrow text-muted">{ownerLabel}</span>
        </div>
        <span
          className="shrink-0 font-mono text-badge tabular-nums text-faint"
          data-testid={`tasks-kanban-card-time-${task.id}`}
        >
          {timestamp}
        </span>
      </div>

      {canRetry ? (
        <Button
          aria-label={`Retry ${task.title}`}
          className="self-start"
          data-testid={`tasks-kanban-card-retry-${task.id}`}
          onClick={event => {
            event.stopPropagation();
            if (activeRun) {
              onRetry?.(activeRun.id);
            }
          }}
          size="xs"
          type="button"
          variant="neutral"
        >
          Retry
        </Button>
      ) : null}
    </div>
  );
}
