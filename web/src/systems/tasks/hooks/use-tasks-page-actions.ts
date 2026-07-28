import { useSelector, useStore } from "@xstate/store-react";

import { useApproveTask, useRejectTask } from "./use-task-actions";
import { useRetryTaskRun } from "./use-task-run-actions";
import { useArchiveTask, useDismissTask, useMarkTaskRead } from "./use-task-triage-actions";
import { tasksPageActionsLogic } from "./tasks-page-actions-store";

export function useTasksPageActions() {
  const retryMutation = useRetryTaskRun();
  const approveMutation = useApproveTask();
  const rejectMutation = useRejectTask();
  const markReadMutation = useMarkTaskRead();
  const archiveMutation = useArchiveTask();
  const dismissMutation = useDismissTask();
  const store = useStore(tasksPageActionsLogic);
  const pendingApprove = useSelector(store, snapshot => snapshot.context.pending.approve);
  const pendingArchive = useSelector(store, snapshot => snapshot.context.pending.archive);
  const pendingDismiss = useSelector(store, snapshot => snapshot.context.pending.dismiss);
  const pendingMarkRead = useSelector(store, snapshot => snapshot.context.pending.markRead);
  const pendingReject = useSelector(store, snapshot => snapshot.context.pending.reject);
  const pendingRetry = useSelector(store, snapshot => snapshot.context.pending.retry);

  const handleApproveTask = (taskId: string) =>
    store.trigger.actionRequested({
      execute: () => approveMutation.mutateAsync({ id: taskId }),
      failureMessage: "Failed to approve task",
      id: taskId,
      kind: "approve",
      successMessage: "Task approved.",
    });
  const handleRejectTask = (taskId: string) =>
    store.trigger.actionRequested({
      execute: () => rejectMutation.mutateAsync({ id: taskId }),
      failureMessage: "Failed to reject task",
      id: taskId,
      kind: "reject",
      successMessage: "Task rejected.",
    });
  const handleMarkTaskRead = (taskId: string) =>
    store.trigger.actionRequested({
      execute: () => markReadMutation.mutateAsync({ id: taskId }),
      failureMessage: "Failed to mark task read",
      id: taskId,
      kind: "markRead",
    });
  const handleArchiveTask = (taskId: string) =>
    store.trigger.actionRequested({
      execute: () => archiveMutation.mutateAsync({ id: taskId }),
      failureMessage: "Failed to archive task",
      id: taskId,
      kind: "archive",
      successMessage: "Task archived.",
    });
  const handleDismissTask = (taskId: string) =>
    store.trigger.actionRequested({
      execute: () => dismissMutation.mutateAsync({ id: taskId }),
      failureMessage: "Failed to dismiss task",
      id: taskId,
      kind: "dismiss",
    });
  const handleRetryRun = (runId: string) =>
    store.trigger.actionRequested({
      execute: () => retryMutation.mutateAsync({ runId }),
      failureMessage: "Failed to retry run",
      id: runId,
      kind: "retry",
      successMessage: "Run retry requested.",
    });

  return {
    handleApproveTask,
    handleArchiveTask,
    handleDismissTask,
    handleMarkTaskRead,
    handleRejectTask,
    handleRetryRun,
    pendingApproveIds: new Set(Object.keys(pendingApprove)) as ReadonlySet<string>,
    pendingArchiveIds: new Set(Object.keys(pendingArchive)) as ReadonlySet<string>,
    pendingDismissIds: new Set(Object.keys(pendingDismiss)) as ReadonlySet<string>,
    pendingMarkReadIds: new Set(Object.keys(pendingMarkRead)) as ReadonlySet<string>,
    pendingRejectIds: new Set(Object.keys(pendingReject)) as ReadonlySet<string>,
    pendingRetryIds: new Set(Object.keys(pendingRetry)) as ReadonlySet<string>,
  };
}
