package managedsidecar

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	compozyconfig "github.com/compozy/compozy/internal/config"
)

// SidecarPath derives a sidecar file path from an agent directory or AGENT.md path.
func SidecarPath(agentPath, sidecarName string) (string, error) {
	trimmed := strings.TrimSpace(agentPath)
	if trimmed == "" {
		return "", errors.New("agent path is required")
	}
	cleaned := filepath.Clean(trimmed)
	if strings.EqualFold(filepath.Base(cleaned), compozyconfig.AgentDefinitionFileName) {
		return filepath.Join(filepath.Dir(cleaned), sidecarName), nil
	}
	return filepath.Join(cleaned, sidecarName), nil
}

// ValidAgentName reports whether value is one safe path component.
func ValidAgentName(value string) bool {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" || trimmed == "." || trimmed == ".." {
		return false
	}
	return !strings.Contains(trimmed, "/") && !strings.Contains(trimmed, `\`)
}

// RevisionSourcePath derives the authored sidecar source owned by an AGENT.md definition.
func RevisionSourcePath(agentDefinitionPath, sidecarName string) (string, error) {
	cleaned := filepath.Clean(strings.TrimSpace(agentDefinitionPath))
	if cleaned == "." || !strings.EqualFold(filepath.Base(cleaned), compozyconfig.AgentDefinitionFileName) {
		return "", errors.New("purge agent history requires an AGENT.md source path")
	}
	agentDir := filepath.Dir(cleaned)
	agentsDir := filepath.Dir(agentDir)
	if filepath.Base(agentsDir) != compozyconfig.AgentsDirName {
		return "", errors.New("purge agent history source path is outside an agents root")
	}
	root := filepath.Dir(agentsDir)
	if filepath.Base(root) == compozyconfig.DirName {
		root = filepath.Dir(root)
	}
	relative, err := filepath.Rel(root, filepath.Join(agentDir, sidecarName))
	if err != nil {
		return "", fmt.Errorf("derive purge history source path: %w", err)
	}
	if relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) ||
		filepath.IsAbs(relative) {
		return "", errors.New("purge history source path escapes its authored root")
	}
	return filepath.ToSlash(relative), nil
}
