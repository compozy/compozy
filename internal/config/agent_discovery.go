package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/compozy/compozy/internal/fileutil"
)

// LoadWorkspaceAgentDefs loads workspace-visible agents using root, additional, then global precedence.
func LoadWorkspaceAgentDefs(
	rootDir string,
	additionalDirs []string,
	homePaths HomePaths,
	profileNames ...string,
) ([]AgentDef, error) {
	if profileName := firstProfileName(profileNames); profileName != "" {
		if err := ValidateResourceProfileName(profileName); err != nil {
			return nil, err
		}
	}
	roots := WorkspaceDiscoveryRoots(rootDir, additionalDirs, homePaths, profileNames...)
	if len(roots) == 0 {
		return nil, nil
	}

	agents := make([]AgentDef, 0)
	seen := make(map[string]int)
	for _, root := range roots {
		loaded, err := loadAgentDefsFromRoot(root)
		if err != nil {
			return nil, err
		}
		for _, agent := range loaded {
			agent.SourceLayer = AgentLayerName(root.Source)
			if winnerIndex, ok := seen[agent.Name]; ok {
				agents[winnerIndex].ShadowedDefinitions = append(
					agents[winnerIndex].ShadowedDefinitions,
					AgentDefinitionRef{Layer: agent.SourceLayer, Path: agent.SourcePath},
				)
				continue
			}
			seen[agent.Name] = len(agents)
			agents = append(agents, agent)
		}
	}

	return agents, nil
}

func loadAgentDefsFromRoot(root WorkspaceDiscoveryRoot) (agents []AgentDef, err error) {
	dirPath := root.AgentsDir()
	directory, err := fileutil.OpenDirectory(dirPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("read agents directory %q: %w", dirPath, err)
	}
	defer func() {
		if closeErr := directory.Close(); closeErr != nil {
			agents = nil
			err = errors.Join(err, fmt.Errorf("close agents directory %q: %w", dirPath, closeErr))
		}
	}()

	names, err := directory.ReadDir()
	if err != nil {
		return nil, fmt.Errorf("read agents directory %q: %w", dirPath, err)
	}
	sort.Strings(names)

	agents = make([]AgentDef, 0, len(names))
	for _, name := range names {
		if IsReservedAgentName(name) {
			continue
		}

		agentDirectory, openErr := directory.OpenDirectory(name)
		if errors.Is(openErr, fileutil.ErrNotDirectory) {
			continue
		}
		if openErr != nil {
			return nil, fmt.Errorf("open agent directory %q: %w", filepath.Join(dirPath, name), openErr)
		}

		agentPath := filepath.Join(dirPath, name, agentDefName)
		agent, loadErr := loadAgentDefFromDirectory(agentDirectory, agentDefName, agentPath)
		if closeErr := agentDirectory.Close(); closeErr != nil {
			if loadErr != nil {
				return nil, errors.Join(
					loadErr,
					fmt.Errorf("close agent directory %q: %w", filepath.Join(dirPath, name), closeErr),
				)
			}
			return nil, fmt.Errorf("close agent directory %q: %w", filepath.Join(dirPath, name), closeErr)
		}
		if errors.Is(loadErr, os.ErrNotExist) {
			continue
		}
		if loadErr != nil {
			return nil, loadErr
		}
		target := NormalizeAgentName(name)
		if agent.Name != target {
			return nil, fmt.Errorf("agent file %q defines name %q, expected %q", agentPath, agent.Name, target)
		}
		agents = append(agents, agent)
	}

	return agents, nil
}
