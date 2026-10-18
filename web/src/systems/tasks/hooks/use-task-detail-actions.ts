import {
  useApproveTask,
  useCancelTask,
  useClearTaskBlock,
  useEnqueueTaskRun,
  useFanOutTaskRuns,
  usePauseTask,
  usePublishTask,
  useRecoverTask,
  useRejectTask,
  useResumeTask,
} from "./use-task-actions";
import { useRetryTaskRun } from "./use-task-run-actions";
import { useRecoverTaskRun } from "./use-task-run-recovery";
import { taskRunCanRecover } from "../lib/task-run-recovery";
import { notifyTaskMutation, submitTaskMutation } from "../lib/task-mutation";
import type { FanOutTaskRunsRequest, TaskRunStatus } from "../types";

interface TaskDetailActiveRun {
  id: string;
  attempt: number;
  status?: TaskRunStatus | null;
}

function fanOutSuccessMessage(result: { runs?: readonly unknown[] | null } | undefined): string {
  const count = result?.runs?.length ?? 0;
  return count > 0 ? `Created ${count} run${count === 1 ? "" : "s"}.` : "Runs created.";
}

/** The run a Recover targets; null recovers the task itself. */
function recoverableRunIdFor(
  activeRun: TaskDetailActiveRun | null,
  maxAttempts: number | null | undefined
): string | null {
  return activeRun && taskRunCanRecover(activeRun, maxAttempts) ? activeRun.id : null;
}

/** Recovery verb: a stuck active run recovers in place, otherwise the task requeues. */
function useTaskDetailRecovery(
  taskId: string,
  activeRun: TaskDetailActiveRun | null,
  maxAttempts: number | null | undefined
) {
  const recoverTaskMutation = useRecoverTask();
  const recoverRunMutation = useRecoverTaskRun();
  const activeRunNeedsAttention = activeRun?.status === "needs_attention";
  const recoverableRunId = recoverableRunIdFor(activeRun, maxAttempts);
  const isRecoverPending = recoverTaskMutation.isPending || recoverRunMutation.isPending;

  const handleRecoverTask = async () => {
    if (!taskId || (activeRunNeedsAttention && !recoverableRunId) || isRecoverPending) {
      return;
    }

    const recoverAction: () => Promise<unknown> = recoverableRunId
      ? () => recoverRunMutation.mutateAsync({ runId: recoverableRunId, taskId })
      : () => recoverTaskMutation.mutateAsync({ id: taskId });
    await notifyTaskMutation(
      recoverAction,
      recoverableRunId ? "Run recovered." : "Task recovered.",
      "Failed to recover task"
    );
  };

  return { handleRecoverTask, isRecoverPending };
}

/** Task-level verbs for the detail page; every task verb is a no-op without a task id. */
function useTaskDetailActions(
  taskId: string,
  activeRun: TaskDetailActiveRun | null,
  maxAttempts: number | null | undefined
) {
  const hasTaskId = Boolean(taskId);
  const publishMutation = usePublishTask();
  const cancelMutation = useCancelTask();
  const enqueueMutation = useEnqueueTaskRun();
  const pauseMutation = usePauseTask();
  const resumeMutation = useResumeTask();
  const approveMutation = useApproveTask();
  const rejectMutation = useRejectTask();
  const retryRunMutation = useRetryTaskRun();
  const clearBlockMutation = useClearTaskBlock();
  const fanOutMutation = useFanOutTaskRuns();
  const recovery = useTaskDetailRecovery(taskId, activeRun, maxAttempts);

  const handlePublishTask = () =>
    hasTaskId
      ? notifyTaskMutation(
          () => publishMutation.mutateAsync({ id: taskId }),
          "Task published.",
          "Failed to publish task"
        )
      : Promise.resolve();

  const handleCancelTask = () =>
    hasTaskId
      ? notifyTaskMutation(
          () => cancelMutation.mutateAsync({ id: taskId }),
          "Task canceled.",
          "Failed to cancel task"
        )
      : Promise.resolve();

  const handleEnqueueRun = () =>
    hasTaskId
      ? notifyTaskMutation(
          () => enqueueMutation.mutateAsync({ id: taskId }),
          "Run queued.",
          "Failed to queue run"
        )
      : Promise.resolve();

  const handleApproveTask = () =>
    hasTaskId
      ? notifyTaskMutation(
          () => approveMutation.mutateAsync({ id: taskId }),
          "Task approved.",
          "Failed to approve task"
        )
      : Promise.resolve();

  const handleRejectTask = () =>
    hasTaskId
      ? notifyTaskMutation(
          () => rejectMutation.mutateAsync({ id: taskId }),
          "Task rejected.",
          "Failed to reject task"
        )
      : Promise.resolve();

  const handleRetryRun = (runId: string) =>
    notifyTaskMutation(
      () => retryRunMutation.mutateAsync({ runId }),
      "Retry queued.",
      "Failed to retry run"
    );

  const handleClearBlock = (blockId: string) =>
    hasTaskId
      ? notifyTaskMutation(
          () => clearBlockMutation.mutateAsync({ id: taskId, blockId }),
          "Block cleared.",
          "Failed to clear block"
        )
      : Promise.resolve();

  const handleFanOutRuns = async (data: FanOutTaskRunsRequest) => {
    if (!hasTaskId) return undefined;
    return submitTaskMutation(
      () => fanOutMutation.mutateAsync({ id: taskId, data }),
      fanOutSuccessMessage,
      "Failed to fan out runs"
    );
  };

  const handlePauseTask = async (reason: string) => {
    if (!hasTaskId) return;
    await submitTaskMutation(
      () => pauseMutation.mutateAsync({ id: taskId, data: { reason } }),
      "Task paused.",
      "Failed to pause task"
    );
  };

  const handleResumeTask = async () => {
    if (!hasTaskId) return;
    await notifyTaskMutation(
      () => resumeMutation.mutateAsync({ id: taskId }),
      "Task resumed.",
      "Failed to resume task"
    );
  };

  return {
    handleApproveTask,
    handleCancelTask,
    handleClearBlock,
    handleEnqueueRun,
    handleFanOutRuns,
    handlePauseTask,
    handlePublishTask,
    handleRecoverTask: recovery.handleRecoverTask,
    handleRejectTask,
    handleResumeTask,
    handleRetryRun,
    isApprovePending: approveMutation.isPending,
    isCancelPending: cancelMutation.isPending,
    isClearBlockPending: clearBlockMutation.isPending,
    isEnqueuePending: enqueueMutation.isPending,
    isFanOutPending: fanOutMutation.isPending,
    isPausePending: pauseMutation.isPending,
    isPublishPending: publishMutation.isPending,
    isRecoverPending: recovery.isRecoverPending,
    isRejectPending: rejectMutation.isPending,
    isResumePending: resumeMutation.isPending,
    isRetryPending: retryRunMutation.isPending,
  };
}

export { useTaskDetailActions };
