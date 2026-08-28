package mcp

import (
	"errors"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	terminalpkg "github.com/compozy/compozy/internal/terminal"
	"github.com/compozy/compozy/internal/tools"
)

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
	return nil
}

func hostedToolErrorMessage(err error) string {
	if err == nil {
		return ""
	}
	if toolErr, ok := errors.AsType[*tools.ToolError](err); ok {
		if len(toolErr.ReasonCodes) > 0 {
			return string(toolErr.ReasonCodes[0]) + ": " + toolErr.Error()
		}
		if toolErr.Code != "" {
			return string(toolErr.Code) + ": " + toolErr.Error()
		}
	}
	type responseProvider interface {
		error
		Response() contract.ToolErrorResponse
	}
	if provider, ok := errors.AsType[responseProvider](err); ok && provider != nil {
		payload := provider.Response().Error
		message := strings.TrimSpace(payload.Message)
		if len(payload.ReasonCodes) > 0 {
			return string(payload.ReasonCodes[0]) + ": " + message
		}
		if payload.Code != "" {
			return string(payload.Code) + ": " + message
		}
	}
	return err.Error()
}
