package mcp

import (
	"encoding/json"
	"errors"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	terminalpkg "github.com/compozy/compozy/internal/terminal"
	"github.com/compozy/compozy/internal/tools"
)

type hostedToolResponseProvider interface {
	error
	Response() contract.ToolErrorResponse
}

func hostedToolErrorStructuredContent(err error) any {
	type terminalEnvelopeProvider interface {
		error
		TerminalErrorEnvelope() contract.TerminalErrorResponse
	}
	if provider, ok := errors.AsType[terminalEnvelopeProvider](err); ok && provider != nil {
		return provider.TerminalErrorEnvelope()
	}
	if terminalErr, ok := errors.AsType[*terminalpkg.Error](err); ok && terminalpkg.IsErrorCode(terminalErr.Code) {
		return contract.TerminalErrorResponse{Error: contract.TerminalErrorDetail{
			Code: string(terminalErr.Code), Message: terminalErr.Error(),
			Details: contract.TerminalErrorDetailsFromDomain(terminalErr),
		}}
	}
	if toolErr, ok := errors.AsType[*tools.ToolError](err); ok {
		return contract.ToolErrorResponse{Error: contract.ToolErrorPayload{
			Code: toolErr.Code, Message: toolErr.Error(), ToolID: toolErr.ToolID,
			ReasonCodes:   append([]tools.ReasonCode(nil), toolErr.ReasonCodes...),
			PartialResult: toolErr.PartialResult,
		}}
	}
	if provider, ok := errors.AsType[hostedToolResponseProvider](err); ok && provider != nil {
		return provider.Response()
	}
	return nil
}

func hostedToolErrorMessage(err error) string {
	if err == nil {
		return ""
	}
	if toolErr, ok := errors.AsType[*tools.ToolError](err); ok {
		if toolErr.Code == tools.ErrorCodeInvalidRequest {
			return string(toolErr.Code) + ": " + toolErr.Error()
		}
		if len(toolErr.ReasonCodes) > 0 {
			return string(toolErr.ReasonCodes[0]) + ": " + toolErr.Error()
		}
		if toolErr.Code != "" {
			return string(toolErr.Code) + ": " + toolErr.Error()
		}
	}
	if provider, ok := errors.AsType[hostedToolResponseProvider](err); ok && provider != nil {
		payload := provider.Response().Error
		message := hostedOperatorFailureMessage(strings.TrimSpace(payload.Message), payload.Details)
		if payload.Code == tools.ErrorCodeInvalidRequest {
			return string(payload.Code) + ": " + message
		}
		if len(payload.ReasonCodes) > 0 {
			return string(payload.ReasonCodes[0]) + ": " + message
		}
		if payload.Code != "" {
			return string(payload.Code) + ": " + message
		}
	}
	return err.Error()
}

// hostedOperatorFailureMessage appends the daemon's safe cause and recovery details so a
// masked denial still tells the agent which field and rule refused the call.
func hostedOperatorFailureMessage(message string, details map[string]json.RawMessage) string {
	filtered := contract.FilterToolOperatorFailureDetails(details)
	for _, entry := range []struct{ key, label string }{
		{contract.ToolOperatorCauseDetailKey, "Cause"},
		{contract.ToolOperatorRecoveryDetailKey, "Recovery"},
	} {
		var value string
		if raw, ok := filtered[entry.key]; ok && json.Unmarshal(raw, &value) == nil && value != "" {
			message += "\n" + entry.label + ": " + value
		}
	}
	return message
}
