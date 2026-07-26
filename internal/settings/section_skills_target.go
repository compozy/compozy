package settings

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"strings"

	aghconfig "github.com/compozy/agh/internal/config"

	"github.com/compozy/agh/internal/resources"
	skillspkg "github.com/compozy/agh/internal/skills"
	workspacepkg "github.com/compozy/agh/internal/workspace"
)

func buildSkillsOperationalLinks() []OperationalLink {
	return []OperationalLink{{
		Label: string(SectionSkills),
		Path:  "/marketplace/skills?tab=installed",
	}}
}

func (s *service) resolveEffectiveAgent(
	resolved *workspacepkg.ResolvedWorkspace,
	agentName string,
) (aghconfig.AgentDef, WriteTargetKind, error) {
	target := aghconfig.NormalizeAgentName(agentName)
	if target == "" {
		return aghconfig.AgentDef{}, "", validationError(errors.New("settings: agent_name is required"))
	}

	if resolved != nil {
		for _, diagnostic := range resolved.AgentDiagnostics {
			if aghconfig.NormalizeAgentName(diagnostic.Name) != target {
				continue
			}
			return aghconfig.AgentDef{}, "", unprocessableError(
				fmt.Errorf(
					"settings: agent %q at %q: %s",
					target,
					strings.TrimSpace(diagnostic.Path),
					strings.TrimSpace(diagnostic.Message),
				),
			)
		}
		for _, agent := range resolved.Agents {
			if aghconfig.NormalizeAgentName(agent.Name) != target {
				continue
			}
			return aghconfig.CloneAgentDef(agent), agentWriteTargetKind(s.homePaths, agent.SourcePath), nil
		}
		return aghconfig.AgentDef{}, "", notFoundError(fmt.Errorf("settings: agent %q not found", target))
	}

	path := filepath.Join(s.homePaths.AgentsDir, target, "AGENT.md")
	agent, err := aghconfig.LoadAgentDefFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return aghconfig.AgentDef{}, "", notFoundError(fmt.Errorf("settings: agent %q not found", target))
		}
		return aghconfig.AgentDef{}, "", unprocessableError(
			fmt.Errorf("settings: load agent %q: %w", target, err),
		)
	}
	if aghconfig.NormalizeAgentName(agent.Name) != target {
		return aghconfig.AgentDef{}, "", unprocessableError(
			fmt.Errorf("settings: agent file %q defines %q, expected %q", path, agent.Name, target),
		)
	}
	return agent, WriteTargetGlobalAgentFile, nil
}

func agentWriteTargetKind(homePaths aghconfig.HomePaths, sourcePath string) WriteTargetKind {
	if withinRoot(sourcePath, homePaths.AgentsDir) {
		return WriteTargetGlobalAgentFile
	}
	return WriteTargetWorkspaceAgentFile
}

func withinRoot(path string, root string) bool {
	trimmedPath := strings.TrimSpace(path)
	trimmedRoot := strings.TrimSpace(root)
	if trimmedPath == "" || trimmedRoot == "" {
		return false
	}
	rel, err := filepath.Rel(trimmedRoot, trimmedPath)
	if err != nil {
		return false
	}
	rel = strings.TrimSpace(rel)
	if rel == "" || rel == "." {
		return true
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func mapSkillsSettingsError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, skillspkg.ErrAgentNotFound):
		return notFoundError(err)
	case errors.Is(err, skillspkg.ErrAgentLocalInvalid):
		return unprocessableError(err)
	default:
		return err
	}
}

func sliceToSet(values []string) map[string]struct{} {
	set := make(map[string]struct{}, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		set[trimmed] = struct{}{}
	}
	return set
}

func resourceKindsToStrings(values []resources.ResourceKind) []string {
	if len(values) == 0 {
		return nil
	}
	converted := make([]string, 0, len(values))
	for _, value := range values {
		converted = append(converted, string(value))
	}
	return converted
}

func globalMCPSidecarPath(homePaths aghconfig.HomePaths) string {
	return filepath.Join(homePaths.HomeDir, aghconfig.MCPJSONName)
}
