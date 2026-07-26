package cli

import (
	"github.com/compozy/agh/internal/api/contract"
	automationpkg "github.com/compozy/agh/internal/automation"
	bridgepkg "github.com/compozy/agh/internal/bridges"
)

// AutomationJobQuery captures CLI filters for automation job list calls.
type AutomationJobQuery = automationpkg.JobListQuery

// AutomationJobListRecord is one structured automation job page.
type AutomationJobListRecord = contract.JobsResponse

// AutomationTriggerQuery captures CLI filters for automation trigger list calls.
type AutomationTriggerQuery = automationpkg.TriggerListQuery

// AutomationTriggerListRecord is one structured automation trigger page.
type AutomationTriggerListRecord = contract.TriggersResponse

// AutomationRunQuery captures CLI filters for automation run history calls.
type AutomationRunQuery = automationpkg.RunQuery

// AutomationJobCreateRequest captures the shared automation job create payload.
type AutomationJobCreateRequest = contract.CreateJobRequest

// AutomationJobUpdateRequest captures mutable automation job fields.
type AutomationJobUpdateRequest = contract.UpdateJobRequest

// AutomationTriggerCreateRequest captures the shared automation trigger create payload.
type AutomationTriggerCreateRequest = contract.CreateTriggerRequest

// AutomationTriggerUpdateRequest captures mutable automation trigger fields.
type AutomationTriggerUpdateRequest = contract.UpdateTriggerRequest

// JobRecord is the shared automation job payload.
type JobRecord = contract.JobPayload

// TriggerRecord is the shared automation trigger payload.
type TriggerRecord = contract.TriggerPayload

// RunRecord is the shared automation run payload.
type RunRecord = contract.RunPayload

// TaskSummaryRecord is the shared rich task summary payload.
type TaskSummaryRecord = contract.TaskSummaryPayload

// TaskCatalogItemRecord is one lean task catalog row.
type TaskCatalogItemRecord = contract.TaskCatalogItemPayload

// TaskRecord is the shared single-task payload.
type TaskRecord = contract.TaskPayload

// TaskBlockRecord is the shared task-block payload.
type TaskBlockRecord = contract.TaskBlockPayload

// TaskDetailRecord is the shared expanded task payload.
type TaskDetailRecord = contract.TaskDetailPayload

// TaskInspectRecord is the shared task/run inspect payload.
type TaskInspectRecord = contract.TaskInspectPayload

// TaskDependencyRecord is the shared dependency-edge payload.
type TaskDependencyRecord = contract.TaskDependencyPayload

// TaskRunRecord is the shared task-run payload.
type TaskRunRecord = contract.TaskRunPayload

// TaskRunDetailRecord is the shared expanded task-run payload.
type TaskRunDetailRecord = contract.TaskRunDetailPayload

// PauseTaskRequest captures the shared task-pause payload.
type PauseTaskRequest = contract.PauseTaskRequest

// ResumeTaskRequest captures the shared task-resume payload.
type ResumeTaskRequest = contract.ResumeTaskRequest

// RetryTaskRunRecord is the shared retry response payload.
type RetryTaskRunRecord = contract.RetryTaskRunResponse

// BulkForceTaskRunRecord is the shared bulk force-operation response payload.
type BulkForceTaskRunRecord = contract.BulkForceTaskRunResponse

// BulkForceTaskRunItemRecord records one bulk force-operation row.
type BulkForceTaskRunItemRecord = contract.BulkForceTaskRunItemPayload

// SchedulerStatusRecord is the shared scheduler status payload.
type SchedulerStatusRecord = contract.SchedulerStatusPayload

// SchedulerPauseRequest captures the shared scheduler pause payload.
type SchedulerPauseRequest = contract.SchedulerPauseRequest

// SchedulerResumeRequest captures the shared scheduler resume payload.
type SchedulerResumeRequest = contract.SchedulerResumeRequest

// SchedulerDrainRequest captures the shared scheduler drain payload.
type SchedulerDrainRequest = contract.SchedulerDrainRequest

// SchedulerDrainRecord is the shared scheduler drain response payload.
type SchedulerDrainRecord = contract.SchedulerDrainResponse

// SchedulerBacklogQuery captures scheduler backlog filters.
type SchedulerBacklogQuery = contract.SchedulerBacklogQuery

// SchedulerBacklogRecord is the shared scheduler backlog payload.
type SchedulerBacklogRecord = contract.SchedulerBacklogPayload

// TaskExecutionRecord is the shared task execution-boundary payload.
type TaskExecutionRecord = contract.TaskExecutionResponse

// TaskExecutionProfileRecord is the shared task execution profile payload.
type TaskExecutionProfileRecord = contract.TaskExecutionProfilePayload

// TaskExecutionProfileRequest captures a task execution profile replacement.
type TaskExecutionProfileRequest = contract.SetTaskExecutionProfileRequest

// TaskBridgeNotificationSubscriptionRecord is one task terminal bridge
// notification subscription payload.
type TaskBridgeNotificationSubscriptionRecord = contract.TaskBridgeNotificationSubscriptionPayload

// TaskBridgeNotificationSubscriptionRequest captures one task terminal bridge
// notification subscription request.
type TaskBridgeNotificationSubscriptionRequest = contract.CreateTaskBridgeNotificationSubscriptionRequest

// TaskBridgeNotificationSubscriptionQuery captures CLI filters for bridge
// terminal notification subscriptions.
type TaskBridgeNotificationSubscriptionQuery struct {
	BridgeInstanceID string
	Scope            bridgepkg.Scope
	WorkspaceID      string
	Limit            int
}

// TaskRunReviewRecord is the shared task-run review payload.
type TaskRunReviewRecord = contract.TaskRunReviewPayload

// TaskRunReviewRequest captures one task-run review request payload.
type TaskRunReviewRequest = contract.CreateTaskRunReviewRequest

// TaskRunReviewRequestRecord captures one task-run review request result.
type TaskRunReviewRequestRecord = contract.TaskRunReviewRequestResponse

// TaskRunReviewVerdictRequest captures one task-run review verdict payload.
type TaskRunReviewVerdictRequest = contract.SubmitTaskRunReviewVerdictRequest

// TaskRunReviewVerdictRecord captures one task-run review verdict result.
type TaskRunReviewVerdictRecord = contract.TaskRunReviewVerdictResponse

// AgentMeRecord is the shared agent caller identity payload.
type AgentMeRecord = contract.AgentMePayload

// AgentContextRecord is the shared bounded agent situation payload.
type AgentContextRecord = contract.AgentContextPayload

// AgentSpawnRequest captures one bounded child-session spawn request.
type AgentSpawnRequest = contract.AgentSpawnRequest

// AgentSpawnRecord is the stable child-session spawn response projection.
type AgentSpawnRecord = contract.AgentSpawnPayload

// SpawnPermissionPolicyRecord captures concrete spawn permission atoms.
type SpawnPermissionPolicyRecord = contract.SpawnPermissionPolicyPayload

// AgentChannelRecord is one discoverable coordination channel payload.
type AgentChannelRecord = contract.CoordinationChannelPayload

// AgentChannelMessageRecord is one safe agent channel message payload.
type AgentChannelMessageRecord = contract.AgentChannelMessagePayload

// AgentChannelSendRequest captures one agent channel send payload.
type AgentChannelSendRequest = contract.AgentChannelSendRequest

// AgentChannelReplyRequest captures one agent channel reply payload.
type AgentChannelReplyRequest = contract.AgentChannelReplyRequest

// AgentTaskClaimNextRequest captures one agent next-work request.
type AgentTaskClaimNextRequest = contract.AgentTaskClaimNextRequest

// AgentTaskHeartbeatRequest captures one agent lease heartbeat request.
type AgentTaskHeartbeatRequest = contract.AgentTaskHeartbeatRequest

// AgentTaskCompleteRequest captures one agent lease completion request.
type AgentTaskCompleteRequest = contract.AgentTaskCompleteRequest

// AgentTaskFailRequest captures one agent lease failure request.
type AgentTaskFailRequest = contract.AgentTaskFailRequest

// AgentTaskReleaseRequest captures one agent lease release request.
type AgentTaskReleaseRequest = contract.AgentTaskReleaseRequest

// AgentTaskClaimRecord is the synchronous claim payload returned to agents.
type AgentTaskClaimRecord = contract.AgentTaskClaimPayload

// AgentTaskLeaseRecord is the safe lease payload returned by agent lease mutations.
type AgentTaskLeaseRecord = contract.TaskRunLeaseSummaryPayload

// AgentTaskNextRecord is the stable CLI/client wrapper for next-work polling.
type AgentTaskNextRecord struct {
	Claimed bool                  `json:"claimed"`
	Claim   *AgentTaskClaimRecord `json:"claim,omitempty"`
}

// AgentChannelRecvQuery captures receive options for agent channel messages.
type AgentChannelRecvQuery struct {
	Wait  bool
	Limit int
}

// TaskEventRecord is the shared task audit-event payload.
type TaskEventRecord = contract.TaskEventPayload

// TaskListQuery captures CLI filters for task list calls.
type TaskListQuery = contract.TaskListQuery

// TaskListRecord is the counted task catalog response.
type TaskListRecord = contract.TasksResponse

// TaskRunListQuery captures CLI filters for task-run list calls.
type TaskRunListQuery = contract.TaskRunListQuery

// TaskRunReviewListQuery captures CLI filters for task-run review list calls.
type TaskRunReviewListQuery = contract.TaskRunReviewListQuery

// CreateTaskRequest captures the shared task-create payload.
type CreateTaskRequest = contract.CreateTaskRequest

// CreateTaskChildRequest captures the shared child-task create payload.
type CreateTaskChildRequest = contract.CreateTaskChildRequest

// CreateTaskBlockRequest captures one task-block create payload.
type CreateTaskBlockRequest = contract.CreateTaskBlockRequest

// ClearTaskBlockRequest captures one task-block clear payload.
type ClearTaskBlockRequest = contract.ClearTaskBlockRequest

// RecoverTaskRequest captures one task-level recovery payload.
type RecoverTaskRequest = contract.RecoverTaskRequest

// UpdateTaskRequest captures mutable task fields.
type UpdateTaskRequest = contract.UpdateTaskRequest

// CancelTaskRequest captures the shared task-cancel payload.
type CancelTaskRequest = contract.CancelTaskRequest

// TaskExecutionRequest captures the shared task publish/start/approval payload.
type TaskExecutionRequest = contract.TaskExecutionRequest

// AddTaskDependencyRequest captures the shared dependency-create payload.
type AddTaskDependencyRequest = contract.AddTaskDependencyRequest

// EnqueueTaskRunRequest captures the shared run-enqueue payload.
type EnqueueTaskRunRequest = contract.EnqueueTaskRunRequest

// FanOutTaskRunsRequest captures designated sibling run enqueue inputs.
type FanOutTaskRunsRequest = contract.FanOutTaskRunsRequest

// FanOutTaskRunsRecord captures designated sibling run enqueue results.
type FanOutTaskRunsRecord = contract.FanOutTaskRunsResponse

// StartTaskRunRequest captures the shared run-start payload.
type StartTaskRunRequest = contract.StartTaskRunRequest

// AttachTaskRunSessionRequest captures the shared run-session attach payload.
type AttachTaskRunSessionRequest = contract.AttachTaskRunSessionRequest

// CompleteTaskRunRequest captures the shared run-complete payload.
type CompleteTaskRunRequest = contract.CompleteTaskRunRequest

// FailTaskRunRequest captures the shared run-fail payload.
type FailTaskRunRequest = contract.FailTaskRunRequest

// ForceReleaseTaskRunRequest captures the shared force-release request payload.
type ForceReleaseTaskRunRequest = contract.ForceReleaseTaskRunRequest

// ForceFailTaskRunRequest captures the shared forced-failure request payload.
type ForceFailTaskRunRequest = contract.ForceFailTaskRunRequest

// RetryTaskRunRequest captures the shared retry request payload.
type RetryTaskRunRequest = contract.RetryTaskRunRequest

// RecoverTaskRunRequest captures the shared recovery request payload.
type RecoverTaskRunRequest = contract.RecoverTaskRunRequest

// BulkForceTaskRunRequest captures the shared bounded bulk force-operation payload.
type BulkForceTaskRunRequest = contract.BulkForceTaskRunRequest

// CancelTaskRunRequest captures the shared run-cancel payload.
type CancelTaskRunRequest = contract.CancelTaskRunRequest
