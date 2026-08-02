package mcp

import (
	"errors"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/tools"
)

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
