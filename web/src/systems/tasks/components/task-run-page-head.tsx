import { ArrowUpRight, LifeBuoy, RotateCw } from "lucide-react";

import {
  Button,
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
  Pill,
  StateGlyph,
  TopbarOverflowIcon,
} from "@compozy/ui";

import { taskRunStateGlyph, taskRunStatusLabel } from "../lib/task-formatters";
import type { TaskRunDetailView, TaskRunStatus } from "../types";

const CANCELABLE_STATUSES: ReadonlySet<TaskRunStatus> = new Set([
  "queued",
  "claimed",
  "starting",
  "running",
]);

/** Run status in the window head: a plain indicator (glyph + label), never a button-like pill. */
export function TaskRunPageStatus({ status }: { status: TaskRunStatus }) {
  return (
    <Pill data-state={taskRunStateGlyph(status)} data-testid="tasks-run-status" form="plain">
      <StateGlyph state={taskRunStateGlyph(status)} />
      {taskRunStatusLabel(status)}
    </Pill>
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
        variant="secondary"
      >
        <RotateCw aria-hidden="true" data-icon="inline-start" />
        Retry
      </Button>
    );
  }

  if (!sessionId) return null;

  // Window-head action: secondary at `sm`, never the inverted primary.
  return (
    <Button
      data-testid="tasks-run-open-session"
      onClick={() => onOpenSession(sessionId)}
      size="sm"
      type="button"
      variant="secondary"
    >
      Open session
      <ArrowUpRight aria-hidden="true" data-icon="inline-end" />
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
        render={<Button size="icon-sm" type="button" variant="quiet" />}
      >
        <TopbarOverflowIcon aria-hidden="true" className="size-4" />
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" data-testid="tasks-run-overflow-menu">
        {canRecover ? (
          <DropdownMenuItem
            data-testid="tasks-run-recover"
            disabled={pending.recover}
            onClick={onRecover}
          >
            <LifeBuoy aria-hidden="true" />
            Try again
          </DropdownMenuItem>
        ) : null}
        {canRelease ? (
          <DropdownMenuItem
            data-testid="tasks-run-release"
            disabled={pending.release}
            onClick={onRelease}
          >
            Let another agent take it
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
              Mark as failed…
            </DropdownMenuItem>
          </>
        ) : null}
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
