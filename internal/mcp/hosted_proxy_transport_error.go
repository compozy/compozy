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
	var responseProvider interface {
		Response() contract.ToolErrorResponse
	}
	if errors.As(err, &responseProvider) {
		payload := responseProvider.Response().Error
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
