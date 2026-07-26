package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/compozy/agh/internal/api/contract"

	"github.com/compozy/agh/internal/network/participation"

	taskpkg "github.com/compozy/agh/internal/task"
	"github.com/gin-gonic/gin"
)

func (h *BaseHandlers) createTaskSpecFromRequest(
	ctx context.Context,
	req contract.CreateTaskRequest,
) (taskpkg.CreateTask, error) {
	scope := req.Scope.Normalize()
	workspaceID, err := h.resolveTaskWorkspaceBinding(ctx, scope, req.Workspace, "create_task")
	if err != nil {
		return taskpkg.CreateTask{}, err
	}

	spec := taskpkg.CreateTask{
		ID:                   strings.TrimSpace(req.ID),
		Identifier:           strings.TrimSpace(req.Identifier),
		Scope:                scope,
		WorkspaceID:          workspaceID,
		Title:                strings.TrimSpace(req.Title),
		Description:          strings.TrimSpace(req.Description),
		Priority:             req.Priority.Normalize(),
		MaxAttempts:          req.MaxAttempts,
		AutoEnqueueOnReady:   req.AutoEnqueueOnReady,
		Draft:                req.Draft,
		ApprovalPolicy:       req.ApprovalPolicy.Normalize(),
		Owner:                cloneOwnership(req.Owner),
		WakeCreator:          cloneBoolPtr(req.WakeCreator),
		NetworkParticipation: participation.CloneRequest(req.NetworkParticipation),
		Metadata:             cloneRawMessage(req.Metadata),
	}
	if err := spec.Validate("create_task"); err != nil {
		return taskpkg.CreateTask{}, err
	}
	return spec, nil
}

func (h *BaseHandlers) createChildTaskSpecFromRequest(
	ctx context.Context,
	req contract.CreateTaskChildRequest,
) (taskpkg.CreateTask, error) {
	scope := req.Scope.Normalize()
	workspaceID, err := h.resolveTaskWorkspaceBinding(ctx, scope, req.Workspace, "create_child_task")
	if err != nil {
		return taskpkg.CreateTask{}, err
	}

	spec := taskpkg.CreateTask{
		ID:                   strings.TrimSpace(req.ID),
		Identifier:           strings.TrimSpace(req.Identifier),
		Scope:                scope,
		WorkspaceID:          workspaceID,
		Title:                strings.TrimSpace(req.Title),
		Description:          strings.TrimSpace(req.Description),
		Priority:             req.Priority.Normalize(),
		MaxAttempts:          req.MaxAttempts,
		AutoEnqueueOnReady:   req.AutoEnqueueOnReady,
		Draft:                req.Draft,
		ApprovalPolicy:       req.ApprovalPolicy.Normalize(),
		Owner:                cloneOwnership(req.Owner),
		WakeCreator:          cloneBoolPtr(req.WakeCreator),
		NetworkParticipation: participation.CloneRequest(req.NetworkParticipation),
		Metadata:             cloneRawMessage(req.Metadata),
	}
	if err := spec.Validate("create_child_task"); err != nil {
		return taskpkg.CreateTask{}, err
	}
	return spec, nil
}

func taskPatchFromRequest(req contract.UpdateTaskRequest) (taskpkg.Patch, error) {
	patch := taskpkg.Patch{
		Title:                trimStringPtr(req.Title),
		Description:          trimStringPtr(req.Description),
		Priority:             normalizePriorityPtr(req.Priority),
		MaxAttempts:          req.MaxAttempts,
		AutoEnqueueOnReady:   req.AutoEnqueueOnReady,
		ApprovalPolicy:       normalizeApprovalPolicyPtr(req.ApprovalPolicy),
		Metadata:             cloneRawMessagePtr(req.Metadata),
		Owner:                cloneOwnership(req.Owner),
		ClearOwner:           req.ClearOwner,
		NetworkParticipation: participation.CloneRequest(req.NetworkParticipation),
	}
	if err := patch.Validate("task_patch"); err != nil {
		return taskpkg.Patch{}, err
	}
	return patch, nil
}

func taskExecutionProfileFromRequest(
	taskID string,
	req *contract.SetTaskExecutionProfileRequest,
) (*taskpkg.ExecutionProfile, error) {
	trimmedID := strings.TrimSpace(taskID)
	if trimmedID == "" {
		return nil, NewTaskValidationError(errors.New("task id is required"))
	}
	if req == nil {
		return nil, NewTaskValidationError(errors.New("task execution profile request is required"))
	}
	if strings.TrimSpace(req.TaskID) != "" && strings.TrimSpace(req.TaskID) != trimmedID {
		return nil, NewTaskValidationError(fmt.Errorf(
			"task_execution_profile.task_id must match task id %q",
			trimmedID,
		))
	}
	profile := *req
	profile.TaskID = trimmedID
	profile.CreatedAt = time.Time{}
	profile.UpdatedAt = time.Time{}
	return &profile, nil
}

func cancelTaskFromRequest(req contract.CancelTaskRequest) (taskpkg.CancelTask, error) {
	cancelReq := taskpkg.CancelTask{
		Reason:   strings.TrimSpace(req.Reason),
		Metadata: cloneRawMessage(req.Metadata),
	}
	if err := cancelReq.Validate("cancel_task"); err != nil {
		return taskpkg.CancelTask{}, err
	}
	return cancelReq, nil
}

func addTaskDependencyFromRequest(taskID string, req contract.AddTaskDependencyRequest) (taskpkg.AddDependency, error) {
	kind := req.Kind.Normalize()
	if kind == "" {
		kind = taskpkg.DependencyKindBlocks
	}

	spec := taskpkg.AddDependency{
		TaskID:          strings.TrimSpace(taskID),
		DependsOnTaskID: strings.TrimSpace(req.DependsOnTaskID),
		Kind:            kind,
	}
	if err := spec.Validate("add_dependency"); err != nil {
		return taskpkg.AddDependency{}, err
	}
	return spec, nil
}

func enqueueTaskRunFromRequest(taskID string, req contract.EnqueueTaskRunRequest) (taskpkg.EnqueueRun, error) {
	spec := taskpkg.EnqueueRun{
		TaskID:               strings.TrimSpace(taskID),
		IdempotencyKey:       strings.TrimSpace(req.IdempotencyKey),
		NetworkParticipation: participation.CloneRequest(req.NetworkParticipation),
		Metadata:             append(json.RawMessage(nil), req.Metadata...),
	}
	if err := spec.Validate("enqueue_run"); err != nil {
		return taskpkg.EnqueueRun{}, err
	}
	return spec, nil
}

type fanOutDesignationMetadataPayload struct {
	Designation fanOutDesignationMetadataDetail `json:"designation"`
	Metadata    json.RawMessage                 `json:"metadata,omitempty"`
}

type fanOutDesignationMetadataDetail struct {
	Index int    `json:"index"`
	Brief string `json:"brief"`
}

func fanOutDesignationMetadata(
	index int,
	designation contract.TaskFanOutRunDesignationRequest,
) (json.RawMessage, error) {
	brief := strings.TrimSpace(designation.Brief)
	if brief == "" {
		return nil, NewTaskValidationError(fmt.Errorf("designations[%d].brief is required", index))
	}
	metadata := cloneRawMessage(designation.Metadata)
	if len(metadata) > 0 && !json.Valid(metadata) {
		return nil, NewTaskValidationError(fmt.Errorf("designations[%d].metadata must be valid JSON", index))
	}
	payload := fanOutDesignationMetadataPayload{
		Designation: fanOutDesignationMetadataDetail{Index: index, Brief: brief},
		Metadata:    metadata,
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("task fan-out metadata: %w", err)
	}
	return encoded, nil
}

func fanOutDesignationIdempotencyKey(
	requestKey string,
	designation contract.TaskFanOutRunDesignationRequest,
	index int,
) string {
	if key := strings.TrimSpace(designation.IdempotencyKey); key != "" {
		return key
	}
	if key := strings.TrimSpace(requestKey); key != "" {
		return fmt.Sprintf("%s:%d", key, index)
	}
	return ""
}

func fanOutDesignationRollupJSON(runs []taskpkg.Run, now time.Time) json.RawMessage {
	runIDs := make([]string, 0, len(runs))
	statuses := make(map[string]int)
	var taskID string
	var groupID string
	completed := 0
	failed := 0
	canceled := 0
	running := 0
	queued := 0
	needsAttention := 0
	terminalCount := 0
	for _, run := range runs {
		if id := strings.TrimSpace(run.ID); id != "" {
			runIDs = append(runIDs, id)
		}
		taskID = firstNonEmpty(taskID, strings.TrimSpace(run.TaskID))
		groupID = firstNonEmpty(groupID, strings.TrimSpace(run.DesignationGroupID))
		status := run.Status.Normalize()
		statuses[status.String()]++
		switch status {
		case taskpkg.TaskRunStatusCompleted:
			completed++
			terminalCount++
		case taskpkg.TaskRunStatusFailed:
			failed++
			terminalCount++
		case taskpkg.TaskRunStatusCanceled:
			canceled++
			terminalCount++
		case taskpkg.TaskRunStatusQueued:
			queued++
		case taskpkg.TaskRunStatusNeedsAttention:
			needsAttention++
		default:
			running++
		}
	}
	encoded, err := json.Marshal(map[string]any{
		"designation_group_id":         groupID,
		"task_id":                      taskID,
		"run_ids":                      runIDs,
		"total":                        len(runIDs),
		"terminal_count":               terminalCount,
		taskDesignationRollupCompleted: completed,
		"failed":                       failed,
		taskDesignationRollupCanceled:  canceled,
		"running":                      running,
		"queued":                       queued,
		"needs_attention":              needsAttention,
		"statuses":                     statuses,
		"complete":                     len(runIDs) > 0 && terminalCount == len(runIDs),
		"updated_at":                   now.UTC().Format(time.RFC3339Nano),
	})
	if err != nil {
		return json.RawMessage(`{"run_ids":[],"total":0,"terminal_count":0,"complete":false}`)
	}
	return encoded
}

func taskExecutionRequestFromRequest(req contract.TaskExecutionRequest) (taskpkg.ExecutionRequest, error) {
	spec := taskpkg.ExecutionRequest{
		IdempotencyKey:       strings.TrimSpace(req.IdempotencyKey),
		NetworkParticipation: participation.CloneRequest(req.NetworkParticipation),
		Metadata:             append(json.RawMessage(nil), req.Metadata...),
	}
	if err := spec.Validate("task_execution"); err != nil {
		return taskpkg.ExecutionRequest{}, err
	}
	return spec, nil
}

func startTaskRunFromRequest(req contract.StartTaskRunRequest) (taskpkg.StartRun, error) {
	startReq := taskpkg.StartRun{IdempotencyKey: strings.TrimSpace(req.IdempotencyKey)}
	if err := startReq.Validate("start_run"); err != nil {
		return taskpkg.StartRun{}, err
	}
	return startReq, nil
}

func attachTaskRunSessionIDFromRequest(req contract.AttachTaskRunSessionRequest) (string, error) {
	sessionID := strings.TrimSpace(req.SessionID)
	if sessionID == "" {
		return "", NewTaskValidationError(errors.New("session_id is required"))
	}
	return sessionID, nil
}

func completeTaskRunFromRequest(req contract.CompleteTaskRunRequest) (taskpkg.RunResult, error) {
	result := taskpkg.RunResult{Value: cloneRawMessage(req.Result)}
	if err := result.Validate("run_result"); err != nil {
		return taskpkg.RunResult{}, err
	}
	return result, nil
}

func createTaskBlockFromRequest(
	taskID string,
	req contract.CreateTaskBlockRequest,
	claimToken string,
) (taskpkg.BlockRequest, error) {
	blockReq := taskpkg.BlockRequest{
		TaskID:     strings.TrimSpace(taskID),
		Kind:       req.Kind.Normalize(),
		Reason:     strings.TrimSpace(req.Reason),
		Details:    cloneRawMessage(req.Details),
		RunID:      strings.TrimSpace(req.RunID),
		ClaimToken: strings.TrimSpace(claimToken),
	}
	if req.ExpiresAt != nil {
		blockReq.ExpiresAt = req.ExpiresAt.UTC()
	}
	if blockReq.TaskID == "" {
		return taskpkg.BlockRequest{}, fmt.Errorf("%w: task_block.task_id is required", taskpkg.ErrValidation)
	}
	if err := blockReq.Kind.Validate("task_block.kind"); err != nil {
		return taskpkg.BlockRequest{}, err
	}
	if blockReq.Reason == "" {
		return taskpkg.BlockRequest{}, fmt.Errorf("%w: task_block.reason is required", taskpkg.ErrValidation)
	}
	if blockReq.ExpiresAt.IsZero() {
		return blockReq, nil
	}
	if blockReq.Kind != taskpkg.BlockKindTransient {
		return taskpkg.BlockRequest{}, fmt.Errorf(
			"%w: task_block.expires_at is only valid for %q blocks",
			taskpkg.ErrValidation,
			taskpkg.BlockKindTransient,
		)
	}
	return blockReq, nil
}

func statusForTaskBlockError(err error) int {
	if errors.Is(err, taskpkg.ErrValidation) {
		return http.StatusUnprocessableEntity
	}
	return StatusForTaskError(err)
}

func failTaskRunFromRequest(req contract.FailTaskRunRequest) (taskpkg.RunFailure, error) {
	failure := taskpkg.RunFailure{
		Error:    strings.TrimSpace(req.Error),
		Metadata: cloneRawMessage(req.Metadata),
	}
	if err := failure.Validate("run_failure"); err != nil {
		return taskpkg.RunFailure{}, err
	}
	return failure, nil
}

func cancelTaskRunFromRequest(req contract.CancelTaskRunRequest) (taskpkg.CancelRun, error) {
	cancelReq := taskpkg.CancelRun{
		Reason:   strings.TrimSpace(req.Reason),
		Metadata: cloneRawMessage(req.Metadata),
	}
	if err := cancelReq.Validate("cancel_run"); err != nil {
		return taskpkg.CancelRun{}, err
	}
	return cancelReq, nil
}

func (h *BaseHandlers) resolveTaskWorkspaceBinding(
	ctx context.Context,
	scope taskpkg.Scope,
	workspaceRef string,
	path string,
) (string, error) {
	trimmed := strings.TrimSpace(workspaceRef)
	if err := taskpkg.ValidateScopeBinding(scope, trimmed, path, "workspace"); err != nil {
		return "", err
	}
	if scope.Normalize() != taskpkg.ScopeWorkspace {
		return "", nil
	}
	return h.lookupWorkspaceID(ctx, trimmed)
}

func requiredPathID(raw string, field string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", NewTaskValidationError(fmt.Errorf("%s is required", field))
	}
	return trimmed, nil
}

func decodeOptionalJSON(c *gin.Context, dest any) error {
	if err := decodeStrictJSONBody(c, dest); err != nil && !errors.Is(err, io.EOF) {
		return err
	}
	return nil
}
