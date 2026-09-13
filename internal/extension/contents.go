package extensionpkg

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	compozyconfig "github.com/compozy/compozy/internal/config"
	looppkg "github.com/compozy/compozy/internal/loop"
	"github.com/compozy/compozy/internal/resources"
	skillspkg "github.com/compozy/compozy/internal/skills"
)

// ContentsFor counts loaded resources, not the directories that contain them.
func ContentsFor(manifest *Manifest, kit []KitItem) contract.ExtensionContentsPayload {
	var contents contract.ExtensionContentsPayload
	seen := make(map[struct {
		kind resources.ResourceKind
		name string
	}]struct{}, len(kit))
	for _, item := range kit {
		key := struct {
			kind resources.ResourceKind
			name string
		}{item.Kind, item.Name}
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		switch item.Kind {
		case skillspkg.SkillResourceKind:
			contents.Skills++
		case looppkg.ResourceKind:
			contents.Loops++
		case compozyconfig.AgentResourceKind:
			contents.Agents++
		}
	}
	if manifest != nil {
		contents.MCPServers = len(manifest.Resources.MCPServers)
		contents.Hooks = len(manifest.Resources.Hooks)
		if strings.TrimSpace(manifest.Bridge.Platform) != "" {
			contents.Bridges = 1
		}
	}
	return contents
}

func extensionSnapshotContentsForProfile(ext *Extension, profileName string) contract.ExtensionContentsPayload {
	kit := make([]KitItem, 0, len(ext.Skills)+len(ext.Loops)+len(ext.Agents))
	for _, skill := range ext.Skills {
		if skill != nil {
			kit = append(kit, KitItem{Kind: skillspkg.SkillResourceKind, Name: skill.Meta.Name})
		}
	}
	for _, loop := range ext.Loops {
		kit = append(kit, KitItem{Kind: looppkg.ResourceKind, Name: loop.Name})
	}
	for _, agent := range ext.Agents {
		kit = append(kit, KitItem{Kind: compozyconfig.AgentResourceKind, Name: agent.Name})
	}
	manifest := cloneManifest(ext.Manifest)
	if manifest != nil {
		projectManifestResourcesForProfile(&manifest.Resources, profileName)
	}
	return ContentsFor(manifest, kit)
}

func extensionOriginPayload(value ExtensionProvenance) *contract.MarketplaceOriginPayload {
	if strings.TrimSpace(value.SourceRef) == "" || strings.TrimSpace(value.EntryID) == "" {
		return nil
	}
	return &contract.MarketplaceOriginPayload{
		Source:    value.SourceName,
		SourceRef: value.SourceRef,
		EntryID:   value.EntryID,
	}
}

// InspectPackageContents reads declared files without starting the extension runtime.
func InspectPackageContents(
	ctx context.Context, ext *Extension, profileName string,
) (contract.ExtensionContentsPayload, error) {
	if ctx == nil {
		return contract.ExtensionContentsPayload{}, ErrContextRequired
	}
	if err := ctx.Err(); err != nil {
		return contract.ExtensionContentsPayload{}, err
	}
	if ext == nil || ext.Manifest == nil {
		return contract.ExtensionContentsPayload{}, nil
	}
	manifest := cloneManifest(ext.Manifest)
	projectManifestResourcesForProfile(&manifest.Resources, profileName)
	root := ext.RootDir
	if root == "" {
		root = filepath.Dir(ext.Info.ManifestPath)
	}
	packageState := &managedExtension{info: ext.Info, manifest: manifest, rootDir: root}
	// These declarative readers use only the package snapshot, never manager runtime state.
	reader := &Manager{}
	inspected := *ext
	inspected.Manifest = manifest
	var err error
	inspected.Skills, err = reader.loadSkillResources(packageState)
	if err != nil {
		return contract.ExtensionContentsPayload{}, fmt.Errorf("extension: inspect package skills: %w", err)
	}
	inspected.Loops, err = reader.loadLoopResources(packageState)
	if err != nil {
		return contract.ExtensionContentsPayload{}, fmt.Errorf("extension: inspect package loops: %w", err)
	}
	agents, err := reader.loadAgentResources(packageState)
	if err != nil {
		return contract.ExtensionContentsPayload{}, fmt.Errorf("extension: inspect package agents: %w", err)
	}
	inspected.Agents = make([]compozyconfig.AgentDef, 0, len(agents))
	for _, agent := range agents {
		inspected.Agents = append(inspected.Agents, agent.Agent)
	}
	if err := ctx.Err(); err != nil {
		return contract.ExtensionContentsPayload{}, err
	}
	return extensionSnapshotContentsForProfile(&inspected, profileName), nil
}
