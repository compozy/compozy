import { useTopbarSlot } from "@compozy/ui";

import type { TaskRunLocationController } from "./use-task-run-location";
import { TaskRunPageActions, TaskRunPageOverflow, TaskRunPageStatus } from "@/systems/tasks";
import { useGatewayCapabilities } from "@/systems/gateway";

/** Publishes the run-detail head (crumbs · run status · actions) into the window topbar. */
export function TaskRunTopbar({ controller }: { controller: TaskRunLocationController }) {
  const { page, record } = controller;
  // Run mutations (retry/cancel/recover/release/force-fail) register only on
  // the local surface set (`routes.go` registerRunMutationRoutes): on remote
  // tiers the affordances go absent while the session read stays (BR-1).
  const { localTaskLifecycle } = useGatewayCapabilities();

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
              onRetry={localTaskLifecycle ? () => void page.handleRetryRun() : undefined}
              run={page.run}
            />
          ),
          overflow: (
            <TaskRunPageOverflow
              canRecover={controller.canRecover}
              lifecycleEnabled={localTaskLifecycle}
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
