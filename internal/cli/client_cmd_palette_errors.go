package cli

import (
	"encoding/json"
	"fmt"
	"maps"
	"sort"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
)

type cmdPaletteAPIError struct {
	statusCode int
	status     string
	payload    contract.CmdPaletteError
}

func (e *cmdPaletteAPIError) Error() string {
	if e == nil {
		return nilToolErrorString
	}
	switch e.payload.Error {
	case cmdPaletteInvalidArgs:
		return cmdPaletteInvalidArgumentsMessage(e.payload.Fields)
	case cmdPaletteNoShell:
		return "no attached shell client — " + strings.TrimSpace(e.payload.Message)
	case "multiple_clients":
		clients := make([]string, len(e.payload.Clients))
		for index, client := range e.payload.Clients {
			clients[index] = string(client)
		}
		return fmt.Sprintf("multiple attached clients — pass --client (%s)", strings.Join(clients, ", "))
	case "command_unavailable":
		return "command unavailable — " + strings.TrimSpace(e.payload.Reason)
	default:
		if message := strings.TrimSpace(e.payload.Message); message != "" {
			return message
		}
		return apiErrorMessage(e.payload.Error, e.status)
	}
}

func (e *cmdPaletteAPIError) cliExitCode() int {
	if e != nil && e.statusCode == 422 {
		return 2
	}
	return 1
}

func (e *cmdPaletteAPIError) errorPayload() contract.ErrorPayload {
	if e == nil {
		return contract.ErrorPayload{}
	}
	details := map[string]string{}
	maps.Copy(details, e.payload.Fields)
	if reason := strings.TrimSpace(e.payload.Reason); reason != "" {
		details["reason"] = reason
	}
	if message := strings.TrimSpace(e.payload.Message); message != "" {
		details["message"] = message
	}
	if len(e.payload.Clients) > 0 {
		clients := make([]string, len(e.payload.Clients))
		for index, client := range e.payload.Clients {
			clients[index] = string(client)
		}
		sort.Strings(clients)
		details["clients"] = strings.Join(clients, ",")
	}
	payload := contract.ErrorPayload{Error: e.payload.Error, Code: e.payload.Error}
	if len(details) > 0 {
		payload.Details = details
	}
	return payload
}

func cmdPaletteCommandNotFoundError(commandID string) error {
	return &cmdPaletteAPIError{
		statusCode: 404,
		payload: contract.CmdPaletteError{
			Error: "command_not_found", Message: "command not found: " + commandID,
		},
	}
}

func parseCmdPaletteAPIError(statusCode int, status string, body []byte) (bool, error) {
	var payload contract.CmdPaletteError
	if json.Unmarshal(body, &payload) != nil || !cmdPaletteErrorCode(payload.Error) {
		return false, nil
	}
	payload.Message = redactToolDiagnostic(payload.Message)
	payload.Reason = redactToolDiagnostic(payload.Reason)
	return true, &cmdPaletteAPIError{statusCode: statusCode, status: status, payload: payload}
}

func cmdPaletteErrorCode(code string) bool {
	switch strings.TrimSpace(code) {
	case "command_not_found", cmdPaletteInvalidArgs, "command_unavailable", cmdPaletteNoShell,
		"multiple_clients", "already_running", "cannot_defer_secrets", "client_unauthorized",
		"approval_not_found", "approval_terminal", "runtime_unavailable":
		return true
	default:
		return false
	}
}

func cmdPaletteInvalidArgumentsMessage(fields map[string]string) string {
	if len(fields) == 1 {
		for field, reason := range fields {
			if reason == configRequiredKey {
				return fmt.Sprintf("invalid arguments — missing required %q", field)
			}
		}
	}
	keys := make([]string, 0, len(fields))
	for field := range fields {
		keys = append(keys, field)
	}
	sort.Strings(keys)
	details := make([]string, 0, len(keys))
	for _, field := range keys {
		details = append(details, field+": "+fields[field])
	}
	if len(details) == 0 {
		return "invalid arguments"
	}
	return "invalid arguments — " + strings.Join(details, ", ")
}
