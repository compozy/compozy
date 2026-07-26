package daemon

import (
	"fmt"
	"net/http"

	toolspkg "github.com/compozy/agh/internal/tools"
)

func nativeBundleConfirmationDenied(id toolspkg.ToolID) error {
	const message = "network requirement confirmation requires operator scope"
	return toolspkg.NewToolError(
		toolspkg.ErrorCodeDenied,
		id,
		message,
		fmt.Errorf("%w: %s", toolspkg.ErrToolDenied, message),
		toolspkg.ReasonPolicyDenied,
	)
}

func nativeHTTPStatusToolError(id toolspkg.ToolID, err error, status int) error {
	code := toolspkg.ErrorCodeBackendFailed
	cause := toolspkg.ErrToolBackendFailed
	reason := toolspkg.ReasonBackendUnhealthy
	switch status {
	case http.StatusBadRequest, http.StatusUnprocessableEntity, http.StatusRequestEntityTooLarge:
		code = toolspkg.ErrorCodeInvalidInput
		cause = toolspkg.ErrToolInvalidInput
		reason = toolspkg.ReasonConfigValidationFailed
	case http.StatusForbidden:
		code = toolspkg.ErrorCodeDenied
		cause = toolspkg.ErrToolDenied
		reason = toolspkg.ReasonPolicyDenied
	case http.StatusNotFound:
		code = toolspkg.ErrorCodeNotFound
		cause = toolspkg.ErrToolNotFound
		reason = toolspkg.ReasonToolUnknown
	case http.StatusConflict:
		code = toolspkg.ErrorCodeConflict
		cause = toolspkg.ErrToolConflict
		reason = toolspkg.ReasonConflictedID
	case http.StatusServiceUnavailable:
		code = toolspkg.ErrorCodeUnavailable
		cause = toolspkg.ErrToolUnavailable
		reason = toolspkg.ReasonDependencyMissing
	}
	return toolspkg.NewToolError(
		code,
		id,
		err.Error(),
		fmt.Errorf("%w: %w", cause, err),
		reason,
	)
}
