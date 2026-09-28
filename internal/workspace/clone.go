package workspace

import (
	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/filesnap"
)

func cloneSnapshots(snapshots map[string]filesnap.Snapshot) map[string]filesnap.Snapshot {
	return filesnap.Clone(snapshots)
}

func cloneResolvedWorkspace(src *ResolvedWorkspace) ResolvedWorkspace {
	return ResolvedWorkspace{
		Workspace:   cloneWorkspace(src.Workspace),
		WorkspaceID: src.WorkspaceID,
		ProfileID:   src.ProfileID,
		ProfileName: src.ProfileName,
		ProfileRoot: src.ProfileRoot,
		ProfileDeclarations: append(
			[]ProfileDeclaration(nil),
			src.ProfileDeclarations...,
		),
		Config: compozyconfig.CloneConfig(&src.Config),
		Agents: cloneAgentDefs(src.Agents),
		AgentDiagnostics: append(
			[]AgentDiagnostic(nil),
			src.AgentDiagnostics...,
		),
		Skills:     cloneSkillPaths(src.Skills),
		ResolvedAt: src.ResolvedAt,
	}
}

func cloneWorkspace(src Workspace) Workspace {
	return Workspace{
		ID:             src.ID,
		RootDir:        src.RootDir,
		AdditionalDirs: append([]string(nil), src.AdditionalDirs...),
		Name:           src.Name,
		DefaultAgent:   src.DefaultAgent,
		CreatedAt:      src.CreatedAt,
		UpdatedAt:      src.UpdatedAt,
	}
}

func cloneWorkspaces(src []Workspace) []Workspace {
	if len(src) == 0 {
		return nil
	}

	cloned := make([]Workspace, 0, len(src))
	for _, ws := range src {
		cloned = append(cloned, cloneWorkspace(ws))
	}
	return cloned
}

func cloneConfig(src *compozyconfig.Config) compozyconfig.Config {
	return compozyconfig.CloneConfig(src)
}

func cloneAgentDefs(src []compozyconfig.AgentDef) []compozyconfig.AgentDef {
	if len(src) == 0 {
		return nil
	}

	cloned := make([]compozyconfig.AgentDef, 0, len(src))
	for _, agent := range src {
		cloned = append(cloned, compozyconfig.CloneAgentDef(agent))
	}

	return cloned
}

func cloneSkillPaths(src []SkillPath) []SkillPath {
	if len(src) == 0 {
		return nil
	}

	return append([]SkillPath(nil), src...)
}
