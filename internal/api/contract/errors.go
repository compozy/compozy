package contract

import (
	"errors"
	"net/http"
	"strings"
)

type ErrorCode string

const (
	CodeInvalidRequest        ErrorCode = "invalid_request"
	CodeValidationError       ErrorCode = "validation_error"
	CodeNotFound              ErrorCode = "not_found"
	CodeConflict              ErrorCode = "conflict"
	CodeServiceUnavailable    ErrorCode = "service_unavailable"
	CodeInternalError         ErrorCode = "internal_error"
	CodeSchemaTooNew          ErrorCode = "schema_too_new"
	CodeDaemonNotReady        ErrorCode = "daemon_not_ready"
	CodeDaemonActiveRuns      ErrorCode = "daemon_active_runs"
	CodeWorkspaceRequired     ErrorCode = "workspace_required"
	CodePathRequired          ErrorCode = "path_required"
	CodeNameRequired          ErrorCode = "name_required"
	CodeRoundInvalid          ErrorCode = "round_invalid"
	CodeLimitInvalid          ErrorCode = "limit_invalid"
	CodeInvalidCursor         ErrorCode = "invalid_cursor"
	CodeSyncTargetRequired    ErrorCode = "sync_target_required"
	CodeForceInvalid          ErrorCode = "force_invalid"
	CodeWorkflowForceRequired ErrorCode = "workflow_force_required"
	CodeWorkspacePathNeeded   ErrorCode = "workspace_path_required"
	CodePromptRequired        ErrorCode = "prompt_required"
	CodeStreamUnavailable     ErrorCode = "stream_unavailable"
)

var CanonicalErrorCodes = []ErrorCode{
	CodeInvalidRequest,
	CodeValidationError,
	CodeNotFound,
	CodeConflict,
	CodeServiceUnavailable,
	CodeInternalError,
	CodeSchemaTooNew,
	CodeDaemonNotReady,
	CodeDaemonActiveRuns,
	CodeWorkspaceRequired,
	CodePathRequired,
	CodeNameRequired,
	CodeRoundInvalid,
	CodeLimitInvalid,
	CodeInvalidCursor,
	CodeSyncTargetRequired,
	CodeForceInvalid,
	CodeWorkflowForceRequired,
	CodeWorkspacePathNeeded,
	CodePromptRequired,
	CodeStreamUnavailable,
}

type TransportError struct {
	RequestID string         `json:"request_id"`
	Code      string         `json:"code"`
	Message   string         `json:"message"`
	Details   map[string]any `json:"details,omitempty"`
}

type TransportErrorResponse = TransportError

type Problem struct {
	Status  int
	Code    string
	Message string
	Details map[string]any
	Err     error
}

func NewProblem(status int, code string, message string, details map[string]any, err error) *Problem {
	return &Problem{
		Status:  status,
		Code:    strings.TrimSpace(code),
		Message: strings.TrimSpace(message),
		Details: details,
		Err:     err,
	}
}

func (p *Problem) Error() string {
	if p == nil {
		return ""
	}
	if strings.TrimSpace(p.Message) != "" {
		return p.Message
	}
	if p.Err != nil {
		return p.Err.Error()
	}
	if text := http.StatusText(p.Status); text != "" {
		return text
	}
	return "transport error"
}

func (p *Problem) Unwrap() error {
	if p == nil {
		return nil
	}
	return p.Err
}

func DefaultCodeForStatus(status int) string {
	switch status {
	case http.StatusBadRequest:
		return string(CodeInvalidRequest)
	case http.StatusNotFound:
		return string(CodeNotFound)
	case http.StatusConflict:
		return string(CodeConflict)
	case http.StatusUnprocessableEntity:
		return string(CodeValidationError)
	case http.StatusServiceUnavailable:
		return string(CodeServiceUnavailable)
	default:
		return string(CodeInternalError)
	}
}

func MessageForStatus(status int, err error, maskInternal bool) string {
	var problem *Problem
	if errors.As(err, &problem) && problem != nil && strings.TrimSpace(problem.Message) != "" {
		code := strings.TrimSpace(problem.Code)
		if !maskInternal || status < http.StatusInternalServerError ||
			(code != "" && code != string(CodeInternalError)) {
			return problem.Message
		}
	}

	switch {
	case err == nil:
		if text := http.StatusText(status); text != "" {
			return text
		}
		return "transport error"
	case maskInternal && status >= http.StatusInternalServerError:
		if text := http.StatusText(status); text != "" {
			return text
		}
		return "internal server error"
	default:
		return err.Error()
	}
}

func ErrorCodeForStatus(status int, err error) string {
	var problem *Problem
	if errors.As(err, &problem) && problem != nil && strings.TrimSpace(problem.Code) != "" {
		return problem.Code
	}
	return DefaultCodeForStatus(status)
}

func ErrorDetails(err error, fallback map[string]any) map[string]any {
	if len(fallback) > 0 {
		return fallback
	}
	var problem *Problem
	if errors.As(err, &problem) && problem != nil && len(problem.Details) > 0 {
		return problem.Details
	}
	return nil
}

func TransportErrorEnvelope(
	requestID string,
	status int,
	err error,
	details map[string]any,
	maskInternal bool,
) TransportError {
	return TransportError{
		RequestID: strings.TrimSpace(requestID),
		Code:      ErrorCodeForStatus(status, err),
		Message:   MessageForStatus(status, err, maskInternal),
		Details:   ErrorDetails(err, details),
	}
}
