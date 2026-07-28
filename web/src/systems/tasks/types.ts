import type { OperationQuery, OperationRequestBody, OperationResponse } from "@/lib/api-contract";

export type TaskListItem = OperationResponse<"listTasks", 200>["tasks"][number];
export type TaskListPage = OperationResponse<"listTasks", 200>;
export type TaskListFacets = TaskListPage["facets"];
export type TaskDetailView = OperationResponse<"getTask", 200>["task"];
export type TaskSummary = TaskDetailView["summary"];
export type TaskRecord = TaskDetailView["task"];
export type TaskChildSummary = NonNullable<TaskDetailView["children"]>[number];
export type TaskRun = OperationResponse<"listTaskRuns", 200>["runs"][number];
type TaskRunDetailResponseView = OperationResponse<"getTaskRun", 200>["run"];
export type TaskRunDetailView = TaskRunDetailResponseView;
export type BulkForceTaskRunResult = OperationResponse<"bulkForceReleaseTaskRuns", 200>;
export type TaskInspectView = OperationResponse<"inspectTask", 200>["inspect"];
export type TaskRunInspectView = OperationResponse<"inspectRun", 200>["inspect"];
export type TaskRunLeaseSummary = OperationResponse<"claimNextAgentTask", 200>["claim"]["lease"];
export type TaskTimelineItem = OperationResponse<"getTaskTimeline", 200>["timeline"][number];
export type TaskTreeView = OperationResponse<"getTaskTree", 200>["tree"];
export type TaskTreeNode = TaskTreeView["root"];
export type TaskDashboardView = OperationResponse<"getTaskDashboard", 200>["dashboard"];
export type TaskInboxView = OperationResponse<"getTaskInbox", 200>["inbox"];
export type TaskInboxPage = TaskInboxView;
export type TaskInboxGroup = NonNullable<TaskInboxView["groups"]>[number];
export type TaskInboxItem = NonNullable<TaskInboxGroup["items"]>[number];
export type TaskTriageState = OperationResponse<"markTaskRead", 200>["triage"];
export type TaskStreamPayload = OperationResponse<"streamTask", 200>;
export type TaskStreamTimelineEvent = TaskStreamPayload["timeline"];
export type AgentTaskClaim = OperationResponse<"claimNextAgentTask", 200>["claim"];
export type AgentCoordinationChannel = OperationResponse<
  "listAgentChannels",
  200
>["channels"][number];
export type AgentChannelMessage = OperationResponse<
  "receiveAgentChannelMessages",
  200
>["messages"][number];

export type TaskListFilter = OperationQuery<"listTasks">;
export type TaskListStableFilter = Omit<TaskListFilter, "cursor">;
export type TaskDashboardFilter = OperationQuery<"getTaskDashboard">;
export type TaskInboxFilter = OperationQuery<"getTaskInbox">;
export type TaskInboxStableFilter = Omit<TaskInboxFilter, "cursor">;
export type TaskRunsFilter = OperationQuery<"listTaskRuns">;
export type TaskTimelineFilter = OperationQuery<"getTaskTimeline">;
export type TaskStreamFilter = OperationQuery<"streamTask">;

// Execution profile (typed overlay)
export type TaskExecutionProfile = OperationResponse<"getTaskExecutionProfile", 200>["profile"];
export type TaskExecutionProfileSetRequest = OperationRequestBody<"setTaskExecutionProfile">;
export type TaskExecutionProfileWorker = TaskExecutionProfile["worker"];
export type TaskExecutionProfileCoordinator = TaskExecutionProfile["coordinator"];
export type TaskExecutionProfileReviewSelectors = TaskExecutionProfile["review"];
export type TaskExecutionProfileSandbox = TaskExecutionProfile["sandbox"];
export type TaskExecutionProfileParticipants = TaskExecutionProfile["participants"];
export type TaskExecutionProfileWorkerMode = TaskExecutionProfileWorker["mode"];
export type TaskExecutionProfileCoordinatorMode = TaskExecutionProfileCoordinator["mode"];
export type TaskExecutionProfileSandboxMode = TaskExecutionProfileSandbox["mode"];

// Run reviews (review gate)
export type TaskRunReview = OperationResponse<"listTaskRunReviews", 200>["reviews"][number];
export type TaskRunReviewsFilter = OperationQuery<"listTaskRunReviews">;
export type TaskReviewsFilter = OperationQuery<"listTaskReviews">;
export type TaskRunReviewRequest = OperationRequestBody<"requestTaskRunReview">;
export type TaskRunReviewRequestResult = OperationResponse<"requestTaskRunReview", 200>;
export type TaskRunReviewVerdictRequest = OperationRequestBody<"submitTaskRunReviewVerdict">;
export type TaskRunReviewVerdict = TaskRunReviewVerdictRequest["verdict"];
export type TaskRunReviewVerdictResult = OperationResponse<"submitTaskRunReviewVerdict", 200>;
export type TaskRunReviewStatus = TaskRunReview["status"];
export type TaskRunReviewOutcome = NonNullable<TaskRunReview["outcome"]>;
export type TaskRunReviewPolicy = TaskRunReview["policy"];
export type TaskRunReviewContinuationRun = NonNullable<
  TaskRunReviewVerdictResult["continuation_run"]
>;

// Bridge notification diagnostics (cursor primitive + bridge subscriptions)
export type TaskBridgeNotificationSubscription = OperationResponse<
  "listTaskBridgeNotificationSubscriptions",
  200
>["subscriptions"][number];
export type TaskBridgeNotificationCursor = TaskBridgeNotificationSubscription["cursor"];
export type TaskBridgeNotificationSubscriptionsFilter =
  OperationQuery<"listTaskBridgeNotificationSubscriptions">;
export type TaskBridgeNotificationSubscriptionCreateRequest =
  OperationRequestBody<"createTaskBridgeNotificationSubscription">;
export type TaskBridgeNotificationDeliveryMode =
  TaskBridgeNotificationSubscription["delivery_mode"];
export type TaskBridgeNotificationSubscriptionScope = TaskBridgeNotificationSubscription["scope"];

// Agent task context bundle (current_run + execution profile + replay seed)
export type AgentContextView = OperationResponse<"getAgentContext", 200>["context"];
export type AgentTaskContextSection = AgentContextView["task"];
export type TaskContextBundle = NonNullable<AgentTaskContextSection["bundle"]>;
export type TaskContextRecentEvent = TaskContextBundle["recent_events"][number];
export type TaskContextReviewHistoryEntry = TaskContextBundle["review_history"][number];
export type TaskContextReviewContinuation = NonNullable<TaskContextBundle["review_continuation"]>;
export type TaskContextCurrentRun = NonNullable<TaskContextBundle["current_run"]>;
export type TaskContextPriorAttempt = TaskContextBundle["prior_attempts"][number];

export type CreateTaskRequest = OperationRequestBody<"createTask">;
export type UpdateTaskRequest = OperationRequestBody<"updateTask">;
export type CancelTaskRequest = OperationRequestBody<"cancelTask">;
export type CreateChildTaskRequest = OperationRequestBody<"createChildTask">;
export type AddTaskDependencyRequest = OperationRequestBody<"addTaskDependency">;
export type EnqueueTaskRunRequest = OperationRequestBody<"enqueueTaskRun">;
export type AttachTaskRunSessionRequest = OperationRequestBody<"attachTaskRunSession">;
export type CancelTaskRunRequest = OperationRequestBody<"cancelTaskRun">;
export type CompleteTaskRunRequest = OperationRequestBody<"completeTaskRun">;
export type FailTaskRunRequest = OperationRequestBody<"failTaskRun">;
export type ForceReleaseTaskRunRequest = OperationRequestBody<"forceReleaseTaskRun">;
export type ForceFailTaskRunRequest = OperationRequestBody<"forceFailTaskRun">;
export type RecoverTaskRunRequest = OperationRequestBody<"recoverTaskRun">;
export type RecoverTaskRunResult = OperationResponse<"recoverTaskRun", 201>;
export type RetryTaskRunRequest = OperationRequestBody<"retryTaskRun">;
export type RetryTaskRunResult = OperationResponse<"retryTaskRun", 201>;
export type FanOutTaskRunsRequest = OperationRequestBody<"fanOutTaskRuns">;
export type FanOutTaskRunsResponse = OperationResponse<"fanOutTaskRuns", 201>;
export type PauseTaskRequest = OperationRequestBody<"pauseTask">;
export type ResumeTaskRequest = OperationRequestBody<"resumeTask">;
export type RecoverTaskRequest = OperationRequestBody<"recoverTask">;
export type BulkForceTaskRunRequest = OperationRequestBody<"bulkForceReleaseTaskRuns">;
export type StartTaskRunRequest = OperationRequestBody<"startTaskRun">;

export type TaskStatus = TaskRecord["status"];
export type TaskBlockedReason = NonNullable<TaskRecord["blocked_reasons"]>[number];
export type TaskBlockedReasonSource = TaskBlockedReason["source"];
export type TaskBlockKind = NonNullable<TaskBlockedReason["kind"]>;
export type TaskPriority = NonNullable<TaskRecord["priority"]>;
export type TaskScope = TaskRecord["scope"];
export type TaskApprovalPolicy = NonNullable<TaskRecord["approval_policy"]>;
export type TaskApprovalState = NonNullable<TaskRecord["approval_state"]>;
export type TaskOwnerKind = NonNullable<NonNullable<TaskRecord["owner"]>["kind"]>;
export type TaskRunStatus = TaskRun["status"];
export type TaskInboxLane = TaskInboxItem["lane"];

export type TaskViewMode = "list" | "kanban" | "dashboard" | "inbox";

/** Sort dimension for the tasks list surface toolbar. */
export type TaskListSortKey = "recent" | "priority";
