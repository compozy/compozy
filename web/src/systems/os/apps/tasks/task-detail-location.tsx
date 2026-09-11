import { AlertCircle, ClipboardList } from "lucide-react";

import {
  Button,
  cn,
  Empty,
  LaneTabs,
  PAGE_CONTENT_GUTTER,
  Skeleton,
  TabsContent,
  type LaneTabsItem,
} from "@compozy/ui";

import { TaskDetailOverlays } from "./task-detail-overlays";
import { TASK_DETAIL_GRID_CLASS, TASK_DETAIL_RAIL_CLASS } from "./task-detail-layout";
import { TaskDetailTopbar } from "./task-detail-topbar";
import { useTaskDetailLocation } from "./use-task-detail-location";
import { useGatewayCapabilities } from "@/systems/gateway";
import {
  type ResolvedTaskDetailSearch,
  latestTaskRun,
  TaskActivityPanel,
  type TaskDetailTab,
  TaskOverviewPanel,
  TaskPropertiesRail,
  type TaskRunReview,
  TaskRunsPanel,
  TasksDetailSubhead,
  useTaskRunResult,
} from "@/systems/tasks";

function buildTabItems(runCount: number): ReadonlyArray<LaneTabsItem<TaskDetailTab>> {
  return [
    { value: "overview", label: "Overview", testId: "tasks-detail-tab-overview" },
    { value: "runs", label: "Runs", count: runCount, testId: "tasks-detail-tab-runs" },
    { value: "activity", label: "Activity", testId: "tasks-detail-tab-activity" },
  ];
}

function groupReviewsByRun(
  reviews: readonly TaskRunReview[]
): ReadonlyMap<string, readonly TaskRunReview[]> {
  const map = new Map<string, TaskRunReview[]>();
  for (const review of reviews) {
    if (!review.run_id) continue;
    const bucket = map.get(review.run_id);
    if (bucket) {
      bucket.push(review);
    } else {
      map.set(review.run_id, [review]);
    }
  }
  return map;
}

function TaskDetailLoading() {
  return (
    <div
      className={cn(PAGE_CONTENT_GUTTER, "@container flex min-h-0 flex-1 flex-col gap-4 py-5")}
      data-testid="tasks-detail-loading"
    >
      <Skeleton className="h-6 w-72" />
      <Skeleton className="h-10 w-full max-w-md" />
      <div className={TASK_DETAIL_GRID_CLASS}>
        <div className="flex flex-col gap-3">
          <Skeleton className="h-20 rounded-lg" />
          <Skeleton className="h-32 rounded-lg" />
          <Skeleton className="h-40 rounded-lg" />
        </div>
        <Skeleton className="hidden h-80 rounded-lg @min-task-detail-rail:block" />
      </div>
    </div>
  );
}

export function TaskDetailLocation({
  taskId,
  search,
}: {
  taskId: string;
  search: ResolvedTaskDetailSearch;
}) {
  const controller = useTaskDetailLocation(taskId, search);
  // Task/run/review lifecycle mutations register only on the local surface
  // set (`routes.go` `includeTaskMutations`): the page body renders those
  // affordances only when the tier can execute them (BR-1 — absent, never
  // disabled), while read affordances stay on every tier.
  const { localTaskLifecycle } = useGatewayCapabilities();
  const { page, detail, record, command } = controller;
  const completedRun = latestTaskRun(page.runs, "completed");
  const completedRunResult = useTaskRunResult({
    resultBytes: completedRun?.result_bytes ?? 0,
    resultRef: completedRun?.result_ref ?? "",
    runId: completedRun?.id ?? "",
    workspaceId: record?.workspace_id ?? "",
  });

  if (page.detailLoading) {
    return <TaskDetailLoading />;
  }

  if (!page.notFound && page.detailError) {
    return (
      <div
        className={cn(PAGE_CONTENT_GUTTER, "flex flex-1 items-center justify-center py-8")}
        data-testid="tasks-detail-error"
      >
        <Empty
          action={
            <Button onClick={() => void page.handleRetryDetail()} size="sm" type="button">
              Retry
            </Button>
          }
          description={page.detailError.message}
          icon={AlertCircle}
          title="Couldn't load task"
        />
      </div>
    );
  }

  if (page.notFound || !detail || !record || !command) {
    return (
      <div
        className={cn(PAGE_CONTENT_GUTTER, "flex flex-1 items-center justify-center py-8")}
        data-testid="tasks-detail-not-found"
      >
        <Empty
          icon={AlertCircle}
          title="Task not found"
          description={page.fatalError?.message ?? `No task with id "${taskId}" in this workspace.`}
          action={
            <Button onClick={controller.backToTasks} size="sm" type="button" variant="ghost">
              <ClipboardList aria-hidden="true" className="size-3" />
              Back to tasks
            </Button>
          }
        />
      </div>
    );
  }

  const reviewsByRun = groupReviewsByRun(page.reviews);
  const canStartRun = command.primary?.kind === "start";

  return (
    <div
      className="flex min-h-0 min-w-0 flex-1 flex-col overflow-hidden"
      data-testid="tasks-detail-content"
    >
      <TaskDetailTopbar controller={controller} />
      <TaskDetailOverlays controller={controller} />

      <div className="min-h-0 flex-1 overflow-y-auto">
        <div className={cn(PAGE_CONTENT_GUTTER, "@container pt-5 pb-20")}>
          <TasksDetailSubhead detail={detail} />
          <LaneTabs<TaskDetailTab>
            ariaLabel="Task views"
            className="gap-0"
            data-testid="tasks-detail-tabs"
            items={buildTabItems(page.runs.length)}
            listClassName="w-full"
            onChange={controller.setTab}
            value={controller.search.tab}
          >
            <div className={cn(TASK_DETAIL_GRID_CLASS, "pt-5")}>
              <main className="min-w-0" data-testid="tasks-detail-panels">
                <TabsContent value="overview">
                  <TaskOverviewPanel
                    activeRunElapsed={controller.activeElapsed}
                    completedResult={
                      completedRun
                        ? {
                            external: completedRun.result_ref ? completedRunResult : undefined,
                            run: completedRun,
                          }
                        : undefined
                    }
                    detail={detail}
                    isLive={page.isLive}
                    nowHandlers={{
                      onOpenRun: controller.openRun,
                      onOpenTask: controller.openTask,
                      onApprove: localTaskLifecycle
                        ? () => void page.handleApproveTask()
                        : undefined,
                      onReject: localTaskLifecycle ? () => void page.handleRejectTask() : undefined,
                      onResume: localTaskLifecycle ? () => void page.handleResumeTask() : undefined,
                      onRecover: localTaskLifecycle
                        ? () => void page.handleRecoverTask()
                        : undefined,
                      onClearBlock: localTaskLifecycle
                        ? blockId => void page.handleClearBlock(blockId)
                        : undefined,
                      onViewResult: controller.scrollToResult,
                    }}
                    nowPending={{
                      approve: page.isApprovePending,
                      reject: page.isRejectPending,
                      resume: page.isResumePending,
                      recover: page.isRecoverPending,
                      clearBlock: page.isClearBlockPending,
                    }}
                    onViewAllActivity={() => controller.setTab("activity")}
                    runs={page.runs}
                    timeline={page.timeline}
                  />
                </TabsContent>
                <TabsContent value="runs">
                  <TaskRunsPanel
                    emptyDescription={
                      record.draft || record.status === "draft"
                        ? "Saved intent only. Runs appear after you publish, start, or approve a task."
                        : undefined
                    }
                    errorMessage={page.runsError?.message ?? null}
                    isLoading={page.runsLoading}
                    isStartPending={page.isEnqueuePending}
                    // Start run enqueues a local-only run: the affordance
                    // needs BOTH the tier capability and a start-able command
                    // state (BR-1 — absent, never disabled).
                    onStartRun={
                      localTaskLifecycle && canStartRun
                        ? () => void page.handleEnqueueRun()
                        : undefined
                    }
                    reviewsByRun={reviewsByRun}
                    runDurations={controller.runDurations}
                    runs={page.runs}
                    taskId={taskId}
                    workerName={page.profile?.worker?.agent_name ?? null}
                  />
                </TabsContent>
                <TabsContent value="activity">
                  <TaskActivityPanel
                    canLoadMore={page.isTimelineSaturated}
                    errorMessage={page.timelineError?.message ?? null}
                    isLive={page.isLive}
                    isLoading={page.timelineLoading}
                    items={page.timeline}
                    onLoadMore={page.handleTimelineLoadMore}
                  />
                </TabsContent>
              </main>
              <aside className={TASK_DETAIL_RAIL_CLASS}>
                <TaskPropertiesRail
                  approvalPending={{
                    approve: page.isApprovePending,
                    reject: page.isRejectPending,
                  }}
                  detail={detail}
                  onApprove={localTaskLifecycle ? () => void page.handleApproveTask() : undefined}
                  // Priority/auto-enqueue persist through task PATCH
                  // (`UpdateTask`), a local-only mutation route: the editors
                  // go absent on remote tiers while their read rows stay
                  // (BR-1 — absent, never disabled).
                  onAutoEnqueueChange={
                    localTaskLifecycle
                      ? enabled => void controller.handleAutoEnqueueChange(enabled)
                      : undefined
                  }
                  // Edit setup saves through the execution-profile PUT
                  // (`SetTaskExecutionProfile`), a local-only mutation route:
                  // the entry goes absent on remote tiers while the execution
                  // read rows stay (BR-1 — absent, never disabled).
                  onEditSetup={localTaskLifecycle ? () => controller.setSetupOpen(true) : undefined}
                  onInspect={() => controller.setInspectOpen(true)}
                  onPriorityChange={
                    localTaskLifecycle
                      ? priority => void controller.handlePriorityChange(priority)
                      : undefined
                  }
                  onReject={localTaskLifecycle ? () => void page.handleRejectTask() : undefined}
                  profile={page.profile}
                  runs={page.runs}
                  updatePending={controller.updatePending}
                />
              </aside>
            </div>
          </LaneTabs>
        </div>
      </div>
    </div>
  );
}
