package config

import (
	"context"
	"errors"
	"strings"

	"github.com/compozy/agh/internal/resources"
)

const (
	// AgentResourceKind is the canonical desired-state resource kind for agent definitions.
	AgentResourceKind     resources.ResourceKind = "agent"
	agentResourceMaxBytes                        = 512 << 10
)

// NewAgentResourceCodec builds the canonical agent resource codec.
func NewAgentResourceCodec() (resources.KindCodec[AgentDef], error) {
	return resources.NewJSONCodec(AgentResourceKind, agentResourceMaxBytes, validateAgentResourceSpec)
}

func validateAgentResourceSpec(
	_ context.Context,
	scope resources.ResourceScope,
	spec AgentDef,
) (AgentDef, error) {
	normalizedScope := scope.Normalize()
	if err := normalizedScope.Validate("scope"); err != nil {
		return AgentDef{}, err
	}

	normalized := AgentDef{
		Name:            strings.TrimSpace(spec.Name),
		Provider:        strings.TrimSpace(spec.Provider),
		Command:         strings.TrimSpace(spec.Command),
		Model:           strings.TrimSpace(spec.Model),
		ReasoningEffort: strings.TrimSpace(spec.ReasoningEffort),
		Tools:           normalizeAgentToolPatterns(spec.Tools),
		Toolsets:        normalizeAgentToolsetRefs(spec.Toolsets),
		DenyTools:       normalizeAgentToolPatterns(spec.DenyTools),
		Permissions:     strings.TrimSpace(spec.Permissions),
		Skills:          normalizeAgentSkillsConfig(spec.Skills),
		CategoryPath:    normalizeAgentCategoryPath(spec.CategoryPath),
		MCPServers:      cloneMCPServers(spec.MCPServers),
		Hooks:           cloneHookDecls(spec.Hooks),
		Prompt:          strings.TrimSpace(spec.Prompt),
	}
	if spec.Capabilities != nil {
		capabilities, err := normalizeCapabilityCatalog(spec.Capabilities, "agent.capabilities")
		if err != nil {
			return AgentDef{}, errors.Join(resources.ErrValidation, err)
		}
		normalized.Capabilities = capabilities
	}
	for idx, server := range normalized.MCPServers {
		normalized.MCPServers[idx] = normalizeMCPServerResourceSpec(server)
	}

	if err := normalized.Validate(); err != nil {
		return AgentDef{}, errors.Join(resources.ErrValidation, err)
	}
	return normalized, nil
}
