package settings

import (
	"reflect"
	"strings"

	compozyconfig "github.com/compozy/compozy/internal/config"
)

func diffRolesSettings(current *compozyconfig.RolesConfig, desired *compozyconfig.RolesConfig) []string {
	var changed []string
	changed = append(changed, diffCoordinatorRoleSettings(current.Coordinator, desired.Coordinator)...)
	return append(changed, diffRoleSettings(compozyconfig.RoleAutoTitle, current.AutoTitle, desired.AutoTitle)...)
}

func diffCoordinatorRoleSettings(
	current compozyconfig.CoordinatorRoleConfig,
	desired compozyconfig.CoordinatorRoleConfig,
) []string {
	changed := diffRoleSettings(compozyconfig.RoleCoordinator, current.RoleConfig, desired.RoleConfig)
	prefix := "roles." + string(compozyconfig.RoleCoordinator) + "."
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
	role compozyconfig.RoleName,
	current compozyconfig.RoleConfig,
	desired compozyconfig.RoleConfig,
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
	if current.Speed != desired.Speed {
		changed = append(changed, prefix+"speed")
	}
	if !roleACPOptionsEqual(current.ACPOptions, desired.ACPOptions) {
		changed = append(changed, prefix+"acp_options")
	}
	if !roleFallbacksEqual(current.FallbackChain, desired.FallbackChain) {
		changed = append(changed, prefix+"fallback_chain")
	}
	return changed
}

func applyRolesSettings(editor *compozyconfig.OverlayEditor, roles *compozyconfig.RolesConfig) error {
	tables := []struct {
		role   compozyconfig.RoleName
		values map[string]any
	}{
		{role: compozyconfig.RoleCoordinator, values: coordinatorRoleTable(roles.Coordinator)},
		{role: compozyconfig.RoleAutoTitle, values: roleTable(roles.AutoTitle)},
	}
	for _, table := range tables {
		if err := editor.SetTable([]string{"roles", string(table.role)}, table.values); err != nil {
			return err
		}
	}
	return nil
}

func roleTable(role compozyconfig.RoleConfig) map[string]any {
	return map[string]any{
		sectionsEnabledKey:         role.Enabled,
		string(ScopeAgent):         role.Agent,
		sectionsProviderKey:        role.Provider,
		sectionsModelKey:           role.Model,
		sectionsReasoningEffortKey: role.ReasoningEffort,
		sectionsSpeedKey:           string(role.Speed),
		sectionsACPOptionsKey:      roleACPOptionTables(role.ACPOptions),
		sectionsFallbackChainKey:   roleFallbackTables(role.FallbackChain),
	}
}

func coordinatorRoleTable(role compozyconfig.CoordinatorRoleConfig) map[string]any {
	table := roleTable(role.RoleConfig)
	table["ttl"] = role.TTL.String()
	table["max_children"] = role.MaxChildren
	table["max_active_sessions_per_workspace"] = role.MaxActiveSessionsPerWorkspace
	return table
}

func roleFallbackTables(fallbacks []compozyconfig.RoleFallback) []map[string]any {
	tables := make([]map[string]any, 0, len(fallbacks))
	for _, fallback := range fallbacks {
		table := map[string]any{
			sectionsProviderKey:        fallback.Provider,
			sectionsModelKey:           fallback.Model,
			sectionsReasoningEffortKey: fallback.ReasoningEffort,
			sectionsSpeedKey:           string(fallback.Speed),
			sectionsACPOptionsKey:      roleACPOptionTables(fallback.ACPOptions),
		}
		// An empty command inherits, so it is omitted rather than persisted as "".
		if command := strings.TrimSpace(fallback.Command); command != "" {
			table["command"] = command
		}
		tables = append(tables, table)
	}
	return tables
}

func roleACPOptionTables(options []compozyconfig.ACPOptionSelection) []map[string]any {
	tables := make([]map[string]any, 0, len(options))
	for _, option := range options {
		table := map[string]any{"id": option.ID}
		if option.ValueID != "" {
			table["value_id"] = option.ValueID
		}
		if option.BoolValue != nil {
			table["bool_value"] = *option.BoolValue
		}
		tables = append(tables, table)
	}
	return tables
}

func roleACPOptionsEqual(left, right []compozyconfig.ACPOptionSelection) bool {
	return nilEmptyRoleSliceEqual(left, right)
}

func roleFallbacksEqual(left, right []compozyconfig.RoleFallback) bool {
	return nilEmptyRoleSliceEqual(left, right)
}

func nilEmptyRoleSliceEqual[T any](left, right []T) bool {
	if len(left) == 0 && len(right) == 0 {
		return true
	}
	return reflect.DeepEqual(left, right)
}
