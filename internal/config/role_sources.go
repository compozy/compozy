package config

import (
	"maps"
	"strings"
)

const (
	RoleFieldSourceDefault   = "default"
	RoleFieldSourceGlobal    = "global"
	RoleFieldSourceWorkspace = "workspace"
	RoleFieldEnabled         = "enabled"
	RoleFieldAgent           = "agent"
	RoleFieldProvider        = "provider"
	RoleFieldModel           = "model"
	RoleFieldReasoning       = "reasoning_effort"
	RoleFieldFallbacks       = "fallback_chain"
)

// RoleFieldSources records which config layer last wrote each routed role field.
type RoleFieldSources map[RoleName]map[string]string

// RoleFieldSource reports the winning config layer for one routed role field.
func (c *Config) RoleFieldSource(role RoleName, field string) string {
	if c == nil || c.RoleSources == nil {
		return ""
	}
	return strings.TrimSpace(c.RoleSources[role][strings.TrimSpace(field)])
}

// CloneRoleFieldSources returns an ownership-safe copy of role source metadata.
func CloneRoleFieldSources(sources RoleFieldSources) RoleFieldSources {
	if sources == nil {
		return nil
	}
	cloned := make(RoleFieldSources, len(sources))
	for role, fields := range sources {
		clonedFields := make(map[string]string, len(fields))
		maps.Copy(clonedFields, fields)
		cloned[role] = clonedFields
	}
	return cloned
}

func defaultRoleFieldSources() RoleFieldSources {
	sources := make(RoleFieldSources, len(allRoleNames))
	for _, role := range allRoleNames {
		fields := []string{
			RoleFieldEnabled,
			RoleFieldProvider,
			RoleFieldModel,
			RoleFieldReasoning,
			RoleFieldFallbacks,
		}
		if role != RoleMemoryController {
			fields = append(fields, RoleFieldAgent)
		}
		switch role {
		case RoleCoordinator:
			fields = append(fields, "ttl", "max_children", "max_active_sessions_per_workspace")
		case RoleMemoryController:
			fields = append(fields, "timeout", "top_k", "prompt_version", "max_tokens_out")
		}
		sources[role] = make(map[string]string, len(fields))
		for _, field := range fields {
			sources[role][field] = RoleFieldSourceDefault
		}
	}
	return sources
}

func (o rolesOverlay) recordSources(dst *Config, source string) {
	if dst == nil {
		return
	}
	if dst.RoleSources == nil {
		dst.RoleSources = defaultRoleFieldSources()
	}
	recordRoleOverlaySources(dst.RoleSources, RoleCoordinator, o.Coordinator.roleOverlay, source)
	recordRoleOverlaySources(dst.RoleSources, RoleDream, o.Dream, source)
	recordRoleOverlaySources(dst.RoleSources, RoleCheckpointSummary, o.CheckpointSummary, source)
	recordRoleOverlaySources(dst.RoleSources, RoleMemoryExtractor, o.MemoryExtractor, source)
	recordRoleOverlaySources(dst.RoleSources, RoleAutoTitle, o.AutoTitle, source)
	recordMemoryControllerSources(dst.RoleSources, o.MemoryController, source)
	recordSource(dst.RoleSources, RoleCoordinator, "ttl", source, o.Coordinator.TTL != nil)
	recordSource(dst.RoleSources, RoleCoordinator, "max_children", source, o.Coordinator.MaxChildren != nil)
	recordSource(
		dst.RoleSources,
		RoleCoordinator,
		"max_active_sessions_per_workspace",
		source,
		o.Coordinator.MaxActiveSessionsPerWorkspace != nil,
	)
}

func recordRoleOverlaySources(sources RoleFieldSources, role RoleName, overlay roleOverlay, source string) {
	recordSource(sources, role, RoleFieldEnabled, source, overlay.Enabled != nil)
	recordSource(sources, role, RoleFieldAgent, source, overlay.Agent != nil)
	recordSource(sources, role, RoleFieldProvider, source, overlay.Provider != nil)
	recordSource(sources, role, RoleFieldModel, source, overlay.Model != nil)
	recordSource(sources, role, RoleFieldReasoning, source, overlay.ReasoningEffort != nil)
	recordSource(sources, role, RoleFieldFallbacks, source, overlay.FallbackChain != nil)
}

func recordMemoryControllerSources(
	sources RoleFieldSources,
	overlay memoryControllerRoleOverlay,
	source string,
) {
	recordSource(sources, RoleMemoryController, RoleFieldEnabled, source, overlay.Enabled != nil)
	recordSource(sources, RoleMemoryController, RoleFieldProvider, source, overlay.Provider != nil)
	recordSource(sources, RoleMemoryController, RoleFieldModel, source, overlay.Model != nil)
	recordSource(sources, RoleMemoryController, RoleFieldReasoning, source, overlay.ReasoningEffort != nil)
	recordSource(sources, RoleMemoryController, "timeout", source, overlay.Timeout != nil)
	recordSource(sources, RoleMemoryController, "top_k", source, overlay.TopK != nil)
	recordSource(sources, RoleMemoryController, "prompt_version", source, overlay.PromptVersion != nil)
	recordSource(sources, RoleMemoryController, "max_tokens_out", source, overlay.MaxTokensOut != nil)
	recordSource(sources, RoleMemoryController, RoleFieldFallbacks, source, overlay.FallbackChain != nil)
}

func recordSource(
	sources RoleFieldSources,
	role RoleName,
	field string,
	source string,
	present bool,
) {
	if !present {
		return
	}
	if sources[role] == nil {
		sources[role] = make(map[string]string)
	}
	sources[role][field] = source
}
