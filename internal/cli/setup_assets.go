package cli

import (
	"context"
	"fmt"
	"os"
	"strings"

	extensions "github.com/compozy/compozy/internal/core/extension"
	"github.com/compozy/compozy/internal/setup"
)

func loadEffectiveSetupCatalog(
	ctx context.Context,
	resolver setup.ResolverOptions,
) (setup.EffectiveCatalog, error) {
	workspaceRoot, homeDir, err := resolveSetupAssetRoots(resolver)
	if err != nil {
		return setup.EffectiveCatalog{}, err
	}

	discovery, err := extensions.Discovery{
		WorkspaceRoot: workspaceRoot,
		HomeDir:       homeDir,
	}.Discover(ctx)
	if err != nil {
		return setup.EffectiveCatalog{}, fmt.Errorf("discover setup extension assets: %w", err)
	}

	return effectiveSetupCatalogFromDiscovery(discovery)
}

func effectiveSetupCatalogFromDiscovery(discovery extensions.DiscoveryResult) (setup.EffectiveCatalog, error) {
	bundledSkills, err := setup.ListBundledSkills()
	if err != nil {
		return setup.EffectiveCatalog{}, err
	}
	bundledReusableAgents, err := setup.ListBundledReusableAgents()
	if err != nil {
		return setup.EffectiveCatalog{}, err
	}
	extensionSkills, err := setup.ListExtensionSkills(extensionSkillSources(discovery.SkillPacks.Packs))
	if err != nil {
		return setup.EffectiveCatalog{}, err
	}
	extensionReusableAgents, err := setup.ListExtensionReusableAgents(
		extensionReusableAgentSources(discovery.ReusableAgents.Agents),
	)
	if err != nil {
		return setup.EffectiveCatalog{}, err
	}

	return setup.BuildEffectiveCatalog(
		bundledSkills,
		extensionSkills,
		bundledReusableAgents,
		extensionReusableAgents,
	), nil
}

func effectiveExtensionSkillSources(discovery extensions.DiscoveryResult) ([]setup.SkillPackSource, error) {
	catalog, err := effectiveSetupCatalogFromDiscovery(discovery)
	if err != nil {
		return nil, err
	}
	return setup.ExtensionSkillPackSources(catalog.Skills), nil
}

func resolveSetupAssetRoots(options setup.ResolverOptions) (string, string, error) {
	workspaceRoot := strings.TrimSpace(options.CWD)
	if workspaceRoot == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return "", "", fmt.Errorf("resolve setup workspace root: %w", err)
		}
		workspaceRoot = cwd
	}

	homeDir := strings.TrimSpace(options.HomeDir)
	if homeDir == "" {
		resolvedHomeDir, err := os.UserHomeDir()
		if err != nil {
			return "", "", fmt.Errorf("resolve setup home directory: %w", err)
		}
		homeDir = resolvedHomeDir
	}

	return workspaceRoot, homeDir, nil
}
