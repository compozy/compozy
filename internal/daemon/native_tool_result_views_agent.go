package daemon

import (
	"fmt"

	"github.com/compozy/compozy/internal/api/contract"
)

type compactAgentRecord struct {
	Name             string                            `json:"name"`
	Label            string                            `json:"label"`
	Description      string                            `json:"description,omitempty"`
	Kind             string                            `json:"kind,omitempty"`
	CategoryPath     []string                          `json:"category_path,omitempty"`
	Origin           contract.AgentOrigin              `json:"origin"`
	WorkspaceID      string                            `json:"workspace_id,omitempty"`
	Layer            string                            `json:"layer,omitempty"`
	PackageOwned     bool                              `json:"package_owned,omitzero"`
	Diagnostics      []contract.AgentDiagnosticPayload `json:"diagnostics,omitempty"`
	DefinitionDigest string                            `json:"definition_digest"`
	EffectiveRuntime *compactAgentRuntime              `json:"effective_runtime,omitzero"`
}

func compactAgent(p *contract.AgentPayload) compactAgentRecord {
	result := compactAgentRecord{
		Name:             p.Name,
		Label:            p.Label,
		Description:      p.Description,
		Kind:             p.Kind,
		CategoryPath:     p.CategoryPath,
		Origin:           p.Origin,
		WorkspaceID:      p.WorkspaceID,
		Layer:            p.Layer,
		PackageOwned:     p.PackageOwned,
		Diagnostics:      p.Diagnostics,
		DefinitionDigest: p.DefinitionDigest,
	}
	if p.EffectiveRuntime != nil {
		value := compactAgentEffectiveRuntime(*p.EffectiveRuntime)
		result.EffectiveRuntime = &value
	}
	return result
}

type compactAgentRuntime struct {
	Provider        string                   `json:"provider"`
	Model           string                   `json:"model,omitempty"`
	ReasoningEffort contract.ReasoningEffort `json:"reasoning_effort,omitempty"`
	Speed           contract.Speed           `json:"speed,omitempty"`
	Permissions     string                   `json:"permissions,omitempty"`
}

func compactAgentEffectiveRuntime(p contract.AgentEffectiveRuntimePayload) compactAgentRuntime {
	return compactAgentRuntime{
		Provider:        p.Provider,
		Model:           p.Model,
		ReasoningEffort: p.ReasoningEffort,
		Speed:           p.Speed,
		Permissions:     p.Permissions,
	}
}

var agentRecordViews = resultViews[contract.AgentPayload]{
	Compact: func(p contract.AgentPayload) any { return map[string]any{daemonAgentField: compactAgent(&p)} },
	Full:    func(p contract.AgentPayload) any { return map[string]any{daemonAgentField: p} },
	Preview: func(p contract.AgentPayload) string { return "agent " + p.Name },
}

var agentListViews = resultViews[[]contract.AgentPayload]{
	Compact: func(p []contract.AgentPayload) any {
		var agents []compactAgentRecord
		if p != nil {
			agents = make([]compactAgentRecord, len(p))
			for i := range p {
				agents[i] = compactAgent(&p[i])
			}
		}
		return map[string]any{nativeToolsAgentsKey: agents}
	},
	Full:    func(p []contract.AgentPayload) any { return map[string]any{nativeToolsAgentsKey: p} },
	Preview: func(p []contract.AgentPayload) string { return fmt.Sprintf("%d agents", len(p)) },
}
