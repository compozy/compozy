import { Link } from "@tanstack/react-router";
import { ArrowUpRight, LifeBuoy, RotateCw } from "lucide-react";

import {
  Button,
  cn,
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
  Pill,
  Spinner,
  TopbarOverflowIcon,
} from "@compozy/ui";

import {
  HEAD_STATUS_TONE_TEXT,
  taskHandoffActionCopy,
  taskStatusLabel,
  taskStatusSignal,
} from "../lib/task-formatters";
import type { TaskCommandState, TaskPrimaryCommand } from "../lib/task-command-state";
import type { TaskStatus } from "../types";

/** Single source of task status in the window head (w2-status contract). */
export function TaskPageStatus({ status }: { status?: TaskStatus | null }) {
  const signal = taskStatusSignal(status);
  return (
    <span
      className={cn(
        "inline-flex items-center gap-1.5 text-badge font-semibold",
        HEAD_STATUS_TONE_TEXT[signal.tone]
      )}
      data-testid="tasks-detail-status"
    >
      <Pill.Dot tone={signal.tone} pulse={signal.pulse} />
      {taskStatusLabel(status)}
    </span>
  );
}

export interface TaskPageActionHandlers {
  onPublish: () => void;
  onApprove: () => void;
  onStartRun: () => void;
  onOpenRun: (runId: string) => void;
  onResume: () => void;
  onRecover: () => void;
  onRetry: (runId: string) => void;
  onEdit: () => void;
  onPause: () => void;
  onReject: () => void;
}

export interface TaskPageActionsProps {
  command: TaskCommandState;
  handlers: TaskPageActionHandlers;
  pending?: {
    approve?: boolean;
    publish?: boolean;
    recover?: boolean;
    reject?: boolean;
    resume?: boolean;
    retry?: boolean;
    start?: boolean;
  };
}

const PRIMARY_LABEL: Record<NonNullable<TaskCommandState["primary"]>["kind"], string> = {
  publish: "Publish",
  approve: "Approve",
  start: "Start run",
  open_run: "Open run",
  resume: "Resume",
  recover: "Try again",
  retry: "Retry",
};

const PRIMARY_PENDING_LABEL: Record<
  Exclude<NonNullable<TaskCommandState["primary"]>["kind"], "open_run">,
  string
> = {
  approve: "Approving…",
  publish: "Publishing…",
  recover: "Restarting…",
  resume: "Resuming…",
  retry: "Retrying…",
  start: "Starting…",
};

function primaryActionTitle(primary: TaskPrimaryCommand): string | undefined {
  switch (primary.kind) {
    case "publish":
    case "approve":
    case "start":
    case "retry":
      return taskHandoffActionCopy(primary.kind).tooltip;
    case "open_run":
    case "resume":
    case "recover":
      return undefined;
  }
}

function runPrimaryCommand(primary: TaskPrimaryCommand, handlers: TaskPageActionHandlers) {
  switch (primary.kind) {
    case "publish":
      return handlers.onPublish();
    case "approve":
      return handlers.onApprove();
    case "start":
      return handlers.onStartRun();
    case "open_run":
      return handlers.onOpenRun(primary.runId);
    case "resume":
      return handlers.onResume();
    case "recover":
      return handlers.onRecover();
    case "retry":
      return handlers.onRetry(primary.runId);
  }
}

/**
 * The one accent target in the head, driven by the task command state machine.
 *
 * @see docs/design/opendesign/tasks/TASK-DETAILS-REDESIGN-PLAN.md §6
 */
export function TaskPageActions({ command, handlers, pending = {} }: TaskPageActionsProps) {
  const { primary, secondary } = command;
  if (!primary && !secondary.edit && !secondary.pause && !secondary.reject) {
    return null;
  }
  const anyPending = Object.values(pending).some(Boolean);

  return (
    <div className="flex items-center gap-1.5">
      {secondary.edit ? (
        <Button
          className="min-h-6"
          data-testid="tasks-detail-edit-button"
          disabled={anyPending}
          onClick={handlers.onEdit}
          size="sm"
          type="button"
          variant="neutral"
        >
          Edit
        </Button>
      ) : null}
      {secondary.pause ? (
        <Button
          className="min-h-6"
          disabled={anyPending}
          onClick={handlers.onPause}
          size="sm"
          type="button"
          variant="neutral"
        >
          Pause
        </Button>
      ) : null}
      {secondary.reject ? (
        <Button
          aria-busy={pending.reject || undefined}
          className="min-h-6"
          data-testid="tasks-detail-reject-button"
          disabled={anyPending}
          onClick={handlers.onReject}
          size="sm"
          type="button"
          variant="neutral"
        >
          {pending.reject ? <Spinner aria-hidden="true" className="size-3" /> : null}
          {pending.reject ? "Rejecting…" : "Reject"}
        </Button>
      ) : null}
      {primary ? (
        <TaskPrimaryActionButton
          disabled={anyPending}
          onClick={() => runPrimaryCommand(primary, handlers)}
          pending={primary.kind !== "open_run" && Boolean(pending[primary.kind])}
          primary={primary}
        />
      ) : null}
    </div>
  );
}

function TaskPrimaryActionButton({
  primary,
  pending,
  disabled,
  onClick,
}: {
  primary: TaskPrimaryCommand;
  pending: boolean;
  disabled: boolean;
  onClick: () => void;
}) {
  const label =
    pending && primary.kind !== "open_run"
      ? PRIMARY_PENDING_LABEL[primary.kind]
      : PRIMARY_LABEL[primary.kind];
  return (
    <Button
      aria-busy={pending || undefined}
      className="min-h-6"
      data-testid={`tasks-detail-primary-${primary.kind}`}
      disabled={disabled}
      onClick={onClick}
      size="sm"
      title={primaryActionTitle(primary)}
      type="button"
    >
      {pending ? <Spinner aria-hidden="true" className="size-3" /> : null}
      {!pending && primary.kind === "recover" ? (
        <LifeBuoy aria-hidden="true" className="size-3" />
      ) : null}
      {!pending && primary.kind === "retry" ? (
        <RotateCw aria-hidden="true" className="size-3" />
      ) : null}
      {label}
      {primary.kind === "open_run" ? <ArrowUpRight aria-hidden="true" className="size-3" /> : null}
    </Button>
  );
}

export interface TaskPageOverflowProps {
  taskId: string;
  command: TaskCommandState;
  pending: {
    cancel?: boolean;
    pause?: boolean;
    resume?: boolean;
    enqueue?: boolean;
  };
  showFanOut: boolean;
  onPause: () => void;
  onResume: () => void;
  onCancel: () => void;
  onStartNewRun: () => void;
  onFanOut: () => void;
  onCopyId: () => void;
  onDelete: () => void;
}

/** Low-frequency verbs behind the head `⋯` trigger (destructive last). */
export function TaskPageOverflow({
  taskId,
  command,
  pending,
  showFanOut,
  onPause,
  onResume,
  onCancel,
  onStartNewRun,
  onFanOut,
  onCopyId,
  onDelete,
}: TaskPageOverflowProps) {
  const { overflow } = command;
  return (
    <DropdownMenu>
      <DropdownMenuTrigger
        aria-label="More actions"
        data-testid="tasks-detail-overflow"
        render={<Button className="size-6" type="button" variant="ghost" size="icon-sm" />}
      >
        <TopbarOverflowIcon aria-hidden="true" className="size-3" />
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" data-testid="tasks-detail-overflow-menu">
        {overflow.edit ? (
          <DropdownMenuItem
            aria-busy={pending.pause || undefined}
            data-testid="tasks-detail-edit"
            render={<Link params={{ id: taskId }} to="/tasks/$id/edit" />}
          >
            Edit
          </DropdownMenuItem>
        ) : null}
        {overflow.pause ? (
          <TaskOverflowPendingItem
            label="Pause"
            onClick={onPause}
            pending={pending.pause}
            pendingLabel="Pausing…"
            testId="tasks-detail-pause"
          />
        ) : null}
        {overflow.resume ? (
          <TaskOverflowPendingItem
            announceBusy
            label="Resume"
            onClick={onResume}
            pending={pending.resume}
            pendingLabel="Resuming…"
            testId="tasks-detail-resume"
          />
        ) : null}
        {overflow.cancel ? (
          <TaskOverflowPendingItem
            announceBusy
            label="Cancel task"
            onClick={onCancel}
            pending={pending.cancel}
            pendingLabel="Canceling…"
            testId="tasks-detail-cancel"
          />
        ) : null}
        {overflow.startNewRun ? (
          <TaskOverflowPendingItem
            announceBusy
            label="Start new run"
            onClick={onStartNewRun}
            pending={pending.enqueue}
            pendingLabel="Starting…"
            testId="tasks-detail-start-new-run"
          />
        ) : null}
        {showFanOut ? (
          <DropdownMenuItem data-testid="tasks-detail-fan-out" onClick={onFanOut}>
            Run in parallel…
          </DropdownMenuItem>
        ) : null}
        <DropdownMenuItem data-testid="tasks-detail-copy-id" onClick={onCopyId}>
          Copy task id
        </DropdownMenuItem>
        {overflow.delete ? (
          <>
            <DropdownMenuSeparator />
            <DropdownMenuItem
              data-testid="tasks-detail-delete"
              onClick={onDelete}
              variant="destructive"
            >
              Delete task…
            </DropdownMenuItem>
          </>
        ) : null}
      </DropdownMenuContent>
    </DropdownMenu>
  );
}

function TaskOverflowPendingItem({
  testId,
  label,
  pendingLabel,
  pending,
  announceBusy = false,
  onClick,
}: {
  testId: string;
  label: string;
  pendingLabel: string;
  pending?: boolean;
  /** Mirrors `pending` into `aria-busy`. */
  announceBusy?: boolean;
  onClick: () => void;
}) {
  return (
    <DropdownMenuItem
      aria-busy={(announceBusy && pending) || undefined}
      data-testid={testId}
      disabled={pending}
      onClick={onClick}
    >
      {pending ? <Spinner aria-hidden="true" className="size-3" /> : null}
      {pending ? pendingLabel : label}
    </DropdownMenuItem>
  );
}
