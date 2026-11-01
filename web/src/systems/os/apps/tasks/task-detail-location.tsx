import { AlertCircle, ClipboardList } from "@compozy/ui/icons";
import { useDeferredValue, ViewTransition } from "react";

import {
  Button,
  cn,
  Empty,
  LaneTabs,
  PAGE_CONTENT_GUTTER,
  TabsContent,
  VIEW_TRANSITION_CLASS,
} from "@compozy/ui";

import {
  type TaskOverviewCompletedResult,
  useCompletedRunResult,
} from "./hooks/use-completed-run-result";
import { TaskDetailOverlays } from "./task-detail-overlays";
import {
  buildTabItems,
  TASK_DETAIL_GRID_CLASS,
  TASK_DETAIL_RAIL_CLASS,
} from "./task-detail-layout";
import { TaskDetailLoadingTopbar, TaskDetailTopbar } from "./task-detail-topbar";
import { TaskDetailLoadingGrid, TaskPageLoadingFrame } from "./task-loading-frames";
import {
  type TaskDetailLocationController,
  useTaskDetailLocation,
} from "./use-task-detail-location";
import {
  type ResolvedTaskDetailSearch,
  TaskActivityPanel,
  type TaskDetailTab,
  TaskOverviewPanel,
  TaskPropertiesRail,
  type TaskRunReview,
  TaskRunsPanel,
  TasksDetailSubhead,
} from "@/systems/tasks";

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

/**
 * The task detail while its record loads. The head, subhead frame and tabs are already the
 * real ones, and the grid below them (the main column and the rail) is the single loading
 * region, so the record arrives into the same places and nothing around it moves.
 */
function TaskDetailLoading({ controller }: { controller: TaskDetailLocationController }) {
  const tab = controller.search.tab;
  return (
    <TaskPageLoadingFrame
      bottomPadding="pb-20"
      head={<TaskDetailLoadingTopbar controller={controller} />}
    >
      <LaneTabs<TaskDetailTab>
        ariaLabel="Task views"
        className="gap-0"
        data-testid="tasks-detail-tabs"
        items={buildTabItems("pending")}
        listClassName="w-full"
        onChange={controller.setTab}
        value={tab}
      >
        <TaskDetailLoadingGrid tab={tab} />
      </LaneTabs>
    </TaskPageLoadingFrame>
  );
}

type TaskDetailData = {
  detail: NonNullable<TaskDetailLocationController["detail"]>;
  record: NonNullable<TaskDetailLocationController["record"]>;
  command: NonNullable<TaskDetailLocationController["command"]>;
};

export function TaskDetailLocation({
  taskId,
  search,
}: {
  taskId: string;
  search: ResolvedTaskDetailSearch;
}) {
  const controller = useTaskDetailLocation(taskId, search);
  const { page, detail, record, command } = controller;
  const completedResult = useCompletedRunResult(page.runs, record?.workspace_id);

  if (page.detailLoading) {
    return <TaskDetailLoading controller={controller} />;
  }
  if (!page.notFound && page.detailError) {
    return <TaskDetailError message={page.detailError.message} onRetry={page.handleRetryDetail} />;
  }
  if (page.notFound || !detail || !record || !command) {
    return (
      <TaskDetailNotFound message={page.fatalError?.message} onBack={controller.backToTasks} />
    );
  }
  return (
    <TaskDetailContent
      command={command}
      completedResult={completedResult}
      controller={controller}
      detail={detail}
      record={record}
      taskId={taskId}
    />
  );
}

function TaskDetailError({
  message,
  onRetry,
}: {
  message: string;
  onRetry: () => Promise<unknown> | void;
}) {
  return (
    <div
      className={cn(PAGE_CONTENT_GUTTER, "flex flex-1 items-center justify-center py-8")}
      data-testid="tasks-detail-error"
    >
      <Empty
        action={
          <Button onClick={() => void onRetry()} size="sm" type="button">
            Retry
          </Button>
        }
        description={message}
        icon={AlertCircle}
        title="Couldn't load task"
      />
    </div>
  );
}

function TaskDetailNotFound({ message, onBack }: { message?: string; onBack: () => void }) {
  return (
    <div
      className={cn(PAGE_CONTENT_GUTTER, "flex flex-1 items-center justify-center py-8")}
      data-testid="tasks-detail-not-found"
    >
      <Empty
        icon={AlertCircle}
        title="Task not found"
        description={message ?? `This task isn't in this project.`}
        action={
          <Button onClick={onBack} size="sm" type="button" variant="ghost">
            <ClipboardList aria-hidden="true" data-icon="inline-start" />
            Back to tasks
          </Button>
        }
      />
    </div>
  );
}

function TaskDetailContent({
  controller,
  taskId,
  detail,
  record,
  command,
  completedResult,
}: TaskDetailData & {
  controller: TaskDetailLocationController;
  taskId: string;
  completedResult: TaskOverviewCompletedResult;
}) {
  const { page } = controller;
  // The tab arrives from the window-manager store, whose updates are never React transitions, so
  // the tabs and their panel read it deferred: the swap renders as the transition the panel's
  // boundary animates.
  const tab = useDeferredValue(controller.search.tab);
  const reviewsByRun = groupReviewsByRun(page.reviews);
  const canStartRun = command.primary?.kind === "start";
  const isDraft = record.draft || record.status === "draft";

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
            items={buildTabItems(page.runsLoading ? "pending" : page.runs.length)}
            listClassName="w-full"
            onChange={controller.setTab}
            value={tab}
          >
            <div className={cn(TASK_DETAIL_GRID_CLASS, "pt-5")}>
              <main className="min-w-0" data-testid="tasks-detail-panels">
                {/* A tab swap cross-fades the panel. Keyed by tab so only the swap triggers it. */}
                <ViewTransition
                  key={tab}
                  default="none"
                  enter={VIEW_TRANSITION_CLASS.fade}
                  exit={VIEW_TRANSITION_CLASS.fade}
                >
                  <TabsContent value="overview">
                    <TaskOverviewPanel
                      activeRunElapsed={controller.activeElapsed}
                      completedResult={completedResult}
                      detail={detail}
                      isLive={page.isLive}
                      nowHandlers={{
                        onOpenRun: controller.openRun,
                        onOpenTask: controller.openTask,
                        onApprove: () => void page.handleApproveTask(),
                        onReject: () => void page.handleRejectTask(),
                        onResume: () => void page.handleResumeTask(),
                        onRecover: () => void page.handleRecoverTask(),
                        onClearBlock: blockId => void page.handleClearBlock(blockId),
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
                      timelineLoading={page.timelineLoading}
                    />
                  </TabsContent>
                  <TabsContent value="runs">
                    <TaskRunsPanel
                      emptyDescription={
                        isDraft
                          ? "Saved intent only. Runs appear after you publish, start, or approve a task."
                          : undefined
                      }
                      errorMessage={page.runsError?.message ?? null}
                      isLoading={page.runsLoading}
                      isStartPending={page.isEnqueuePending}
                      onStartRun={canStartRun ? () => void page.handleEnqueueRun() : undefined}
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
                </ViewTransition>
              </main>
              <aside className={TASK_DETAIL_RAIL_CLASS}>
                <TaskPropertiesRail
                  detail={detail}
                  onAutoEnqueueChange={enabled => void controller.handleAutoEnqueueChange(enabled)}
                  onEditSetup={() => controller.setSetupOpen(true)}
                  onInspect={() => controller.setInspectOpen(true)}
                  onPriorityChange={priority => void controller.handlePriorityChange(priority)}
                  profile={page.profile}
                  profileLoading={page.profileLoading}
                  runs={page.runs}
                  runsLoading={page.runsLoading}
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
