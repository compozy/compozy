package mcp

import (
	"context"
	"errors"
	"slices"
	"sync"

	"github.com/compozy/compozy/internal/diagnostics"
	toolspkg "github.com/compozy/compozy/internal/tools"
)

type mcpRequestRedactionContextKey struct{}

type mcpRequestRedactions struct {
	mu             sync.Mutex
	registerSecret func(string) func()
	cleanups       []func()
	closed         bool
}

type redactedMCPError struct {
	message string
	cause   error
}

func (e *redactedMCPError) Error() string {
	return e.message
}

func (e *redactedMCPError) Unwrap() error {
	return e.cause
}

func withMCPRequestRedactions(ctx context.Context) (context.Context, func()) {
	redactions := &mcpRequestRedactions{registerSecret: diagnostics.RegisterRequiredSecret}
	return context.WithValue(ctx, mcpRequestRedactionContextKey{}, redactions), redactions.cleanup
}

func registerMCPRequestSecret(ctx context.Context, value string) {
	if ctx == nil {
		return
	}
	redactions, ok := ctx.Value(mcpRequestRedactionContextKey{}).(*mcpRequestRedactions)
	if !ok || redactions == nil {
		return
	}
	redactions.register(value)
}

func (r *mcpRequestRedactions) register(value string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return
	}
	cleanup := r.registerSecret(value)
	r.cleanups = append(r.cleanups, cleanup)
}

func (r *mcpRequestRedactions) cleanup() {
	r.mu.Lock()
	if r.closed {
		r.mu.Unlock()
		return
	}
	r.closed = true
	cleanups := r.cleanups
	r.cleanups = nil
	r.mu.Unlock()

	for _, cleanup := range slices.Backward(cleanups) {
		cleanup()
	}
}

func redactMCPError(err error) error {
	if err == nil {
		return nil
	}
	switch typed := err.(type) {
	case *toolspkg.ToolError:
		return redactMCPToolError(typed)
	case *toolspkg.ValidationError:
		if typed == nil {
			return redactedMCPMissingError()
		}
		return toolspkg.NewValidationError(
			redactMCPDiagnosticString(typed.Field),
			typed.Reason,
			redactMCPDiagnosticString(typed.Detail),
		)
	case *InsufficientScopeError:
		if typed == nil {
			return redactedMCPMissingError()
		}
		return &InsufficientScopeError{Scopes: redactMCPStrings(typed.Scopes)}
	case *UnsupportedCapabilityError:
		if typed == nil {
			return redactedMCPMissingError()
		}
		return &UnsupportedCapabilityError{Capability: redactMCPDiagnosticString(typed.Capability)}
	case *UnsupportedProtocolVersionError:
		if typed == nil {
			return redactedMCPMissingError()
		}
		return &UnsupportedProtocolVersionError{Version: redactMCPDiagnosticString(typed.Version)}
	}
	if unwrap, ok := err.(interface{ Unwrap() []error }); ok {
		return redactMCPJoinedError(unwrap.Unwrap())
	}
	if unwrap, ok := err.(interface{ Unwrap() error }); ok {
		return &redactedMCPError{
			message: redactMCPDiagnosticString(err.Error()),
			cause:   redactMCPError(unwrap.Unwrap()),
		}
	}
	return redactMCPUnknownError(err)
}

func redactMCPJoinedError(children []error) error {
	redacted := make([]error, 0, len(children))
	for _, child := range children {
		if child != nil {
			redacted = append(redacted, redactMCPError(child))
		}
	}
	if len(redacted) == 0 {
		return redactedMCPMissingError()
	}
	return errors.Join(redacted...)
}

// redactMCPToolError preserves ToolError identity and maps opaque upstream data to stable classifications.
func redactMCPToolError(err *toolspkg.ToolError) *toolspkg.ToolError {
	if err == nil {
		return toolspkg.NewToolError(
			toolspkg.ErrorCodeBackendFailed,
			"",
			"mcp: upstream error",
			toolspkg.ErrToolBackendFailed,
		)
	}
	message := redactMCPDiagnosticString(err.Message)
	if message == "" {
		message = redactMCPDiagnosticString(err.Error())
	}
	redacted := toolspkg.NewToolError(
		err.Code,
		toolspkg.ToolID(redactMCPDiagnosticString(string(err.ToolID))),
		message,
		redactMCPToolErrorCause(err),
		err.ReasonCodes...,
	)
	if err.Operator != nil {
		redacted.Operator = &toolspkg.OperatorFailure{
			Cause:    redactMCPDiagnosticString(err.Operator.Cause),
			Recovery: redactMCPDiagnosticString(err.Operator.Recovery),
		}
	}
	return redacted
}

func redactMCPToolErrorCause(err *toolspkg.ToolError) error {
	classification := mcpToolErrorClassification(err.Code)
	if err.Err == nil {
		return classification
	}
	cause := redactMCPError(err.Err)
	if errors.Is(cause, classification) {
		return cause
	}
	return errors.Join(classification, cause)
}

func mcpToolErrorClassification(code toolspkg.ErrorCode) error {
	switch code {
	case toolspkg.ErrorCodeNotFound:
		return toolspkg.ErrToolNotFound
	case toolspkg.ErrorCodeConflict:
		return toolspkg.ErrToolConflict
	case toolspkg.ErrorCodeUnavailable:
		return toolspkg.ErrToolUnavailable
	case toolspkg.ErrorCodeDenied:
		return toolspkg.ErrToolDenied
	case toolspkg.ErrorCodeApprovalRequired:
		return toolspkg.ErrToolApprovalRequired
	case toolspkg.ErrorCodeInvalidInput:
		return toolspkg.ErrToolInvalidInput
	case toolspkg.ErrorCodeResultTooLarge:
		return toolspkg.ErrToolResultTooLarge
	case toolspkg.ErrorCodeResultPersistenceFailed:
		return toolspkg.ErrToolResultPersistence
	case toolspkg.ErrorCodeCanceled:
		return toolspkg.ErrToolCanceled
	case toolspkg.ErrorCodeTimedOut:
		return toolspkg.ErrToolTimedOut
	default:
		return toolspkg.ErrToolBackendFailed
	}
}

// redactMCPUnknownError replaces unsupported upstream types with a safe backend classification.
func redactMCPUnknownError(err error) error {
	return &redactedMCPError{
		message: redactMCPDiagnosticString(err.Error()),
		cause:   redactMCPKnownCause(err),
	}
}

func redactedMCPMissingError() error {
	return &redactedMCPError{
		message: "mcp: upstream error",
		cause:   toolspkg.ErrToolBackendFailed,
	}
}

func redactMCPKnownCause(err error) error {
	switch {
	case errors.Is(err, context.Canceled):
		return context.Canceled
	case errors.Is(err, context.DeadlineExceeded):
		return context.DeadlineExceeded
	case errors.Is(err, ErrAuthorizationRequired):
		return ErrAuthorizationRequired
	case errors.Is(err, toolspkg.ErrToolNotFound):
		return toolspkg.ErrToolNotFound
	case errors.Is(err, toolspkg.ErrToolConflict):
		return toolspkg.ErrToolConflict
	case errors.Is(err, toolspkg.ErrToolUnavailable):
		return toolspkg.ErrToolUnavailable
	case errors.Is(err, toolspkg.ErrToolDenied):
		return toolspkg.ErrToolDenied
	case errors.Is(err, toolspkg.ErrToolApprovalRequired):
		return toolspkg.ErrToolApprovalRequired
	case errors.Is(err, toolspkg.ErrToolInvalidInput):
		return toolspkg.ErrToolInvalidInput
	case errors.Is(err, toolspkg.ErrToolResultTooLarge):
		return toolspkg.ErrToolResultTooLarge
	case errors.Is(err, toolspkg.ErrToolResultPersistence):
		return toolspkg.ErrToolResultPersistence
	case errors.Is(err, toolspkg.ErrToolCanceled):
		return toolspkg.ErrToolCanceled
	case errors.Is(err, toolspkg.ErrToolTimedOut):
		return toolspkg.ErrToolTimedOut
	default:
		return toolspkg.ErrToolBackendFailed
	}
}

func redactMCPStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	redacted := make([]string, len(values))
	for index := range values {
		redacted[index] = redactMCPDiagnosticString(values[index])
	}
	return redacted
}

func redactMCPDiagnosticString(value string) string {
	return diagnostics.RedactExactBinary(diagnostics.Redact(value))
}
