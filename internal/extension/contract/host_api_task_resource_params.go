package contract

import (
	"encoding/json"

	apicontract "github.com/compozy/agh/internal/api/contract"

	bridgepkg "github.com/compozy/agh/internal/bridges/contract"

	"github.com/compozy/agh/internal/resources"
)

// TasksParams filters visible tasks.
type TasksParams = apicontract.TaskListQuery

// TaskTargetParams identifies one task by id.
type TaskTargetParams struct {
	ID string `json:"id"`
}

// TaskTimelineParams queries one task timeline by task id.
type TaskTimelineParams struct {
	ID string `json:"id"`
	apicontract.TaskTimelineQuery
}

// TaskTreeParams queries one task tree by task id.
type TaskTreeParams = TaskTargetParams

// TaskDashboardParams filters observer-backed task dashboard reads.
type TaskDashboardParams = apicontract.TaskDashboardQuery

// TaskInboxParams filters observer-backed task inbox reads.
type TaskInboxParams = apicontract.TaskInboxQuery

// TaskCreateParams creates one task.
type TaskCreateParams = apicontract.CreateTaskRequest

// TaskUpdateParams patches one task.
type TaskUpdateParams struct {
	ID string `json:"id"`
	apicontract.UpdateTaskRequest
}

// TaskCancelParams requests cancellation for one task.
type TaskCancelParams struct {
	ID string `json:"id"`
	apicontract.CancelTaskRequest
}

// TaskRunsParams filters runs for one task.
type TaskRunsParams struct {
	ID string `json:"id"`
	apicontract.TaskRunListQuery
}

// TaskRunGetParams identifies one task run by id for richer detail reads.
type TaskRunGetParams struct {
	ID string `json:"id"`
}

// TaskRunEnqueueParams enqueues one run for a task.
type TaskRunEnqueueParams struct {
	TaskID string `json:"task_id"`
	apicontract.EnqueueTaskRunRequest
}

// TaskRunStartParams starts one claimed run.
type TaskRunStartParams struct {
	ID string `json:"id"`
	apicontract.StartTaskRunRequest
}

// TaskRunAttachSessionParams attaches one existing session to a run.
type TaskRunAttachSessionParams struct {
	ID string `json:"id"`
	apicontract.AttachTaskRunSessionRequest
}

// TaskRunCompleteParams completes one run.
type TaskRunCompleteParams struct {
	ID string `json:"id"`
	apicontract.CompleteTaskRunRequest
}

// TaskRunFailParams fails one run.
type TaskRunFailParams struct {
	ID string `json:"id"`
	apicontract.FailTaskRunRequest
}

// TaskRunCancelParams cancels one run.
type TaskRunCancelParams struct {
	ID string `json:"id"`
	apicontract.CancelTaskRunRequest
}

// ResourcesListParams filters same-source resource visibility for one extension actor.
type ResourcesListParams struct {
	Kind  resources.ResourceKind   `json:"kind,omitempty"`
	Scope *resources.ResourceScope `json:"scope,omitempty"`
	Limit int                      `json:"limit,omitempty"`
}

// ResourceGetParams identifies one canonical resource record by kind and id.
type ResourceGetParams struct {
	Kind resources.ResourceKind `json:"kind"`
	ID   string                 `json:"id"`
}

// ResourceSnapshotRecord carries one snapshot-authored resource definition.
type ResourceSnapshotRecord struct {
	Kind  resources.ResourceKind  `json:"kind"`
	ID    string                  `json:"id"`
	Scope resources.ResourceScope `json:"scope"`
	Spec  json.RawMessage         `json:"spec"`
}

// ResourcesSnapshotParams replaces one extension source snapshot.
type ResourcesSnapshotParams struct {
	SourceVersion int64                    `json:"source_version"`
	Records       []ResourceSnapshotRecord `json:"records"`
}

// BridgesMessagesIngestParams carries one normalized inbound bridge message.
type BridgesMessagesIngestParams = bridgepkg.InboundMessageEnvelope

// BridgeInstanceTargetParams identifies one provider-owned bridge instance.
type BridgeInstanceTargetParams = bridgepkg.BridgeInstanceTargetParams

// BridgesInstancesReportStateParams reports one adapter-observed instance status update.
type BridgesInstancesReportStateParams = bridgepkg.BridgesInstancesReportStateParams
