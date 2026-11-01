import { Button, MonoId, OwnerAvatar, Pill, StateGlyph } from "@compozy/ui";

import { cn } from "@/lib/utils";
import { CARD_SURFACE_CLASS } from "./task-kanban-card-classes";

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
import { WorkspaceBadge, type WorkspaceBadgeIdentity } from "@/systems/workspace";

export interface TaskKanbanCardProps {
  task: TaskListItem;
  selected?: boolean;
  onSelect?: (taskId: string) => void;
  onRetry?: (runId: string) => void;
  profileOwner?: ProfileOwner;
  /** The task's project, under All projects only. */
  projectBadge?: WorkspaceBadgeIdentity;
}

/**
 * A card on the sunken column: `card` plus shadow-card (its hairline). The real card and its
 * skeleton share this surface, so the placeholder measures the same box.
 */
export function TaskKanbanCard({
  task,
  selected = false,
  onSelect,
  onRetry,
  profileOwner,
  projectBadge,
}: TaskKanbanCardProps) {
  const retryRunId = task.status === "failed" && onRetry ? task.active_run?.id : undefined;
  const clickable = onSelect !== undefined;

  return (
    <div
      data-selected={selected ? "true" : undefined}
      data-status={task.status}
      data-testid={`tasks-kanban-card-${task.id}`}
      className={cn(
        // Steps to surface-2 on hover and to the selected plate when chosen.
        CARD_SURFACE_CLASS,
        "transition-colors duration-base ease-out",
        clickable &&
          "cursor-pointer hover:bg-surface-2 has-[[data-card-open]:focus-visible]:shadow-focus-ring",
        selected && "bg-selected inset-ring-1 inset-ring-line-strong"
      )}
    >
      <TaskKanbanCardHeader onSelect={onSelect} selected={selected} task={task} />
      <TaskKanbanCardMeta projectBadge={projectBadge} task={task} />
      <TaskKanbanCardError task={task} />

      <TaskKanbanCardFooter profileOwner={profileOwner} task={task} />

      {retryRunId ? <TaskKanbanCardRetry onRetry={onRetry} runId={retryRunId} task={task} /> : null}
    </div>
  );
}

/**
 * The title opens the task. Its hit area stretches over the whole card (the card holds its own
 * Retry button, so the card itself cannot be a button); Retry sits above it.
 */
function TaskKanbanCardHeader({
  task,
  selected,
  onSelect,
}: {
  task: TaskListItem;
  selected: boolean;
  onSelect?: (taskId: string) => void;
}) {
  // Single-status columns already name the status; only the mixed Done column
  // needs to tell failed and canceled cards apart from completed ones.
  const showStatus = task.status === "failed" || task.status === "canceled";
  return (
    <div className="flex min-w-0 items-start justify-between gap-2">
      <h3 className="line-clamp-2 min-w-0 text-small-body font-medium leading-snug text-fg">
        {onSelect ? (
          <button
            aria-pressed={selected}
            className="text-left after:absolute after:inset-0 after:rounded-lg focus-visible:outline-none"
            data-card-open=""
            data-testid={`tasks-kanban-card-open-${task.id}`}
            onClick={() => onSelect(task.id)}
            type="button"
          >
            {task.title}
          </button>
        ) : (
          task.title
        )}
      </h3>
      {showStatus ? (
        <Pill data-testid={`tasks-kanban-card-status-${task.id}`} form="plain">
          <StateGlyph size="sm" state={taskStateGlyph(task.status)} />
          {taskStatusLabel(task.status)}
        </Pill>
      ) : null}
    </div>
  );
}

function TaskKanbanCardMeta({
  task,
  projectBadge,
}: {
  task: TaskListItem;
  projectBadge?: WorkspaceBadgeIdentity;
}) {
  if (!task.identifier && !projectBadge) return null;
  return (
    <div className="flex min-w-0 items-center gap-2">
      {projectBadge ? (
        <WorkspaceBadge
          className="min-w-0"
          data-testid={`tasks-kanban-card-project-${task.id}`}
          identity={projectBadge}
        />
      ) : null}
      {task.identifier ? (
        <MonoId value={taskShortId(task)} size="sm" data-slot="k-card-id" />
      ) : null}
    </div>
  );
}

function TaskKanbanCardError({ task }: { task: TaskListItem }) {
  const failedError = task.status === "failed" ? task.active_run?.error : null;
  if (!failedError) return null;
  return (
    <p
      className="min-w-0 truncate text-form-hint text-destructive"
      data-testid={`tasks-kanban-card-error-${task.id}`}
      title={failedError}
    >
      {failedError}
    </p>
  );
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
        <span className="min-w-0 truncate text-eyebrow text-muted-foreground">{ownerLabel}</span>
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
      className="relative z-10 self-start"
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
