package daemon

import (
	"github.com/compozy/compozy/internal/api/contract"
)

type compactWorkspaceAgent struct {
	DisplayName     string                            `json:"display_name,omitempty"`
	Description     string                            `json:"description,omitempty"`
	Kind            string                            `json:"kind,omitempty"`
	Name            string                            `json:"name"`
	Provider        string                            `json:"provider"`
	Model           string                            `json:"model,omitempty"`
	ReasoningEffort contract.ReasoningEffort          `json:"reasoning_effort,omitempty"`
	Speed           contract.Speed                    `json:"speed,omitempty"`
	Origin          contract.AgentOrigin              `json:"origin"`
	PackageOwned    bool                              `json:"package_owned,omitempty"`
	WorkspaceID     string                            `json:"workspace_id,omitempty"`
	Layer           string                            `json:"layer,omitempty"`
	Diagnostics     []contract.AgentDiagnosticPayload `json:"diagnostics,omitempty"`
}

func projectCompactWorkspaceAgent(p *contract.AgentPayload) compactWorkspaceAgent {
	return compactWorkspaceAgent{
		DisplayName:     p.DisplayName,
		Description:     p.Description,
		Kind:            p.Kind,
		Name:            p.Name,
		Provider:        p.Provider,
		Model:           p.Model,
		ReasoningEffort: p.ReasoningEffort,
		Speed:           p.Speed,
		Origin:          p.Origin,
		PackageOwned:    p.PackageOwned,
		WorkspaceID:     p.WorkspaceID,
		Layer:           p.Layer,
		Diagnostics:     p.Diagnostics,
	}
}

type compactSkill struct {
	Name   string `json:"name"`
	Source string `json:"source"`
}

func projectCompactSkill(p contract.WorkspaceSkillPayload) compactSkill {
	return compactSkill{
		Name:   p.Name,
		Source: p.Source,
	}
}

type compactProvider struct {
	Name        string `json:"name"`
	DisplayName string `json:"display_name,omitempty"`
}

func projectCompactProvider(p contract.SessionProviderOptionPayload) compactProvider {
	return compactProvider{
		Name:        p.Name,
		DisplayName: p.DisplayName,
	}
}

type workspaceDescribeResult struct {
	Workspace contract.WorkspacePayload               `json:"workspace"`
	Sessions  []contract.SessionPayload               `json:"sessions"`
	Agents    []contract.AgentPayload                 `json:"agents"`
	Skills    []contract.WorkspaceSkillPayload        `json:"skills"`
	Providers []contract.SessionProviderOptionPayload `json:"providers"`
}

func compactWorkspaceDescribe(p workspaceDescribeResult) any {
	var sessions []compactSessionRecord
	if p.Sessions != nil {
		sessions = make([]compactSessionRecord, len(p.Sessions))
		for i := range p.Sessions {
			sessions[i] = compactSession(&p.Sessions[i])
		}
	}
	var agents []compactWorkspaceAgent
	if p.Agents != nil {
		agents = make([]compactWorkspaceAgent, len(p.Agents))
		for i := range p.Agents {
			agents[i] = projectCompactWorkspaceAgent(&p.Agents[i])
		}
	}
	var skills []compactSkill
	if p.Skills != nil {
		skills = make([]compactSkill, len(p.Skills))
		for i := range p.Skills {
			skills[i] = projectCompactSkill(p.Skills[i])
		}
	}
	var providers []compactProvider
	if p.Providers != nil {
		providers = make([]compactProvider, len(p.Providers))
		for i := range p.Providers {
			providers[i] = projectCompactProvider(p.Providers[i])
		}
	}
	return struct {
		Workspace contract.WorkspacePayload `json:"workspace"`
		Sessions  []compactSessionRecord    `json:"sessions"`
		Agents    []compactWorkspaceAgent   `json:"agents"`
		Skills    []compactSkill            `json:"skills"`
		Providers []compactProvider         `json:"providers"`
	}{p.Workspace, sessions, agents, skills, providers}
}
