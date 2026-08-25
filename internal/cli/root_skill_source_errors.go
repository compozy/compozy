package cli

import (
	"encoding/json"
	"errors"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	compozyconfig "github.com/compozy/compozy/internal/config"
)

func renderSkillSourceExecutionError(err error) (string, bool) {
	payload, ok := skillSourceErrorPayload(err)
	if !ok {
		return "", false
	}
	message := strings.TrimSpace(payload.Error.Message)
	if payload.Error.Code == "unknown_skill_source" && len(payload.Error.Valid) > 0 {
		message = strings.TrimSuffix(message, "; valid: "+strings.Join(payload.Error.Valid, ", "))
		message = strings.TrimSuffix(message, " · valid: "+strings.Join(payload.Error.Valid, ", "))
		message += " · valid: " + strings.Join(payload.Error.Valid, ", ")
	}
	return "Error: " + message, true
}

func marshalSkillSourceExecutionError(args []string, err error) ([]byte, bool) {
	payload, ok := skillSourceErrorPayload(err)
	if !ok {
		return nil, false
	}
	switch requestedOutputFormat(args) {
	case OutputJSON:
		encoded, marshalErr := json.Marshal(payload)
		return encoded, marshalErr == nil
	case OutputJSONL:
		encoded, marshalErr := json.Marshal(payload)
		if marshalErr != nil {
			return nil, false
		}
		return append(encoded, '\n'), true
	default:
		return nil, false
	}
}

func skillSourceErrorPayload(err error) (contract.SkillSourceValidationErrorResponse, bool) {
	if apiErr, ok := errors.AsType[interface {
		error
		skillSourceErrorPayload() contract.SkillSourceValidationErrorResponse
	}](err); ok {
		return apiErr.skillSourceErrorPayload(), true
	}
	if sourceErr, ok := errors.AsType[*compozyconfig.SkillSourceValidationError](err); ok {
		return contract.SkillSourceValidationErrorResponse{
			Error: contract.SkillSourceValidationErrorPayload{
				Code:           sourceErr.Code,
				Message:        sourceErr.Message,
				Field:          sourceErr.Field,
				Path:           sourceErr.Path,
				ExistingSource: sourceErr.ExistingSource,
				Valid:          append([]string(nil), sourceErr.Valid...),
				Suggestion:     sourceErr.Suggestion,
			},
		}, true
	}
	return contract.SkillSourceValidationErrorResponse{}, false
}
