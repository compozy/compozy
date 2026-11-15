package store

import "slices"

// DecodeIntent translates pre-marker rows without consulting another session.
func DecodeIntent(policy SessionPermissionPolicy, kind LineageKind, sessionType string) SessionPermissionPolicy {
	policy = NormalizeSessionPermissionPolicy(policy)
	delegated := kind == LineageKindSpawn || NormalizeSessionType(sessionType) == sessionTypeSpawned ||
		NormalizeSessionType(sessionType) == "coordinator"
	if policy.MCPServersMode == "" {
		policy.MCPServersMode = PermissionInherit
		if delegated {
			policy.MCPServersMode = PermissionList
		}
	}
	if policy.ToolsMode == "" {
		policy.ToolsMode = PermissionInherit
		if delegated {
			policy.ToolsMode = PermissionList
		}
	}
	return policy
}

// DecodeInstanceCeilingIntent uses depth because legacy ceilings cleared materialization flags.
func DecodeInstanceCeilingIntent(policy SessionPermissionPolicy, depth int) SessionPermissionPolicy {
	policy = NormalizeSessionPermissionPolicy(policy)
	mode := PermissionInherit
	if depth >= 1 {
		mode = PermissionList
	}
	if policy.MCPServersMode == "" {
		policy.MCPServersMode = mode
	}
	if policy.ToolsMode == "" {
		policy.ToolsMode = mode
	}
	return policy
}

// BoundMCPServers resolves a policy against the admitted names in its own scope.
func BoundMCPServers(policy SessionPermissionPolicy, admitted []string) []string {
	result := make([]string, 0, len(admitted))
	for _, name := range normalizePolicyAtoms(admitted) {
		if policy.MCPServersMode != PermissionInherit && !slices.Contains(policy.MCPServers, name) {
			continue
		}
		if policy.AncestorMCPCeiling != nil && !slices.Contains(*policy.AncestorMCPCeiling, name) {
			continue
		}
		result = append(result, name)
	}
	return result
}

// IntersectMCPCeiling preserves the distinction between absent and explicitly empty.
func IntersectMCPCeiling(ceiling *[]string, names []string) *[]string {
	if ceiling == nil {
		return new(normalizePolicyAtoms(names))
	}
	result := make([]string, 0)
	for _, name := range *ceiling {
		if slices.Contains(names, name) {
			result = append(result, name)
		}
	}
	return new(normalizePolicyAtoms(result))
}
