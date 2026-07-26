import { AlertCircle } from "lucide-react";

import { Button, cn, Empty, PAGE_CONTENT_GUTTER, Section, Skeleton } from "@agh/ui";

import {
  formatTaskRunBounds,
  TaskRunConversationPanel,
  TaskRunCoordinationInvitationHost,
  useTaskRunConversation,
} from "@/systems/network";
import {
  TaskRunActivitySection,
  TaskRunForceFailDialog,
  TaskRunInspectDrawer,
  TaskRunOutcome,
  TaskRunRail,
  TaskRunReviewCard,
  TaskResultSection,
  TaskRunSubhead,
} from "@/systems/tasks";

import { TaskRunTopbar } from "./task-run-topbar";
import { TASK_DETAIL_GRID_CLASS, TASK_DETAIL_RAIL_CLASS } from "./task-detail-layout";
import { useTaskRunLocation } from "./use-task-run-location";

export function TaskRunLocation({ taskId, runId }: { taskId: string; runId: string }) {
  const controller = useTaskRunLocation(taskId, runId);
  const { page, record } = controller;
  const conversation = useTaskRunConversation(runId, page.run?.network);

  if (page.runLoading) {
    return (
      <div
        className={cn(PAGE_CONTENT_GUTTER, "@container flex min-h-0 flex-1 flex-col gap-4 py-5")}
        data-testid="tasks-run-detail-loading"
      >
        <Skeleton className="h-6 w-72" />
        <div className={TASK_DETAIL_GRID_CLASS}>
          <div className="flex flex-col gap-3">
            <Skeleton className="h-24 rounded-lg" />
            <Skeleton className="h-48 rounded-lg" />
          </div>
          <Skeleton className="hidden h-72 rounded-lg @min-task-detail-rail:block" />
        </div>
      </div>
    );
  }

  if (!page.notFound && page.runError) {
    return (
      <div
        className={cn(PAGE_CONTENT_GUTTER, "flex flex-1 items-center justify-center py-8")}
        data-testid="tasks-run-detail-error"
      >
        <Empty
          action={
            <Button onClick={() => void page.handleRetryLoad()} size="sm" type="button">
              Retry
            </Button>
          }
          description={page.runError.message}
          icon={AlertCircle}
          title="Couldn't load run"
        />
      </div>
    );
  }

  if (page.notFound || !page.run || !record) {
    return (
      <div
        className={cn(PAGE_CONTENT_GUTTER, "flex flex-1 items-center justify-center py-8")}
        data-testid="tasks-run-detail-not-found"
      >
        <Empty
          description={page.fatalError?.message ?? `Run ${runId} was not found.`}
          icon={AlertCircle}
          title="Run not found"
        />
      </div>
    );
  }

  const participation = record.resolved_network_participation;
  const coordinationWorkspaceId =
    (participation?.mode === "live" ? participation.workspace_id : undefined) ??
    page.task?.task.workspace_id ??
    "";
  const nextAttempt =
    controller.taskRuns.find(candidate => candidate.previous_run_id === record.id) ?? null;

  return (
    <div
      className="flex min-h-0 min-w-0 flex-1 flex-col overflow-hidden"
      data-testid="tasks-run-detail-content"
    >
      <TaskRunTopbar controller={controller} />
      <TaskRunForceFailDialog
        dialog={controller.forceFailDialog}
        isPending={page.isForceFailPending}
      />
      <TaskRunInspectDrawer
        errorMessage={page.inspectError?.message ?? null}
        inspect={page.inspect}
        isLoading={page.inspectLoading}
        onOpenChange={controller.setInspectOpen}
        open={controller.inspectOpen}
        run={page.run}
      />

      <div className="min-h-0 flex-1 overflow-y-auto">
        <div className={cn(PAGE_CONTENT_GUTTER, "@container pt-5 pb-16")}>
          <TaskRunSubhead duration={controller.runDuration} run={page.run} />

          <div className={TASK_DETAIL_GRID_CLASS}>
            <main className="flex min-w-0 flex-col gap-6" data-testid="tasks-run-detail-main">
              <TaskRunOutcome nextAttempt={nextAttempt} run={page.run} />
              <TaskResultSection
                data-testid="tasks-run-result"
                emptyMessage={
                  record.status === "completed"
                    ? "No result recorded."
                    : record.ended_at
                      ? "No result recorded. The run ended before reaching a checkpoint."
                      : "No result yet. It lands here the moment the run completes."
                }
                jsonTestId="tasks-run-result-json"
                result={record.result}
              />

              {page.reviewsLoading || page.reviewsError || page.reviews.length > 0 ? (
                <Section
                  count={page.reviews.length || undefined}
                  data-testid="tasks-run-reviews"
                  label="Reviews"
                >
                  {page.reviewsLoading ? (
                    <Skeleton className="h-20 rounded-lg" />
                  ) : page.reviewsError ? (
                    <p className="text-small-body text-danger" role="alert">
                      {page.reviewsError.message}
                    </p>
                  ) : (
                    <div className="flex flex-col gap-2.5">
                      {page.reviews.map(review => (
                        <TaskRunReviewCard key={review.review_id} review={review} />
                      ))}
                    </div>
                  )}
                </Section>
              ) : null}

              {controller.authoritativeTaskId && coordinationWorkspaceId ? (
                <TaskRunCoordinationInvitationHost
                  runId={record.id}
                  taskId={controller.authoritativeTaskId}
                  workspaceId={coordinationWorkspaceId}
                />
              ) : null}
              {conversation && page.run.network ? (
                <TaskRunConversationPanel
                  boundsLabel={formatTaskRunBounds(record, page.run.network)}
                  conversationError={conversation.error}
                  conversationLoading={conversation.isLoading}
                  hasMoreMessages={conversation.hasOlder}
                  isFetchingMore={conversation.isLoadingOlder}
                  messages={conversation.messages}
                  onLoadMore={conversation.loadOlder}
                  streamError={conversation.streamError}
                  usage={conversation.usage}
                />
              ) : null}

              <TaskRunActivitySection
                errorMessage={controller.timelineError?.message}
                isLive={page.isLive}
                isLoading={controller.timelineLoading}
                runId={record.id}
                timeline={controller.timelineItems}
              />
            </main>

            <aside className={TASK_DETAIL_RAIL_CLASS}>
              <TaskRunRail
                duration={controller.runDuration}
                onInspect={() => controller.setInspectOpen(true)}
                run={page.run}
                taskId={controller.authoritativeTaskId || taskId}
                taskRuns={controller.taskRuns}
                taskRunsErrorMessage={controller.taskRunsError?.message ?? null}
                taskRunsLoading={controller.taskRunsLoading}
              />
            </aside>
          </div>
        </div>
      </div>
    </div>
  );
}
