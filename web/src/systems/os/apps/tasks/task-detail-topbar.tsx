import { useTopbarSlot } from "@compozy/ui";

import type { TaskDetailLocationController } from "./use-task-detail-location";
import type { TaskCommandState } from "@/systems/tasks";
import { TaskPageActions, TaskPageOverflow, TaskPageStatus } from "@/systems/tasks";
import { useGatewayCapabilities } from "@/systems/gateway";

/**
 * Strips the lifecycle mutations from the command state. Task/run mutation
 * routes register only under `includeTaskMutations` (`routes.go:216-252`), so
 * on remote tiers those affordances go absent (BR-1) while navigation stays:
 * `open_run` is a read, and with no primary and no secondaries left the
 * actions render nothing.
 */
function readOnlyCommand(command: TaskCommandState): TaskCommandState {
  return {
    ...command,
    primary: command.primary?.kind === "open_run" ? command.primary : null,
    secondary: { edit: false, pause: false, reject: false },
    overflow: {
      edit: false,
      pause: false,
      resume: false,
      cancel: false,
      startNewRun: false,
      delete: false,
    },
  };
}

/**
 * Publishes task-detail crumbs, status, and actions into the window topbar.
 *
 * @see docs/design/opendesign/tasks/TASK-DETAILS-REDESIGN-PLAN.md §6
 */
export function TaskDetailTopbar({ controller }: { controller: TaskDetailLocationController }) {
  const { page, record, command } = controller;
  // Task lifecycle mutations (publish/approve/start/pause/resume/recover/
  // retry/cancel/fan-out/delete) register only on the local surface set: on
  // remote tiers the affordances go absent while reads stay (BR-1).
  const { localTaskLifecycle } = useGatewayCapabilities();

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
              command={localTaskLifecycle ? command : readOnlyCommand(command)}
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
              command={localTaskLifecycle ? command : readOnlyCommand(command)}
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
              // Fan-out enqueues runs: local-only like the rest of the
              // lifecycle, so the entry goes absent on remote tiers.
              showFanOut={localTaskLifecycle && controller.showFanOut}
              taskId={record.id}
            />
          ),
        }
      : null
  );

  return null;
}
