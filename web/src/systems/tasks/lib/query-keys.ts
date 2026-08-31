import type {
  AgentContextIdentity,
  TaskBridgeNotificationSubscriptionsFilter,
  TaskDashboardFilter,
  TaskInboxFilter,
  TaskListFilter,
  TaskReviewsFilter,
  TaskRunReviewsFilter,
  TaskRunsFilter,
  TaskStreamFilter,
  TaskTimelineFilter,
} from "../types";
import { taskInboxStableFilter } from "./task-inbox-query";
import { taskListStableFilter } from "./task-list-query";
import { PROFILE_AGGREGATE, type ProfileScopeParams } from "@/systems/profiles";

function normalizeText(value?: string | null): string {
  return typeof value === "string" ? value : "";
}

function normalizeFlag(value?: boolean): string {
  return value === undefined ? "" : value ? "1" : "0";
}

function normalizeNumber(value?: number): string {
  return value === undefined ? "" : String(value);
}

function profileLens(scope: { profile?: string | null; all_profiles?: boolean | null }): string {
  return scope.all_profiles ? PROFILE_AGGREGATE : normalizeText(scope.profile);
}

export const tasksKeys = {
  all: ["tasks"] as const,

  lists: () => [...tasksKeys.all, "list"] as const,
  list: (filters: TaskListFilter = {}) => {
    const normalized = taskListStableFilter(filters);
    return [
      ...tasksKeys.lists(),
      normalizeText(normalized.scope),
      normalizeText(normalized.workspace),
      // Part of the key, not just the request: two window scopes must not share
      // one cache entry, or one window's list would serve another's.
      normalizeText(normalized.worktree),
      normalizeText(normalized.status),
      normalizeText(normalized.priority),
      normalizeFlag(normalized.include_drafts),
      // Part of the key, not just the request: a revealed list and a calm list
      // are different populations with different facets and cursors, so they
      // must never share one cache entry.
      normalizeFlag(normalized.include_loop),
      normalizeText(normalized.loop_run_id),
      normalizeText(normalized.approval_state),
      normalizeText(normalized.owner_kind),
      normalizeText(normalized.owner_ref),
      normalizeText(normalized.parent_task_id),
      normalizeText(normalized.participation_channel),
      normalizeText(normalized.query),
      normalizeText(normalized.sort),
      normalizeNumber(normalized.limit),
      profileLens(normalized),
    ] as const;
  },

  details: () => [...tasksKeys.all, "detail"] as const,
  detail: (id: string, scope?: ProfileScopeParams) =>
    scope === undefined
      ? ([...tasksKeys.details(), id] as const)
      : ([...tasksKeys.details(), id, profileLens(scope)] as const),

  inspectRoot: () => [...tasksKeys.all, "inspect"] as const,
  inspectTask: (id: string) => [...tasksKeys.inspectRoot(), "task", id] as const,
  inspectRun: (runId: string) => [...tasksKeys.inspectRoot(), "run", runId] as const,

  runsRoot: () => [...tasksKeys.all, "runs"] as const,
  runs: (id: string, filters: TaskRunsFilter = {}) =>
    [
      ...tasksKeys.runsRoot(),
      id,
      normalizeText(filters.status),
      normalizeText(filters.session_id),
      normalizeNumber(filters.limit),
    ] as const,

  timelineRoot: () => [...tasksKeys.all, "timeline"] as const,
  timeline: (id: string, filters: TaskTimelineFilter = {}) =>
    [
      ...tasksKeys.timelineRoot(),
      id,
      normalizeNumber(filters.after_sequence),
      normalizeNumber(filters.limit),
    ] as const,

  treeRoot: () => [...tasksKeys.all, "tree"] as const,
  tree: (id: string) => [...tasksKeys.treeRoot(), id] as const,

  runDetails: () => [...tasksKeys.all, "run-detail"] as const,
  runDetail: (runId: string) => [...tasksKeys.runDetails(), runId] as const,

  runResults: () => [...tasksKeys.all, "run-result"] as const,
  runResult: (
    workspaceId: string,
    runId: string,
    resultRef: string,
    offset: number,
    limit: number
  ) => [...tasksKeys.runResults(), workspaceId, runId, resultRef, offset, limit] as const,

  dashboardRoot: () => [...tasksKeys.all, "dashboard"] as const,
  dashboard: (filters: TaskDashboardFilter = {}) =>
    [
      ...tasksKeys.dashboardRoot(),
      normalizeText(filters.scope),
      normalizeText(filters.workspace),
      normalizeText(filters.worktree),
      normalizeText(filters.owner_kind),
      normalizeText(filters.owner_ref),
      normalizeText(filters.participation_channel),
      normalizeText(filters.origin_kind),
      // The profile axis: a dashboard scoped to one profile is not the machine's.
      profileLens(filters),
    ] as const,

  inboxRoot: () => [...tasksKeys.all, "inbox"] as const,
  inbox: (filters: TaskInboxFilter = {}) => {
    const normalized = taskInboxStableFilter(filters);
    return [
      ...tasksKeys.inboxRoot(),
      normalizeText(normalized.scope),
      normalizeText(normalized.workspace),
      normalizeText(normalized.worktree),
      normalizeText(normalized.owner_kind),
      normalizeText(normalized.owner_ref),
      normalizeText(normalized.lane),
      normalizeText(normalized.status),
      normalizeText(normalized.priority),
      normalizeFlag(normalized.unread),
      normalizeText(normalized.query),
      normalizeNumber(normalized.limit),
      profileLens(normalized),
    ] as const;
  },

  triageRoot: () => [...tasksKeys.all, "triage"] as const,

  // Execution profile (per-task overlay)
  profilesRoot: () => [...tasksKeys.all, "profile"] as const,
  profile: (taskId: string) => [...tasksKeys.profilesRoot(), taskId] as const,

  // Review state
  reviewsRoot: () => [...tasksKeys.all, "reviews"] as const,
  reviewsByRun: (runId: string, filters: TaskRunReviewsFilter = {}) =>
    [
      ...tasksKeys.reviewsRoot(),
      "run",
      runId,
      normalizeText(filters.status),
      normalizeText(filters.reviewer_session_id),
      normalizeNumber(filters.limit),
    ] as const,
  reviewsByTask: (taskId: string, filters: TaskReviewsFilter = {}) =>
    [
      ...tasksKeys.reviewsRoot(),
      "task",
      taskId,
      normalizeText(filters.status),
      normalizeText(filters.reviewer_session_id),
      normalizeNumber(filters.limit),
    ] as const,
  reviewDetail: (reviewId: string) => [...tasksKeys.reviewsRoot(), "detail", reviewId] as const,

  // Agent task context bundle (extracted from /api/agent/context)
  agentContextsRoot: () => [...tasksKeys.all, "agent-context"] as const,
  agentContext: (identity: AgentContextIdentity) =>
    [...tasksKeys.agentContextsRoot(), identity.sessionId, identity.agentName] as const,

  // SSE stream metadata (resume seed reflects after_sequence + last-event-id intent)
  streamsRoot: () => [...tasksKeys.all, "stream"] as const,
  stream: (taskId: string, filters: TaskStreamFilter = {}) =>
    [...tasksKeys.streamsRoot(), taskId, normalizeNumber(filters.after_sequence)] as const,

  // Bridge notification diagnostics
  bridgeNotificationsRoot: () => [...tasksKeys.all, "bridge-notifications"] as const,
  bridgeNotifications: (taskId: string, filters: TaskBridgeNotificationSubscriptionsFilter = {}) =>
    [
      ...tasksKeys.bridgeNotificationsRoot(),
      taskId,
      filters.bridge_instance_id ?? null,
      normalizeText(filters.scope),
      filters.workspace_id ?? null,
      normalizeNumber(filters.limit),
    ] as const,
  bridgeNotification: (taskId: string, subscriptionId: string) =>
    [...tasksKeys.bridgeNotificationsRoot(), taskId, "detail", subscriptionId] as const,
};
