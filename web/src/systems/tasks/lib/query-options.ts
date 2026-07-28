import { infiniteQueryOptions, queryOptions } from "@tanstack/react-query";

import {
  getAgentContext,
  getTask,
  getTaskBridgeNotificationSubscription,
  getTaskContextBundle,
  getTaskDashboard,
  getTaskExecutionProfile,
  getTaskInbox,
  getTaskRun,
  getTaskRunReview,
  getTaskTimeline,
  getTaskTree,
  inspectRun,
  inspectTask,
  listTaskBridgeNotificationSubscriptions,
  listTaskReviews,
  listTaskRunReviews,
  listTaskRuns,
  listTasks,
} from "../adapters/tasks-api";
import { tasksKeys } from "./query-keys";
import { taskInboxPageRequest, taskInboxStableFilter } from "./task-inbox-query";
import { taskListPageRequest, taskListStableFilter } from "./task-list-query";
import type {
  TaskBridgeNotificationSubscriptionsFilter,
  TaskDashboardFilter,
  TaskInboxFilter,
  TaskListFilter,
  TaskReviewsFilter,
  TaskRunReviewsFilter,
  TaskRunsFilter,
  TaskTimelineFilter,
} from "../types";

const DEFAULT_STALE_TIME = 15_000;
const DEFAULT_REFETCH_INTERVAL = 30_000;
const LIVE_STALE_TIME = 5_000;
const LIVE_REFETCH_INTERVAL = 15_000;
const INITIAL_CURSOR: string | undefined = undefined;

export function tasksListOptions(filters: TaskListFilter = {}, enabled = true) {
  const stableFilters = taskListStableFilter(filters);
  return infiniteQueryOptions({
    queryKey: tasksKeys.list(stableFilters),
    queryFn: ({ pageParam, signal }) =>
      listTasks(taskListPageRequest(stableFilters, pageParam), signal),
    initialPageParam: INITIAL_CURSOR,
    getNextPageParam: lastPage => (lastPage.page.has_more ? lastPage.page.next_cursor : undefined),
    staleTime: DEFAULT_STALE_TIME,
    enabled,
  });
}

export function taskDetailOptions(id: string, enabled = true) {
  return queryOptions({
    queryKey: tasksKeys.detail(id),
    queryFn: ({ signal }) => getTask(id, signal),
    staleTime: DEFAULT_STALE_TIME,
    refetchInterval: DEFAULT_REFETCH_INTERVAL,
    enabled: Boolean(id) && enabled,
  });
}

export function taskInspectOptions(id: string, enabled = true) {
  return queryOptions({
    queryKey: tasksKeys.inspectTask(id),
    queryFn: ({ signal }) => inspectTask(id, signal),
    staleTime: LIVE_STALE_TIME,
    refetchInterval: LIVE_REFETCH_INTERVAL,
    enabled: Boolean(id) && enabled,
  });
}

export function taskRunsOptions(id: string, filters: TaskRunsFilter = {}, enabled = true) {
  return queryOptions({
    queryKey: tasksKeys.runs(id, filters),
    queryFn: ({ signal }) => listTaskRuns(id, filters, signal),
    staleTime: DEFAULT_STALE_TIME,
    refetchInterval: LIVE_REFETCH_INTERVAL,
    enabled: Boolean(id) && enabled,
  });
}

export function taskTimelineOptions(id: string, filters: TaskTimelineFilter = {}, enabled = true) {
  return queryOptions({
    queryKey: tasksKeys.timeline(id, filters),
    queryFn: ({ signal }) => getTaskTimeline(id, filters, signal),
    staleTime: LIVE_STALE_TIME,
    refetchInterval: LIVE_REFETCH_INTERVAL,
    enabled: Boolean(id) && enabled,
  });
}

export function taskTreeOptions(id: string, enabled = true) {
  return queryOptions({
    queryKey: tasksKeys.tree(id),
    queryFn: ({ signal }) => getTaskTree(id, signal),
    staleTime: LIVE_STALE_TIME,
    refetchInterval: LIVE_REFETCH_INTERVAL,
    enabled: Boolean(id) && enabled,
  });
}

export function taskRunDetailOptions(runId: string, enabled = true) {
  return queryOptions({
    queryKey: tasksKeys.runDetail(runId),
    queryFn: ({ signal }) => getTaskRun(runId, signal),
    staleTime: LIVE_STALE_TIME,
    refetchInterval: LIVE_REFETCH_INTERVAL,
    enabled: Boolean(runId) && enabled,
  });
}

export function taskRunInspectOptions(runId: string, enabled = true) {
  return queryOptions({
    queryKey: tasksKeys.inspectRun(runId),
    queryFn: ({ signal }) => inspectRun(runId, signal),
    staleTime: LIVE_STALE_TIME,
    refetchInterval: LIVE_REFETCH_INTERVAL,
    enabled: Boolean(runId) && enabled,
  });
}

export function taskDashboardOptions(filters: TaskDashboardFilter = {}, enabled = true) {
  return queryOptions({
    queryKey: tasksKeys.dashboard(filters),
    queryFn: ({ signal }) => getTaskDashboard(filters, signal),
    staleTime: DEFAULT_STALE_TIME,
    refetchInterval: DEFAULT_REFETCH_INTERVAL,
    enabled,
  });
}

function taskInboxQueryOptions(filters: TaskInboxFilter, enabled: boolean, staleTime: number) {
  const stableFilters = taskInboxStableFilter(filters);
  return infiniteQueryOptions({
    queryKey: tasksKeys.inbox(stableFilters),
    queryFn: ({ pageParam, signal }) =>
      getTaskInbox(taskInboxPageRequest(stableFilters, pageParam), signal),
    initialPageParam: INITIAL_CURSOR,
    getNextPageParam: lastPage => (lastPage.page.has_more ? lastPage.page.next_cursor : undefined),
    staleTime,
    enabled,
  });
}

export function taskInboxOptions(filters: TaskInboxFilter = {}, enabled = true) {
  return taskInboxQueryOptions(filters, enabled, 0);
}

/** Minimal unread/count projection preloaded for the always-visible Tasks badge. */
export function taskInboxBadgeOptions(filters: TaskInboxFilter = {}, enabled = true) {
  return taskInboxQueryOptions(filters, enabled, DEFAULT_STALE_TIME);
}

export function taskExecutionProfileOptions(taskId: string, enabled = true) {
  return queryOptions({
    queryKey: tasksKeys.profile(taskId),
    queryFn: ({ signal }) => getTaskExecutionProfile(taskId, signal),
    staleTime: DEFAULT_STALE_TIME,
    refetchInterval: DEFAULT_REFETCH_INTERVAL,
    enabled: Boolean(taskId) && enabled,
  });
}

export function taskRunReviewsOptions(
  runId: string,
  filters: TaskRunReviewsFilter = {},
  enabled = true
) {
  return queryOptions({
    queryKey: tasksKeys.reviewsByRun(runId, filters),
    queryFn: ({ signal }) => listTaskRunReviews(runId, filters, signal),
    staleTime: LIVE_STALE_TIME,
    refetchInterval: LIVE_REFETCH_INTERVAL,
    enabled: Boolean(runId) && enabled,
  });
}

export function taskReviewsOptions(
  taskId: string,
  filters: TaskReviewsFilter = {},
  enabled = true
) {
  return queryOptions({
    queryKey: tasksKeys.reviewsByTask(taskId, filters),
    queryFn: ({ signal }) => listTaskReviews(taskId, filters, signal),
    staleTime: LIVE_STALE_TIME,
    refetchInterval: LIVE_REFETCH_INTERVAL,
    enabled: Boolean(taskId) && enabled,
  });
}

export function taskRunReviewDetailOptions(reviewId: string, enabled = true) {
  return queryOptions({
    queryKey: tasksKeys.reviewDetail(reviewId),
    queryFn: ({ signal }) => getTaskRunReview(reviewId, signal),
    staleTime: LIVE_STALE_TIME,
    refetchInterval: LIVE_REFETCH_INTERVAL,
    enabled: Boolean(reviewId) && enabled,
  });
}

export function agentContextOptions(enabled = true) {
  return queryOptions({
    queryKey: tasksKeys.agentContext(),
    queryFn: ({ signal }) => getAgentContext(signal),
    staleTime: LIVE_STALE_TIME,
    refetchInterval: LIVE_REFETCH_INTERVAL,
    enabled,
  });
}

export function taskContextBundleOptions(enabled = true) {
  return queryOptions({
    queryKey: tasksKeys.contextBundle(),
    queryFn: ({ signal }) => getTaskContextBundle(signal),
    staleTime: LIVE_STALE_TIME,
    refetchInterval: LIVE_REFETCH_INTERVAL,
    enabled,
  });
}

export function taskBridgeNotificationSubscriptionsOptions(
  taskId: string,
  filters: TaskBridgeNotificationSubscriptionsFilter = {},
  enabled = true
) {
  return queryOptions({
    queryKey: tasksKeys.bridgeNotifications(taskId, filters),
    queryFn: ({ signal }) => listTaskBridgeNotificationSubscriptions(taskId, filters, signal),
    staleTime: DEFAULT_STALE_TIME,
    refetchInterval: DEFAULT_REFETCH_INTERVAL,
    enabled: Boolean(taskId) && enabled,
  });
}

export function taskBridgeNotificationSubscriptionOptions(
  taskId: string,
  subscriptionId: string,
  enabled = true
) {
  return queryOptions({
    queryKey: tasksKeys.bridgeNotification(taskId, subscriptionId),
    queryFn: ({ signal }) => getTaskBridgeNotificationSubscription(taskId, subscriptionId, signal),
    staleTime: DEFAULT_STALE_TIME,
    refetchInterval: DEFAULT_REFETCH_INTERVAL,
    enabled: Boolean(taskId) && Boolean(subscriptionId) && enabled,
  });
}
