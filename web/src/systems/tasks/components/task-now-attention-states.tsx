import { Button, Time } from "@agh/ui";

import { taskRunCanRecover } from "../lib/task-run-recovery";
import type { TaskDetailView } from "../types";
import type { TaskNowPendingState, TaskNowStripHandlers } from "./task-now-contract";
import { TaskStateBand } from "./task-state-band";

type BlockedReason = NonNullable<TaskDetailView["task"]["blocked_reasons"]>[number];

interface TaskNowAttentionStatesProps {
  detail: TaskDetailView;
  handlers: TaskNowStripHandlers;
  pending: TaskNowPendingState;
  needsAttention: boolean;
}

function dependencyTitle(
  detail: TaskDetailView,
  ids: readonly string[] | undefined
): string | null {
  const first = ids?.[0];
  if (!first) return null;
  const ref = detail.dependency_references?.find(entry => entry.depends_on.id === first);
  const dependency = ref?.depends_on;
  if (!dependency) return null;
  return dependency.title ?? dependency.identifier ?? dependency.id;
}

function blockerKey(reason: BlockedReason, index: number): string {
  return reason.block_id ?? reason.depends_on_task_ids?.[0] ?? `${reason.source}-${index}`;
}

function TaskApprovalState({
  handlers,
  pending,
}: Pick<TaskNowAttentionStatesProps, "handlers" | "pending">) {
  const approvalPending = Boolean(pending.approve || pending.reject);
  return (
    <TaskStateBand
      actions={
        <>
          <Button
            className="min-h-6"
            data-testid="tasks-detail-now-reject"
            disabled={approvalPending}
            onClick={handlers.onReject}
            size="sm"
            type="button"
            variant="ghost"
          >
            Reject
          </Button>
          <Button
            className="min-h-6"
            data-testid="tasks-detail-now-approve"
            disabled={approvalPending}
            onClick={handlers.onApprove}
            size="sm"
            type="button"
            variant="neutral"
          >
            Approve
          </Button>
        </>
      }
      body="A manual check is required before this task can start."
      data-testid="tasks-detail-now-approval"
      title="Waiting for your approval"
      tone="info"
    />
  );
}

function TaskBlockingState({
  detail,
  handlers,
  index,
  pending,
  reason,
}: Pick<TaskNowAttentionStatesProps, "detail" | "handlers" | "pending"> & {
  index: number;
  reason: BlockedReason;
}) {
  const key = blockerKey(reason, index);

  if (reason.source === "dependency") {
    const dependencyId = reason.depends_on_task_ids?.[0];
    const title = dependencyTitle(detail, reason.depends_on_task_ids);
    return (
      <TaskStateBand
        actions={
          dependencyId ? (
            <Button
              className="min-h-6"
              data-testid={`tasks-detail-now-open-blocking-${key}`}
              onClick={() => handlers.onOpenTask(dependencyId)}
              size="sm"
              type="button"
              variant="neutral"
            >
              Open blocking task
            </Button>
          ) : undefined
        }
        body={
          title
            ? `“${title}” has to finish first.`
            : "Another task has to finish before this one can start."
        }
        data-testid={`tasks-detail-now-dependency-${key}`}
        title="Waits on another task"
        tone="warning"
      />
    );
  }

  if (reason.source === "paused") {
    return (
      <TaskStateBand
        actions={
          <Button
            className="min-h-6"
            data-testid="tasks-detail-now-resume"
            disabled={pending.resume}
            onClick={handlers.onResume}
            size="sm"
            type="button"
            variant="neutral"
          >
            Resume
          </Button>
        }
        body={detail.task.paused_reason || "New runs stay queued until the task is resumed."}
        data-testid="tasks-detail-now-paused"
        title="Paused"
        tone="warning"
      />
    );
  }

  const blockId = reason.block_id;
  return (
    <TaskStateBand
      actions={
        blockId ? (
          <Button
            className="min-h-6"
            data-testid={`tasks-detail-now-clear-block-${key}`}
            disabled={pending.clearBlock}
            onClick={() => handlers.onClearBlock(blockId)}
            size="sm"
            type="button"
            variant="neutral"
          >
            Clear block
          </Button>
        ) : undefined
      }
      body={reason.reason || "A block is holding this task."}
      data-testid={`tasks-detail-now-block-${key}`}
      title={reason.kind === "needs_input" ? "Needs input to continue" : "Blocked"}
      tone="warning"
    />
  );
}

function TaskNeedsAttentionState({
  detail,
  handlers,
  pending,
}: Pick<TaskNowAttentionStatesProps, "detail" | "handlers" | "pending">) {
  const record = detail.task;
  const activeRun = detail.summary?.active_run ?? null;
  const heartbeat = activeRun?.heartbeat_at;
  const canRecover = activeRun
    ? taskRunCanRecover(activeRun, record.max_attempts)
    : record.status === "needs_attention";

  return (
    <TaskStateBand
      actions={
        canRecover ? (
          <Button
            className="min-h-6"
            data-testid="tasks-detail-now-recover"
            disabled={pending.recover}
            onClick={handlers.onRecover}
            size="sm"
            type="button"
            variant="neutral"
          >
            Recover
          </Button>
        ) : undefined
      }
      body={
        <>
          {record.needs_attention_reason?.trim() || "This attempt requires operator attention."}
          {heartbeat ? (
            <>
              {" "}
              Last heartbeat <Time iso={heartbeat} mode="relative" />. Recover requeues the work as
              a fresh attempt; nothing done so far is lost.
            </>
          ) : (
            <> Recover requeues the work as a fresh attempt; nothing done so far is lost.</>
          )}
        </>
      }
      data-testid="tasks-detail-now-stuck"
      micro={activeRun ? `task_run_stuck · ${activeRun.id}` : undefined}
      title="This attempt needs attention"
      tone="danger"
    />
  );
}

/** Approval, blocker, and recovery renderers for the task Overview Now strip. */
export function TaskNowAttentionStates({
  detail,
  handlers,
  pending,
  needsAttention,
}: TaskNowAttentionStatesProps) {
  return (
    <>
      {detail.task.approval_state === "pending" ? (
        <TaskApprovalState handlers={handlers} pending={pending} />
      ) : null}
      {(detail.task.blocked_reasons ?? []).map((reason, index) =>
        reason.source === "approval" ? null : (
          <TaskBlockingState
            detail={detail}
            handlers={handlers}
            index={index}
            key={blockerKey(reason, index)}
            pending={pending}
            reason={reason}
          />
        )
      )}
      {needsAttention ? (
        <TaskNeedsAttentionState detail={detail} handlers={handlers} pending={pending} />
      ) : null}
    </>
  );
}
