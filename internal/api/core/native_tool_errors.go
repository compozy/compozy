package core

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"github.com/compozy/compozy/internal/diagnostics"
	toolspkg "github.com/compozy/compozy/internal/tools"
)

// NativeToolError preserves domain classifications at the native provider boundary.
func NativeToolError(id toolspkg.ToolID, err error) error {
	if err == nil || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return err
	}
	cause := err
	mapped := toolspkg.NewToolError(toolspkg.ErrorCodeBackendFailed, id, "",
		fmt.Errorf("%w: %w", toolspkg.ErrToolBackendFailed, err), toolspkg.ReasonBackendUnhealthy)
	if existing, ok := errors.AsType[*toolspkg.ToolError](err); ok {
		// Specialized adapters already expose their domain outcome through a
		// specific code/reason or a structured partial result. Preserve that contract.
		if existing.PartialResult != nil || !genericNativeToolCode(existing.Code) ||
			!genericNativeToolReasons(existing.ReasonCodes) {
			return existing
		}
		cloned := *existing
		mapped = &cloned
		if existing.Err != nil {
			cause = existing.Err
		}
	}
	cause = nativeToolDomainCause(cause)
	payload := ErrorPayloadForError(cause)
	if existing, ok := errors.AsType[*toolspkg.ToolError](err); ok && payload.Code == "" &&
		preserveUnclassifiedNativeError(existing, cause) {
		return existing
	}
	if payload.Code != "" && genericNativeToolCode(mapped.Code) {
		mapped.Code = toolspkg.ErrorCode(payload.Code)
		mapped.HTTPStatus = nativeDomainErrorStatus(cause)
		mapped.ReasonCodes = nil
	}
	if mapped.ToolID == "" {
		mapped.ToolID = id
	}
	message := mapped.Message
	if message == "" || payload.Code != "" {
		message = payload.Error
	}
	if message == string(mapped.Code) {
		if readable := safeSessionToolErrorMessage(mapped.Code); readable != "" {
			message = readable
		}
	}
	mapped.PublicMessage = sanitizeNativeToolMessage(message)
	mapped.Message = mapped.PublicMessage
	return mapped
}

func preserveUnclassifiedNativeError(existing *toolspkg.ToolError, cause error) bool {
	return existing.Code != toolspkg.ErrorCodeBackendFailed && existing.Code != toolspkg.ErrorCodeConflict ||
		cause == toolspkg.ErrToolBackendFailed || cause == toolspkg.ErrToolConflict
}

func genericNativeToolReasons(reasons []toolspkg.ReasonCode) bool {
	for _, reason := range reasons {
		switch reason {
		case toolspkg.ReasonBackendUnhealthy, toolspkg.ReasonConflictedID,
			toolspkg.ReasonConfigValidationFailed, toolspkg.ReasonToolUnknown,
			toolspkg.ReasonDependencyMissing, toolspkg.ReasonSchemaInvalid:
		default:
			return false
		}
	}
	return true
}

func nativeToolDomainCause(err error) error {
	joined, ok := err.(interface{ Unwrap() []error })
	if !ok {
		return err
	}
	causes := joined.Unwrap()
	if len(causes) != 2 {
		return err
	}
	switch causes[0] {
	case toolspkg.ErrToolBackendFailed, toolspkg.ErrToolConflict, toolspkg.ErrToolInvalidInput,
		toolspkg.ErrToolNotFound, toolspkg.ErrToolUnavailable, toolspkg.ErrToolDenied:
		return causes[1]
	default:
		return err
	}
}

func genericNativeToolCode(code toolspkg.ErrorCode) bool {
	switch code {
	case toolspkg.ErrorCodeBackendFailed, toolspkg.ErrorCodeConflict, toolspkg.ErrorCodeInvalidInput,
		toolspkg.ErrorCodeNotFound, toolspkg.ErrorCodeUnavailable:
		return true
	default:
		return false
	}
}

func nativeDomainErrorStatus(err error) int {
	if WorktreeErrorCode(err) != "" {
		return StatusForWorktreeError(err)
	}
	for _, classify := range []func(error) int{
		StatusForSessionError, StatusForWorkspaceError, StatusForGatewayError,
		StatusForLoopError, StatusForSettingsError,
	} {
		if status := classify(err); status != http.StatusInternalServerError {
			return status
		}
	}
	return http.StatusInternalServerError
}

// Unquoted paths may contain spaces. Conservatively redact through the next
// error-chain separator rather than exposing the remaining path components.
var nativeToolHostPath = regexp.MustCompile(
	`"(?:[A-Za-z]:[\\/]|\\\\|~/|/)[^"]*"|'(?:[A-Za-z]:[\\/]|\\\\|~/|/)[^']*'|(?:[A-Za-z]:[\\/]|\\\\|~/|/)[^:\r\n"'<>]*`,
)

func sanitizeNativeToolMessage(message string) string {
	message = diagnostics.Redact(message)
	message = nativeToolHostPath.ReplaceAllString(message, "[path]")
	return diagnostics.RedactAndBound(strings.TrimSpace(message), 2048)
}
