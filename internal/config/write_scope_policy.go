package config

import (
	"fmt"
	"slices"
	"strings"
)

const toolSurfaceShellKey = "shell"

// globalOnlyConfigRoots are sections that only the home config may set.
var globalOnlyConfigRoots = []string{
	toolSurfaceMarketplaceKey, GatewayDirName, ProviderUsageConfigKey, toolSurfaceShellKey, desktopConfigKey,
	vaultConfigKey,
}

const (
	skillsConfigSection           = "skills"
	skillsSourcesField            = "sources"
	skillsCustomSourcesField      = "custom_sources"
	workspaceFieldForbiddenCode   = "workspace_scope_field_forbidden"
	workspaceFieldForbiddenReason = "only sources and custom_sources may be written at this scope"
)

// bootConsumedGlobalPath reports keys the daemon reads once at boot from the home config; a
// workspace-layered write would validate yet never take effect.
func bootConsumedGlobalPath(clean []string) bool {
	hasPrefix := func(prefix []string) bool {
		return slices.Equal(clean[:min(len(clean), len(prefix))], prefix)
	}
	return slices.Equal(clean, toolSurfaceToolsClarifyTimeoutSegments) ||
		slices.Equal(clean, mcpStatusProbeTimeoutSegments) ||
		hasPrefix(mcpStdioConfigSegments) ||
		hasPrefix(sessionAuthRecoverySegments)
}

// ValidateConfigWriteScope rejects paths that the selected runtime scope cannot consume.
func ValidateConfigWriteScope(scope WriteScope, path []string) error {
	if err := scope.Validate(); err != nil {
		return err
	}
	clean, err := normalizeMutationPath(path)
	if err != nil {
		return err
	}
	if scope == WriteScopeWorkspace && slices.Contains(globalOnlyConfigRoots, clean[0]) {
		return fmt.Errorf(
			"config: path %q is global-only and cannot be written at workspace scope",
			strings.Join(clean, "."),
		)
	}
	if scope == WriteScopeWorkspace && bootConsumedGlobalPath(clean) {
		return fmt.Errorf(
			"config: path %q is global-only and cannot be written at workspace scope; write this key with --scope user",
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
