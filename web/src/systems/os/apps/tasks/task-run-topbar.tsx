import { SkeletonText, useTopbarSlot } from "@compozy/ui";

import type { TaskRunLocationController } from "./use-task-run-location";
import { TaskRunPageActions, TaskRunPageOverflow, TaskRunPageStatus } from "@/systems/tasks";

/** Publishes the run-detail head (crumbs · run status · actions) into the window topbar. */
export function TaskRunTopbar({ controller }: { controller: TaskRunLocationController }) {
  const { page, record } = controller;

  useTopbarSlot(
    page.run && record
      ? {
          onBack: controller.backToTask,
          crumbs: [
            { id: "tasks", label: "Tasks", onSelect: controller.backToTasks },
            {
              id: "task",
              label: (
                <span data-testid="tasks-run-breadcrumb-task">
                  {page.task?.task.title ?? page.run.task?.identifier ?? controller.taskId}
                </span>
              ),
              onSelect: controller.backToTask,
            },
          ],
          crumb: <span data-testid="tasks-run-title">Attempt {record.attempt}</span>,
          status: <TaskRunPageStatus status={record.status} />,
          actions: (
            <TaskRunPageActions
              isRetryPending={page.isRetryPending}
              maxAttempts={page.task?.task.max_attempts}
              onOpenSession={controller.openSession}
              onRetry={() => void page.handleRetryRun()}
              run={page.run}
            />
          ),
          overflow: (
            <TaskRunPageOverflow
              canRecover={controller.canRecover}
              onCancel={() => void page.handleCancelRun()}
              onCopyRunId={controller.copyRunId}
              onForceFail={controller.forceFailDialog.open}
              onRecover={() => void page.handleRecoverRun()}
              onRelease={() => void page.handleForceReleaseRun()}
              pending={{
                cancel: page.isCancelPending,
                release: page.isForceReleasePending,
                recover: page.isRecoverPending,
              }}
              run={page.run}
            />
          ),
        }
      : null
  );

  return null;
}

/**
 * The head while the run loads: the real way back to the list, and the task and attempt as
 * blocks, so the head is already in its drill-in shape when the run arrives into it.
 */
export function TaskRunLoadingTopbar({
  controller,
}: {
  controller: Pick<TaskRunLocationController, "backToTasks">;
}) {
  useTopbarSlot({
    onBack: controller.backToTasks,
    crumbs: [
      { id: "tasks", label: "Tasks", onSelect: controller.backToTasks },
      { id: "task", label: <SkeletonText className="w-32" />, onSelect: controller.backToTasks },
    ],
    crumb: <SkeletonText className="w-20" />,
  });

  return null;
}
