import { AlertCircle } from "@compozy/ui/icons";

import { Button, cn, Empty, PAGE_CONTENT_GUTTER, Section } from "@compozy/ui";

import { TaskRunLoadingTopbar, TaskRunTopbar } from "./task-run-topbar";
import { TASK_DETAIL_GRID_CLASS, TASK_DETAIL_RAIL_CLASS } from "./task-detail-layout";
import { TaskPageLoadingFrame, TaskRunLoadingGrid } from "./task-loading-frames";
import { type TaskRunLocationController, useTaskRunLocation } from "./use-task-run-location";
import {
  TaskResultSection,
  TaskRunActivitySection,
  TaskRunForceFailDialog,
  TaskRunInspectDrawer,
  TaskRunOutcome,
  TaskRunRail,
  TaskRunReviewCard,
  TaskRunSubhead,
  useTaskRunResult,
} from "@/systems/tasks";
import type { TaskRun } from "@/systems/tasks";

/**
 * The run page while its record loads: the subhead frame, then the grid (the main column and the
 * rail) as the single loading region, in the same scroll container the run fills.
 */
function TaskRunLoading({ controller }: { controller: TaskRunLocationController }) {
  return (
    <TaskPageLoadingFrame
      bottomPadding="pb-16"
      head={<TaskRunLoadingTopbar controller={controller} />}
    >
      <TaskRunLoadingGrid />
    </TaskPageLoadingFrame>
  );
}

export function TaskRunLocation({ taskId, runId }: { taskId: string; runId: string }) {
  const controller = useTaskRunLocation(taskId, runId);
  const { page, record } = controller;
  if (page.runLoading) return <TaskRunLoading controller={controller} />;

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
              <TaskRunResult
                key={record.id}
                run={record}
                workspaceId={page.task?.task.workspace_id ?? ""}
              />

              <TaskRunReviews page={page} />

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

function TaskRunResult({ run, workspaceId }: { run: TaskRun; workspaceId: string }) {
  const external = useTaskRunResult({
    resultBytes: run.result_bytes ?? 0,
    resultRef: run.result_ref ?? "",
    runId: run.id,
    workspaceId,
  });
  return (
    <TaskResultSection
      data-testid="tasks-run-result"
      emptyMessage={
        run.status === "completed"
          ? "No result recorded."
          : run.ended_at
            ? "No result recorded. The run ended before reaching a checkpoint."
            : "No result yet. It lands here the moment the run completes."
      }
      external={run.result_ref ? external : undefined}
      jsonTestId="tasks-run-result-json"
      result={run.result}
      resultBytes={run.result_bytes}
      resultRef={run.result_ref}
    />
  );
}

function TaskRunReviews({ page }: { page: ReturnType<typeof useTaskRunLocation>["page"] }) {
  return (
    <>
      {/* Reviews are opt-in, so most runs never have this section: while the read is pending it
          holds no placeholder (a skeleton for a section that then never appears moved the run
          activity below it). It renders once reviews or an error arrive. */}
      {page.reviewsError || page.reviews.length > 0 ? (
        <Section
          count={page.reviews.length || undefined}
          data-testid="tasks-run-reviews"
          label="Reviews"
        >
          {page.reviewsError ? (
            <p className="text-small-body text-destructive" role="alert">
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
    </>
  );
}
