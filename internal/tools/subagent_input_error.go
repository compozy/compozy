package tools

import (
	"encoding/json"
	jsonv2 "encoding/json/v2"
	"errors"
	"strings"
)

func subagentInputError(id ToolID, input json.RawMessage, err error) error {
	if err == nil {
		return nil
	}
	switch id {
	case ToolIDSubagentCapabilities, ToolIDSubagentDelegate, ToolIDSubagentStatus, ToolIDSubagentCancel:
	default:
		return err
	}
	original, ok := errors.AsType[*ToolError](err)
	if !ok || original.Code != ErrorCodeInvalidInput {
		return err
	}
	cloned := *original
	cloned.Code = ErrorCode("invalid_request")
	if original.Err != nil {
		cloned.Message = strings.TrimPrefix(original.Err.Error(), ErrToolInvalidInput.Error()+": ")
	}
	var fields map[string]json.RawMessage
	if parseErr := jsonv2.Unmarshal(input, &fields); parseErr == nil {
		key := "subagent_id"
		if id == ToolIDSubagentDelegate {
			key = "task"
		}
		if id != ToolIDSubagentCapabilities {
			var value string
			raw, exists := fields[key]
			if !exists {
				cloned.Message = key + " is required."
			} else if decodeErr := jsonv2.Unmarshal(raw, &value); decodeErr == nil && strings.TrimSpace(value) == "" {
				cloned.Message = key + " is required."
			}
		}
	}
	return &cloned
}
