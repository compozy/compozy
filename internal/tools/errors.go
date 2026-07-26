package tools

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
)

const nilErrorText = "<nil>"

var (
	// ErrToolNotFound reports an unknown tool id.
	ErrToolNotFound = errors.New("tools: tool not found")
	// ErrToolConflict reports a canonical id or sanitized-name conflict.
	ErrToolConflict = errors.New("tools: tool conflict")
	// ErrToolUnavailable reports an unavailable tool.
	ErrToolUnavailable = errors.New("tools: tool unavailable")
	// ErrToolDenied reports policy denial.
	ErrToolDenied = errors.New("tools: tool denied")
	// ErrToolApprovalRequired reports required approval.
	ErrToolApprovalRequired = errors.New("tools: tool approval required")
	// ErrToolInvalidInput reports invalid tool input.
	ErrToolInvalidInput = errors.New("tools: invalid tool input")
	// ErrToolResultTooLarge reports result budget overflow.
	ErrToolResultTooLarge = errors.New("tools: tool result too large")
	// ErrToolResultPersistence reports oversized result offload failure.
	ErrToolResultPersistence = errors.New("tools: tool result persistence failed")
	// ErrToolBackendFailed reports a backend adapter failure.
	ErrToolBackendFailed = errors.New("tools: backend failed")
	// ErrToolCanceled reports call cancellation.
	ErrToolCanceled = errors.New("tools: tool call canceled")
	// ErrToolTimedOut reports call deadline expiration.
	ErrToolTimedOut = errors.New("tools: tool call timed out")
)

// ErrorCode is the stable public tool error code.
type ErrorCode string

const (
	// ErrorCodeNotFound maps to ErrToolNotFound.
	ErrorCodeNotFound ErrorCode = "tool_not_found"
	// ErrorCodeConflict maps to ErrToolConflict.
	ErrorCodeConflict ErrorCode = "tool_conflict"
	// ErrorCodeUnavailable maps to ErrToolUnavailable.
	ErrorCodeUnavailable ErrorCode = "tool_unavailable"
	// ErrorCodeDenied maps to ErrToolDenied.
	ErrorCodeDenied ErrorCode = "tool_denied"
	// ErrorCodeApprovalRequired maps to ErrToolApprovalRequired.
	ErrorCodeApprovalRequired ErrorCode = "tool_approval_required"
	// ErrorCodeInvalidInput maps to ErrToolInvalidInput.
	ErrorCodeInvalidInput ErrorCode = "tool_invalid_input"
	// ErrorCodeResultTooLarge maps to ErrToolResultTooLarge.
	ErrorCodeResultTooLarge ErrorCode = "tool_result_too_large"
	// ErrorCodeResultPersistenceFailed maps to ErrToolResultPersistence.
	ErrorCodeResultPersistenceFailed ErrorCode = "tool_result_persistence_failed"
	// ErrorCodeBackendFailed maps to ErrToolBackendFailed.
	ErrorCodeBackendFailed ErrorCode = "tool_backend_failed"
	// ErrorCodeCanceled maps to ErrToolCanceled.
	ErrorCodeCanceled ErrorCode = "tool_canceled"
	// ErrorCodeTimedOut maps to ErrToolTimedOut.
	ErrorCodeTimedOut ErrorCode = "tool_timed_out"
	// ErrorCodeModelNotFound reports an unknown provider-model curation target.
	ErrorCodeModelNotFound ErrorCode = "model_not_found"
	// ErrorCodeReasoningEffortUnsupported reports an unavailable model effort.
	ErrorCodeReasoningEffortUnsupported ErrorCode = "reasoning_effort_unsupported"
	// ErrorCodeAgentNameReserved reports an authored agent identity reserved for daemon roles.
	ErrorCodeAgentNameReserved ErrorCode = "agent_name_reserved"
)

// ToolError carries stable reason codes with a wrapped cause.
type ToolError struct {
	Code          ErrorCode        `json:"code"`
	ToolID        ToolID           `json:"tool_id,omitempty"`
	Message       string           `json:"message"`
	ReasonCodes   []ReasonCode     `json:"reason_codes,omitempty"`
	Operator      *OperatorFailure `json:"operator,omitempty"`
	PartialResult *ToolResult      `json:"partial_result,omitempty"`
	Err           error            `json:"-"`
}

// WithPartialResult returns a cloned error carrying a redacted bounded result preview.
func (e *ToolError) WithPartialResult(result ToolResult) *ToolError {
	if e == nil {
		return nil
	}
	cloned := *e
	partial := cloneToolResult(result)
	cloned.PartialResult = &partial
	return &cloned
}

// Error returns the public error message.
func (e *ToolError) Error() string {
	if e == nil {
		return nilErrorText
	}
	if e.Message != "" {
		return e.Message
	}
	if e.Err != nil {
		return e.Err.Error()
	}
	return string(e.Code)
}

// Unwrap returns the wrapped cause.
func (e *ToolError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

// NewToolError builds a stable tool error envelope.
func NewToolError(code ErrorCode, id ToolID, message string, err error, reasons ...ReasonCode) *ToolError {
	return &ToolError{
		Code:        code,
		ToolID:      id,
		Message:     message,
		ReasonCodes: append([]ReasonCode(nil), reasons...),
		Err:         err,
	}
}

// ValidationError describes deterministic contract validation failures.
type ValidationError struct {
	Field  string
	Reason ReasonCode
	Detail string
}

// Error returns a stable validation message.
func (e *ValidationError) Error() string {
	if e == nil {
		return nilErrorText
	}
	var builder strings.Builder
	builder.WriteString("tools: validation failed")
	if e.Field != "" {
		builder.WriteString(": ")
		builder.WriteString(e.Field)
	}
	if e.Reason != "" {
		builder.WriteString(": ")
		builder.WriteString(string(e.Reason))
	}
	if e.Detail != "" {
		builder.WriteString(": ")
		builder.WriteString(e.Detail)
	}
	return builder.String()
}

// NewValidationError builds a deterministic validation error.
func NewValidationError(field string, reason ReasonCode, detail string) *ValidationError {
	return &ValidationError{Field: field, Reason: reason, Detail: detail}
}

// ReasonOf extracts the primary deterministic reason from an error.
func ReasonOf(err error) (ReasonCode, bool) {
	if validation, ok := errors.AsType[*ValidationError](err); ok {
		return validation.Reason, true
	}
	var toolErr *ToolError
	if errors.As(err, &toolErr) && len(toolErr.ReasonCodes) > 0 {
		return toolErr.ReasonCodes[0], true
	}
	return "", false
}

func wrapField(err error, field string) error {
	if validation, ok := errors.AsType[*ValidationError](err); ok {
		return NewValidationError(field, validation.Reason, validation.Detail)
	}
	return fmt.Errorf("%s: %w", field, err)
}

func isNilInterface(value any) bool {
	if value == nil {
		return true
	}
	reflected := reflect.ValueOf(value)
	switch reflected.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return reflected.IsNil()
	default:
		return false
	}
}
