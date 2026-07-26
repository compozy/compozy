package daemon

import (
	"errors"
	"fmt"

	taskpkg "github.com/compozy/agh/internal/task"
	toolspkg "github.com/compozy/agh/internal/tools"
)

func nativeAutonomyToolError(id toolspkg.ToolID, err error) error {
	if err == nil {
		return nil
	}
	if reason, ok := taskpkg.AutonomyReasonOf(err); ok {
		code, toolReason, cause := autonomyToolErrorCodeAndReason(reason)
		return toolspkg.NewToolError(
			code,
			id,
			taskpkg.RedactClaimTokens(err.Error()),
			fmt.Errorf("%w: %w", cause, err),
			toolReason,
		)
	}
	switch {
	case errors.Is(err, taskpkg.ErrValidation),
		errors.Is(err, taskpkg.ErrInvalidScopeBinding),
		errors.Is(err, taskpkg.ErrImmutableField):
		return toolspkg.NewToolError(
			toolspkg.ErrorCodeInvalidInput,
			id,
			taskpkg.RedactClaimTokens(err.Error()),
			fmt.Errorf("%w: %w", toolspkg.ErrToolInvalidInput, err),
			toolspkg.ReasonSchemaInvalid,
		)
	case errors.Is(err, taskpkg.ErrActiveRunLease):
		return toolspkg.NewToolError(
			toolspkg.ErrorCodeConflict,
			id,
			taskpkg.RedactClaimTokens(err.Error()),
			fmt.Errorf("%w: %w", toolspkg.ErrToolConflict, err),
			toolspkg.ReasonAutonomyLeaseAlreadyHeld,
		)
	case errors.Is(err, taskpkg.ErrWorkspaceActiveRunCapReached):
		return toolspkg.NewToolError(
			toolspkg.ErrorCodeConflict,
			id,
			taskpkg.RedactClaimTokens(err.Error()),
			fmt.Errorf("%w: %w", toolspkg.ErrToolConflict, err),
			toolspkg.ReasonAutonomyWorkspaceCapacity,
		)
	case errors.Is(err, taskpkg.ErrPermissionDenied):
		return toolspkg.NewToolError(
			toolspkg.ErrorCodeDenied,
			id,
			taskpkg.RedactClaimTokens(err.Error()),
			fmt.Errorf("%w: %w", toolspkg.ErrToolDenied, err),
			toolspkg.ReasonSessionDenied,
		)
	case errors.Is(err, taskpkg.ErrInvalidClaimToken),
		errors.Is(err, taskpkg.ErrLeaseExpired):
		return toolspkg.NewToolError(
			toolspkg.ErrorCodeConflict,
			id,
			taskpkg.RedactClaimTokens(err.Error()),
			fmt.Errorf("%w: %w", toolspkg.ErrToolConflict, err),
			toolspkg.ReasonAutonomyLeaseExpired,
		)
	case errors.Is(err, taskpkg.ErrInvalidStatusTransition),
		errors.Is(err, taskpkg.ErrConflict),
		errors.Is(err, taskpkg.ErrHallucinatedTaskRefs):
		return toolspkg.NewToolError(
			toolspkg.ErrorCodeConflict,
			id,
			taskpkg.RedactClaimTokens(err.Error()),
			fmt.Errorf("%w: %w", toolspkg.ErrToolConflict, err),
		)
	default:
		return err
	}
}

func autonomyToolErrorCodeAndReason(reason taskpkg.AutonomyReasonCode) (
	toolspkg.ErrorCode,
	toolspkg.ReasonCode,
	error,
) {
	switch reason {
	case taskpkg.AutonomySessionRequired:
		return toolspkg.ErrorCodeDenied, toolspkg.ReasonAutonomySessionRequired, toolspkg.ErrToolDenied
	case taskpkg.AutonomyForeignRun:
		return toolspkg.ErrorCodeDenied, toolspkg.ReasonAutonomyForeignRun, toolspkg.ErrToolDenied
	case taskpkg.AutonomyNoActiveLease:
		return toolspkg.ErrorCodeConflict, toolspkg.ReasonAutonomyNoActiveLease, toolspkg.ErrToolConflict
	case taskpkg.AutonomyLeaseExpired:
		return toolspkg.ErrorCodeConflict, toolspkg.ReasonAutonomyLeaseExpired, toolspkg.ErrToolConflict
	case taskpkg.AutonomyLeaseAlreadyHeld:
		return toolspkg.ErrorCodeConflict, toolspkg.ReasonAutonomyLeaseAlreadyHeld, toolspkg.ErrToolConflict
	default:
		return toolspkg.ErrorCodeConflict, toolspkg.ReasonAutonomyLeaseExpired, toolspkg.ErrToolConflict
	}
}
