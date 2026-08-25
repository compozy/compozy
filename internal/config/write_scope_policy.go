package config

import (
	"fmt"
	"strings"
)

const toolSurfaceShellKey = "shell"

const (
	skillsConfigSection           = "skills"
	skillsSourcesField            = "sources"
	skillsCustomSourcesField      = "custom_sources"
	workspaceFieldForbiddenCode   = "workspace_scope_field_forbidden"
	workspaceFieldForbiddenReason = "only sources and custom_sources may be written at this scope"
)

// ValidateConfigWriteScope rejects paths that the selected runtime scope cannot consume.
func ValidateConfigWriteScope(scope WriteScope, path []string) error {
	if err := scope.Validate(); err != nil {
		return err
	}
	clean, err := normalizeMutationPath(path)
	if err != nil {
		return err
	}
	if scope == WriteScopeWorkspace &&
		(clean[0] == toolSurfaceMarketplaceKey || clean[0] == GatewayDirName || clean[0] == toolSurfaceShellKey) {
		return fmt.Errorf(
			"config: path %q is global-only and cannot be written at workspace scope",
			strings.Join(clean, "."),
		)
	}
	if scope == WriteScopeWorkspace && clean[0] == skillsConfigSection {
		isSourceField := len(clean) == 2 &&
			(clean[1] == skillsSourcesField || clean[1] == skillsCustomSourcesField)
		if !isSourceField {
			field := skillsConfigSection
			if len(clean) > 1 {
				field = clean[1]
			}
			return &SkillSourceValidationError{
				Code: workspaceFieldForbiddenCode, Field: field,
				Message: workspaceFieldForbiddenReason,
			}
		}
	}
	if scope == WriteScopeProfile {
		return validateProfileMutationPath(clean)
	}
	return nil
}
