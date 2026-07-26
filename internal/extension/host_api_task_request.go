package extensionpkg

import (
	"context"

	"errors"
	"fmt"
	"strings"

	apicontract "github.com/compozy/agh/internal/api/contract"
	"github.com/compozy/agh/internal/network"
	"github.com/compozy/agh/internal/network/participation"

	taskpkg "github.com/compozy/agh/internal/task"
	workspacepkg "github.com/compozy/agh/internal/workspace"
)

func (h *HostAPIHandler) createTaskSpecFromRequest(
	ctx context.Context,
	req apicontract.CreateTaskRequest,
) (taskpkg.CreateTask, error) {
	scope := req.Scope.Normalize()
	if err := scope.Validate("create_task.scope"); err != nil {
		return taskpkg.CreateTask{}, invalidParamsRPCError(err)
	}
	workspaceID, err := h.resolveTaskWorkspaceBinding(ctx, scope, strings.TrimSpace(req.Workspace), "create_task")
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
		WakeCreator:          cloneBoolPointer(req.WakeCreator),
		NetworkParticipation: participation.CloneRequest(req.NetworkParticipation),
		Metadata:             cloneRawMessage(req.Metadata),
	}
	if err := spec.Validate("create_task"); err != nil {
		return taskpkg.CreateTask{}, invalidParamsRPCError(err)
	}
	return spec, nil
}

func taskPatchFromRequest(req apicontract.UpdateTaskRequest) (taskpkg.Patch, error) {
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
		return taskpkg.Patch{}, invalidParamsRPCError(err)
	}
	return patch, nil
}

func cancelTaskFromRequest(req apicontract.CancelTaskRequest) (taskpkg.CancelTask, error) {
	cancelReq := taskpkg.CancelTask{
		Reason:   strings.TrimSpace(req.Reason),
		Metadata: cloneRawMessage(req.Metadata),
	}
	if err := cancelReq.Validate("cancel_task"); err != nil {
		return taskpkg.CancelTask{}, invalidParamsRPCError(err)
	}
	return cancelReq, nil
}

func enqueueTaskRunFromRequest(taskID string, req apicontract.EnqueueTaskRunRequest) (taskpkg.EnqueueRun, error) {
	spec := taskpkg.EnqueueRun{
		TaskID:               strings.TrimSpace(taskID),
		IdempotencyKey:       strings.TrimSpace(req.IdempotencyKey),
		NetworkParticipation: participation.CloneRequest(req.NetworkParticipation),
		Metadata:             cloneRawMessage(req.Metadata),
	}
	if err := spec.Validate("enqueue_run"); err != nil {
		return taskpkg.EnqueueRun{}, invalidParamsRPCError(err)
	}
	return spec, nil
}

func startTaskRunFromRequest(req apicontract.StartTaskRunRequest) (taskpkg.StartRun, error) {
	startReq := taskpkg.StartRun{IdempotencyKey: strings.TrimSpace(req.IdempotencyKey)}
	if err := startReq.Validate("start_run"); err != nil {
		return taskpkg.StartRun{}, invalidParamsRPCError(err)
	}
	return startReq, nil
}

func attachTaskRunSessionIDFromRequest(req apicontract.AttachTaskRunSessionRequest) (string, error) {
	sessionID := strings.TrimSpace(req.SessionID)
	if sessionID == "" {
		return "", invalidParamsRPCError(errors.New("session_id is required"))
	}
	return sessionID, nil
}

func completeTaskRunFromRequest(req apicontract.CompleteTaskRunRequest) (taskpkg.RunResult, error) {
	result := taskpkg.RunResult{Value: cloneRawMessage(req.Result)}
	if err := result.Validate("run_result"); err != nil {
		return taskpkg.RunResult{}, invalidParamsRPCError(err)
	}
	return result, nil
}

func failTaskRunFromRequest(req apicontract.FailTaskRunRequest) (taskpkg.RunFailure, error) {
	failure := taskpkg.RunFailure{
		Error:    strings.TrimSpace(req.Error),
		Metadata: cloneRawMessage(req.Metadata),
	}
	if err := failure.Validate("run_failure"); err != nil {
		return taskpkg.RunFailure{}, invalidParamsRPCError(err)
	}
	return failure, nil
}

func cancelTaskRunFromRequest(req apicontract.CancelTaskRunRequest) (taskpkg.CancelRun, error) {
	cancelReq := taskpkg.CancelRun{
		Reason:   strings.TrimSpace(req.Reason),
		Metadata: cloneRawMessage(req.Metadata),
	}
	if err := cancelReq.Validate("cancel_run"); err != nil {
		return taskpkg.CancelRun{}, invalidParamsRPCError(err)
	}
	return cancelReq, nil
}

func (h *HostAPIHandler) resolveTaskWorkspaceBinding(
	ctx context.Context,
	scope taskpkg.Scope,
	workspaceRef string,
	path string,
) (string, error) {
	trimmed := strings.TrimSpace(workspaceRef)
	if err := taskpkg.ValidateScopeBinding(scope, trimmed, path, "workspace"); err != nil {
		return "", invalidParamsRPCError(err)
	}
	if scope.Normalize() != taskpkg.ScopeWorkspace {
		return "", nil
	}
	return h.resolveTaskWorkspaceID(ctx, trimmed)
}

func (h *HostAPIHandler) resolveTaskWorkspaceID(ctx context.Context, workspaceRef string) (string, error) {
	trimmed := strings.TrimSpace(workspaceRef)
	if trimmed == "" {
		return "", nil
	}
	if h.workspaces == nil {
		return trimmed, nil
	}

	resolved, err := h.workspaces.Resolve(ctx, trimmed)
	if err != nil {
		if errors.Is(err, workspacepkg.ErrWorkspaceNotFound) {
			return "", notFoundRPCError("workspace", trimmed, err)
		}
		return "", err
	}
	return hostAPIResolvedWorkspaceRegistrationID(&resolved)
}

func validateTaskChannel(path string, channel string) error {
	trimmed := strings.TrimSpace(channel)
	if trimmed == "" {
		return nil
	}
	if err := network.ValidateChannel(trimmed); err != nil {
		return invalidParamsRPCError(fmt.Errorf("%s: %w", path, err))
	}
	return nil
}

func mapTaskRPCError(id string, err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, workspacepkg.ErrWorkspaceNotFound):
		return notFoundRPCError("workspace", id, err)
	case errors.Is(err, taskpkg.ErrTaskNotFound):
		return notFoundRPCError("task", id, err)
	case errors.Is(err, taskpkg.ErrTaskRunNotFound):
		return notFoundRPCError("task_run", id, err)
	case errors.Is(err, taskpkg.ErrTaskDependencyNotFound):
		return notFoundRPCError("task_dependency", id, err)
	case errors.Is(err, taskpkg.ErrValidation),
		errors.Is(err, taskpkg.ErrImmutableField),
		errors.Is(err, taskpkg.ErrInvalidScopeBinding),
		errors.Is(err, taskpkg.ErrPayloadTooLarge),
		errors.Is(err, taskpkg.ErrGraphLimitExceeded),
		errors.Is(err, taskpkg.ErrCycleDetected),
		errors.Is(err, taskpkg.ErrInvalidStatusTransition),
		errors.Is(err, taskpkg.ErrSessionAlreadyBound),
		errors.Is(err, taskpkg.ErrSessionAttachNotAllowed),
		errors.Is(err, taskpkg.ErrPermissionDenied):
		return invalidParamsRPCError(err)
	default:
		return err
	}
}
