package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/compozy/agh/internal/agentidentity"
	"github.com/compozy/agh/internal/api/contract"
	diagnosticspkg "github.com/compozy/agh/internal/diagnostics"
)

type daemonAPIError struct {
	statusCode int
	status     string
	payload    contract.ErrorPayload
}

type windowManagerAPIError struct {
	statusCode int
	status     string
	payload    contract.WindowManagerErrorPayload
}

func (e *windowManagerAPIError) Error() string {
	if e == nil {
		return nilToolErrorString
	}
	if message := strings.TrimSpace(e.payload.Error); message != "" {
		return message
	}
	return strings.TrimSpace(e.status)
}

func (e *windowManagerAPIError) cliExitCode() int {
	if e == nil {
		return 1
	}
	return (&daemonAPIError{statusCode: e.statusCode}).cliExitCode()
}

func (e *windowManagerAPIError) windowManagerErrorPayload() contract.WindowManagerErrorPayload {
	if e == nil {
		return contract.WindowManagerErrorPayload{}
	}
	return e.payload
}

func (e *daemonAPIError) Error() string {
	if e == nil {
		return nilToolErrorString
	}
	if message := strings.TrimSpace(e.payload.Error); message != "" {
		return message
	}
	return strings.TrimSpace(e.status)
}

func (e *daemonAPIError) cliExitCode() int {
	if e == nil {
		return 1
	}
	switch e.statusCode {
	case http.StatusBadRequest, http.StatusConflict:
		return agentidentity.ExitIdentityInvalid
	case http.StatusUnauthorized, http.StatusForbidden:
		return agentidentity.ExitUnauthorized
	case http.StatusUnprocessableEntity:
		return agentidentity.ExitConfigInvalid
	case http.StatusNotFound, http.StatusGone, http.StatusRequestTimeout,
		http.StatusTooManyRequests, http.StatusInternalServerError,
		http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return agentidentity.ExitUnavailable
	default:
		return 1
	}
}

func (e *daemonAPIError) errorPayload() contract.ErrorPayload {
	if e == nil {
		return contract.ErrorPayload{}
	}
	return e.payload
}

func readAPIError(response *http.Response) error {
	body, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		return fmt.Errorf("cli: read api error response: %w", err)
	}
	return readAPIErrorBody(response.StatusCode, response.Status, body)
}

func readAPIErrorBody(statusCode int, status string, body []byte) error {
	var windowManagerPayload contract.WindowManagerErrorPayload
	if len(body) > 0 && json.Unmarshal(body, &windowManagerPayload) == nil &&
		windowManagerPayload.Code != "" {
		windowManagerPayload.Error = redactToolDiagnostic(windowManagerPayload.Error)
		return &windowManagerAPIError{
			statusCode: statusCode,
			status:     status,
			payload:    windowManagerPayload,
		}
	}
	var payload contract.ErrorPayload
	if len(body) > 0 && json.Unmarshal(body, &payload) == nil && strings.TrimSpace(payload.Error) != "" {
		payload.Error = redactToolDiagnostic(payload.Error)
		cause := errors.New(payload.Error)
		if payload.Diagnostic != nil {
			return diagnosticspkg.NewStructuredError(*payload.Diagnostic, cause)
		}
		return &daemonAPIError{statusCode: statusCode, status: status, payload: payload}
	}
	var memoryPayload contract.MemoryErrorPayload
	if len(body) > 0 && json.Unmarshal(body, &memoryPayload) == nil &&
		strings.TrimSpace(memoryPayload.Code) != "" {
		message := strings.TrimSpace(memoryPayload.Message)
		if message == "" {
			message = strings.TrimSpace(memoryPayload.Code)
		}
		return fmt.Errorf("%s: %s", strings.TrimSpace(memoryPayload.Code), redactToolDiagnostic(message))
	}
	var toolPayload contract.ToolErrorResponse
	if len(body) > 0 && json.Unmarshal(body, &toolPayload) == nil && toolPayload.Error.Code != "" {
		return newToolAPIError(statusCode, status, toolPayload)
	}

	message := strings.TrimSpace(string(body))
	if message == "" {
		message = status
	}
	message = redactToolDiagnostic(message)
	if strings.TrimSpace(status) != "" {
		message = fmt.Sprintf("daemon api %s: %s", status, message)
	}
	return &daemonAPIError{
		statusCode: statusCode,
		status:     status,
		payload:    contract.ErrorPayload{Error: message},
	}
}

func drainResponseBody(method string, path string, body io.Reader) error {
	if _, err := io.Copy(io.Discard, body); err != nil {
		return fmt.Errorf("cli: drain %s %s response: %w", method, path, err)
	}
	return nil
}
