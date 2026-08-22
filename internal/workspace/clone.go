package workspace

import (
	"maps"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/filesnap"
	"github.com/compozy/compozy/internal/sandbox"
)

func cloneSnapshots(snapshots map[string]filesnap.Snapshot) map[string]filesnap.Snapshot {
	return filesnap.Clone(snapshots)
}

func cloneResolvedWorkspace(src *ResolvedWorkspace) ResolvedWorkspace {
	return ResolvedWorkspace{
		Workspace:   cloneWorkspace(src.Workspace),
		WorkspaceID: src.WorkspaceID,
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
		Sandbox:    cloneSandboxResolved(src.Sandbox),
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
		SandboxRef:     src.SandboxRef,
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

func cloneSandboxResolved(src sandbox.Resolved) sandbox.Resolved {
	cloned := src
	cloned.Env = cloneStringMap(src.Env)
	cloned.SecretEnv = cloneStringMap(src.SecretEnv)
	cloned.Network.AllowList = append([]string(nil), src.Network.AllowList...)
	cloned.Network.DenyList = append([]string(nil), src.Network.DenyList...)
	if src.Daytona != nil {
		daytona := *src.Daytona
		cloned.Daytona = &daytona
	}
	return cloned
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

func cloneStringMap(src map[string]string) map[string]string {
	if len(src) == 0 {
		return nil
	}

	cloned := make(map[string]string, len(src))
	maps.Copy(cloned, src)
	return cloned
}
