package contract

import (
	"encoding/json"
	"time"

	"github.com/compozy/agh/internal/network/participation"
	taskpkg "github.com/compozy/agh/internal/task"
)

// CancelTaskRequest is the shared task-cancel request payload.
type CancelTaskRequest struct {
	Reason   string          `json:"reason,omitempty"`
	Metadata json.RawMessage `json:"metadata,omitempty"`
}

// AddTaskDependencyRequest is the shared dependency-create request payload.
type AddTaskDependencyRequest struct {
	DependsOnTaskID string                 `json:"depends_on_task_id"`
	Kind            taskpkg.DependencyKind `json:"kind,omitempty"`
}

// EnqueueTaskRunRequest is the shared run-enqueue request payload.
type EnqueueTaskRunRequest struct {
	IdempotencyKey       string                 `json:"idempotency_key,omitempty"`
	NetworkParticipation *participation.Request `json:"network_participation,omitempty"`
	Metadata             json.RawMessage        `json:"metadata,omitempty"`
}

// CreateTaskBlockRequest captures one task-block create request.
type CreateTaskBlockRequest struct {
	Kind      taskpkg.BlockKind `json:"kind"`
	Reason    string            `json:"reason"`
	Details   json.RawMessage   `json:"details,omitempty"`
	ExpiresAt *time.Time        `json:"expires_at,omitempty"`
	RunID     string            `json:"run_id,omitempty"`
}

// ClearTaskBlockRequest captures one task-block clear request.
type ClearTaskBlockRequest struct {
	Note string `json:"note,omitempty"`
}

// RecoverTaskRequest captures one task-level needs_attention recovery request.
type RecoverTaskRequest struct {
	Note string `json:"note,omitempty"`
}

// TaskExecutionRequest is the shared task publish/start/approval execution payload.
type TaskExecutionRequest struct {
	IdempotencyKey       string                 `json:"idempotency_key,omitempty"`
	NetworkParticipation *participation.Request `json:"network_participation,omitempty"`
	Metadata             json.RawMessage        `json:"metadata,omitempty"`
}

// StartTaskRunRequest is the shared run-start request payload.
type StartTaskRunRequest struct {
	IdempotencyKey string `json:"idempotency_key,omitempty"`
}

// AttachTaskRunSessionRequest is the shared run-session attach request payload.
type AttachTaskRunSessionRequest struct {
	SessionID string `json:"session_id"`
}

// CompleteTaskRunRequest is the shared run-complete request payload.
type CompleteTaskRunRequest struct {
	Result         json.RawMessage `json:"result,omitempty"`
	CreatedTaskIDs []string        `json:"created_task_ids,omitempty"`
}

// FailTaskRunRequest is the shared run-fail request payload.
type FailTaskRunRequest struct {
	Error    string          `json:"error"`
	Metadata json.RawMessage `json:"metadata,omitempty"`
}

// ForceReleaseTaskRunRequest is the shared force-release request payload.
type ForceReleaseTaskRunRequest struct {
	Reason   string          `json:"reason,omitempty"`
	Metadata json.RawMessage `json:"metadata,omitempty"`
}

// ForceFailTaskRunRequest is the shared forced-failure request payload.
type ForceFailTaskRunRequest struct {
	Reason   string          `json:"reason"`
	Metadata json.RawMessage `json:"metadata,omitempty"`
}

// RetryTaskRunRequest is the shared retry request payload.
type RetryTaskRunRequest struct {
	Metadata json.RawMessage `json:"metadata,omitempty"`
}

// RecoverTaskRunRequest is the shared recovery request payload for a needs_attention run.
type RecoverTaskRunRequest struct {
	Reason   string          `json:"reason,omitempty"`
	Metadata json.RawMessage `json:"metadata,omitempty"`
}

// PauseTaskRequest captures one per-task pause request.
type PauseTaskRequest struct {
	Reason   string          `json:"reason"`
	Metadata json.RawMessage `json:"metadata,omitempty"`
}

// ResumeTaskRequest captures one per-task resume request.
type ResumeTaskRequest struct {
	Metadata json.RawMessage `json:"metadata,omitempty"`
}

// BulkForceTaskRunRequest is the shared bounded bulk force-operation payload.
type BulkForceTaskRunRequest struct {
	RunIDs   []string        `json:"run_ids"`
	Reason   string          `json:"reason,omitempty"`
	Metadata json.RawMessage `json:"metadata,omitempty"`
}

// BulkForceTaskRunItemPayload records one per-row bulk force-operation outcome.
type BulkForceTaskRunItemPayload struct {
	RunID string          `json:"run_id"`
	OK    bool            `json:"ok"`
	Run   *TaskRunPayload `json:"run,omitempty"`
	Error *ErrorPayload   `json:"error,omitempty"`
}

// CancelTaskRunRequest is the shared run-cancel request payload.
type CancelTaskRunRequest struct {
	Reason   string          `json:"reason,omitempty"`
	Metadata json.RawMessage `json:"metadata,omitempty"`
}
