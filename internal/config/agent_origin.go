package config

import (
	"path/filepath"
	"strings"
)

// AgentOrigin identifies the authored root that owns an agent definition.
type AgentOrigin string

const (
	// AgentOriginGlobal identifies a definition stored under AGH_HOME.
	AgentOriginGlobal AgentOrigin = "global"
	// AgentOriginWorkspace identifies a definition stored under a workspace or additional root.
	AgentOriginWorkspace AgentOrigin = "workspace"
)

// AgentOriginFor classifies an AGENT.md source path and returns its workspace root when applicable.
func AgentOriginFor(home HomePaths, sourcePath string) (AgentOrigin, string) {
	cleaned := filepath.Clean(strings.TrimSpace(sourcePath))
	if cleaned == "." || !strings.EqualFold(filepath.Base(cleaned), AgentDefinitionFileName) {
		return "", ""
	}
	if pathWithin(filepath.Clean(home.AgentsDir), cleaned) {
		return AgentOriginGlobal, ""
	}

	agentDir := filepath.Dir(cleaned)
	agentsDir := filepath.Dir(agentDir)
	metadataDir := filepath.Dir(agentsDir)
	if filepath.Base(agentsDir) != AgentsDirName || filepath.Base(metadataDir) != DirName {
		return "", ""
	}
	workspaceRoot := filepath.Dir(metadataDir)
	if strings.TrimSpace(workspaceRoot) == "" || workspaceRoot == "." {
		return "", ""
	}
	return AgentOriginWorkspace, workspaceRoot
}

func pathWithin(root string, target string) bool {
	trimmedRoot := strings.TrimSpace(root)
	trimmedTarget := strings.TrimSpace(target)
	if trimmedRoot == "" || trimmedTarget == "" {
		return false
	}
	relative, err := filepath.Rel(filepath.Clean(trimmedRoot), filepath.Clean(trimmedTarget))
	if err != nil {
		return false
	}
	return relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}
