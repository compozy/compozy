import { AlertCircle, Archive, ArchiveX, Check, Eye, RotateCcw, X } from "lucide-react";
import { Link } from "@tanstack/react-router";
import type { ReactNode } from "react";

import { Button, Eyebrow, MonoId, Pill } from "@agh/ui";

import { cn } from "@/lib/utils";

import type { InboxGroupId } from "../lib/inbox-grouping";
import {
  formatAttemptLabel,
  formatRelativeTime,
  taskApprovalStateLabel,
  taskShortId,
  taskStatusLabel,
  taskStatusTone,
} from "../lib/task-formatters";
import type { TaskInboxItem } from "../types";
import { TasksInboxRow } from "./tasks-inbox-row";

export interface TasksInboxItemProps {
  item: TaskInboxItem;
  group: InboxGroupId;
  onApprove?: (taskId: string) => void;
  onReject?: (taskId: string) => void;
  onRetry?: (runId: string) => void;
  onArchive?: (taskId: string) => void;
  onDismiss?: (taskId: string) => void;
  onMarkRead?: (taskId: string) => void;
  onOpen?: (taskId: string) => void;
  pendingApproveIds?: ReadonlySet<string>;
  pendingRejectIds?: ReadonlySet<string>;
  pendingRetryIds?: ReadonlySet<string>;
  pendingArchiveIds?: ReadonlySet<string>;
  pendingDismissIds?: ReadonlySet<string>;
  pendingMarkReadIds?: ReadonlySet<string>;
}

export function TasksInboxItem({
  item,
  group,
  onApprove,
  onReject,
  onRetry,
  onArchive,
  onDismiss,
  onMarkRead,
  onOpen,
  pendingApproveIds,
  pendingRejectIds,
  pendingRetryIds,
  pendingArchiveIds,
  pendingDismissIds,
  pendingMarkReadIds,
}: TasksInboxItemProps) {
  const { task, run, triage, lane } = item;
  const taskId = task.id;
  const unread = !triage.read && !triage.dismissed;
  const isApprovalItem = lane === "approvals";
  const isFailedRun = lane === "failed_runs";
  const isArchived = lane === "archived" || triage.archived;
  const failedError = run?.error ?? null;
  const identifier = taskShortId(task);
  const ownerLabel = task.owner?.ref ?? "--";
  const lastActivity = formatRelativeTime(item.latest_activity_at);

  const handleSelect = onOpen ? () => onOpen(taskId) : undefined;

  const top = (
    <>
      <h3
        className={cn(
          "min-w-0 max-w-full truncate text-small-body text-fg-strong",
          unread ? "font-medium" : "font-normal"
        )}
        data-slot="tasks-inbox-row-title"
      >
        {task.title}
      </h3>
      <MonoId value={identifier} size="sm" data-slot="tasks-inbox-row-id" />
      <Pill size="xs" tone={taskStatusTone(task.status)}>
        {taskStatusLabel(task.status)}
      </Pill>
      {run ? (
        <span
          className="inline-flex items-center gap-1.5 text-small-body text-muted"
          data-testid={`tasks-inbox-item-run-${taskId}`}
        >
          <MonoId value={run.id} size="sm" />
          {typeof run.attempt === "number" ? (
            <Eyebrow>{formatAttemptLabel(run.attempt, run.max_attempts) ?? ""}</Eyebrow>
          ) : null}
        </span>
      ) : null}
    </>
  );

  const detail = (
    <>
      {item.blocking_reason ? (
        <p data-testid={`tasks-inbox-item-blocking-${taskId}`}>{item.blocking_reason}</p>
      ) : null}

      {failedError ? (
        <p
          className="flex items-start gap-1 text-danger"
          data-testid={`tasks-inbox-item-error-${taskId}`}
        >
          <AlertCircle aria-hidden="true" className="mt-0.5 size-3 shrink-0" />
          <span className="min-w-0 truncate">{failedError}</span>
        </p>
      ) : null}

      {item.approval_policy === "manual" && item.approval_state ? (
        <p data-testid={`tasks-inbox-item-approval-${taskId}`}>
          Approval state: {taskApprovalStateLabel(item.approval_state)}
        </p>
      ) : null}

      <p className="flex flex-wrap items-center gap-1.5 text-subtle">
        <span data-testid={`tasks-inbox-item-owner-${taskId}`}>{ownerLabel}</span>
        <span aria-hidden="true" className="text-faint opacity-60">
          ·
        </span>
        <span>{lastActivity} ago</span>
      </p>
    </>
  );

  const actions = (
    <InboxItemActions
      isApprovalItem={isApprovalItem}
      isArchived={isArchived}
      isFailedRun={isFailedRun}
      onApprove={onApprove}
      onArchive={onArchive}
      onDismiss={onDismiss}
      onMarkRead={onMarkRead}
      onReject={onReject}
      onRetry={onRetry}
      pendingApproveIds={pendingApproveIds}
      pendingArchiveIds={pendingArchiveIds}
      pendingDismissIds={pendingDismissIds}
      pendingMarkReadIds={pendingMarkReadIds}
      pendingRejectIds={pendingRejectIds}
      pendingRetryIds={pendingRetryIds}
      runId={run?.id}
      taskId={taskId}
      unread={unread}
    />
  );

  return (
    <TasksInboxRow
      actions={actions}
      data-lane={lane}
      detail={detail}
      group={group}
      onSelect={handleSelect}
      taskId={taskId}
      top={top}
      unread={unread}
    />
  );
}

interface InboxItemActionsProps {
  taskId: string;
  runId?: string;
  unread: boolean;
  isApprovalItem: boolean;
  isFailedRun: boolean;
  isArchived: boolean;
  onApprove?: (taskId: string) => void;
  onReject?: (taskId: string) => void;
  onRetry?: (runId: string) => void;
  onArchive?: (taskId: string) => void;
  onDismiss?: (taskId: string) => void;
  onMarkRead?: (taskId: string) => void;
  pendingApproveIds?: ReadonlySet<string>;
  pendingRejectIds?: ReadonlySet<string>;
  pendingRetryIds?: ReadonlySet<string>;
  pendingArchiveIds?: ReadonlySet<string>;
  pendingDismissIds?: ReadonlySet<string>;
  pendingMarkReadIds?: ReadonlySet<string>;
}

function InboxItemActions({
  taskId,
  runId,
  unread,
  isApprovalItem,
  isFailedRun,
  isArchived,
  onApprove,
  onReject,
  onRetry,
  onArchive,
  onDismiss,
  onMarkRead,
  pendingApproveIds,
  pendingRejectIds,
  pendingRetryIds,
  pendingArchiveIds,
  pendingDismissIds,
  pendingMarkReadIds,
}: InboxItemActionsProps) {
  return (
    <>
      {isApprovalItem && onReject ? (
        <ActionButton
          icon={<X />}
          label="Reject"
          onClick={() => onReject(taskId)}
          pending={pendingRejectIds?.has(taskId) ?? false}
          testId={`tasks-inbox-item-reject-${taskId}`}
          variant="destructive-ghost"
        />
      ) : null}
      {isApprovalItem && onApprove ? (
        <ActionButton
          icon={<Check />}
          label="Approve"
          onClick={() => onApprove(taskId)}
          pending={pendingApproveIds?.has(taskId) ?? false}
          testId={`tasks-inbox-item-approve-${taskId}`}
          variant="primary"
        />
      ) : null}
      {isFailedRun && onDismiss ? (
        <ActionButton
          icon={<ArchiveX />}
          label="Dismiss"
          onClick={() => onDismiss(taskId)}
          pending={pendingDismissIds?.has(taskId) ?? false}
          testId={`tasks-inbox-item-dismiss-${taskId}`}
          variant="ghost"
        />
      ) : null}
      {isFailedRun && onRetry && runId ? (
        <ActionButton
          icon={<RotateCcw />}
          label="Retry"
          onClick={() => onRetry(runId)}
          pending={pendingRetryIds?.has(runId) ?? false}
          testId={`tasks-inbox-item-retry-${taskId}`}
          variant="ghost"
        />
      ) : null}
      {!isApprovalItem && !isFailedRun && onMarkRead && unread ? (
        <ActionButton
          icon={<Eye />}
          label="Mark read"
          onClick={() => onMarkRead(taskId)}
          pending={pendingMarkReadIds?.has(taskId) ?? false}
          testId={`tasks-inbox-item-mark-read-${taskId}`}
          variant="ghost"
        />
      ) : null}
      {!isArchived && onArchive ? (
        <ActionButton
          icon={<Archive />}
          label="Archive"
          onClick={() => onArchive(taskId)}
          pending={pendingArchiveIds?.has(taskId) ?? false}
          testId={`tasks-inbox-item-archive-${taskId}`}
          variant="ghost"
        />
      ) : null}
      <Button
        data-testid={`tasks-inbox-item-open-${taskId}`}
        nativeButton={false}
        render={<Link params={{ id: taskId }} to="/tasks/$id" />}
        size="xs"
        variant="ghost"
      >
        Open
      </Button>
    </>
  );
}

interface ActionButtonProps {
  label: string;
  icon: ReactNode;
  onClick: () => void;
  pending: boolean;
  testId: string;
  /**
   * `primary` -- solid accent CTA (max one per card).
   * `ghost` -- neutral secondary action.
   * `destructive-ghost` -- ghost with `text-danger`. Solid-filled destructive
   * buttons only belong inside a confirmation dialog, not inline on a row.
   */
  variant: "primary" | "ghost" | "destructive-ghost";
}

function ActionButton({ label, icon, onClick, pending, testId, variant }: ActionButtonProps) {
  const buttonVariant = variant === "primary" ? "default" : "ghost";
  return (
    <Button
      aria-busy={pending}
      aria-label={label}
      data-testid={testId}
      data-variant={variant}
      disabled={pending}
      onClick={onClick}
      size="xs"
      type="button"
      variant={buttonVariant}
      className={cn(variant === "destructive-ghost" && "text-danger hover:text-danger")}
    >
      {icon}
      {label}
    </Button>
  );
}
