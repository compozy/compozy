import { useState } from "react";

import { useTask, useTaskRuns } from "./use-tasks";
import { useTaskInspect, useTaskTimeline } from "./use-task-live";
import { useTaskExecutionProfile } from "./use-task-profile";
import { useTaskReviews } from "./use-task-reviews";
import type { TaskRunsFilter, TaskTimelineFilter } from "../types";

interface TaskDetailReadOptions {
  initialTimelineLimit?: number;
  runFilters?: TaskRunsFilter;
  timelineFilters?: TaskTimelineFilter;
  enableTimeline?: boolean;
  enableRuns?: boolean;
  enableInspect?: boolean;
  liveDataEnabled?: boolean;
}

const DEFAULT_TIMELINE_LIMIT = 50;
const TIMELINE_PAGE_SIZE = 50;

type DetailQuery = ReturnType<typeof useTask>;
type TimelineQuery = ReturnType<typeof useTaskTimeline>;
type RunsQuery = ReturnType<typeof useTaskRuns>;
type InspectQuery = ReturnType<typeof useTaskInspect>;
type ProfileQuery = ReturnType<typeof useTaskExecutionProfile>;
type ReviewsQuery = ReturnType<typeof useTaskReviews>;

/** Which reads run: every read needs a task id and a live window; tabs gate their own. */
function taskDetailReadGates(taskId: string, options: TaskDetailReadOptions) {
  const liveDataEnabled = options.liveDataEnabled ?? true;
  const queryEnabled = Boolean(taskId) && liveDataEnabled;
  return {
    liveDataEnabled,
    queryEnabled,
    timelineEnabled: queryEnabled && (options.enableTimeline ?? true),
    runsEnabled: queryEnabled && (options.enableRuns ?? true),
    inspectEnabled: queryEnabled && (options.enableInspect ?? true),
  };
}

function isRunActive(status?: string | null): boolean {
  return (
    status === "running" || status === "claimed" || status === "starting" || status === "queued"
  );
}

function detailReadModel(taskId: string, detailQuery: DetailQuery) {
  const hasTaskId = Boolean(taskId);
  return {
    detail: detailQuery.data ?? null,
    detailError: detailQuery.error ?? null,
    // `isPending`, not `isLoading`: a suspended window disables the read, and a
    // disabled read with no data yet is still waiting, not a missing task.
    detailLoading: hasTaskId && detailQuery.isPending,
    fatalError: hasTaskId ? (detailQuery.error ?? null) : new Error("Missing task id"),
    handleRetryDetail: () => detailQuery.refetch(),
    notFound: detailQuery.isError && detailQuery.error?.message?.includes("not found"),
  };
}

function timelineReadModel(timelineQuery: TimelineQuery, timelineLimit: number) {
  const timeline = timelineQuery.data ?? [];
  return {
    isTimelineSaturated: timeline.length >= timelineLimit,
    timeline,
    timelineError: timelineQuery.error ?? null,
    timelineLimit,
    timelineLoading: timelineQuery.isLoading && timeline.length === 0,
  };
}

function sideReadModel(
  runsQuery: RunsQuery,
  inspectQuery: InspectQuery,
  profileQuery: ProfileQuery,
  reviewsQuery: ReviewsQuery
) {
  const runs = runsQuery.data ?? [];
  const inspect = inspectQuery.data ?? null;
  const profile = profileQuery.data ?? null;
  const reviews = reviewsQuery.data ?? [];
  return {
    inspect,
    inspectError: inspectQuery.error ?? null,
    inspectLoading: inspectQuery.isLoading && !inspect,
    profile,
    profileError: profileQuery.error ?? null,
    profileLoading: profileQuery.isLoading && !profile,
    reviews,
    reviewsError: reviewsQuery.error ?? null,
    reviewsLoading: reviewsQuery.isLoading && reviews.length === 0,
    runs,
    runsError: runsQuery.error ?? null,
    runsLoading: runsQuery.isLoading && runs.length === 0,
  };
}

/**
 * Task detail reads: the detail payload plus the per-tab timeline, runs,
 * inspect, profile, and review queries. They poll together while a run is live.
 */
function useTaskDetailReads(taskId: string, options: TaskDetailReadOptions) {
  const [timelineLimit, setTimelineLimit] = useState<number>(
    options.initialTimelineLimit ?? DEFAULT_TIMELINE_LIMIT
  );
  const gates = taskDetailReadGates(taskId, options);
  const timelineFilters: TaskTimelineFilter = {
    limit: timelineLimit,
    after_sequence: options.timelineFilters?.after_sequence,
  };

  const detailQuery = useTask(taskId, { enabled: gates.queryEnabled });
  const activeRun = detailQuery.data?.summary?.active_run ?? null;
  const isLive = isRunActive(activeRun?.status);
  const refetchIntervalMs = isLive && gates.liveDataEnabled ? undefined : false;
  const timelineQuery = useTaskTimeline(taskId, timelineFilters, {
    enabled: gates.timelineEnabled,
    refetchIntervalMs,
  });
  const runsQuery = useTaskRuns(taskId, options.runFilters ?? {}, {
    enabled: gates.runsEnabled,
    refetchIntervalMs,
  });
  const inspectQuery = useTaskInspect(taskId, {
    enabled: gates.inspectEnabled,
    refetchIntervalMs,
  });
  const profileQuery = useTaskExecutionProfile(taskId, {
    enabled: gates.queryEnabled,
    refetchIntervalMs,
  });
  const reviewsQuery = useTaskReviews(
    taskId,
    {},
    { enabled: gates.queryEnabled, refetchIntervalMs }
  );

  return {
    ...detailReadModel(taskId, detailQuery),
    ...timelineReadModel(timelineQuery, timelineLimit),
    ...sideReadModel(runsQuery, inspectQuery, profileQuery, reviewsQuery),
    activeRun,
    handleTimelineLoadMore: () => setTimelineLimit(current => current + TIMELINE_PAGE_SIZE),
    isLive,
  };
}

export { useTaskDetailReads };
export type { TaskDetailReadOptions };
