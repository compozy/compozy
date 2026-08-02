package extensionpkg

import (
	"context"
	"errors"
	"path/filepath"
	"strings"

	compozyconfig "github.com/compozy/compozy/internal/config"
)

// InspectPackageResources loads one installed package's declarative resources
// without registering it, starting a subprocess, or mutating runtime state.
func (m *Manager) InspectPackageResources(ctx context.Context, name string) (*Extension, error) {
	if m == nil {
		return nil, ErrManagerRequired
	}
	if ctx == nil {
		return nil, errors.New("extension: package inspection context is required")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	info, err := m.registry.Get(strings.TrimSpace(name))
	if err != nil {
		return nil, err
	}
	rootDir := filepath.Dir(strings.TrimSpace(info.ManifestPath))
	manifest, err := LoadManifest(rootDir)
	if err != nil {
		return nil, err
	}
	managed := &managedExtension{info: *info, rootDir: rootDir, manifest: manifest}
	managed.staticAgents, err = m.loadAgentResources(managed)
	if err != nil {
		return nil, err
	}
	managed.agents = make([]compozyconfig.AgentDef, 0, len(managed.staticAgents))
	for _, agent := range managed.staticAgents {
		managed.agents = append(managed.agents, compozyconfig.CloneAgentDef(agent.Agent))
	}
	managed.skills, err = m.loadSkillResources(managed)
	if err != nil {
		return nil, err
	}
	managed.loops, err = m.loadLoopResources(managed)
	if err != nil {
		return nil, err
	}
	managed.automationJobs, managed.automationTriggers, err = m.loadAutomationResources(managed, managed.staticAgents)
	if err != nil {
		return nil, err
	}
	managed.layouts, err = m.loadLayoutResources(managed)
	if err != nil {
		return nil, err
	}
	managed.hooks, err = m.loadHookResources(managed)
	if err != nil {
		return nil, err
	}
	return m.cloneExtension(managed), nil
}
