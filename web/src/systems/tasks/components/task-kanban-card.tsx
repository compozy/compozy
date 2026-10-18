import * as React from "react";

import { Button, MonoId, OwnerAvatar, Pill, StateGlyph } from "@compozy/ui";

import { cn } from "@/lib/utils";

import {
  formatRelativeTime,
  ownerAvatarKindFor,
  taskOwnerLabel,
  taskShortId,
  taskStateGlyph,
  taskStatusLabel,
} from "../lib/task-formatters";
import type { TaskListItem } from "../types";
import { ProfileOwnerTag, type ProfileOwner } from "@/systems/profiles";

export interface TaskKanbanCardProps {
  task: TaskListItem;
  selected?: boolean;
  onSelect?: (taskId: string) => void;
  onRetry?: (runId: string) => void;
  profileOwner?: ProfileOwner;
}

export function TaskKanbanCard({
  task,
  selected = false,
  onSelect,
  onRetry,
  profileOwner,
}: TaskKanbanCardProps) {
  const activeRun = task.active_run ?? null;
  const isFailed = task.status === "failed";
  const failedError = isFailed && activeRun?.error ? activeRun.error : null;
  const retryRunId = isFailed && onRetry ? activeRun?.id : undefined;
  // Single-status columns already name the status; only the mixed Done column
  // needs to tell failed and canceled cards apart from completed ones.
  const showStatus = task.status === "failed" || task.status === "canceled";
  const selection = kanbanCardSelection(task.id, selected, onSelect);

  return (
    <div
      role="button"
      {...selection.props}
      data-status={task.status}
      data-testid={`tasks-kanban-card-${task.id}`}
      className={cn(
        // A card on the sunken column: `card` plus shadow-card (its hairline),
        // stepping to surface-2 on hover and to the selected plate when chosen.
        "relative flex w-full min-w-0 shrink-0 flex-col gap-2 overflow-hidden rounded-lg bg-card p-3 text-left shadow-card transition-colors duration-base ease-out",
        selection.clickable && "cursor-pointer hover:bg-surface-2",
        selection.clickable && "focus-visible:shadow-focus-ring focus-visible:outline-none",
        selected && "bg-selected inset-ring-1 inset-ring-line-strong"
      )}
    >
      <div className="flex min-w-0 items-start justify-between gap-2">
        <h3 className="line-clamp-2 min-w-0 text-small-body font-medium leading-snug text-fg">
          {task.title}
        </h3>
        {showStatus ? (
          <Pill data-testid={`tasks-kanban-card-status-${task.id}`} form="plain">
            <StateGlyph size="sm" state={taskStateGlyph(task.status)} />
            {taskStatusLabel(task.status)}
          </Pill>
        ) : null}
      </div>

      {task.identifier ? (
        <MonoId value={taskShortId(task)} size="sm" data-slot="k-card-id" />
      ) : null}

      {failedError ? (
        <p
          className="min-w-0 truncate text-form-hint text-danger"
          data-testid={`tasks-kanban-card-error-${task.id}`}
          title={failedError}
        >
          {failedError}
        </p>
      ) : null}

      <TaskKanbanCardFooter profileOwner={profileOwner} task={task} />

      {retryRunId ? <TaskKanbanCardRetry onRetry={onRetry} runId={retryRunId} task={task} /> : null}
    </div>
  );
}

function kanbanCardSelection(
  taskId: string,
  selected: boolean,
  onSelect: ((taskId: string) => void) | undefined
) {
  if (onSelect === undefined) {
    return {
      clickable: false,
      props: {
        "aria-disabled": true,
        "data-selected": selected ? "true" : undefined,
      },
    } as const;
  }
  return {
    clickable: true,
    props: {
      "aria-disabled": false,
      tabIndex: 0,
      "aria-pressed": selected,
      "data-selected": selected ? "true" : undefined,
      onClick: () => onSelect(taskId),
      onKeyDown: (event: React.KeyboardEvent<HTMLDivElement>) => {
        if (event.target !== event.currentTarget) return;
        if (event.key === "Enter" || event.key === " ") {
          event.preventDefault();
          onSelect(taskId);
        }
      },
    },
  } as const;
}

function TaskKanbanCardFooter({
  task,
  profileOwner,
}: {
  task: TaskListItem;
  profileOwner?: ProfileOwner;
}) {
  const ownerLabel = taskOwnerLabel(task.owner);
  const ownerId = task.owner?.ref ?? task.owner?.kind ?? "unassigned";
  return (
    <div className="flex min-w-0 items-center justify-between gap-2">
      <div
        className="flex min-w-0 items-center gap-1.5"
        data-testid={`tasks-kanban-card-owner-${task.id}`}
      >
        <OwnerAvatar
          data-testid={`tasks-kanban-card-avatar-${task.id}`}
          name={ownerLabel}
          ownerId={ownerId}
          ownerKind={ownerAvatarKindFor(task.owner?.kind)}
          size="sm"
        />
        <span className="min-w-0 truncate text-eyebrow text-muted">{ownerLabel}</span>
        {profileOwner ? (
          <ProfileOwnerTag
            data-testid={`tasks-kanban-card-profile-${task.id}`}
            owner={profileOwner}
          />
        ) : null}
      </div>
      <span
        className="shrink-0 font-mono text-badge tabular-nums text-faint"
        data-testid={`tasks-kanban-card-time-${task.id}`}
      >
        {formatRelativeTime(task.last_activity_at ?? task.updated_at)}
      </span>
    </div>
  );
}

function TaskKanbanCardRetry({
  task,
  runId,
  onRetry,
}: {
  task: TaskListItem;
  runId: string;
  onRetry?: (runId: string) => void;
}) {
  return (
    <Button
      aria-label={`Retry ${task.title}`}
      className="self-start"
      data-testid={`tasks-kanban-card-retry-${task.id}`}
      onClick={event => {
        event.stopPropagation();
        onRetry?.(runId);
      }}
      size="xs"
      type="button"
      variant="neutral"
    >
      Retry
    </Button>
  );
}
