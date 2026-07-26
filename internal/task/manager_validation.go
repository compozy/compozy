package task

import (
	"context"

	"fmt"

	"strings"

	"github.com/compozy/agh/internal/network/participation"
)

func requireReadAuthority(actor ActorContext) error {
	if err := actor.Validate(); err != nil {
		return err
	}
	if !actor.Authority.Read {
		return ErrPermissionDenied
	}
	return nil
}

func requireWriteAuthority(actor ActorContext) error {
	if err := actor.Validate(); err != nil {
		return err
	}
	if !actor.Authority.Write {
		return ErrPermissionDenied
	}
	return nil
}

func requireCreateAuthority(actor ActorContext, scope Scope) error {
	if err := requireWriteAuthority(actor); err != nil {
		return err
	}

	switch scope.Normalize() {
	case ScopeGlobal:
		if !actor.Authority.CreateGlobal {
			return ErrPermissionDenied
		}
	case ScopeWorkspace:
		if !actor.Authority.CreateWorkspace {
			return ErrPermissionDenied
		}
	default:
		return fmt.Errorf("%w: create_task.scope is required", ErrValidation)
	}
	return nil
}

func normalizeCreateTaskSpec(spec CreateTask) (CreateTask, error) {
	normalized := spec
	normalized.ID = strings.TrimSpace(normalized.ID)
	normalized.Identifier = strings.TrimSpace(normalized.Identifier)
	normalized.Scope = normalized.Scope.Normalize()
	normalized.WorkspaceID = strings.TrimSpace(normalized.WorkspaceID)
	normalized.ParentTaskID = strings.TrimSpace(normalized.ParentTaskID)
	normalized.Title = strings.TrimSpace(normalized.Title)
	normalized.Description = strings.TrimSpace(normalized.Description)
	normalized.Priority = normalizePriorityOrDefault(normalized.Priority)
	if normalized.MaxAttempts != nil {
		maxAttempts := *normalized.MaxAttempts
		normalized.MaxAttempts = &maxAttempts
	}
	if normalized.WakeCreator != nil {
		wakeCreator := *normalized.WakeCreator
		normalized.WakeCreator = &wakeCreator
	}
	normalized.ApprovalPolicy = normalizeApprovalPolicyOrDefault(normalized.ApprovalPolicy)
	if normalized.Owner != nil {
		normalized.Owner = normalizeOwnership(normalized.Owner)
	}
	if normalized.NetworkParticipation != nil {
		request, err := participation.NormalizeIntent(*normalized.NetworkParticipation)
		if err != nil {
			return CreateTask{}, fmt.Errorf("create_task.network_participation: %w", err)
		}
		normalized.NetworkParticipation = &request
	}
	normalized.Metadata = normalizeRawJSON(normalized.Metadata)
	if err := normalized.Validate("create_task"); err != nil {
		return CreateTask{}, err
	}
	return normalized, nil
}

func createTaskWakeCreator(spec CreateTask) bool {
	if spec.WakeCreator == nil {
		return true
	}
	return *spec.WakeCreator
}

func normalizeTaskPatch(patch Patch) (Patch, error) {
	normalized := patch
	if normalized.Title != nil {
		title := strings.TrimSpace(*normalized.Title)
		normalized.Title = &title
	}
	if normalized.Description != nil {
		description := strings.TrimSpace(*normalized.Description)
		normalized.Description = &description
	}
	if normalized.Priority != nil {
		priority := normalized.Priority.Normalize()
		normalized.Priority = &priority
	}
	if normalized.MaxAttempts != nil {
		maxAttempts := *normalized.MaxAttempts
		normalized.MaxAttempts = &maxAttempts
	}
	if normalized.AutoEnqueueOnReady != nil {
		autoEnqueue := *normalized.AutoEnqueueOnReady
		normalized.AutoEnqueueOnReady = &autoEnqueue
	}
	if normalized.ApprovalPolicy != nil {
		approvalPolicy := normalized.ApprovalPolicy.Normalize()
		normalized.ApprovalPolicy = &approvalPolicy
	}
	if normalized.Metadata != nil {
		metadata := normalizeRawJSON(*normalized.Metadata)
		normalized.Metadata = &metadata
	}
	if normalized.Owner != nil {
		normalized.Owner = normalizeOwnership(normalized.Owner)
	}
	if normalized.NetworkParticipation != nil {
		request, err := participation.NormalizeIntent(*normalized.NetworkParticipation)
		if err != nil {
			return Patch{}, fmt.Errorf("task_patch.network_participation: %w", err)
		}
		normalized.NetworkParticipation = &request
	}
	if err := normalized.Validate("task_patch"); err != nil {
		return Patch{}, err
	}
	return normalized, nil
}

func normalizeAddDependencySpec(spec AddDependency) (AddDependency, error) {
	normalized := spec
	normalized.TaskID = strings.TrimSpace(normalized.TaskID)
	normalized.DependsOnTaskID = strings.TrimSpace(normalized.DependsOnTaskID)
	normalized.Kind = normalized.Kind.Normalize()
	if err := normalized.Validate("add_dependency"); err != nil {
		return AddDependency{}, err
	}
	return normalized, nil
}

func normalizeCancelTask(req CancelTask) (CancelTask, error) {
	normalized := req
	normalized.Reason = strings.TrimSpace(normalized.Reason)
	normalized.Metadata = normalizeRawJSON(normalized.Metadata)
	if err := normalized.Validate("cancel_task"); err != nil {
		return CancelTask{}, err
	}
	return normalized, nil
}

func normalizeTaskExecutionRequest(req ExecutionRequest) (ExecutionRequest, error) {
	normalized := req
	normalized.IdempotencyKey = strings.TrimSpace(normalized.IdempotencyKey)
	if normalized.NetworkParticipation != nil {
		request, err := participation.NormalizeIntent(*normalized.NetworkParticipation)
		if err != nil {
			return ExecutionRequest{}, fmt.Errorf("task_execution.network_participation: %w", err)
		}
		normalized.NetworkParticipation = &request
	}
	normalized.Metadata = normalizeRawJSON(normalized.Metadata)
	if err := normalized.Validate("task_execution"); err != nil {
		return ExecutionRequest{}, err
	}
	return normalized, nil
}

func taskExecutionIdempotencyKey(taskID string, action ExecutionAction, requested string) string {
	if trimmed := strings.TrimSpace(requested); trimmed != "" {
		return trimmed
	}
	return fmt.Sprintf("task.%s.%s", strings.TrimSpace(string(action)), strings.TrimSpace(taskID))
}

func normalizeEnqueueRunSpec(spec EnqueueRun) (EnqueueRun, error) {
	normalized := spec
	normalized.TaskID = strings.TrimSpace(normalized.TaskID)
	normalized.RunKind = normalizeRunKindOrDefault(normalized.RunKind)
	normalized.LoopRunID = strings.TrimSpace(normalized.LoopRunID)
	normalized.IdempotencyKey = strings.TrimSpace(normalized.IdempotencyKey)
	normalized.NetworkParticipationSource = participation.Source(strings.TrimSpace(
		string(normalized.NetworkParticipationSource),
	))
	if normalized.NetworkParticipation != nil {
		request, err := participation.NormalizeIntent(*normalized.NetworkParticipation)
		if err != nil {
			return EnqueueRun{}, fmt.Errorf("enqueue_run.network_participation: %w", err)
		}
		normalized.NetworkParticipation = &request
	}
	normalized.DesignationGroupID = strings.TrimSpace(normalized.DesignationGroupID)
	normalized.Metadata = normalizeRawJSON(normalized.Metadata)
	if err := normalized.Validate("enqueue_run"); err != nil {
		return EnqueueRun{}, err
	}
	return normalized, nil
}

func normalizeStartRun(req StartRun) (StartRun, error) {
	normalized := req
	normalized.IdempotencyKey = strings.TrimSpace(normalized.IdempotencyKey)
	normalized.ClaimToken = strings.TrimSpace(normalized.ClaimToken)
	if err := normalized.Validate("start_run"); err != nil {
		return StartRun{}, err
	}
	return normalized, nil
}

func normalizeCancelRun(req CancelRun) (CancelRun, error) {
	normalized := req
	normalized.Reason = strings.TrimSpace(normalized.Reason)
	normalized.Metadata = normalizeRawJSON(normalized.Metadata)
	if err := normalized.Validate("cancel_run"); err != nil {
		return CancelRun{}, err
	}
	return normalized, nil
}

func normalizeRunResult(result RunResult) (RunResult, error) {
	normalized := result
	normalized.Value = normalizeRawJSON(normalized.Value)
	if normalized.CoordinatorControl != nil {
		control := *normalized.CoordinatorControl
		control.Kind = strings.TrimSpace(control.Kind)
		control.Payload = normalizeRawJSON(control.Payload)
		normalized.CoordinatorControl = &control
	}
	if err := normalized.Validate("run_result"); err != nil {
		return RunResult{}, err
	}
	return normalized, nil
}

func normalizeRunFailure(failure RunFailure) (RunFailure, error) {
	normalized := failure
	normalized.Error = strings.TrimSpace(normalized.Error)
	normalized.Metadata = normalizeRawJSON(normalized.Metadata)
	if err := normalized.Validate("run_failure"); err != nil {
		return RunFailure{}, err
	}
	return normalized, nil
}

func normalizeRunBootRecovery(recovery RunBootRecovery) (RunBootRecovery, error) {
	normalized := recovery
	normalized.Action = normalized.Action.Normalize()
	normalized.Reason = strings.TrimSpace(normalized.Reason)
	normalized.SessionState = strings.TrimSpace(normalized.SessionState)
	normalized.Classification = strings.TrimSpace(normalized.Classification)
	normalized.Detail = strings.TrimSpace(normalized.Detail)
	if err := normalized.Validate("run_boot_recovery"); err != nil {
		return RunBootRecovery{}, err
	}
	return normalized, nil
}

func requireLifecycleIdempotency(actor ActorContext, key string, path string) error {
	if actor.Actor.Kind.Normalize() == ActorKindHuman {
		return nil
	}
	if strings.TrimSpace(key) == "" {
		return fmt.Errorf(
			"%w: %s.idempotency_key is required for non-human actors",
			ErrValidation,
			path,
		)
	}
	return nil
}

func (m *Service) validateParentConstraints(ctx context.Context, spec CreateTask) error {
	if strings.TrimSpace(spec.ParentTaskID) == "" {
		return nil
	}

	parent, err := m.store.GetTask(ctx, spec.ParentTaskID)
	if err != nil {
		return err
	}
	if err := validateParentChildScope(parent, spec.Scope, spec.WorkspaceID); err != nil {
		return err
	}

	childCount, err := m.store.CountDirectChildren(ctx, parent.ID)
	if err != nil {
		return err
	}
	if err := ValidateDirectChildCount(childCount + 1); err != nil {
		return err
	}

	parentDepth, err := m.taskDepth(ctx, parent)
	if err != nil {
		return err
	}
	return ValidateHierarchyDepth(parentDepth + 1)
}

func validateParentChildScope(parent Task, childScope Scope, childWorkspaceID string) error {
	switch parent.Scope.Normalize() {
	case ScopeGlobal:
		return nil
	case ScopeWorkspace:
		if childScope.Normalize() != ScopeWorkspace {
			return fmt.Errorf(
				"%w: workspace-scoped parent tasks require workspace-scoped children",
				ErrValidation,
			)
		}
		if strings.TrimSpace(parent.WorkspaceID) != strings.TrimSpace(childWorkspaceID) {
			return fmt.Errorf(
				"%w: child workspace_id must match workspace-scoped parent",
				ErrValidation,
			)
		}
		return nil
	default:
		return fmt.Errorf("%w: parent task has unsupported scope %q", ErrValidation, parent.Scope)
	}
}

func (m *Service) taskDepth(ctx context.Context, record Task) (int, error) {
	depth := 1
	current := record
	seen := map[string]struct{}{strings.TrimSpace(record.ID): {}}

	for strings.TrimSpace(current.ParentTaskID) != "" {
		parentID := strings.TrimSpace(current.ParentTaskID)
		if _, ok := seen[parentID]; ok {
			return 0, fmt.Errorf(
				"%w: task hierarchy contains a cycle at %q",
				ErrValidation,
				parentID,
			)
		}
		seen[parentID] = struct{}{}

		parent, err := m.store.GetTask(ctx, parentID)
		if err != nil {
			return 0, err
		}
		depth++
		current = parent
	}

	return depth, nil
}
