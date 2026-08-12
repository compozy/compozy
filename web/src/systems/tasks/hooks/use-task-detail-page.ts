import { useState } from "react";
import { useSelector } from "@xstate/store-react";

import { useStoreBinding } from "@/hooks/use-store-binding";

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
import { useTask, useTaskRuns } from "./use-tasks";
import { useTaskInspect, useTaskTimeline } from "./use-task-live";
import { useTaskExecutionProfile } from "./use-task-profile";
import { useTaskReviews } from "./use-task-reviews";
import { useRecoverTaskRun } from "./use-task-run-recovery";
import { useTaskStream } from "./use-task-stream";
import { taskRunCanRecover } from "../lib/task-run-recovery";
import { taskStreamStatusLogic } from "../lib/task-stream-state";
import { notifyTaskMutation, submitTaskMutation } from "../lib/task-mutation";
import type { FanOutTaskRunsRequest, TaskRunsFilter, TaskTimelineFilter } from "../types";

interface UseTaskDetailPageOptions {
  initialTimelineLimit?: number;
  runFilters?: TaskRunsFilter;
  timelineFilters?: TaskTimelineFilter;
  enableTimeline?: boolean;
  enableRuns?: boolean;
  enableInspect?: boolean;
  enableStream?: boolean;
  liveDataEnabled?: boolean;
}

const DEFAULT_TIMELINE_LIMIT = 50;
const TIMELINE_PAGE_SIZE = 50;

/**
 * View model for the 3-tab task detail page. Tab state lives in the window
 * location (URL-addressable); this hook owns queries, the SSE stream, and
 * every task-level verb handler.
 */
function useTaskDetailPage(taskId: string, options: UseTaskDetailPageOptions = {}) {
  const [timelineLimit, setTimelineLimit] = useState<number>(
    options.initialTimelineLimit ?? DEFAULT_TIMELINE_LIMIT
  );
  const hasTaskId = Boolean(taskId);
  const enableTimeline = options.enableTimeline ?? true;
  const enableRuns = options.enableRuns ?? true;
  const enableInspect = options.enableInspect ?? true;
  const enableStream = options.enableStream ?? true;
  const liveDataEnabled = options.liveDataEnabled ?? true;

  const timelineFilters: TaskTimelineFilter = {
    limit: timelineLimit,
    after_sequence: options.timelineFilters?.after_sequence,
  };

  const queryEnabled = hasTaskId && liveDataEnabled;
  const detailQuery = useTask(taskId, { enabled: queryEnabled });
  const detail = detailQuery.data ?? null;
  const activeRun = detail?.summary?.active_run ?? null;
  const isLive = isRunActive(activeRun?.status ?? null);
  const refetchIntervalMs = isLive && liveDataEnabled ? undefined : false;
  const timelineQuery = useTaskTimeline(taskId, timelineFilters, {
    enabled: queryEnabled && enableTimeline,
    refetchIntervalMs,
  });
  const runsQuery = useTaskRuns(taskId, options.runFilters ?? {}, {
    enabled: queryEnabled && enableRuns,
    refetchIntervalMs,
  });
  const inspectQuery = useTaskInspect(taskId, {
    enabled: queryEnabled && enableInspect,
    refetchIntervalMs,
  });
  const profileQuery = useTaskExecutionProfile(taskId, {
    enabled: queryEnabled,
    refetchIntervalMs,
  });
  const reviewsQuery = useTaskReviews(taskId, {}, { enabled: queryEnabled, refetchIntervalMs });

  const publishMutation = usePublishTask();
  const cancelMutation = useCancelTask();
  const enqueueMutation = useEnqueueTaskRun();
  const pauseMutation = usePauseTask();
  const resumeMutation = useResumeTask();
  const recoverTaskMutation = useRecoverTask();
  const recoverRunMutation = useRecoverTaskRun();
  const approveMutation = useApproveTask();
  const rejectMutation = useRejectTask();
  const retryRunMutation = useRetryTaskRun();
  const clearBlockMutation = useClearTaskBlock();
  const fanOutMutation = useFanOutTaskRuns();

  const runs = runsQuery.data ?? [];
  const timeline = timelineQuery.data ?? [];
  const inspect = inspectQuery.data ?? null;
  const profile = profileQuery.data ?? null;
  const reviews = reviewsQuery.data ?? [];

  const activeRunNeedsAttention = activeRun?.status === "needs_attention";
  const recoverableRunId =
    activeRun && taskRunCanRecover(activeRun, detail?.task.max_attempts) ? activeRun.id : null;
  // Keep the page fresh from run-lifecycle SSE events. Wait for the detail
  // payload before connecting so we seed from the real cursor instead of
  // after_sequence=0 (a full-history replay + immediate reconnect when
  // latest_event_seq resolves).
  const detailEventSeq = detail?.task?.latest_event_seq;
  const hasEventSeq = typeof detailEventSeq === "number";
  const streamEnabled = hasTaskId && enableStream && liveDataEnabled && hasEventSeq;
  const streamKey = streamEnabled ? `${taskId}:${detailEventSeq}` : "disabled";
  const { store: streamStatusStore } = useStoreBinding(streamKey, () =>
    taskStreamStatusLogic.createStore({ enabled: streamEnabled })
  );
  const currentStreamStatus = useSelector(streamStatusStore, snapshot => snapshot.context);
  useTaskStream(taskId, {
    enabled: streamEnabled,
    afterSequence: hasEventSeq ? Math.max(0, detailEventSeq) : undefined,
    onEvent: () => streamStatusStore.trigger.frameReceived(),
    onError: error =>
      streamStatusStore.trigger.streamFailed({
        error:
          error instanceof Error
            ? error.message
            : typeof error === "string"
              ? error
              : "Stream connection failed",
      }),
  });

  const fatalError = hasTaskId ? (detailQuery.error ?? null) : new Error("Missing task id");

  const handleTimelineLoadMore = () => {
    setTimelineLimit(current => current + TIMELINE_PAGE_SIZE);
  };

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
    if (!hasTaskId) return;
    await submitTaskMutation(
      () => fanOutMutation.mutateAsync({ id: taskId, data }),
      result => {
        const count = result?.runs?.length ?? 0;
        return count > 0 ? `Created ${count} run${count === 1 ? "" : "s"}.` : "Runs created.";
      },
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

  const handleRecoverTask = async () => {
    if (
      !hasTaskId ||
      (activeRunNeedsAttention && !recoverableRunId) ||
      recoverTaskMutation.isPending ||
      recoverRunMutation.isPending
    ) {
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

  const isTimelineSaturated =
    typeof timelineFilters.limit === "number" && timeline.length >= timelineFilters.limit;

  return {
    activeRun,
    detail,
    detailError: detailQuery.error ?? null,
    detailLoading: detailQuery.isLoading && !detail,
    fatalError,
    handleApproveTask,
    handleCancelTask,
    handleClearBlock,
    handleEnqueueRun,
    handleFanOutRuns,
    handlePauseTask,
    handlePublishTask,
    handleRecoverTask,
    handleRejectTask,
    handleRetryDetail: () => detailQuery.refetch(),
    handleResumeTask,
    handleRetryRun,
    handleTimelineLoadMore,
    inspect,
    inspectError: inspectQuery.error ?? null,
    inspectLoading: inspectQuery.isLoading && !inspect,
    isApprovePending: approveMutation.isPending,
    isCancelPending: cancelMutation.isPending,
    isClearBlockPending: clearBlockMutation.isPending,
    isEnqueuePending: enqueueMutation.isPending,
    isFanOutPending: fanOutMutation.isPending,
    isLive,
    isPausePending: pauseMutation.isPending,
    isPublishPending: publishMutation.isPending,
    isRecoverPending: recoverTaskMutation.isPending || recoverRunMutation.isPending,
    isRejectPending: rejectMutation.isPending,
    isResumePending: resumeMutation.isPending,
    isRetryPending: retryRunMutation.isPending,
    isTimelineSaturated,
    notFound: detailQuery.isError && detailQuery.error?.message?.includes("not found"),
    profile,
    profileError: profileQuery.error ?? null,
    profileLoading: profileQuery.isLoading && !profile,
    reviews,
    reviewsError: reviewsQuery.error ?? null,
    reviewsLoading: reviewsQuery.isLoading && reviews.length === 0,
    runs,
    runsError: runsQuery.error ?? null,
    runsLoading: runsQuery.isLoading && runs.length === 0,
    taskId,
    streamErrorMessage: currentStreamStatus.error,
    streamSeedSequence: hasEventSeq ? Math.max(0, detailEventSeq) : 0,
    streamState: currentStreamStatus.state,
    timeline,
    timelineError: timelineQuery.error ?? null,
    timelineLimit,
    timelineLoading: timelineQuery.isLoading && timeline.length === 0,
  };
}

function isRunActive(status?: string | null): boolean {
  return (
    status === "running" || status === "claimed" || status === "starting" || status === "queued"
  );
}

export { useTaskDetailPage };
export type { UseTaskDetailPageOptions };
