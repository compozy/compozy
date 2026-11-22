package daemon

import (
	"cmp"
	"context"
	"slices"
	"time"

	"github.com/compozy/compozy/internal/api/contract"
	extensionpkg "github.com/compozy/compozy/internal/extension"
)

// dependencyUpdateCheckTimeout bounds the check-only marketplace lookup behind an `extension update` hint, so a
// slow source only drops the command instead of delaying the refusal.
const dependencyUpdateCheckTimeout = 5 * time.Second

// dependencyRetryCommand names the command that unblocks a version_unmet or installed_elsewhere dependency, or ""
// when no command can (rule 12, D9). It is `compozy extension update` only when the installed package is
// marketplace-installed and its source's release reaches the requirement; otherwise an installed_elsewhere package
// is reinstalled from the dependency's own source when that source satisfies it. cell is the dependent's cell, and
// the flags select the installation visible there (D11), which may be global; nil selects the package's first
// installation in a deterministic order. A failed lookup only drops the command.
func (s *daemonExtensionService) dependencyRetryCommand(
	ctx context.Context, item contract.ExtensionDependencyPayload, cell *extensionpkg.InstallationScope,
) string {
	elsewhere := item.State == dependencyStateInstalledElsewhere
	if !elsewhere && (item.State != dependencyStateVersionUnmet || item.InstalledVersion == "") {
		return ""
	}
	cell = s.dependencyBlockingCell(ctx, item.Name, cell)
	if cell == nil {
		return ""
	}
	flags, ok := s.dependencyCellFlags(ctx, item.Name, *cell)
	if !ok {
		return ""
	}
	if s.dependencyUpdateSatisfies(ctx, item) {
		return "compozy extension update " + item.Name + flags
	}
	if !elsewhere || !extensionpkg.VersionSatisfies(item.Version, item.MinVersion) {
		return ""
	}
	command := "compozy extension install " + dependencyInstallArgument(item)
	if contract.InstallExtensionSource(item.Source) == contract.InstallExtensionSourceLocalPath {
		command += " --replace"
	}
	return command + flags
}

// dependencyUpdateSatisfies reports whether `extension update` can reach the requirement: the package is
// marketplace-installed and its source's release satisfies min_version and, for a candidate, is its version.
func (s *daemonExtensionService) dependencyUpdateSatisfies(
	ctx context.Context, item contract.ExtensionDependencyPayload,
) bool {
	info, err := s.registry.Get(item.Name)
	if err != nil || info.Source != extensionpkg.SourceMarketplace || info.RegistrySlug == nil ||
		*info.RegistrySlug == "" {
		return false
	}
	timeout := cmp.Or(s.dependencyUpdateHintTimeout, dependencyUpdateCheckTimeout)
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	if s.marketplaceCache != nil {
		release, err := s.marketplaceCache.Hold(ctx)
		if err != nil {
			s.logger.Warn("extension dependency update check", "dependency", item.Name, "error", err)
			return false
		}
		defer release()
	}
	cfg := s.marketplaceConfig()
	items, err := extensionpkg.UpdateMarketplaceManaged(ctx, s.homePaths, s.registry, s.marketplaceSourceLoader(),
		extensionpkg.MarketplaceUpdateRequest{
			Names: []string{item.Name}, CheckOnly: true, PolicyAllowsUnverified: cfg.Trust.AllowUnverified,
			ResolveTrust: s.marketplaceTrustResolver(), ResolvePlugin: s.resolveMarketplacePluginOrigin,
		}, nil)
	if err != nil || len(items) != 1 {
		s.logger.Warn("extension dependency update check", "dependency", item.Name, "error", err)
		return false
	}
	latest := items[0].LatestVersion
	return extensionpkg.VersionSatisfies(latest, item.MinVersion) && (item.Version == "" || latest == item.Version)
}

// dependencyBlockingCell is the installation the command must select: the one the dependent's cell resolves
// (a global installation is visible from a workspace), or the package's first installation without a cell.
func (s *daemonExtensionService) dependencyBlockingCell(
	ctx context.Context, name string, cell *extensionpkg.InstallationScope,
) *extensionpkg.InstallationScope {
	if cell == nil {
		return s.dependencyInstallationCell(ctx, name)
	}
	installation, err := s.registry.ResolveInstallation(ctx, name, *cell)
	if err != nil {
		s.logger.Warn("extension dependency retry installation lookup", "dependency", name, "error", err)
		return nil
	}
	return &installation.Scope
}

// dependencyInstallationCell picks the package's first installation, global before workspaces, then by profile.
func (s *daemonExtensionService) dependencyInstallationCell(
	ctx context.Context, name string,
) *extensionpkg.InstallationScope {
	installations, err := s.registry.Installations(ctx, name)
	if err != nil || len(installations) == 0 {
		return nil
	}
	first := slices.MinFunc(installations, func(left, right extensionpkg.Installation) int {
		return cmp.Or(cmp.Compare(left.Scope.WorkspaceID, right.Scope.WorkspaceID),
			cmp.Compare(left.Scope.ProfileID, right.Scope.ProfileID))
	})
	return &first.Scope
}

// dependencyCellFlags renders the scope and profile flags that select one installation cell.
func (s *daemonExtensionService) dependencyCellFlags(
	ctx context.Context, name string, cell extensionpkg.InstallationScope,
) (string, bool) {
	flags := " --scope global"
	if cell.WorkspaceID != "" {
		flags = " --workspace " + cell.WorkspaceID
	}
	profile, err := s.dependencyProfileName(ctx, cell.ProfileID)
	if err != nil {
		s.logger.Warn("extension dependency retry profile lookup", "dependency", name, "error", err)
		return "", false
	}
	if profile != "" {
		flags += " --profile " + profile
	}
	return flags, true
}

// dependencyInstallArgument renders the dependency source and ref as the `extension install` argument.
func dependencyInstallArgument(item contract.ExtensionDependencyPayload) string {
	switch contract.InstallExtensionSource(item.Source) {
	case contract.InstallExtensionSourceMarketplace:
		return "marketplace:" + item.Ref
	case contract.InstallExtensionSourceGitHub:
		return "github:" + item.Ref
	case contract.InstallExtensionSourceGit:
		return "git:" + item.Ref
	default:
		return item.Ref
	}
}
