import type { ComponentPropsWithoutRef } from "react";

import { cn } from "@agh/ui";

import type { TaskDetailView, TaskRun } from "../types";
import { TaskNowActiveRun } from "./task-now-active-run";
import { TaskNowAttentionStates } from "./task-now-attention-states";
import type { TaskNowPendingState, TaskNowStripHandlers } from "./task-now-contract";
import { TaskNowTerminalState } from "./task-now-terminal-state";

export type { TaskNowPendingState, TaskNowStripHandlers } from "./task-now-contract";

export interface TaskNowStripProps extends ComponentPropsWithoutRef<"div"> {
  detail: TaskDetailView;
  runs: readonly TaskRun[];
  handlers: TaskNowStripHandlers;
  pending?: TaskNowPendingState;
}

const EMPTY_PENDING_STATE: TaskNowPendingState = {};

/**
 * Renders only while approval, blocking, an active run, or a terminal outcome
 * requires attention. Returns null for a quiet task.
 *
 * @see docs/design/opendesign/tasks/TASK-DETAILS-REDESIGN-PLAN.md §4.3
 */
export function TaskNowStrip({
  detail,
  runs,
  handlers,
  pending = EMPTY_PENDING_STATE,
  className,
  ...props
}: TaskNowStripProps) {
  const record = detail.task;
  const activeRun = detail.summary?.active_run ?? null;
  const needsAttention =
    record.status === "needs_attention" || activeRun?.status === "needs_attention";
  const isTerminal =
    record.status === "failed" || record.status === "completed" || record.status === "canceled";
  const hasBlockingState = (record.blocked_reasons ?? []).some(
    reason => reason.source !== "approval"
  );
  const isVisible =
    record.approval_state === "pending" ||
    hasBlockingState ||
    needsAttention ||
    Boolean(activeRun) ||
    isTerminal;

  if (!isVisible) return null;

  return (
    <div
      {...props}
      className={cn("flex flex-col gap-2.5", className)}
      data-testid="tasks-detail-now-strip"
    >
      <TaskNowAttentionStates
        detail={detail}
        handlers={handlers}
        needsAttention={needsAttention}
        pending={pending}
      />
      {activeRun && !needsAttention ? (
        <TaskNowActiveRun
          maxAttempts={record.max_attempts}
          onOpenRun={handlers.onOpenRun}
          run={activeRun}
        />
      ) : null}
      {isTerminal ? (
        <TaskNowTerminalState
          detail={detail}
          onOpenRun={handlers.onOpenRun}
          onViewResult={handlers.onViewResult}
          runs={runs}
        />
      ) : null}
    </div>
  );
}
