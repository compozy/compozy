package tools

import (
	"encoding/json"
	jsonv2 "encoding/json/v2"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"
)

func descriptorInputError(descriptor Descriptor, input json.RawMessage, err error) error {
	if err == nil {
		return nil
	}
	if kindErr := agentCreateKindInputError(descriptor.ID, input); kindErr != nil {
		return kindErr
	}

	if descriptor.ExecutionMetadata().InputErrorCode == "" {
		return err
	}
	original, ok := errors.AsType[*ToolError](err)
	if !ok || original.Code != ErrorCodeInvalidInput {
		return err
	}
	cloned := *original
	cloned.Code = descriptor.ExecutionMetadata().InputErrorCode
	if original.Err != nil {
		cloned.Message = strings.TrimPrefix(original.Err.Error(), ErrToolInvalidInput.Error()+": ")
	}
	var fields map[string]json.RawMessage
	if parseErr := jsonv2.Unmarshal(input, &fields); parseErr == nil {
		var schema struct {
			Required   []string `json:"required"`
			Properties map[string]struct {
				MaxLength int `json:"maxLength"`
			} `json:"properties"`
		}
		if decodeErr := jsonv2.Unmarshal(descriptor.InputSchema, &schema); decodeErr != nil {
			return &cloned
		}
		for _, key := range schema.Required {
			var value string
			raw, exists := fields[key]
			if !exists || (jsonv2.Unmarshal(raw, &value) == nil && strings.TrimSpace(value) == "") {
				cloned.Message = key + " is required."
				return &cloned
			}
		}
		for key, property := range schema.Properties {
			var value string
			if property.MaxLength > 0 && jsonv2.Unmarshal(fields[key], &value) == nil &&
				utf8.RuneCountInString(value) > property.MaxLength {
				cloned.Message = fmt.Sprintf("%s exceeds %d characters.", key, property.MaxLength)
				break
			}
		}
	}
	return &cloned
}

func agentCreateKindInputError(id ToolID, input json.RawMessage) error {
	if id != ToolIDAgentCreate {
		return nil
	}
	var fields struct {
		Kind string `json:"kind"`
	}
	if decodeErr := jsonv2.Unmarshal(input, &fields); decodeErr != nil {
		return nil
	}
	switch fields.Kind {
	case "", "on_demand", "always_on":
		return nil
	}
	return NewToolError(ErrorCode("agent_kind_invalid"), id,
		"agent kind must be on_demand or always_on (stays_on was renamed to always_on in v0.5.1)",
		ErrToolInvalidInput, ReasonSchemaInvalid)
}
