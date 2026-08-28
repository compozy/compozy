package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	diagnosticspkg "github.com/compozy/compozy/internal/diagnostics"
	terminalpkg "github.com/compozy/compozy/internal/terminal"
)

type terminalTransportError struct {
	code    string
	message string
	err     error
}

const (
	terminalTransportCodeInvalidRequest = "invalid_request"
	terminalTransportCodeUnavailable    = "service_unavailable"
	terminalTransportCodeInternal       = "internal_error"
)

func newTerminalTransportError(code, message string, err error) *terminalTransportError {
	return &terminalTransportError{code: strings.TrimSpace(code), message: strings.TrimSpace(message), err: err}
}

func terminalInvalidRequest(message string, cause error) error {
	return newTerminalTransportError(terminalTransportCodeInvalidRequest, message, cause)
}

func (e *terminalTransportError) Error() string {
	if e == nil {
		return "terminal transport error"
	}
	if e.message != "" {
		return e.message
	}
	if e.err != nil {
		return e.err.Error()
	}
	return e.code
}

func (e *terminalTransportError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.err
}

func (e *terminalTransportError) TerminalErrorEnvelope() contract.TerminalErrorResponse {
	if e == nil {
		return contract.TerminalErrorResponse{}
	}
	return contract.TerminalErrorResponse{Error: contract.TerminalErrorDetail{
		Code: e.code, Message: e.Error(),
	}}
}

func renderTerminalExecutionError(err error) (string, bool) {
	terminalErr, ok := errors.AsType[*terminalpkg.Error](err)
	if !ok || !contract.IsTerminalErrorCode(contract.TerminalErrorCode(terminalErr.Code)) {
		return "", false
	}
	code := string(terminalErr.Code)
	message := strings.TrimSpace(terminalErr.Error())
	if message == "" || message == code {
		return "error: " + code, true
	}
	return "error: " + code + " — " + message, true
}

func terminalExecutionErrorPayload(err *terminalpkg.Error) contract.TerminalErrorResponse {
	return contract.TerminalErrorResponse{Error: contract.TerminalErrorDetail{
		Code:    string(err.Code),
		Message: err.Error(),
		Details: contract.TerminalErrorDetailsFromDomain(err),
	}}
}

func marshalTerminalExecutionError(args []string, payload contract.TerminalErrorResponse) ([]byte, bool) {
	switch requestedOutputFormat(args) {
	case OutputJSON:
		encoded, err := json.Marshal(payload)
		return encoded, err == nil
	case OutputJSONL:
		encoded, err := json.Marshal(payload)
		return append(encoded, '\n'), err == nil
	case OutputToon:
		return []byte(renderToonObject(
			"error",
			[]string{cliCodeKey, clientMessageKey},
			[]string{string(payload.Error.Code), payload.Error.Message},
		)), true
	default:
		return nil, false
	}
}

func marshalTerminalDiagnosticExecutionError(args []string, err error) ([]byte, bool) {
	if len(args) == 0 || args[0] != terminalCommandKey {
		return nil, false
	}
	terminalErr, ok := errors.AsType[*terminalpkg.Error](err)
	if ok && contract.IsTerminalErrorCode(contract.TerminalErrorCode(terminalErr.Code)) {
		payload := terminalExecutionErrorPayload(terminalErr)
		payload.Error.Message = diagnosticspkg.Redact(err.Error())
		return marshalTerminalExecutionError(args, payload)
	}
	transportErr, transport := errors.AsType[*terminalTransportError](err)
	if !transport {
		transportErr = newTerminalTransportError(
			terminalTransportCodeInternal,
			diagnosticspkg.Redact(err.Error()),
			fmt.Errorf("terminal command: %w", err),
		)
	}
	payload := transportErr.TerminalErrorEnvelope()
	return marshalTerminalExecutionError(args, payload)
}
