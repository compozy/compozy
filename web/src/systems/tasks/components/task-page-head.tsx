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
} from "@agh/ui";

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
  recover: "Recover",
  retry: "Retry",
};

const PRIMARY_PENDING_LABEL: Record<
  Exclude<NonNullable<TaskCommandState["primary"]>["kind"], "open_run">,
  string
> = {
  approve: "Approving…",
  publish: "Publishing…",
  recover: "Recovering…",
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

/**
 * The one accent target in the head, driven by the task command state machine.
 *
 * @see docs/design/opendesign/tasks/TASK-DETAILS-REDESIGN-PLAN.md §6
 */
export function TaskPageActions({ command, handlers, pending = {} }: TaskPageActionsProps) {
  const primary = command.primary;
  const primaryPending =
    primary?.kind === "open_run" ? false : Boolean(primary && pending[primary.kind]);
  const anyPending = Object.values(pending).some(Boolean);

  const onClick = () => {
    if (!primary) return;
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
  };

  if (
    !primary &&
    !command.secondary.edit &&
    !command.secondary.pause &&
    !command.secondary.reject
  ) {
    return null;
  }

  return (
    <div className="flex items-center gap-1.5">
      {command.secondary.edit ? (
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
      {command.secondary.pause ? (
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
      {command.secondary.reject ? (
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
        <Button
          aria-busy={primaryPending || undefined}
          className="min-h-6"
          data-testid={`tasks-detail-primary-${primary.kind}`}
          disabled={anyPending}
          onClick={onClick}
          size="sm"
          title={primaryActionTitle(primary)}
          type="button"
        >
          {primaryPending ? <Spinner aria-hidden="true" className="size-3" /> : null}
          {!primaryPending && primary.kind === "recover" ? (
            <LifeBuoy aria-hidden="true" className="size-3" />
          ) : null}
          {!primaryPending && primary.kind === "retry" ? (
            <RotateCw aria-hidden="true" className="size-3" />
          ) : null}
          {primaryPending && primary.kind !== "open_run"
            ? PRIMARY_PENDING_LABEL[primary.kind]
            : PRIMARY_LABEL[primary.kind]}
          {primary.kind === "open_run" ? (
            <ArrowUpRight aria-hidden="true" className="size-3" />
          ) : null}
        </Button>
      ) : null}
    </div>
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
          <DropdownMenuItem
            data-testid="tasks-detail-pause"
            disabled={pending.pause}
            onClick={onPause}
          >
            {pending.pause ? <Spinner aria-hidden="true" className="size-3" /> : null}
            {pending.pause ? "Pausing…" : "Pause"}
          </DropdownMenuItem>
        ) : null}
        {overflow.resume ? (
          <DropdownMenuItem
            aria-busy={pending.resume || undefined}
            data-testid="tasks-detail-resume"
            disabled={pending.resume}
            onClick={onResume}
          >
            {pending.resume ? <Spinner aria-hidden="true" className="size-3" /> : null}
            {pending.resume ? "Resuming…" : "Resume"}
          </DropdownMenuItem>
        ) : null}
        {overflow.cancel ? (
          <DropdownMenuItem
            aria-busy={pending.cancel || undefined}
            data-testid="tasks-detail-cancel"
            disabled={pending.cancel}
            onClick={onCancel}
          >
            {pending.cancel ? <Spinner aria-hidden="true" className="size-3" /> : null}
            {pending.cancel ? "Canceling…" : "Cancel task"}
          </DropdownMenuItem>
        ) : null}
        {overflow.startNewRun ? (
          <DropdownMenuItem
            aria-busy={pending.enqueue || undefined}
            data-testid="tasks-detail-start-new-run"
            disabled={pending.enqueue}
            onClick={onStartNewRun}
          >
            {pending.enqueue ? <Spinner aria-hidden="true" className="size-3" /> : null}
            {pending.enqueue ? "Starting…" : "Start new run"}
          </DropdownMenuItem>
        ) : null}
        {showFanOut ? (
          <DropdownMenuItem data-testid="tasks-detail-fan-out" onClick={onFanOut}>
            Fan out runs…
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
