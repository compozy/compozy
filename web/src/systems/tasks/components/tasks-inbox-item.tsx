import { AlertCircle, Archive, ArchiveX, Check, Eye, RotateCcw, X } from "lucide-react";
import { Link } from "@tanstack/react-router";
import type { ReactNode } from "react";

import {
  Button,
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
  type StateGlyphState,
  Time,
  TopbarOverflowIcon,
} from "@compozy/ui";

import { cn } from "@/lib/utils";

import { inboxBlockingReasonLabel, type InboxGroupId } from "../lib/inbox-grouping";
import { taskStateGlyph, taskStatusLabel } from "../lib/task-formatters";
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
  const ownerLabel = task.owner?.ref ?? "Unassigned";
  const blockingReason = inboxBlockingReasonLabel(item.blocking_reason);

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
      <span className="shrink-0 text-small-body text-muted" data-slot="tasks-inbox-row-status">
        {taskStatusLabel(task.status)}
      </span>
    </>
  );

  const detail = (
    <>
      {blockingReason ? (
        <p data-testid={`tasks-inbox-item-blocking-${taskId}`}>{blockingReason}</p>
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

      <p className="flex flex-wrap items-center gap-1.5 text-subtle">
        <span data-testid={`tasks-inbox-item-owner-${taskId}`}>{ownerLabel}</span>
        <span aria-hidden="true" className="text-faint opacity-60">
          ·
        </span>
        {item.latest_activity_at ? <Time iso={item.latest_activity_at} mode="relative" /> : null}
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
      state={inboxItemState(item)}
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
  // Triage verbs only apply to their lane; a gated-off handler hides its item.
  const markRead = !isApprovalItem && !isFailedRun && unread ? onMarkRead : undefined;
  const dismiss = isFailedRun ? onDismiss : undefined;
  const archive = isArchived ? undefined : onArchive;
  return (
    <>
      {isApprovalItem ? (
        <InboxApprovalActions
          onApprove={onApprove}
          onReject={onReject}
          pendingApproveIds={pendingApproveIds}
          pendingRejectIds={pendingRejectIds}
          taskId={taskId}
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
      <Button
        data-testid={`tasks-inbox-item-open-${taskId}`}
        nativeButton={false}
        render={<Link params={{ id: taskId }} to="/tasks/$id" />}
        size="xs"
        variant="ghost"
      >
        Open
      </Button>
      <InboxItemOverflowMenu
        onArchive={archive}
        onDismiss={dismiss}
        onMarkRead={markRead}
        pendingArchiveIds={pendingArchiveIds}
        pendingDismissIds={pendingDismissIds}
        pendingMarkReadIds={pendingMarkReadIds}
        taskId={taskId}
      />
    </>
  );
}

function InboxApprovalActions({
  taskId,
  onApprove,
  onReject,
  pendingApproveIds,
  pendingRejectIds,
}: Pick<
  InboxItemActionsProps,
  "taskId" | "onApprove" | "onReject" | "pendingApproveIds" | "pendingRejectIds"
>) {
  return (
    <>
      {onReject ? (
        <ActionButton
          icon={<X />}
          label="Reject"
          onClick={() => onReject(taskId)}
          pending={pendingRejectIds?.has(taskId) ?? false}
          testId={`tasks-inbox-item-reject-${taskId}`}
          variant="destructive-ghost"
        />
      ) : null}
      {onApprove ? (
        <ActionButton
          icon={<Check />}
          label="Approve"
          onClick={() => onApprove(taskId)}
          pending={pendingApproveIds?.has(taskId) ?? false}
          testId={`tasks-inbox-item-approve-${taskId}`}
          variant="primary"
        />
      ) : null}
    </>
  );
}

function InboxItemOverflowMenu({
  taskId,
  onMarkRead,
  onDismiss,
  onArchive,
  pendingMarkReadIds,
  pendingDismissIds,
  pendingArchiveIds,
}: Pick<
  InboxItemActionsProps,
  | "taskId"
  | "onMarkRead"
  | "onDismiss"
  | "onArchive"
  | "pendingMarkReadIds"
  | "pendingDismissIds"
  | "pendingArchiveIds"
>) {
  if (!onMarkRead && !onDismiss && !onArchive) return null;
  return (
    <DropdownMenu>
      <DropdownMenuTrigger
        aria-label="More inbox actions"
        data-testid={`tasks-inbox-item-more-${taskId}`}
        render={<Button size="icon-xs" type="button" variant="ghost" />}
      >
        <TopbarOverflowIcon aria-hidden="true" className="size-3" />
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end">
        {onMarkRead ? (
          <DropdownMenuItem
            data-testid={`tasks-inbox-item-mark-read-${taskId}`}
            disabled={pendingMarkReadIds?.has(taskId) ?? false}
            onClick={() => onMarkRead(taskId)}
          >
            <Eye aria-hidden="true" className="size-3" />
            Mark read
          </DropdownMenuItem>
        ) : null}
        {onDismiss ? (
          <DropdownMenuItem
            data-testid={`tasks-inbox-item-dismiss-${taskId}`}
            disabled={pendingDismissIds?.has(taskId) ?? false}
            onClick={() => onDismiss(taskId)}
          >
            <ArchiveX aria-hidden="true" className="size-3" />
            Dismiss
          </DropdownMenuItem>
        ) : null}
        {onArchive ? (
          <DropdownMenuItem
            data-testid={`tasks-inbox-item-archive-${taskId}`}
            disabled={pendingArchiveIds?.has(taskId) ?? false}
            onClick={() => onArchive(taskId)}
          >
            <Archive aria-hidden="true" className="size-3" />
            Archive
          </DropdownMenuItem>
        ) : null}
      </DropdownMenuContent>
    </DropdownMenu>
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

/**
 * The inbox row's leading state: a failed run reads as failed and an approval
 * waits on a person; everything else follows the task status.
 */
function inboxItemState(item: TaskInboxItem): StateGlyphState {
  if (item.lane === "failed_runs") return "failed";
  if (item.lane === "approvals") return "attention";
  return taskStateGlyph(item.task.status);
}
