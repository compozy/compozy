package config

import "strings"

// CloneAgentDef returns a normalized deep copy of an agent definition.
func CloneAgentDef(agent AgentDef) AgentDef {
	cloned := AgentDef{
		SessionEnv: MergeSessionEnv(nil, agent.SessionEnv),
		Name:       strings.TrimSpace(agent.Name),
		DisplayName: strings.TrimSpace(
			agent.DisplayName,
		),
		Description:     agent.Description,
		Kind:            agent.Kind,
		Appearance:      CloneAgentAppearance(agent.Appearance),
		Provider:        strings.TrimSpace(agent.Provider),
		Command:         strings.TrimSpace(agent.Command),
		Model:           strings.TrimSpace(agent.Model),
		ReasoningEffort: strings.TrimSpace(agent.ReasoningEffort),
		Tools:           cloneStrings(agent.Tools),
		Toolsets:        cloneStrings(agent.Toolsets),
		DenyTools:       cloneStrings(agent.DenyTools),
		Permissions:     strings.TrimSpace(agent.Permissions),
		WorkspaceAccess: agent.WorkspaceAccess,
		Skills:          normalizeAgentSkillsConfig(agent.Skills),
		CategoryPath:    normalizeAgentCategoryPath(agent.CategoryPath),
		MCPServers:      cloneMCPServers(agent.MCPServers),
		Hooks:           cloneHookDecls(agent.Hooks),
		FallbackChain:   normalizeRoleFallbacks(agent.FallbackChain),
		Capabilities:    agent.Capabilities.Clone(),
		Prompt:          strings.TrimSpace(agent.Prompt),
		SourcePath:      strings.TrimSpace(agent.SourcePath),
		SourceLayer:     strings.TrimSpace(agent.SourceLayer),
		ShadowedDefinitions: append(
			[]AgentDefinitionRef(nil),
			agent.ShadowedDefinitions...,
		),
	}
	cloned.SetSpeed(agent.SpeedValue())
	cloned.SetACPOptions(agent.ACPOptionsValue())
	return cloned
}
