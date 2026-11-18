package tools

import (
	"context"

	"errors"
	"fmt"
)

func contextErr(ctx context.Context, id ToolID) error {
	if ctx == nil {
		return NewToolError(
			ErrorCodeCanceled,
			id,
			"tool call context is required",
			ErrToolCanceled,
			ReasonCallCanceled,
		)
	}
	switch err := ctx.Err(); {
	case errors.Is(err, context.Canceled):
		return NewToolError(ErrorCodeCanceled, id, "tool call canceled", ErrToolCanceled, ReasonCallCanceled)
	case errors.Is(err, context.DeadlineExceeded):
		return NewToolError(ErrorCodeTimedOut, id, "tool call timed out", ErrToolTimedOut, ReasonCallTimedOut)
	default:
		return nil
	}
}

func normalizeBackendError(id ToolID, err error) error {
	if contextError := contextErrFromError(id, err); contextError != nil {
		return contextError
	}
	if disabled := providerDisabledToolError(id, err); disabled != nil {
		return disabled
	}
	if toolErr, ok := errors.AsType[*ToolError](err); ok {
		return normalizeToolError(id, toolErr)
	}
	return normalizeToolError(id, NewToolError(
		ErrorCodeBackendFailed,
		id,
		fmt.Sprintf("tool %q backend failed", id),
		fmt.Errorf("%w: %w", ErrToolBackendFailed, err),
	))
}

func normalizeHookError(id ToolID, err error) error {
	if contextError := contextErrFromError(id, err); contextError != nil {
		return contextError
	}
	return NewToolError(
		ErrorCodeDenied,
		id,
		fmt.Sprintf("tool %q denied by hook", id),
		fmt.Errorf("%w: %w", ErrToolDenied, err),
		ReasonHookDenied,
	)
}

func invalidInputError(id ToolID, message string, err error) error {
	return NewToolError(
		ErrorCodeInvalidInput,
		id,
		message,
		fmt.Errorf("%w: %w", ErrToolInvalidInput, err),
		ReasonSchemaInvalid,
	)
}

func normalizeToolError(id ToolID, err error) error {
	if err == nil {
		return nil
	}
	if disabled := providerDisabledToolError(id, err); disabled != nil {
		return disabled
	}
	if toolErr, ok := errors.AsType[*ToolError](err); ok {
		if toolErr.ToolID == "" && id != "" {
			cloned := *toolErr
			cloned.ToolID = id
			return &cloned
		}
		return toolErr
	}
	if contextError := contextErrFromError(id, err); contextError != nil {
		return contextError
	}
	return NewToolError(
		ErrorCodeBackendFailed,
		id,
		fmt.Sprintf("tool %q failed", id),
		fmt.Errorf("%w: %w", ErrToolBackendFailed, err),
	)
}

func contextErrFromError(id ToolID, err error) error {
	switch {
	case errors.Is(err, context.Canceled), errors.Is(err, ErrToolCanceled):
		return NewToolError(ErrorCodeCanceled, id, "tool call canceled", ErrToolCanceled, ReasonCallCanceled)
	case errors.Is(err, context.DeadlineExceeded), errors.Is(err, ErrToolTimedOut):
		return NewToolError(ErrorCodeTimedOut, id, "tool call timed out", ErrToolTimedOut, ReasonCallTimedOut)
	default:
		return nil
	}
}

// providerDisabledToolError promotes any error chain carrying the provider_disabled diagnostic
// code, including one a domain mapper already wrapped in another tool error, so every native
// start tool reports the same stable code.
func providerDisabledToolError(id ToolID, err error) error {
	if toolErr, ok := errors.AsType[*ToolError](err); ok && toolErr.Code == ErrorCodeProviderDisabled {
		return nil
	}
	carrier, ok := errors.AsType[interface {
		error
		DiagnosticCode() string
	}](err)
	if !ok || carrier.DiagnosticCode() != string(ErrorCodeProviderDisabled) {
		return nil
	}
	toolErr := NewToolError(ErrorCodeProviderDisabled, id, carrier.Error(), fmt.Errorf("%w: %w", ErrToolConflict, err))
	// The message names the provider and how to re-enable it; it carries no secrets.
	toolErr.PublicMessage = carrier.Error()
	return toolErr
}
