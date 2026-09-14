package extensionpkg

import (
	"context"
	"errors"
	"path/filepath"

	"github.com/compozy/compozy/internal/diagnosticcontract"
	marketplacepkg "github.com/compozy/compozy/internal/marketplace"
	registrypkg "github.com/compozy/compozy/internal/registry"
)

// Origin returns acquisition identity independently of the source display name.
func (p *PreparedMarketplaceManagedInstall) Origin() marketplacepkg.Origin {
	provenance := marketplaceInstallProvenance(p.install, p.request)
	return marketplacepkg.Origin{SourceRef: provenance.SourceRef, EntryID: provenance.EntryID}
}

// ValidateReinstall refuses another classified origin and packages outside the managed root.
// Associating an unclassified package with a listing requires operator authorization at the caller.
func (p *PreparedMarketplaceManagedInstall) ValidateReinstall(info *ExtensionInfo) error {
	origin := marketplacepkg.Origin{SourceRef: info.Provenance.SourceRef, EntryID: info.Provenance.EntryID}
	if origin != (marketplacepkg.Origin{}) && origin != p.Origin() {
		return &ExtensionNameConflictError{
			Name:            info.Name,
			InstalledOrigin: origin,
			SourceName:      info.Provenance.SourceName,
		}
	}
	if info.Name != p.Name() || info.Source == SourceBundled {
		return &ExtensionExistsError{Name: info.Name}
	}
	if origin == (marketplacepkg.Origin{}) && p.Origin() == (marketplacepkg.Origin{}) &&
		(dereferenceOptionalString(info.RegistrySlug) != p.install.slug ||
			dereferenceOptionalString(info.RegistryName) != p.install.detail.Source) {
		return &ExtensionExistsError{Name: info.Name}
	}
	installedDir, err := InstalledExtensionDir(info)
	if err != nil {
		return err
	}
	if filepath.Clean(installedDir) != filepath.Clean(p.install.finalDir) {
		return &ExtensionExistsError{Name: info.Name}
	}
	return nil
}

// Reinstall reuses the staged acquisition and update transaction, preserving existing attachments.
// The caller holds the package lifecycle lock and authorizes unclassified-origin association.
func (p *PreparedMarketplaceManagedInstall) Reinstall(
	ctx context.Context, info *ExtensionInfo, preflight MarketplaceUpdatePreflight,
	commit MarketplaceUpdateCommit, rollback MarketplaceUpdateRollback, reload, complete MutationReload,
) ([]diagnosticcontract.DiagnosticItem, error) {
	if p.committed {
		return nil, errors.New("extension: prepared marketplace install is already committed")
	}
	if err := p.ValidateReinstall(info); err != nil {
		return nil, err
	}
	result := &registrypkg.InstallResult{
		InstallPath: p.install.installPath, Checksum: p.install.checksum,
		ArchiveDigestSHA256: p.install.archiveDigest, DigestMatched: p.install.digestMatched,
		Version: p.install.remoteVersion,
	}
	out, err := applyMarketplaceUpdateCandidate(ctx, &marketplaceUpdateCommitInput{
		registry: p.registry, info: *info, installDir: p.install.finalDir,
		result: result, manifest: p.install.manifest, slug: p.install.slug,
		registryName: p.install.detail.Source, latestVersion: p.install.remoteVersion,
		provenance:      marketplaceInstallProvenance(p.install, p.request),
		commitCandidate: commit, rollbackCandidate: rollback, reload: reload, afterReload: complete,
	}, preflight, marketplaceUpdateCleanupForRequest(MarketplaceUpdateRequest{}))
	if err != nil {
		return nil, err
	}
	p.committed = out.committed
	return out.warnings, nil
}

// MatchesInstalled identifies a retry of the same acquisition; callers still validate input readiness.
func (p *PreparedMarketplaceManagedInstall) MatchesInstalled(info *ExtensionInfo) bool {
	return info.Checksum == p.install.checksum && info.Version == p.install.manifest.Version &&
		info.Provenance.SourceRef == p.Origin().SourceRef && info.Provenance.EntryID == p.Origin().EntryID &&
		info.Provenance.ArchiveDigestSHA256 == p.install.archiveDigest
}
