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
	if toolErr, ok := errors.AsType[*ToolError](err); ok {
		return normalizeToolError(id, toolErr)
	}
	return normalizeToolError(id, NewToolError(
		ErrorCodeBackendFailed,
		id,
		fmt.Sprintf("tool %q backend failed", id),
		fmt.Errorf("%w: %w", ErrToolBackendFailed, err),
		ReasonBackendUnhealthy,
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
		ReasonBackendUnhealthy,
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
