package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/compozy/compozy/internal/agentidentity"
	"github.com/compozy/compozy/internal/api/contract"
	diagnosticspkg "github.com/compozy/compozy/internal/diagnostics"
)

type daemonAPIError struct {
	statusCode int
	status     string
	payload    contract.ErrorPayload
}

type extensionOperationAPIError struct {
	statusCode int
	status     string
	payload    contract.ExtensionOperationErrorPayload
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

func (e *extensionOperationAPIError) Error() string {
	if e == nil {
		return nilToolErrorString
	}
	if message := strings.TrimSpace(e.payload.Error); message != "" {
		return message
	}
	return strings.TrimSpace(e.status)
}

func (e *extensionOperationAPIError) cliExitCode() int {
	if e == nil {
		return 1
	}
	return (&daemonAPIError{statusCode: e.statusCode}).cliExitCode()
}

func (e *extensionOperationAPIError) extensionOperationErrorPayload() contract.ExtensionOperationErrorPayload {
	if e == nil {
		return contract.ExtensionOperationErrorPayload{}
	}
	return e.payload
}

func (e *extensionOperationAPIError) DiagnosticItem() contract.DiagnosticItem {
	if e == nil || e.payload.Diagnostic == nil {
		var item contract.DiagnosticItem
		return item
	}
	return diagnosticspkg.RedactItem(*e.payload.Diagnostic)
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
	if len(body) > 0 {
		for _, parse := range []func(int, string, []byte) (bool, error){
			parseExtensionOperationAPIError,
			parseWindowManagerAPIError,
			parseDaemonAPIError,
			parseMemoryAPIError,
			parseToolAPIError,
		} {
			if ok, err := parse(statusCode, status, body); ok {
				return err
			}
		}
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

func parseExtensionOperationAPIError(statusCode int, status string, body []byte) (bool, error) {
	var extensionPayload contract.ExtensionOperationErrorPayload
	if json.Unmarshal(body, &extensionPayload) != nil ||
		!strings.HasPrefix(strings.TrimSpace(extensionPayload.Code), "extension_") {
		return false, nil
	}
	extensionPayload.Error = redactToolDiagnostic(extensionPayload.Error)
	if extensionPayload.Diagnostic != nil {
		redacted := diagnosticspkg.RedactItem(*extensionPayload.Diagnostic)
		extensionPayload.Diagnostic = &redacted
	}
	return true, &extensionOperationAPIError{
		statusCode: statusCode,
		status:     status,
		payload:    extensionPayload,
	}
}

func parseWindowManagerAPIError(statusCode int, status string, body []byte) (bool, error) {
	var windowManagerPayload contract.WindowManagerErrorPayload
	if json.Unmarshal(body, &windowManagerPayload) != nil || windowManagerPayload.Code == "" {
		return false, nil
	}
	windowManagerPayload.Error = redactToolDiagnostic(windowManagerPayload.Error)
	return true, &windowManagerAPIError{
		statusCode: statusCode,
		status:     status,
		payload:    windowManagerPayload,
	}
}

func parseDaemonAPIError(statusCode int, status string, body []byte) (bool, error) {
	var payload contract.ErrorPayload
	if json.Unmarshal(body, &payload) != nil || strings.TrimSpace(payload.Error) == "" {
		return false, nil
	}
	payload.Error = redactToolDiagnostic(payload.Error)
	cause := errors.New(payload.Error)
	if payload.Diagnostic != nil {
		return true, diagnosticspkg.NewStructuredError(*payload.Diagnostic, cause)
	}
	return true, &daemonAPIError{statusCode: statusCode, status: status, payload: payload}
}

func parseMemoryAPIError(_ int, _ string, body []byte) (bool, error) {
	var memoryPayload contract.MemoryErrorPayload
	if json.Unmarshal(body, &memoryPayload) != nil || strings.TrimSpace(memoryPayload.Code) == "" {
		return false, nil
	}
	message := strings.TrimSpace(memoryPayload.Message)
	if message == "" {
		message = strings.TrimSpace(memoryPayload.Code)
	}
	return true, fmt.Errorf(
		"%s: %s",
		strings.TrimSpace(memoryPayload.Code),
		redactToolDiagnostic(message),
	)
}

func parseToolAPIError(statusCode int, status string, body []byte) (bool, error) {
	var toolPayload contract.ToolErrorResponse
	if json.Unmarshal(body, &toolPayload) != nil || toolPayload.Error.Code == "" {
		return false, nil
	}
	return true, newToolAPIError(statusCode, status, toolPayload)
}

func drainResponseBody(method string, path string, body io.Reader) error {
	if _, err := io.Copy(io.Discard, body); err != nil {
		return fmt.Errorf("cli: drain %s %s response: %w", method, path, err)
	}
	return nil
}
