import { useTopbarSlot } from "@agh/ui";

import { TaskPageActions, TaskPageOverflow, TaskPageStatus } from "@/systems/tasks";

import type { TaskDetailLocationController } from "./use-task-detail-location";

/**
 * Publishes task-detail crumbs, status, and actions into the window topbar.
 *
 * @see docs/design/opendesign/tasks/TASK-DETAILS-REDESIGN-PLAN.md §6
 */
export function TaskDetailTopbar({ controller }: { controller: TaskDetailLocationController }) {
  const { page, record, command } = controller;

  useTopbarSlot(
    record && command
      ? {
          onBack: controller.backToTasks,
          crumbs: [
            {
              id: "tasks",
              label: <span data-testid="tasks-detail-breadcrumb-tasks">Tasks</span>,
              onSelect: controller.backToTasks,
            },
          ],
          crumb: <span data-testid="tasks-detail-title">{record.title}</span>,
          status: <TaskPageStatus status={record.status} />,
          actions: (
            <TaskPageActions
              command={command}
              handlers={{
                onPublish: () => void page.handlePublishTask(),
                onApprove: () => void page.handleApproveTask(),
                onStartRun: () => void page.handleEnqueueRun(),
                onOpenRun: controller.openRun,
                onResume: () => void page.handleResumeTask(),
                onRecover: () => void page.handleRecoverTask(),
                onRetry: runId => void page.handleRetryRun(runId),
                onEdit: controller.openEdit,
                onPause: controller.pauseDialog.open,
                onReject: () => void page.handleRejectTask(),
              }}
              pending={{
                approve: page.isApprovePending,
                publish: page.isPublishPending,
                recover: page.isRecoverPending,
                reject: page.isRejectPending,
                resume: page.isResumePending,
                retry: page.isRetryPending,
                start: page.isEnqueuePending,
              }}
            />
          ),
          overflow: (
            <TaskPageOverflow
              command={command}
              onCancel={() => void page.handleCancelTask()}
              onCopyId={controller.copyTaskId}
              onDelete={() => controller.setDeleteOpen(true)}
              onFanOut={() => controller.setFanOutOpen(true)}
              onPause={controller.pauseDialog.open}
              onResume={() => void page.handleResumeTask()}
              onStartNewRun={() => void page.handleEnqueueRun()}
              pending={{
                cancel: page.isCancelPending,
                pause: page.isPausePending,
                resume: page.isResumePending,
                enqueue: page.isEnqueuePending,
              }}
              showFanOut={controller.showFanOut}
              taskId={record.id}
            />
          ),
        }
      : null
  );

  return null;
}
