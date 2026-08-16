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

const (
	apiErrorBodyLimit      int64 = 1 << 20
	responseBodyDrainLimit int64 = 64 << 10
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

type worktreeRemovalAPIError struct {
	statusCode int
	status     string
	payload    contract.WorktreeRemovalRefusalPayload
}

func (e *worktreeRemovalAPIError) Error() string {
	if e == nil {
		return nilToolErrorString
	}
	return apiErrorMessage(e.payload.Code, e.status)
}

func (e *worktreeRemovalAPIError) cliExitCode() int {
	if e == nil {
		return 1
	}
	return apiStatusExitCode(e.statusCode)
}

func (e *worktreeRemovalAPIError) worktreeRemovalErrorPayload() contract.WorktreeRemovalRefusalPayload {
	if e == nil {
		return contract.WorktreeRemovalRefusalPayload{}
	}
	return e.payload
}

func (e *windowManagerAPIError) Error() string {
	if e == nil {
		return nilToolErrorString
	}
	return apiErrorMessage(e.payload.Error, e.status)
}

func (e *extensionOperationAPIError) Error() string {
	if e == nil {
		return nilToolErrorString
	}
	return apiErrorMessage(e.payload.Error, e.status)
}

func (e *extensionOperationAPIError) cliExitCode() int {
	if e == nil {
		return 1
	}
	if strings.HasPrefix(strings.TrimSpace(e.payload.Code), "extension_agent_plugin_") {
		return 1
	}
	return apiStatusExitCode(e.statusCode)
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
	return apiStatusExitCode(e.statusCode)
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
	return apiErrorMessage(e.payload.Error, e.status)
}

func (e *daemonAPIError) cliExitCode() int {
	if e == nil {
		return 1
	}
	return apiStatusExitCode(e.statusCode)
}

func apiErrorMessage(payloadMessage string, status string) string {
	if message := strings.TrimSpace(payloadMessage); message != "" {
		return message
	}
	return strings.TrimSpace(status)
}

func apiStatusExitCode(statusCode int) int {
	switch statusCode {
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
	body, readErr := io.ReadAll(io.LimitReader(response.Body, apiErrorBodyLimit))
	drainErr := discardResponseBodyBounded(response.Body)
	if drainErr != nil {
		drainErr = fmt.Errorf("cli: drain api error response: %w", drainErr)
	}
	if readErr != nil {
		return errors.Join(fmt.Errorf("cli: read api error response: %w", readErr), drainErr)
	}
	return errors.Join(readAPIErrorBody(response.StatusCode, response.Status, body), drainErr)
}

func readAPIErrorBody(statusCode int, status string, body []byte) error {
	if len(body) > 0 {
		for _, parse := range []func(int, string, []byte) (bool, error){
			parseExtensionOperationAPIError,
			parseWindowManagerAPIError,
			parseWorktreeRemovalAPIError,
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

func parseWorktreeRemovalAPIError(statusCode int, status string, body []byte) (bool, error) {
	var payload contract.WorktreeRemovalRefusalPayload
	if json.Unmarshal(body, &payload) != nil ||
		(payload.Code != "worktree_dirty_requires_force" &&
			payload.Code != "worktree_unpushed_requires_force") {
		return false, nil
	}
	return true, &worktreeRemovalAPIError{statusCode: statusCode, status: status, payload: payload}
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
	if json.Unmarshal(body, &windowManagerPayload) != nil ||
		!strings.HasPrefix(string(windowManagerPayload.Code), "window_manager_") {
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
	if err := discardResponseBodyBounded(body); err != nil {
		return fmt.Errorf("cli: drain %s %s response: %w", method, path, err)
	}
	return nil
}

func decodeJSONResponseBody(method string, path string, body io.Reader, target any) error {
	decoder := json.NewDecoder(body)
	decodeErr := decoder.Decode(target)
	drainErr := drainResponseBody(method, path, io.MultiReader(decoder.Buffered(), body))
	if decodeErr != nil {
		decodeErr = fmt.Errorf("cli: decode %s %s response: %w", method, path, decodeErr)
	}
	return errors.Join(decodeErr, drainErr)
}

func discardResponseBodyBounded(body io.Reader) error {
	if body == nil {
		return nil
	}
	_, err := io.Copy(io.Discard, io.LimitReader(body, responseBodyDrainLimit))
	return err
}
