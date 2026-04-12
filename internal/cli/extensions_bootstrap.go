package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/core/agent"
	extensions "github.com/compozy/compozy/internal/core/extension"
	"github.com/compozy/compozy/internal/core/modelprovider"
	"github.com/compozy/compozy/internal/core/provider"
	"github.com/compozy/compozy/internal/setup"
)

type declarativeAssets struct {
	Discovery extensions.DiscoveryResult
}

func (s *commandState) bootstrapDeclarativeAssetsForWorkspaceRoot(
	ctx context.Context,
	workspaceRoot string,
	invokingCommand string,
) (declarativeAssets, func(), error) {
	if !s.requiresDeclarativeAssetBootstrap() {
		return declarativeAssets{}, func() {}, nil
	}

	discovery, err := extensions.Discovery{WorkspaceRoot: workspaceRoot}.Discover(ctx)
	if err != nil {
		return declarativeAssets{}, nil, fmt.Errorf("discover declarative extension assets: %w", err)
	}

	restoreModelOverlay, err := modelprovider.ActivateOverlay(modelProviderOverlayEntries(discovery.Providers.Model))
	if err != nil {
		return declarativeAssets{}, nil, fmt.Errorf("activate model provider overlay: %w", err)
	}

	providerEntries, err := providerOverlayEntries(discovery.Providers.Review, workspaceRoot, invokingCommand)
	if err != nil {
		restoreModelOverlay()
		return declarativeAssets{}, nil, fmt.Errorf("build review provider overlay: %w", err)
	}

	restoreProviderOverlay, err := provider.ActivateOverlay(providerEntries)
	if err != nil {
		restoreModelOverlay()
		return declarativeAssets{}, nil, fmt.Errorf("activate review provider overlay: %w", err)
	}

	restoreAgentOverlay, err := agent.ActivateOverlay(agentOverlayEntries(discovery.Providers.IDE))
	if err != nil {
		restoreProviderOverlay()
		restoreModelOverlay()
		return declarativeAssets{}, nil, fmt.Errorf("activate ACP runtime overlay: %w", err)
	}

	cleanup := func() {
		restoreAgentOverlay()
		restoreProviderOverlay()
		restoreModelOverlay()
	}

	return declarativeAssets{Discovery: discovery}, cleanup, nil
}

func (s *commandState) requiresDeclarativeAssetBootstrap() bool {
	if s == nil {
		return false
	}

	switch s.kind {
	case commandKindFetchReviews, commandKindFixReviews, commandKindExec, commandKindStart:
		return true
	default:
		return false
	}
}

func agentOverlayEntries(entries []extensions.DeclaredProvider) []agent.OverlayEntry {
	if len(entries) == 0 {
		return nil
	}

	overlays := make([]agent.OverlayEntry, 0, len(entries))
	for i := range entries {
		entry := &entries[i]
		overlays = append(overlays, agent.OverlayEntry{
			Name:               entry.Name,
			DisplayName:        entry.DisplayName,
			Command:            entry.Command,
			SetupAgentName:     entry.SetupAgentName,
			DefaultModel:       entry.DefaultModel,
			SupportsAddDirs:    cloneBool(entry.SupportsAddDirs),
			UsesBootstrapModel: cloneBool(entry.UsesBootstrapModel),
			DocsURL:            entry.DocsURL,
			InstallHint:        entry.InstallHint,
			FullAccessModeID:   entry.FullAccessModeID,
			FixedArgs:          cloneNormalizedStringSlice(entry.FixedArgs),
			ProbeArgs:          cloneNormalizedStringSlice(entry.ProbeArgs),
			EnvVars:            mapsClone(entry.Env),
			Fallbacks:          agentOverlayFallbacks(entry.Fallbacks),
			Bootstrap: agent.OverlayBootstrap{
				ModelFlag: bootstrapValue(
					entry.Bootstrap,
					func(v *extensions.ProviderBootstrap) string { return v.ModelFlag },
				),
				ReasoningEffortFlag: bootstrapValue(
					entry.Bootstrap,
					func(v *extensions.ProviderBootstrap) string { return v.ReasoningEffortFlag },
				),
				AddDirFlag: bootstrapValue(
					entry.Bootstrap,
					func(v *extensions.ProviderBootstrap) string { return v.AddDirFlag },
				),
				DefaultAccessModeArgs: bootstrapSlice(
					entry.Bootstrap,
					func(v *extensions.ProviderBootstrap) []string { return v.DefaultAccessModeArgs },
				),
				FullAccessModeArgs: bootstrapSlice(
					entry.Bootstrap,
					func(v *extensions.ProviderBootstrap) []string { return v.FullAccessModeArgs },
				),
			},
			Metadata: mapsClone(entry.Metadata),
		})
	}
	return overlays
}

func providerOverlayEntries(
	entries []extensions.DeclaredProvider,
	workspaceRoot string,
	invokingCommand string,
) ([]provider.OverlayEntry, error) {
	if len(entries) == 0 {
		return nil, nil
	}

	bridges := make(map[string]provider.ExtensionBridge)
	overlays := make([]provider.OverlayEntry, 0, len(entries))
	for i := range entries {
		entry := &entries[i]
		kind := providerEntryReviewKind(entry.ProviderEntry)
		var bridge provider.ExtensionBridge
		if kind == extensions.ProviderKindExtension {
			key := entry.ManifestPath
			if existing, ok := bridges[key]; ok {
				bridge = existing
			} else {
				built, err := extensions.NewReviewProviderBridge(*entry, workspaceRoot, invokingCommand)
				if err != nil {
					return nil, err
				}
				bridge = built
				bridges[key] = bridge
			}
		}
		overlays = append(overlays, provider.OverlayEntry{
			Name:        entry.Name,
			DisplayName: entry.DisplayName,
			Command:     entry.Command,
			Target:      entry.Target,
			Kind:        provider.OverlayKind(kind),
			Bridge:      bridge,
			Metadata:    mapsClone(entry.Metadata),
		})
	}
	return overlays, nil
}

func modelProviderOverlayEntries(entries []extensions.DeclaredProvider) []modelprovider.OverlayEntry {
	if len(entries) == 0 {
		return nil
	}

	overlays := make([]modelprovider.OverlayEntry, 0, len(entries))
	for i := range entries {
		entry := &entries[i]
		overlays = append(overlays, modelprovider.OverlayEntry{
			Name:        entry.Name,
			DisplayName: entry.DisplayName,
			Target:      providerEntryModelTarget(entry.ProviderEntry),
			Metadata:    mapsClone(entry.Metadata),
		})
	}
	return overlays
}

func extensionSkillSources(packs []extensions.DeclaredSkillPack) []setup.SkillPackSource {
	if len(packs) == 0 {
		return nil
	}

	sources := make([]setup.SkillPackSource, 0, len(packs))
	for i := range packs {
		pack := &packs[i]
		sources = append(sources, setup.SkillPackSource{
			ExtensionName: pack.Extension.Name,
			ManifestPath:  pack.ManifestPath,
			Pattern:       pack.Pattern,
			ResolvedPath:  pack.ResolvedPath,
			SourceFS:      pack.SourceFS,
			SourceDir:     pack.SourceDir,
		})
	}
	return sources
}

func mapsClone(src map[string]string) map[string]string {
	if len(src) == 0 {
		return nil
	}
	dst := make(map[string]string, len(src))
	for key, value := range src {
		dst[key] = value
	}
	return dst
}

func cloneBool(src *bool) *bool {
	if src == nil {
		return nil
	}
	value := *src
	return &value
}

func agentOverlayFallbacks(values []extensions.ProviderLauncher) []agent.Launcher {
	if len(values) == 0 {
		return nil
	}

	launchers := make([]agent.Launcher, 0, len(values))
	for _, value := range values {
		launchers = append(launchers, agent.Launcher{
			Command:   value.Command,
			FixedArgs: cloneNormalizedStringSlice(value.FixedArgs),
			ProbeArgs: cloneNormalizedStringSlice(value.ProbeArgs),
		})
	}
	return launchers
}

func bootstrapValue[T any](bootstrap *extensions.ProviderBootstrap, getter func(*extensions.ProviderBootstrap) T) T {
	var zero T
	if bootstrap == nil {
		return zero
	}
	return getter(bootstrap)
}

func bootstrapSlice(
	bootstrap *extensions.ProviderBootstrap,
	getter func(*extensions.ProviderBootstrap) []string,
) []string {
	if bootstrap == nil {
		return nil
	}
	return cloneNormalizedStringSlice(getter(bootstrap))
}

func providerEntryReviewKind(entry extensions.ProviderEntry) extensions.ProviderKind {
	switch entry.Kind {
	case extensions.ProviderKindExtension:
		return extensions.ProviderKindExtension
	default:
		return extensions.ProviderKindAlias
	}
}

func providerEntryModelTarget(entry extensions.ProviderEntry) string {
	if target := strings.TrimSpace(entry.Target); target != "" {
		return target
	}
	return strings.TrimSpace(entry.Command)
}

func cloneNormalizedStringSlice(values []string) []string {
	if len(values) == 0 {
		return nil
	}

	cloned := make([]string, 0, len(values))
	for _, value := range values {
		cloned = append(cloned, strings.TrimSpace(value))
	}
	return cloned
}
