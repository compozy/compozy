package setup

import (
	"fmt"
	"os"
	"path/filepath"
)

// LegacyAssetCleanupConfig describes one cleanup run for legacy setup-managed assets
// that changed ownership and must no longer remain active after setup.
type LegacyAssetCleanupConfig struct {
	ResolverOptions

	Global bool
}

// LegacyAssetRemoval captures one legacy setup-managed path removed during cleanup.
type LegacyAssetRemoval struct {
	Kind CatalogAssetKind
	Name string
	Path string
}

// LegacyAssetCleanupResult summarizes one legacy cleanup run.
type LegacyAssetCleanupResult struct {
	Removed []LegacyAssetRemoval
}

var (
	legacyTransferredSkillNames = []string{
		"cy-idea-factory",
	}
	legacyTransferredReusableAgentNames = []string{
		"architect-advisor",
		"devils-advocate",
		"pragmatic-engineer",
		"product-mind",
		"security-advocate",
		"the-thinker",
	}
)

// CleanupLegacyTransferredAssets prunes legacy setup-managed installs for assets that
// moved out of the bundled core catalog. This prevents stale copies from remaining
// active after an ownership transfer to extensions.
func CleanupLegacyTransferredAssets(cfg LegacyAssetCleanupConfig) (LegacyAssetCleanupResult, error) {
	env, err := resolveEnvironment(cfg.ResolverOptions)
	if err != nil {
		return LegacyAssetCleanupResult{}, err
	}

	agents, err := SupportedAgents(cfg.ResolverOptions)
	if err != nil {
		return LegacyAssetCleanupResult{}, err
	}

	removals, err := legacyTransferredAssetRemovals(env, agents, cfg.Global)
	if err != nil {
		return LegacyAssetCleanupResult{}, err
	}

	result := LegacyAssetCleanupResult{
		Removed: make([]LegacyAssetRemoval, 0, len(removals)),
	}
	for i := range removals {
		removal := removals[i]
		if !pathExists(removal.Path) {
			continue
		}
		if err := os.RemoveAll(removal.Path); err != nil {
			return result, fmt.Errorf(
				"remove legacy %s %q at %s: %w",
				removal.Kind,
				removal.Name,
				removal.Path,
				err,
			)
		}
		result.Removed = append(result.Removed, removal)
	}

	return result, nil
}

func legacyTransferredAssetRemovals(
	env resolvedEnvironment,
	agents []Agent,
	global bool,
) ([]LegacyAssetRemoval, error) {
	seen := make(map[string]struct{})
	removals := make([]LegacyAssetRemoval, 0, len(legacyTransferredSkillNames)*len(agents)*2+
		len(legacyTransferredReusableAgentNames))

	for i := range legacyTransferredSkillNames {
		name := legacyTransferredSkillNames[i]
		skill := Skill{Name: name}
		for j := range agents {
			canonicalPath, targetPath, err := resolveInstallPaths(skill, agents[j], env, global)
			if err != nil {
				return nil, fmt.Errorf(
					"resolve legacy skill cleanup paths for %q and agent %q: %w",
					name,
					agents[j].Name,
					err,
				)
			}
			removals = appendLegacyAssetRemoval(removals, seen, LegacyAssetRemoval{
				Kind: CatalogAssetKindSkill,
				Name: name,
				Path: canonicalPath,
			})
			removals = appendLegacyAssetRemoval(removals, seen, LegacyAssetRemoval{
				Kind: CatalogAssetKindSkill,
				Name: name,
				Path: targetPath,
			})
		}
	}

	reusableAgentRoot := reusableAgentsInstallRoot(env, global)
	for i := range legacyTransferredReusableAgentNames {
		name := legacyTransferredReusableAgentNames[i]
		targetPath := filepath.Join(reusableAgentRoot, name)
		if !isPathSafe(reusableAgentRoot, targetPath) {
			return nil, fmt.Errorf(
				"resolve legacy reusable-agent cleanup path for %q: path escaped base directory",
				name,
			)
		}
		removals = appendLegacyAssetRemoval(removals, seen, LegacyAssetRemoval{
			Kind: CatalogAssetKindReusableAgent,
			Name: name,
			Path: targetPath,
		})
	}

	return removals, nil
}

func appendLegacyAssetRemoval(
	removals []LegacyAssetRemoval,
	seen map[string]struct{},
	removal LegacyAssetRemoval,
) []LegacyAssetRemoval {
	cleanPath := filepath.Clean(removal.Path)
	if cleanPath == "" {
		return removals
	}
	if _, ok := seen[cleanPath]; ok {
		return removals
	}
	seen[cleanPath] = struct{}{}
	removal.Path = cleanPath
	return append(removals, removal)
}
