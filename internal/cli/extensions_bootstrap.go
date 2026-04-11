package cli

import (
	"context"
	"fmt"

	core "github.com/compozy/compozy/internal/core"
	"github.com/compozy/compozy/internal/core/agent"
	extensions "github.com/compozy/compozy/internal/core/extension"
	"github.com/compozy/compozy/internal/core/provider"
	"github.com/compozy/compozy/internal/setup"
)

type declarativeAssets struct {
	Discovery extensions.DiscoveryResult
}

func (s *commandState) bootstrapDeclarativeAssets(
	ctx context.Context,
	cfg core.Config,
) (declarativeAssets, func(), error) {
	if !s.requiresDeclarativeAssetBootstrap() {
		return declarativeAssets{}, func() {}, nil
	}

	discovery, err := extensions.Discovery{WorkspaceRoot: cfg.WorkspaceRoot}.Discover(ctx)
	if err != nil {
		return declarativeAssets{}, nil, fmt.Errorf("discover declarative extension assets: %w", err)
	}

	restoreProviderOverlay, err := provider.ActivateOverlay(providerOverlayEntries(discovery.Providers.Review))
	if err != nil {
		return declarativeAssets{}, nil, fmt.Errorf("activate review provider overlay: %w", err)
	}

	restoreAgentOverlay, err := agent.ActivateOverlay(agentOverlayEntries(discovery.Providers.IDE))
	if err != nil {
		restoreProviderOverlay()
		return declarativeAssets{}, nil, fmt.Errorf("activate ACP runtime overlay: %w", err)
	}

	cleanup := func() {
		restoreAgentOverlay()
		restoreProviderOverlay()
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
	for _, entry := range entries {
		overlays = append(overlays, agent.OverlayEntry{
			Name:     entry.Name,
			Command:  entry.Command,
			Metadata: mapsClone(entry.Metadata),
		})
	}
	return overlays
}

func providerOverlayEntries(entries []extensions.DeclaredProvider) []provider.OverlayEntry {
	if len(entries) == 0 {
		return nil
	}

	overlays := make([]provider.OverlayEntry, 0, len(entries))
	for _, entry := range entries {
		overlays = append(overlays, provider.OverlayEntry{
			Name:     entry.Name,
			Command:  entry.Command,
			Metadata: mapsClone(entry.Metadata),
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
