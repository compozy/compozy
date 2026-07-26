package daemon

import (
	"errors"
	"fmt"

	automationpkg "github.com/compozy/agh/internal/automation"
	toolspkg "github.com/compozy/agh/internal/tools"
)

func nativeAutomationToolError(id toolspkg.ToolID, err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, automationpkg.ErrJobNotFound),
		errors.Is(err, automationpkg.ErrSuggestionNotFound),
		errors.Is(err, automationpkg.ErrTriggerNotFound),
		errors.Is(err, automationpkg.ErrRunNotFound),
		errors.Is(err, automationpkg.ErrJobOverlayNotFound),
		errors.Is(err, automationpkg.ErrTriggerOverlayNotFound):
		return toolspkg.NewToolError(
			toolspkg.ErrorCodeNotFound,
			id,
			err.Error(),
			fmt.Errorf("%w: %w", toolspkg.ErrToolNotFound, err),
			toolspkg.ReasonToolUnknown,
		)
	case errors.Is(err, automationpkg.ErrDefinitionReadOnly):
		return toolspkg.NewToolError(
			toolspkg.ErrorCodeDenied,
			id,
			err.Error(),
			fmt.Errorf("%w: %w", toolspkg.ErrToolDenied, err),
			toolspkg.ReasonAutomationScopeForbidden,
		)
	case errors.Is(err, automationpkg.ErrDaemonLifecycleCommandBlocked):
		return toolspkg.NewToolError(
			toolspkg.ErrorCodeInvalidInput,
			id,
			err.Error(),
			fmt.Errorf("%w: %w", toolspkg.ErrToolInvalidInput, err),
			toolspkg.ReasonAutomationValidationFailed,
		)
	case errors.Is(err, automationpkg.ErrManagerNotRunning):
		return toolspkg.NewToolError(
			toolspkg.ErrorCodeUnavailable,
			id,
			err.Error(),
			fmt.Errorf("%w: %w", toolspkg.ErrToolUnavailable, err),
			toolspkg.ReasonDependencyMissing,
		)
	case errors.Is(err, automationpkg.ErrFireLimitReached),
		errors.Is(err, automationpkg.ErrSuggestionResolved),
		errors.Is(err, automationpkg.ErrSuggestionPendingCap),
		errors.Is(err, automationpkg.ErrConcurrencyLimitReached),
		errors.Is(err, automationpkg.ErrScheduledFireAlreadyClaimed):
		return toolspkg.NewToolError(
			toolspkg.ErrorCodeConflict,
			id,
			err.Error(),
			fmt.Errorf("%w: %w", toolspkg.ErrToolConflict, err),
			toolspkg.ReasonAutomationValidationFailed,
		)
	case errors.Is(err, automationpkg.ErrJobNameTaken),
		errors.Is(err, automationpkg.ErrTriggerNameTaken),
		errors.Is(err, automationpkg.ErrTriggerWebhookIDTaken),
		errors.Is(err, automationpkg.ErrWebhookSecretRequired),
		errors.Is(err, automationpkg.ErrOverlayRequiresConfigSource),
		errors.Is(err, automationpkg.ErrListCursorInvalid):
		return nativeAutomationValidationError(id, err)
	default:
		return err
	}
}

func nativeAutomationValidationError(id toolspkg.ToolID, err error) error {
	return toolspkg.NewToolError(
		toolspkg.ErrorCodeInvalidInput,
		id,
		"automation validation failed",
		fmt.Errorf("%w: %w", toolspkg.ErrToolInvalidInput, err),
		toolspkg.ReasonAutomationValidationFailed,
	)
}

func nativeAutomationScopeError(
	id toolspkg.ToolID,
	resource string,
	resourceID string,
	source automationpkg.JobSource,
) error {
	return toolspkg.NewToolError(
		toolspkg.ErrorCodeDenied,
		id,
		fmt.Sprintf("automation %s %q source %q is immutable by tools", resource, resourceID, source),
		toolspkg.ErrToolDenied,
		toolspkg.ReasonAutomationScopeForbidden,
	)
}
