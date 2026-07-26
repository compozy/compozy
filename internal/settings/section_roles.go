package settings

import (
	"slices"

	aghconfig "github.com/compozy/agh/internal/config"
)

func diffRolesSettings(current *aghconfig.RolesConfig, desired *aghconfig.RolesConfig) []string {
	var changed []string
	changed = append(changed, diffCoordinatorRoleSettings(current.Coordinator, desired.Coordinator)...)
	changed = append(changed, diffRoleSettings(aghconfig.RoleDream, current.Dream, desired.Dream)...)
	changed = append(changed, diffRoleSettings(
		aghconfig.RoleCheckpointSummary,
		current.CheckpointSummary,
		desired.CheckpointSummary,
	)...)
	changed = append(changed, diffRoleSettings(
		aghconfig.RoleMemoryExtractor,
		current.MemoryExtractor,
		desired.MemoryExtractor,
	)...)
	changed = append(changed, diffRoleSettings(aghconfig.RoleAutoTitle, current.AutoTitle, desired.AutoTitle)...)
	return append(changed, diffMemoryControllerRoleSettings(current.MemoryController, desired.MemoryController)...)
}

func diffCoordinatorRoleSettings(
	current aghconfig.CoordinatorRoleConfig,
	desired aghconfig.CoordinatorRoleConfig,
) []string {
	changed := diffRoleSettings(aghconfig.RoleCoordinator, current.RoleConfig, desired.RoleConfig)
	prefix := "roles." + string(aghconfig.RoleCoordinator) + "."
	if current.TTL != desired.TTL {
		changed = append(changed, prefix+"ttl")
	}
	if current.MaxChildren != desired.MaxChildren {
		changed = append(changed, prefix+"max_children")
	}
	if current.MaxActiveSessionsPerWorkspace != desired.MaxActiveSessionsPerWorkspace {
		changed = append(changed, prefix+"max_active_sessions_per_workspace")
	}
	return changed
}

func diffRoleSettings(
	role aghconfig.RoleName,
	current aghconfig.RoleConfig,
	desired aghconfig.RoleConfig,
) []string {
	prefix := "roles." + string(role) + "."
	changed := make([]string, 0, 6)
	if current.Enabled != desired.Enabled {
		changed = append(changed, prefix+"enabled")
	}
	if current.Agent != desired.Agent {
		changed = append(changed, prefix+"agent")
	}
	if current.Provider != desired.Provider {
		changed = append(changed, prefix+"provider")
	}
	if current.Model != desired.Model {
		changed = append(changed, prefix+"model")
	}
	if current.ReasoningEffort != desired.ReasoningEffort {
		changed = append(changed, prefix+"reasoning_effort")
	}
	if !slices.Equal(current.FallbackChain, desired.FallbackChain) {
		changed = append(changed, prefix+"fallback_chain")
	}
	return changed
}

func diffMemoryControllerRoleSettings(
	current aghconfig.MemoryControllerRoleConfig,
	desired aghconfig.MemoryControllerRoleConfig,
) []string {
	prefix := "roles." + string(aghconfig.RoleMemoryController) + "."
	changed := diffSharedRoleRouteFields(
		prefix,
		current.Enabled,
		desired.Enabled,
		current.Provider,
		desired.Provider,
		current.Model,
		desired.Model,
		current.ReasoningEffort,
		desired.ReasoningEffort,
		current.FallbackChain,
		desired.FallbackChain,
	)
	if current.Timeout != desired.Timeout {
		changed = append(changed, prefix+sectionsTimeoutKey)
	}
	if current.TopK != desired.TopK {
		changed = append(changed, prefix+"top_k")
	}
	if current.PromptVersion != desired.PromptVersion {
		changed = append(changed, prefix+"prompt_version")
	}
	if current.MaxTokensOut != desired.MaxTokensOut {
		changed = append(changed, prefix+"max_tokens_out")
	}
	return changed
}

func diffSharedRoleRouteFields(
	prefix string,
	currentEnabled bool,
	desiredEnabled bool,
	currentProvider string,
	desiredProvider string,
	currentModel string,
	desiredModel string,
	currentEffort string,
	desiredEffort string,
	currentFallbacks []aghconfig.RoleFallback,
	desiredFallbacks []aghconfig.RoleFallback,
) []string {
	changed := make([]string, 0, 5)
	if currentEnabled != desiredEnabled {
		changed = append(changed, prefix+sectionsEnabledKey)
	}
	if currentProvider != desiredProvider {
		changed = append(changed, prefix+sectionsProviderKey)
	}
	if currentModel != desiredModel {
		changed = append(changed, prefix+sectionsModelKey)
	}
	if currentEffort != desiredEffort {
		changed = append(changed, prefix+sectionsReasoningEffortKey)
	}
	if !slices.Equal(currentFallbacks, desiredFallbacks) {
		changed = append(changed, prefix+sectionsFallbackChainKey)
	}
	return changed
}

func applyRolesSettings(editor *aghconfig.OverlayEditor, roles *aghconfig.RolesConfig) error {
	tables := []struct {
		role   aghconfig.RoleName
		values map[string]any
	}{
		{role: aghconfig.RoleCoordinator, values: coordinatorRoleTable(roles.Coordinator)},
		{role: aghconfig.RoleDream, values: roleTable(roles.Dream)},
		{role: aghconfig.RoleCheckpointSummary, values: roleTable(roles.CheckpointSummary)},
		{role: aghconfig.RoleMemoryExtractor, values: roleTable(roles.MemoryExtractor)},
		{role: aghconfig.RoleAutoTitle, values: roleTable(roles.AutoTitle)},
		{role: aghconfig.RoleMemoryController, values: memoryControllerRoleTable(roles.MemoryController)},
	}
	for _, table := range tables {
		if err := editor.SetTable([]string{"roles", string(table.role)}, table.values); err != nil {
			return err
		}
	}
	return nil
}

func roleTable(role aghconfig.RoleConfig) map[string]any {
	return map[string]any{
		sectionsEnabledKey:         role.Enabled,
		"agent":                    role.Agent,
		sectionsProviderKey:        role.Provider,
		sectionsModelKey:           role.Model,
		sectionsReasoningEffortKey: role.ReasoningEffort,
		sectionsFallbackChainKey:   roleFallbackTables(role.FallbackChain),
	}
}

func coordinatorRoleTable(role aghconfig.CoordinatorRoleConfig) map[string]any {
	table := roleTable(role.RoleConfig)
	table["ttl"] = role.TTL.String()
	table["max_children"] = role.MaxChildren
	table["max_active_sessions_per_workspace"] = role.MaxActiveSessionsPerWorkspace
	return table
}

func memoryControllerRoleTable(role aghconfig.MemoryControllerRoleConfig) map[string]any {
	return map[string]any{
		sectionsEnabledKey:         role.Enabled,
		sectionsProviderKey:        role.Provider,
		sectionsModelKey:           role.Model,
		sectionsReasoningEffortKey: role.ReasoningEffort,
		"timeout":                  role.Timeout.String(),
		"top_k":                    role.TopK,
		"prompt_version":           role.PromptVersion,
		"max_tokens_out":           role.MaxTokensOut,
		sectionsFallbackChainKey:   roleFallbackTables(role.FallbackChain),
	}
}

func roleFallbackTables(fallbacks []aghconfig.RoleFallback) []map[string]any {
	tables := make([]map[string]any, 0, len(fallbacks))
	for _, fallback := range fallbacks {
		tables = append(tables, map[string]any{
			sectionsProviderKey:        fallback.Provider,
			sectionsModelKey:           fallback.Model,
			sectionsReasoningEffortKey: fallback.ReasoningEffort,
		})
	}
	return tables
}
