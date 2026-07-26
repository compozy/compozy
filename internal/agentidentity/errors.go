package agentidentity

import (
	"encoding/json"
	"errors"
	"strings"

	"github.com/compozy/agh/internal/api/contract"
	"github.com/compozy/agh/internal/diagnostics"
)

const (
	errorsAgentErrorKey                      = "agent_error"
	errorsInspectTheDaemonErrorAndRetryValue = "inspect the daemon error and retry"
)

const agentCommandFailedMessage = "agent command failed"

const (
	// ExitOK reports successful agent command execution.
	ExitOK = 0
	// ExitIdentityRequired reports missing caller identity input.
	ExitIdentityRequired = 64
	// ExitIdentityInvalid reports stale or mismatched caller identity.
	ExitIdentityInvalid = 65
	// ExitUnauthorized reports a caller identity that is valid but not allowed for the requested scope.
	ExitUnauthorized = 77
	// ExitUnavailable reports daemon lookup or validation infrastructure failure.
	ExitUnavailable = 69
	// ExitDoctorWarn reports doctor diagnostics with warnings.
	ExitDoctorWarn = 70
	// ExitDoctorError reports doctor diagnostics with errors.
	ExitDoctorError = 71
	// ExitConfigInvalid reports invalid runtime configuration.
	ExitConfigInvalid = 78
)

var (
	// ErrIdentityRequired reports missing required agent caller sandbox.
	ErrIdentityRequired = errors.New("agent identity required")
	// ErrIdentityStale reports a missing, unknown, stopped, or otherwise inactive session identity.
	ErrIdentityStale = errors.New("agent identity stale")
	// ErrIdentityMismatch reports env/header identity that does not match the daemon session record.
	ErrIdentityMismatch = errors.New("agent identity mismatch")
	// ErrIdentityUnauthorized reports a validated identity that is not allowed for the requested scope.
	ErrIdentityUnauthorized = errors.New("agent identity unauthorized")
	// ErrIdentityLookupUnavailable reports validation infrastructure that is not available.
	ErrIdentityLookupUnavailable = errors.New("agent identity lookup unavailable")
)

// Error carries a stable machine-readable identity failure code with an actionable message.
type Error struct {
	Code    string
	Message string
	Action  string
	Err     error
}

var _ = [1]error{(*Error)(nil)}

func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	if strings.TrimSpace(e.Action) == "" {
		return strings.TrimSpace(e.Message)
	}
	return strings.TrimSpace(e.Message) + ": " + strings.TrimSpace(e.Action)
}

func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

// ToDiagnosticItem converts the identity error into the shared diagnostic shape.
func (e *Error) ToDiagnosticItem() contract.DiagnosticItem {
	if e == nil {
		var item contract.DiagnosticItem
		return item
	}
	code := strings.TrimSpace(e.Code)
	if !contract.IsDiagnosticCode(code) {
		code = contract.CodeIdentityLookupUnavailable
	}
	message := strings.TrimSpace(e.Message)
	if message == "" {
		message = agentCommandFailedMessage
	}
	return diagnostics.NewItem(
		"agentidentity."+code,
		code,
		contract.CategorySession,
		"Agent identity error",
		message,
		contract.SeverityError,
		contract.FreshnessLive,
		diagnostics.WithSuggestedCommand(e.Action),
	)
}

// DiagnosticItem exposes the shared diagnostic shape for errors.As callers.
func (e *Error) DiagnosticItem() contract.DiagnosticItem {
	return e.ToDiagnosticItem()
}

// ErrorPayload is the stable machine-readable CLI error shape for agent namespaces.
type ErrorPayload struct {
	Code     string `json:"code"`
	Message  string `json:"message"`
	Action   string `json:"action"`
	ExitCode int    `json:"exit_code"`
}

// ErrorPayloadFor returns the stable machine-readable error payload for agent CLI output.
func ErrorPayloadFor(err error) ErrorPayload {
	payload := ErrorPayload{
		Code:     errorsAgentErrorKey,
		Message:  agentCommandFailedMessage,
		Action:   errorsInspectTheDaemonErrorAndRetryValue,
		ExitCode: ExitCodeForError(err),
	}
	var identityErr *Error
	if errors.As(err, &identityErr) && identityErr != nil {
		payload.Code = strings.TrimSpace(identityErr.Code)
		payload.Message = strings.TrimSpace(identityErr.Message)
		payload.Action = strings.TrimSpace(identityErr.Action)
	}
	if payload.Code == "" {
		payload.Code = errorsAgentErrorKey
	}
	if payload.Message == "" {
		payload.Message = agentCommandFailedMessage
	}
	if payload.Action == "" {
		payload.Action = errorsInspectTheDaemonErrorAndRetryValue
	}
	return payload
}

// MarshalErrorJSON renders a stable JSON error object for agent CLI commands.
func MarshalErrorJSON(err error) ([]byte, error) {
	return json.Marshal(struct {
		Error ErrorPayload `json:"error"`
	}{Error: ErrorPayloadFor(err)})
}

// MarshalErrorJSONL renders one stable JSONL error frame for agent CLI streaming commands.
func MarshalErrorJSONL(err error) ([]byte, error) {
	frame, marshalErr := json.Marshal(struct {
		Type  string       `json:"type"`
		Error ErrorPayload `json:"error"`
	}{
		Type:  "error",
		Error: ErrorPayloadFor(err),
	})
	if marshalErr != nil {
		return nil, marshalErr
	}
	return append(frame, '\n'), nil
}

// ExitCodeForError maps agent identity and command errors to deterministic CLI exit codes.
func ExitCodeForError(err error) int {
	switch {
	case err == nil:
		return ExitOK
	case errors.Is(err, ErrIdentityRequired):
		return ExitIdentityRequired
	case errors.Is(err, ErrIdentityLookupUnavailable):
		return ExitUnavailable
	case errors.Is(err, ErrIdentityMismatch), errors.Is(err, ErrIdentityStale):
		return ExitIdentityInvalid
	case errors.Is(err, ErrIdentityUnauthorized):
		return ExitUnauthorized
	default:
		diagnosticErr, ok := errors.AsType[interface {
			error
			DiagnosticItem() contract.DiagnosticItem
		}](err)
		if ok {
			item := diagnosticErr.DiagnosticItem()
			switch item.Code {
			case contract.CodeConfigInvalid:
				return ExitConfigInvalid
			case contract.CodeDaemonUnavailable:
				return ExitUnavailable
			case contract.CodeIdentityRequired:
				return ExitIdentityRequired
			case contract.CodeIdentityStale, contract.CodeIdentityMismatch:
				return ExitIdentityInvalid
			case contract.CodeIdentityUnauthorized:
				return ExitUnauthorized
			case contract.CodeIdentityLookupUnavailable:
				return ExitUnavailable
			}
			switch item.Severity {
			case contract.SeverityWarn:
				return ExitDoctorWarn
			case contract.SeverityError, contract.SeverityCritical:
				return ExitDoctorError
			}
		}
		return ExitUnavailable
	}
}

func identityError(err error, code string, message string, action string) error {
	return &Error{
		Code:    code,
		Message: strings.TrimSpace(message),
		Action:  strings.TrimSpace(action),
		Err:     err,
	}
}
