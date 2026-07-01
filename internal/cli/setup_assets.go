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
	workspaceRoot, err := resolveSetupWorkspaceRoot(resolver)
	if err != nil {
		return setup.EffectiveCatalog{}, err
	}

	// Discovery resolves the Compozy home root internally (honoring COMPOZY_HOME),
	// so setup asset discovery tracks the same root as the rest of the runtime.
	discovery, err := extensions.Discovery{
		WorkspaceRoot: workspaceRoot,
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

func resolveSetupWorkspaceRoot(options setup.ResolverOptions) (string, error) {
	workspaceRoot := strings.TrimSpace(options.CWD)
	if workspaceRoot == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return "", fmt.Errorf("resolve setup workspace root: %w", err)
		}
		workspaceRoot = cwd
	}

	return workspaceRoot, nil
}
