package daemon

import (
	"context"

	"errors"
	"fmt"

	"strings"
	"time"

	"github.com/compozy/agh/internal/api/contract"
	core "github.com/compozy/agh/internal/api/core"

	"github.com/compozy/agh/internal/network"

	taskpkg "github.com/compozy/agh/internal/task"
	toolspkg "github.com/compozy/agh/internal/tools"
)

func (n *daemonNativeTools) nativeTaskBlockActor(
	ctx context.Context,
	id toolspkg.ToolID,
	scope toolspkg.Scope,
	taskID string,
	runID string,
) (taskpkg.ActorContext, string, string, error) {
	if strings.TrimSpace(scope.SessionID) == "" || scope.Operator {
		actor, err := nativeOperatorActorContext(scope)
		if err != nil {
			return taskpkg.ActorContext{}, "", "", err
		}
		return actor, strings.TrimSpace(runID), "", nil
	}
	actor, handle, err := n.nativeTaskLeaseActor(ctx, id, scope, taskID, runID)
	if err != nil {
		return taskpkg.ActorContext{}, "", "", err
	}
	return actor, handle.RunID, handle.ClaimToken, nil
}

func (n *daemonNativeTools) nativeTaskClearActor(
	ctx context.Context,
	id toolspkg.ToolID,
	scope toolspkg.Scope,
	taskID string,
	runID string,
) (taskpkg.ActorContext, error) {
	if strings.TrimSpace(scope.SessionID) == "" || scope.Operator {
		return nativeOperatorActorContext(scope)
	}
	actor, _, err := n.nativeTaskLeaseActor(ctx, id, scope, taskID, runID)
	return actor, err
}

func (n *daemonNativeTools) nativeTaskLeaseActor(
	ctx context.Context,
	id toolspkg.ToolID,
	scope toolspkg.Scope,
	taskID string,
	runID string,
) (taskpkg.ActorContext, taskpkg.AutonomyLeaseHandle, error) {
	actor, sessionID, err := autonomyActorContext(id, scope)
	if err != nil {
		return taskpkg.ActorContext{}, taskpkg.AutonomyLeaseHandle{}, err
	}
	normalizedTaskID, err := requiredNativeString(id, "task_id", taskID)
	if err != nil {
		return taskpkg.ActorContext{}, taskpkg.AutonomyLeaseHandle{}, err
	}
	normalizedRunID, err := requiredNativeString(id, "run_id", runID)
	if err != nil {
		return taskpkg.ActorContext{}, taskpkg.AutonomyLeaseHandle{}, err
	}
	handle, err := n.lookupAutonomyLease(ctx, id, sessionID, normalizedRunID)
	if err != nil {
		return taskpkg.ActorContext{}, taskpkg.AutonomyLeaseHandle{}, err
	}
	if strings.TrimSpace(handle.TaskID) != normalizedTaskID {
		return taskpkg.ActorContext{}, taskpkg.AutonomyLeaseHandle{}, nativeAutonomyForeignRunTaskError(
			id,
			normalizedTaskID,
			normalizedRunID,
		)
	}
	return actor, handle, nil
}

func nativeTaskBlockExpiresAt(expiresAt *time.Time) time.Time {
	if expiresAt == nil {
		return time.Time{}
	}
	return expiresAt.UTC()
}

func nativeOperatorActorContext(scope toolspkg.Scope) (taskpkg.ActorContext, error) {
	if scope.Operator {
		return taskpkg.DeriveDaemonActorContext("native-tools", "tool.registry")
	}
	return actorContextFromScope(scope)
}

func nativeAutonomyForeignRunTaskError(id toolspkg.ToolID, taskID string, runID string) error {
	message := fmt.Sprintf("run %q is not leased for task %q", strings.TrimSpace(runID), strings.TrimSpace(taskID))
	return toolspkg.NewToolError(
		toolspkg.ErrorCodeDenied,
		id,
		message,
		fmt.Errorf("%w: %s", toolspkg.ErrToolDenied, message),
		toolspkg.ReasonAutonomyForeignRun,
	)
}

func nativeSessionTaskRecoverDenied(id toolspkg.ToolID) error {
	return toolspkg.NewToolError(
		toolspkg.ErrorCodeDenied,
		id,
		"task recover requires operator scope",
		fmt.Errorf("%w: task recover requires operator scope", toolspkg.ErrToolDenied),
		toolspkg.ReasonSessionDenied,
	)
}

func nativeTaskToolError(id toolspkg.ToolID, err error) error {
	if err == nil {
		return nil
	}
	if _, ok := taskpkg.AutonomyReasonOf(err); ok {
		return nativeAutonomyToolError(id, err)
	}
	switch {
	case errors.Is(err, taskpkg.ErrValidation),
		errors.Is(err, taskpkg.ErrInvalidScopeBinding),
		errors.Is(err, taskpkg.ErrImmutableField),
		errors.Is(err, taskpkg.ErrPayloadTooLarge):
		return toolspkg.NewToolError(
			toolspkg.ErrorCodeInvalidInput,
			id,
			taskpkg.RedactClaimTokens(err.Error()),
			fmt.Errorf("%w: %w", toolspkg.ErrToolInvalidInput, err),
			toolspkg.ReasonSchemaInvalid,
		)
	case errors.Is(err, taskpkg.ErrTaskNotFound),
		errors.Is(err, taskpkg.ErrTaskBlockNotFound),
		errors.Is(err, taskpkg.ErrTaskRunNotFound):
		return toolspkg.NewToolError(
			toolspkg.ErrorCodeNotFound,
			id,
			taskpkg.RedactClaimTokens(err.Error()),
			fmt.Errorf("%w: %w", toolspkg.ErrToolNotFound, err),
		)
	case errors.Is(err, taskpkg.ErrConflict),
		errors.Is(err, taskpkg.ErrInvalidClaimToken),
		errors.Is(err, taskpkg.ErrLeaseExpired),
		errors.Is(err, taskpkg.ErrInvalidStatusTransition),
		errors.Is(err, taskpkg.ErrHallucinatedTaskRefs):
		return toolspkg.NewToolError(
			toolspkg.ErrorCodeConflict,
			id,
			taskpkg.RedactClaimTokens(err.Error()),
			fmt.Errorf("%w: %w", toolspkg.ErrToolConflict, err),
		)
	case errors.Is(err, taskpkg.ErrPermissionDenied):
		return toolspkg.NewToolError(
			toolspkg.ErrorCodeDenied,
			id,
			taskpkg.RedactClaimTokens(err.Error()),
			fmt.Errorf("%w: %w", toolspkg.ErrToolDenied, err),
			toolspkg.ReasonSessionDenied,
		)
	default:
		return toolspkg.NewToolError(
			toolspkg.ErrorCodeBackendFailed,
			id,
			taskpkg.RedactClaimTokens(err.Error()),
			fmt.Errorf("%w: %w", toolspkg.ErrToolBackendFailed, err),
			toolspkg.ReasonBackendUnhealthy,
		)
	}
}

func autonomyActorContext(id toolspkg.ToolID, scope toolspkg.Scope) (taskpkg.ActorContext, string, error) {
	sessionID := strings.TrimSpace(scope.SessionID)
	if sessionID == "" {
		return taskpkg.ActorContext{}, "", toolspkg.NewToolError(
			toolspkg.ErrorCodeDenied,
			id,
			"autonomy tool requires a caller session",
			fmt.Errorf("%w: session_id is required", toolspkg.ErrToolDenied),
			toolspkg.ReasonAutonomySessionRequired,
		)
	}
	actor, err := taskpkg.DeriveAgentSessionActorContext(sessionID, scope.WorkspaceID)
	if err != nil {
		return taskpkg.ActorContext{}, "", nativeAutonomyToolError(id, err)
	}
	return actor, sessionID, nil
}

func (n *daemonNativeTools) lookupAutonomyLease(
	ctx context.Context,
	id toolspkg.ToolID,
	sessionID string,
	runID string,
) (taskpkg.AutonomyLeaseHandle, error) {
	authority, ok := n.deps.Tasks.(taskpkg.AutonomyLeaseAuthority)
	if !ok {
		return taskpkg.AutonomyLeaseHandle{}, toolspkg.NewToolError(
			toolspkg.ErrorCodeUnavailable,
			id,
			"autonomy lease authority is unavailable",
			fmt.Errorf("%w: task autonomy lease authority is unavailable", toolspkg.ErrToolUnavailable),
			toolspkg.ReasonBackendUnhealthy,
		)
	}
	handle, err := authority.LookupActiveRunForSession(ctx, sessionID, runID)
	if err != nil {
		return taskpkg.AutonomyLeaseHandle{}, nativeAutonomyToolError(id, err)
	}
	return handle, nil
}

func autonomyLeaseDuration(seconds int64) (time.Duration, error) {
	switch {
	case seconds < 0:
		return 0, fmt.Errorf("%w: lease_seconds must be zero or positive: %d", taskpkg.ErrValidation, seconds)
	case seconds == 0:
		return 0, nil
	case seconds > int64(taskpkg.MaxRunLeaseDuration/time.Second):
		return 0, fmt.Errorf(
			"%w: lease_seconds exceeds %d",
			taskpkg.ErrValidation,
			int64(taskpkg.MaxRunLeaseDuration/time.Second),
		)
	default:
		return time.Duration(seconds) * time.Second, nil
	}
}

func nativeNetworkSendToolError(id toolspkg.ToolID, err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, contract.ErrRawClaimTokenMetadata) {
		return toolspkg.NewToolError(
			toolspkg.ErrorCodeInvalidInput,
			id,
			"network send payload must not contain raw claim_token fields",
			fmt.Errorf("%w: %w", toolspkg.ErrToolInvalidInput, err),
			toolspkg.ReasonNetworkRawTokenRejected,
		)
	}
	if errors.Is(err, core.ErrNetworkValidation) {
		return toolspkg.NewToolError(
			toolspkg.ErrorCodeInvalidInput,
			id,
			err.Error(),
			fmt.Errorf("%w: %w", toolspkg.ErrToolInvalidInput, err),
			toolspkg.ReasonSchemaInvalid,
		)
	}
	return err
}

func nativeNetworkToolError(id toolspkg.ToolID, err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, network.ErrMissingField) || errors.Is(err, network.ErrInvalidField) ||
		errors.Is(err, core.ErrNetworkValidation) {
		return nativeNetworkInputError(id, err)
	}
	return err
}

func nativeNetworkInputError(id toolspkg.ToolID, err error) error {
	return toolspkg.NewToolError(
		toolspkg.ErrorCodeInvalidInput,
		id,
		taskpkg.RedactClaimTokens(err.Error()),
		fmt.Errorf("%w: %w", toolspkg.ErrToolInvalidInput, err),
		toolspkg.ReasonSchemaInvalid,
	)
}
