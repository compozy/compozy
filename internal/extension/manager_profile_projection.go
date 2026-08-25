package extensionpkg

import (
	"context"
	"errors"
	"fmt"
	"slices"
)

// ProjectForProfile returns the extension resources visible through one real profile.
// The boolean is false when the extension is disabled for that profile.
func (m *Manager) ProjectForProfile(
	ctx context.Context,
	key InstanceKey,
	profile ProfileLens,
) (*Extension, bool, error) {
	if ctx == nil {
		return nil, false, ErrContextRequired
	}
	if err := ctx.Err(); err != nil {
		return nil, false, err
	}
	if m == nil {
		return nil, false, ErrManagerRequired
	}
	profile = profile.normalize()
	if !profile.valid() {
		return nil, false, errors.New("extension: profile id and name are required")
	}
	key = key.Normalize()
	if err := key.Validate(); err != nil {
		return nil, false, err
	}

	extension, err := m.GetForInstance(key)
	if err != nil {
		return nil, false, err
	}
	if extension.Manifest == nil {
		extension, err = m.inspectProjectionSource(ctx, key)
		if err != nil {
			return nil, false, err
		}
	}
	enabled := extension.Info.Enabled
	if m.registry != nil {
		enabled, err = m.registry.IsEnabledForProfile(key.Name, profile.ID)
		if err != nil {
			return nil, false, fmt.Errorf(
				"extension: resolve %q enablement for profile %q: %w",
				key.Name,
				profile.Name,
				err,
			)
		}
	}
	extension.Info.Enabled = enabled
	extension.Status.Enabled = enabled
	if extension.Manifest == nil {
		return extension, enabled, nil
	}

	manifest := cloneManifest(extension.Manifest)
	projectedManifest := cloneManifest(extension.Manifest)
	projectManifestResourcesForProfile(&projectedManifest.Resources, profile.Name)
	temporary := &managedExtension{
		key:      key,
		info:     extension.Info,
		rootDir:  extension.RootDir,
		manifest: projectedManifest,
	}
	loaded, err := m.loadDeclarativeResources(ctx, temporary)
	if err != nil {
		return nil, false, fmt.Errorf(
			"extension: project %q resources for profile %q: %w",
			key.Name,
			profile.Name,
			err,
		)
	}
	extension.Manifest = manifest
	extension.Hooks = loaded.hooks
	extension.Agents = loaded.agents
	extension.StaticAgents = loaded.staticAgents
	extension.Skills = loaded.skills
	extension.Loops = loaded.loops
	extension.AutomationJobs = loaded.automationJobs
	extension.AutomationTriggers = loaded.automationTriggers
	extension.Layouts = loaded.layouts
	return extension, enabled, nil
}

func (m *Manager) inspectProjectionSource(ctx context.Context, key InstanceKey) (*Extension, error) {
	if key.WorkspaceID == "" {
		return m.InspectPackageResources(ctx, key.Name)
	}
	link, err := m.registry.GetDevLink(key.Name, key.WorkspaceID)
	if err != nil {
		if errors.Is(err, ErrExtensionNotDevLinked) {
			return m.InspectPackageResources(ctx, key.Name)
		}
		return nil, fmt.Errorf("extension: inspect development projection %q: %w", key.runtimeID(), err)
	}
	verified, err := m.resolveDevGeneration(ctx, key, link.OriginPath, link.BundleGeneration)
	if err != nil {
		return nil, fmt.Errorf("extension: verify development projection %q: %w", key.runtimeID(), err)
	}
	candidate := managedDevExtension(key, verified, m.logRingFor(key))
	loaded, err := m.loadDeclarativeResources(ctx, candidate)
	if err != nil {
		return nil, fmt.Errorf("extension: load development projection %q: %w", key.runtimeID(), err)
	}
	loaded.apply(candidate)
	return m.cloneExtension(candidate), nil
}

func projectManifestResourcesForProfile(resources *ResourcesConfig, profileName string) {
	if resources == nil {
		return
	}
	resources.Skills = visibleManifestResourcePaths(resources.Skills, profileName)
	resources.Loops = visibleManifestResourcePaths(resources.Loops, profileName)
	resources.Agents = visibleManifestResourcePaths(resources.Agents, profileName)
	resources.Automation = visibleManifestResourcePaths(resources.Automation, profileName)
	resources.Layouts = visibleManifestResourcePaths(resources.Layouts, profileName)
	resources.Hooks = slices.DeleteFunc(resources.Hooks, func(value HookConfig) bool {
		return !manifestPlacementVisible(value.Profile, profileName)
	})
	tools := make(map[string]ToolConfig, len(resources.Tools))
	for name, value := range resources.Tools {
		if manifestPlacementVisible(value.Profile, profileName) {
			tools[name] = value
		}
	}
	resources.Tools = tools
	servers := make(map[string]MCPServerConfig, len(resources.MCPServers))
	for name, value := range resources.MCPServers {
		if manifestPlacementVisible(value.Profile, profileName) {
			servers[name] = value
		}
	}
	resources.MCPServers = servers
	resources.CommandGroups = slices.DeleteFunc(resources.CommandGroups, func(value manifestCommandGroupSpec) bool {
		return !manifestPlacementVisible(value.Profile, profileName)
	})
	resources.CmdPalette.Commands = slices.DeleteFunc(
		resources.CmdPalette.Commands,
		func(value CmdPaletteCommand) bool {
			return !cmdPaletteCommandVisibleForResources(
				value,
				resources.Tools,
				resources.CmdPalette.Views,
				profileName,
			)
		},
	)
	resources.CmdPalette.Views = slices.DeleteFunc(
		resources.CmdPalette.Views,
		func(value CmdPaletteView) bool {
			return !cmdPaletteViewVisibleForResources(value, resources.Tools, profileName)
		},
	)
}

func visibleManifestResourcePaths(
	values []ManifestResourcePath,
	profileName string,
) []ManifestResourcePath {
	return slices.DeleteFunc(values, func(value ManifestResourcePath) bool {
		return !manifestPlacementVisible(value.Profile, profileName)
	})
}
