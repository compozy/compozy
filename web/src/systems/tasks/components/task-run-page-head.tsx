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
  TopbarOverflowIcon,
} from "@agh/ui";

import {
  HEAD_STATUS_TONE_TEXT,
  taskRunStatusLabel,
  taskRunStatusTone,
} from "../lib/task-formatters";
import type { TaskRunDetailView, TaskRunStatus } from "../types";

const CANCELABLE_STATUSES: ReadonlySet<TaskRunStatus> = new Set([
  "queued",
  "claimed",
  "starting",
  "running",
]);

export function TaskRunPageStatus({ status }: { status: TaskRunStatus }) {
  const isActive = status === "running" || status === "starting";
  const tone = taskRunStatusTone(status);
  return (
    <span
      className={cn(
        "inline-flex items-center gap-1.5 text-badge font-semibold",
        HEAD_STATUS_TONE_TEXT[tone]
      )}
      data-testid="tasks-run-status"
    >
      <Pill.Dot pulse={isActive} tone={tone} />
      {taskRunStatusLabel(status)}
    </span>
  );
}

export interface TaskRunPageActionsProps {
  run: TaskRunDetailView;
  maxAttempts?: number | null;
  isRetryPending?: boolean;
  onOpenSession: (sessionId: string) => void;
  onRetry: () => void;
}

/** Opens the live session or retries a failed run when another attempt is available. */
export function TaskRunPageActions({
  run,
  maxAttempts,
  isRetryPending = false,
  onOpenSession,
  onRetry,
}: TaskRunPageActionsProps) {
  const record = run.run;
  const sessionId = record.session_id ?? run.session?.session_id ?? null;
  const attemptsRemain =
    maxAttempts !== undefined && (maxAttempts === null || record.attempt < maxAttempts);

  if (record.status === "failed" && attemptsRemain) {
    return (
      <Button
        data-testid="tasks-run-retry"
        disabled={isRetryPending}
        onClick={onRetry}
        size="sm"
        type="button"
      >
        <RotateCw aria-hidden="true" className="size-3" />
        Retry
      </Button>
    );
  }

  if (!sessionId) return null;

  const isTerminal =
    record.status === "completed" || record.status === "failed" || record.status === "canceled";

  return (
    <Button
      data-testid="tasks-run-open-session"
      onClick={() => onOpenSession(sessionId)}
      size="sm"
      type="button"
      variant={isTerminal ? "neutral" : "default"}
    >
      Open session
      <ArrowUpRight aria-hidden="true" className="size-3" />
    </Button>
  );
}

export interface TaskRunPageOverflowProps {
  run: TaskRunDetailView;
  canRecover: boolean;
  pending: {
    cancel?: boolean;
    release?: boolean;
    recover?: boolean;
  };
  onCancel: () => void;
  onRelease: () => void;
  onRecover: () => void;
  onForceFail: () => void;
  onCopyRunId: () => void;
}

/** Operator verbs behind the run head `⋯` (no delete — runs terminalize). */
export function TaskRunPageOverflow({
  run,
  canRecover,
  pending,
  onCancel,
  onRelease,
  onRecover,
  onForceFail,
  onCopyRunId,
}: TaskRunPageOverflowProps) {
  const status = run.run.status;
  const isCancelable = CANCELABLE_STATUSES.has(status);
  const canRelease = status === "claimed";
  const canForceFail = status === "queued" || status === "claimed";

  return (
    <DropdownMenu>
      <DropdownMenuTrigger
        aria-label="More actions"
        data-testid="tasks-run-overflow"
        render={<Button className="size-6" type="button" variant="ghost" size="icon-sm" />}
      >
        <TopbarOverflowIcon aria-hidden="true" className="size-3" />
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" data-testid="tasks-run-overflow-menu">
        {canRecover ? (
          <DropdownMenuItem
            data-testid="tasks-run-recover"
            disabled={pending.recover}
            onClick={onRecover}
          >
            <LifeBuoy aria-hidden="true" className="size-3" />
            Recover
          </DropdownMenuItem>
        ) : null}
        {canRelease ? (
          <DropdownMenuItem
            data-testid="tasks-run-release"
            disabled={pending.release}
            onClick={onRelease}
          >
            Release claim
          </DropdownMenuItem>
        ) : null}
        {isCancelable ? (
          <DropdownMenuItem
            data-testid="tasks-run-cancel"
            disabled={pending.cancel}
            onClick={onCancel}
          >
            Cancel run
          </DropdownMenuItem>
        ) : null}
        <DropdownMenuItem data-testid="tasks-run-copy-id" onClick={onCopyRunId}>
          Copy run id
        </DropdownMenuItem>
        {canForceFail ? (
          <>
            <DropdownMenuSeparator />
            <DropdownMenuItem
              data-testid="tasks-run-force-fail"
              onClick={onForceFail}
              variant="destructive"
            >
              Force fail…
            </DropdownMenuItem>
          </>
        ) : null}
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
