package daemon

import (
	"bytes"

	"encoding/json"

	"fmt"

	"strings"
	"time"

	"github.com/compozy/agh/internal/api/contract"

	"github.com/compozy/agh/internal/network/participation"

	taskpkg "github.com/compozy/agh/internal/task"
	toolspkg "github.com/compozy/agh/internal/tools"
)

type taskCreateInput struct {
	ID                   string                 `json:"id,omitempty"`
	Identifier           string                 `json:"identifier,omitempty"`
	Scope                string                 `json:"scope"`
	WorkspaceID          string                 `json:"workspace_id,omitempty"`
	NetworkParticipation *participation.Request `json:"network_participation,omitempty"`
	Title                string                 `json:"title"`
	Description          string                 `json:"description,omitempty"`
	Priority             string                 `json:"priority,omitempty"`
	MaxAttempts          *int                   `json:"max_attempts,omitempty"`
	Draft                bool                   `json:"draft,omitempty"`
	ApprovalPolicy       string                 `json:"approval_policy,omitempty"`
	Owner                *taskpkg.Ownership     `json:"owner,omitempty"`
	WakeCreator          *bool                  `json:"wake_creator,omitempty"`
	Metadata             json.RawMessage        `json:"metadata,omitempty"`
}

func (i taskCreateInput) spec(scope toolspkg.Scope) taskpkg.CreateTask {
	taskScope := taskpkg.Scope(strings.TrimSpace(i.Scope))
	workspaceID := strings.TrimSpace(i.WorkspaceID)
	if workspaceID == "" && taskScope.Normalize() == taskpkg.ScopeWorkspace {
		workspaceID = strings.TrimSpace(scope.WorkspaceID)
	}
	return taskpkg.CreateTask{
		ID:                   strings.TrimSpace(i.ID),
		Identifier:           strings.TrimSpace(i.Identifier),
		Scope:                taskScope,
		WorkspaceID:          workspaceID,
		Title:                strings.TrimSpace(i.Title),
		Description:          strings.TrimSpace(i.Description),
		Priority:             taskpkg.Priority(strings.TrimSpace(i.Priority)),
		MaxAttempts:          cloneIntPtr(i.MaxAttempts),
		Draft:                i.Draft,
		ApprovalPolicy:       taskpkg.ApprovalPolicy(strings.TrimSpace(i.ApprovalPolicy)),
		Owner:                cloneTaskOwner(i.Owner),
		WakeCreator:          cloneBoolPtr(i.WakeCreator),
		NetworkParticipation: participation.CloneRequest(i.NetworkParticipation),
		Metadata:             cloneJSON(i.Metadata),
	}
}

type taskChildCreateInput struct {
	ParentTaskID string `json:"parent_task_id"`
	taskCreateInput
}

func (i taskChildCreateInput) spec(scope toolspkg.Scope) taskpkg.CreateTask {
	spec := i.taskCreateInput.spec(scope)
	spec.ParentTaskID = strings.TrimSpace(i.ParentTaskID)
	return spec
}

type taskUpdateInput struct {
	TaskID               string                 `json:"task_id"`
	Title                *string                `json:"title,omitempty"`
	Description          *string                `json:"description,omitempty"`
	Priority             *string                `json:"priority,omitempty"`
	MaxAttempts          *int                   `json:"max_attempts,omitempty"`
	ApprovalPolicy       *string                `json:"approval_policy,omitempty"`
	Metadata             *json.RawMessage       `json:"metadata,omitempty"`
	NetworkParticipation *participation.Request `json:"network_participation,omitempty"`
	Owner                *taskpkg.Ownership     `json:"owner,omitempty"`
	ClearOwner           bool                   `json:"clear_owner,omitempty"`
}

func (i taskUpdateInput) patch() taskpkg.Patch {
	return taskpkg.Patch{
		Title:                cloneStringPtr(i.Title),
		Description:          cloneStringPtr(i.Description),
		Priority:             taskPriorityPtr(i.Priority),
		MaxAttempts:          cloneIntPtr(i.MaxAttempts),
		ApprovalPolicy:       taskApprovalPolicyPtr(i.ApprovalPolicy),
		Metadata:             cloneRawMessagePtr(i.Metadata),
		Owner:                cloneTaskOwner(i.Owner),
		ClearOwner:           i.ClearOwner,
		NetworkParticipation: participation.CloneRequest(i.NetworkParticipation),
	}
}

type taskCancelInput struct {
	TaskID   string          `json:"task_id"`
	Reason   string          `json:"reason,omitempty"`
	Metadata json.RawMessage `json:"metadata,omitempty"`
}

func (i taskCancelInput) cancel() taskpkg.CancelTask {
	return taskpkg.CancelTask{
		Reason:   strings.TrimSpace(i.Reason),
		Metadata: cloneJSON(i.Metadata),
	}
}

type taskBlockInput struct {
	TaskID    string          `json:"task_id"`
	Kind      string          `json:"kind"`
	Reason    string          `json:"reason"`
	Details   json.RawMessage `json:"details,omitempty"`
	ExpiresAt *time.Time      `json:"expires_at,omitempty"`
	RunID     string          `json:"run_id,omitempty"`
}

type taskUnblockInput struct {
	TaskID  string `json:"task_id"`
	BlockID string `json:"block_id"`
	RunID   string `json:"run_id,omitempty"`
	Note    string `json:"note,omitempty"`
}

type taskBlocksInput struct {
	TaskID         string `json:"task_id"`
	IncludeCleared bool   `json:"include_cleared,omitempty"`
}

type taskRecoverInput struct {
	TaskID string `json:"task_id"`
	Note   string `json:"note,omitempty"`
}

type taskPromoteFromThreadInput struct {
	WorkspaceID     string          `json:"workspace_id"`
	Channel         string          `json:"channel"`
	ThreadID        string          `json:"thread_id"`
	OriginMessageID string          `json:"origin_message_id"`
	Title           string          `json:"title,omitempty"`
	Description     string          `json:"description,omitempty"`
	Priority        string          `json:"priority,omitempty"`
	Metadata        json.RawMessage `json:"metadata,omitempty"`
}

type taskFanOutRunsInput struct {
	TaskID               string                                     `json:"task_id"`
	NetworkParticipation *participation.Request                     `json:"network_participation,omitempty"`
	Designations         []contract.TaskFanOutRunDesignationRequest `json:"designations"`
	IdempotencyKey       string                                     `json:"idempotency_key,omitempty"`
}

type autonomyHeartbeatInput struct {
	RunID        string `json:"run_id"`
	LeaseSeconds int64  `json:"lease_seconds,omitempty"`
}

type autonomyCompleteInput struct {
	RunID          string          `json:"run_id"`
	Result         json.RawMessage `json:"result,omitempty"`
	CreatedTaskIDs []string        `json:"created_task_ids,omitempty"`
}

type autonomyFailInput struct {
	RunID    string          `json:"run_id"`
	Error    string          `json:"error"`
	Metadata json.RawMessage `json:"metadata,omitempty"`
}

type autonomyReleaseInput struct {
	RunID  string `json:"run_id"`
	Reason string `json:"reason,omitempty"`
}

func decodeNativeInput(req toolspkg.CallRequest, dst any) error {
	raw := req.Input
	if len(bytes.TrimSpace(raw)) == 0 {
		raw = json.RawMessage(`{}`)
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil {
		return toolspkg.NewToolError(
			toolspkg.ErrorCodeInvalidInput,
			req.ToolID,
			fmt.Sprintf("tool %q input is invalid", req.ToolID),
			fmt.Errorf("%w: %w", toolspkg.ErrToolInvalidInput, err),
			toolspkg.ReasonSchemaInvalid,
		)
	}
	return nil
}

func requiredNativeString(id toolspkg.ToolID, field string, value string) (string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", nativeRequiredInputError(id, field)
	}
	return trimmed, nil
}

func nativeRequiredInputError(id toolspkg.ToolID, field string) error {
	return toolspkg.NewToolError(
		toolspkg.ErrorCodeInvalidInput,
		id,
		fmt.Sprintf("%s is required", field),
		toolspkg.ErrToolInvalidInput,
		toolspkg.ReasonSchemaInvalid,
	)
}

func nativeUnavailableError(id toolspkg.ToolID, message string) error {
	return toolspkg.NewToolError(
		toolspkg.ErrorCodeUnavailable,
		id,
		strings.TrimSpace(message),
		fmt.Errorf("%w: %s", toolspkg.ErrToolUnavailable, strings.TrimSpace(message)),
		toolspkg.ReasonBackendUnhealthy,
	)
}
