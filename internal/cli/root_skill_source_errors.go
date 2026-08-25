package cli

import (
	"errors"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	compozyconfig "github.com/compozy/compozy/internal/config"
)

const (
	skillSourceDuplicateCode      = "duplicate_skill_source"
	skillSourceInvalidPathCode    = "invalid_source_path"
	skillSourceUnknownCode        = "unknown_skill_source"
	skillSourceWorkspaceFieldCode = "workspace_scope_field_forbidden"
)

func renderSkillSourceExecutionError(err error) (string, bool) {
	payload, ok := skillSourceErrorPayload(err)
	if !ok {
		return "", false
	}
	message := strings.TrimSpace(payload.Error.Message)
	if payload.Error.Code == skillSourceUnknownCode && len(payload.Error.Valid) > 0 {
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
	return marshalStructuredPayload(args, payload)
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
