package extensionpkg

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/heartbeat"
	"github.com/compozy/compozy/internal/soul"
)

// AgentSidecar is one validated authored-context file shipped beside an agent.
type AgentSidecar struct {
	SourcePath string
	Body       string
}

// StaticAgent is one complete agent directory shipped by an extension.
type StaticAgent struct {
	Agent     compozyconfig.AgentDef
	Soul      *AgentSidecar
	Heartbeat *AgentSidecar
}

// LoadAgentResources loads the strict <entry>/<agent>/AGENT.md resource contract.
func LoadAgentResources(rootDir string, paths []string) ([]StaticAgent, error) {
	loaded := make(map[string]StaticAgent)
	for _, resourcePath := range paths {
		entryRoot, err := resolveResourcePathWithinRoot(rootDir, resourcePath, "agent resource")
		if err != nil {
			return nil, err
		}
		items, err := loadAgentResourceEntry(rootDir, entryRoot)
		if err != nil {
			return nil, err
		}
		for _, item := range items {
			name := strings.TrimSpace(item.Agent.Name)
			if _, exists := loaded[name]; exists {
				return nil, fmt.Errorf("%w: duplicate extension agent %q", ErrManifestInvalid, name)
			}
			loaded[name] = item
		}
	}

	names := make([]string, 0, len(loaded))
	for name := range loaded {
		names = append(names, name)
	}
	slices.Sort(names)
	result := make([]StaticAgent, 0, len(names))
	for _, name := range names {
		result = append(result, cloneStaticAgent(loaded[name]))
	}
	return result, nil
}

func loadAgentResourceEntry(rootDir, entryRoot string) ([]StaticAgent, error) {
	info, err := os.Stat(entryRoot)
	if err != nil {
		return nil, fmt.Errorf("extension: stat agent resource %q: %w", entryRoot, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("%w: agent resource %q must be a directory", ErrManifestInvalid, entryRoot)
	}
	entries, err := os.ReadDir(entryRoot)
	if err != nil {
		return nil, fmt.Errorf("extension: read agent resource %q: %w", entryRoot, err)
	}
	slices.SortFunc(entries, func(left, right os.DirEntry) int {
		return strings.Compare(left.Name(), right.Name())
	})

	result := make([]StaticAgent, 0, len(entries))
	for _, entry := range entries {
		entryPath := filepath.Join(entryRoot, entry.Name())
		if entry.Type()&os.ModeSymlink != 0 {
			relative, err := filepath.Rel(rootDir, entryPath)
			if err != nil {
				return nil, fmt.Errorf("extension: resolve agent resource %q: %w", entryPath, err)
			}
			if _, err := resolveResourcePathWithinRoot(rootDir, relative, "agent resource"); err != nil {
				return nil, err
			}
			return nil, fmt.Errorf("%w: agent resource %q must not be a symlink", ErrManifestInvalid, entryPath)
		}
		if !entry.IsDir() {
			if strings.EqualFold(filepath.Ext(entry.Name()), ".md") {
				return nil, fmt.Errorf(
					"%w: loose markdown file %q is not an agent directory; expected <entry>/<agent>/%s",
					ErrManifestInvalid,
					filepath.Join(entryRoot, entry.Name()),
					compozyconfig.AgentDefinitionFileName,
				)
			}
			continue
		}
		agentRelativePath, err := relativeResourcePath(rootDir, entryPath)
		if err != nil {
			return nil, err
		}
		agentDir, err := resolveResourcePathWithinRoot(rootDir, agentRelativePath, "agent resource")
		if err != nil {
			return nil, err
		}
		agent, err := loadStaticAgent(rootDir, agentDir)
		if err != nil {
			return nil, err
		}
		result = append(result, agent)
	}
	return result, nil
}

func loadStaticAgent(rootDir, agentDir string) (StaticAgent, error) {
	agentPath := filepath.Join(agentDir, compozyconfig.AgentDefinitionFileName)
	agent, err := compozyconfig.LoadAgentDefFile(agentPath)
	if err != nil {
		return StaticAgent{}, wrapResourceValidationError(
			agentPath,
			fmt.Errorf("%w: load agent: %w", ErrManifestInvalid, err),
		)
	}
	result := StaticAgent{Agent: compozyconfig.CloneAgentDef(agent)}
	if result.Soul, err = loadStaticAgentSidecar(rootDir, agentDir, soul.FileName, true); err != nil {
		return StaticAgent{}, err
	}
	if result.Heartbeat, err = loadStaticAgentSidecar(rootDir, agentDir, heartbeat.FileName, false); err != nil {
		return StaticAgent{}, err
	}
	return result, nil
}

func loadStaticAgentSidecar(rootDir, agentDir, fileName string, isSoul bool) (*AgentSidecar, error) {
	path := filepath.Join(agentDir, fileName)
	body, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("extension: read agent sidecar %q: %w", path, err)
	}
	relativePath, err := relativeResourcePath(rootDir, path)
	if err != nil {
		return nil, err
	}
	sourcePath := filepath.ToSlash(relativePath)
	if isSoul {
		_, err = soul.Parse(context.Background(), soul.ParseRequest{
			SourcePath: sourcePath,
			Content:    body,
			Config:     compozyconfig.DefaultSoulConfig(),
		})
	} else {
		_, err = heartbeat.Parse(context.Background(), heartbeat.ParseRequest{
			SourcePath: sourcePath,
			Content:    body,
			Config:     compozyconfig.DefaultHeartbeatConfig(),
		})
	}
	if err != nil {
		return nil, wrapResourceValidationError(
			path,
			fmt.Errorf("%w: agent sidecar %q: %w", ErrManifestInvalid, sourcePath, err),
		)
	}
	return &AgentSidecar{SourcePath: sourcePath, Body: string(body)}, nil
}

func relativeResourcePath(rootDir, path string) (string, error) {
	canonicalRoot, err := filepath.EvalSymlinks(rootDir)
	if err != nil {
		return "", fmt.Errorf("extension: resolve resource root %q: %w", rootDir, err)
	}
	canonicalPath, err := filepath.EvalSymlinks(path)
	if err != nil {
		return "", fmt.Errorf("extension: resolve resource path %q: %w", path, err)
	}
	relative, err := filepath.Rel(canonicalRoot, canonicalPath)
	if err != nil {
		return "", fmt.Errorf("extension: resolve resource path %q relative to %q: %w", path, rootDir, err)
	}
	return relative, nil
}

func cloneStaticAgent(value StaticAgent) StaticAgent {
	clone := StaticAgent{Agent: compozyconfig.CloneAgentDef(value.Agent)}
	if value.Soul != nil {
		sidecar := *value.Soul
		clone.Soul = &sidecar
	}
	if value.Heartbeat != nil {
		sidecar := *value.Heartbeat
		clone.Heartbeat = &sidecar
	}
	return clone
}
