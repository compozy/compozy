package extensionpkg

import (
	"context"
	"errors"
	"fmt"
	"strings"

	aghconfig "github.com/compozy/agh/internal/config"
	diagnosticcontract "github.com/compozy/agh/internal/diagnosticcontract"
	registrypkg "github.com/compozy/agh/internal/registry"
)

// UpdateMarketplaceManaged updates marketplace extensions with rollback on reload failure.
func UpdateMarketplaceManaged(
	ctx context.Context,
	homePaths aghconfig.HomePaths,
	registry LifecycleRegistry,
	loader MarketplaceSourceLoader,
	req MarketplaceUpdateRequest,
	reload MutationReload,
) ([]MarketplaceUpdateResult, error) {
	targets, err := selectMarketplaceExtensionsForUpdate(registry, req.Names, req.All)
	if err != nil {
		return nil, err
	}

	items := make([]MarketplaceUpdateResult, 0, len(targets))
	for _, info := range targets {
		item, err := updateMarketplaceExtension(ctx, homePaths, registry, loader, info, req, reload)
		if err != nil {
			if item.Status != "" {
				items = append(items, item)
			}
			if len(items) == 0 {
				return nil, err
			}
			return items, newMarketplaceUpdateBatchError(info.Name, items, err)
		}
		items = append(items, item)
	}
	return items, nil
}

func updateMarketplaceExtension(
	ctx context.Context,
	homePaths aghconfig.HomePaths,
	registry LifecycleRegistry,
	loader MarketplaceSourceLoader,
	info ExtensionInfo,
	req MarketplaceUpdateRequest,
	reload MutationReload,
) (_ MarketplaceUpdateResult, err error) {
	slug, registryName, err := marketplaceUpdateMetadata(info)
	if err != nil {
		return MarketplaceUpdateResult{}, err
	}
	currentVersion := firstNonEmpty(dereferenceOptionalString(info.RemoteVersion), info.Version)
	resolution, err := resolveMarketplaceUpdate(
		ctx,
		loader,
		info,
		slug,
		registryName,
		currentVersion,
		req,
	)
	if err != nil {
		return MarketplaceUpdateResult{}, err
	}
	if resolution.closeDownloader != nil {
		defer func() {
			err = errors.Join(err, resolution.closeDownloader())
		}()
	}

	installDir, err := InstalledExtensionDir(info)
	if err != nil {
		return MarketplaceUpdateResult{}, err
	}
	item := newMarketplaceUpdateResult(
		info,
		slug,
		resolution.registryName,
		currentVersion,
		resolution.latestVersion,
		installDir,
	)
	if !resolution.hasUpdate {
		item.Status = MarketplaceUpdateStatusCurrent
		return item, nil
	}
	if req.CheckOnly {
		item.Status = MarketplaceUpdateStatusAvailable
		return item, nil
	}

	applied, err := applyResolvedMarketplaceUpdate(
		ctx,
		homePaths,
		registry,
		info,
		req,
		reload,
		resolution,
	)
	if err != nil {
		return MarketplaceUpdateResult{}, err
	}
	item.LatestVersion = applied.remoteVersion
	item.Status = MarketplaceUpdateStatusUpdated
	item.Warnings = append([]diagnosticcontract.DiagnosticItem(nil), applied.warnings...)
	return item, nil
}

func resolveMarketplaceUpdateTrust(
	ctx context.Context,
	resolver MarketplaceTrustResolver,
	slug string,
	version string,
) (*MarketplaceTrustEvidence, error) {
	if resolver == nil {
		return nil, nil
	}
	return resolver(ctx, slug, version)
}

func marketplaceUpdateMetadata(info ExtensionInfo) (string, string, error) {
	slug := dereferenceOptionalString(info.RegistrySlug)
	if slug == "" {
		return "", "", fmt.Errorf("extension: extension %q is missing registry slug metadata", info.Name)
	}
	registryName := dereferenceOptionalString(info.RegistryName)
	if registryName == "" {
		return "", "", fmt.Errorf("extension: extension %q is missing registry source metadata", info.Name)
	}
	return slug, registryName, nil
}

func newMarketplaceUpdateResult(
	info ExtensionInfo,
	slug string,
	registryName string,
	currentVersion string,
	latestVersion string,
	installDir string,
) MarketplaceUpdateResult {
	return MarketplaceUpdateResult{
		Name:           info.Name,
		Slug:           slug,
		Registry:       registryName,
		CurrentVersion: currentVersion,
		LatestVersion:  firstNonEmpty(latestVersion, currentVersion),
		Path:           installDir,
	}
}

func applyMarketplaceExtensionUpdate(
	ctx context.Context,
	homePaths aghconfig.HomePaths,
	registry LifecycleRegistry,
	downloader registrypkg.Downloader,
	info ExtensionInfo,
	latestVersion string,
	registryName string,
	allowUnverified bool,
	installedBy string,
	trust *MarketplaceTrustEvidence,
	observeDigestVerification MarketplaceDigestVerificationObserver,
	reload MutationReload,
	cleanup marketplaceUpdateCleanup,
) (out marketplaceUpdateApplyResult, err error) {
	slug := dereferenceOptionalString(info.RegistrySlug)
	installDir, err := InstalledExtensionDir(info)
	if err != nil {
		return marketplaceUpdateApplyResult{}, err
	}

	stagingDir, err := NewManagedInstallStagingDir(homePaths)
	if err != nil {
		return marketplaceUpdateApplyResult{}, err
	}
	defer finalizeMarketplaceUpdateStagingCleanup(&out, &err, cleanup, info.Name, stagingDir)

	result, err := installMarketplaceUpdateArchive(
		ctx,
		downloader,
		slug,
		latestVersion,
		stagingDir,
		trust,
		observeDigestVerification,
	)
	if err != nil {
		return marketplaceUpdateApplyResult{}, err
	}

	manifest, err := loadMarketplaceUpdatedExtensionManifest(result.InstallPath, info.Name)
	if err != nil {
		return marketplaceUpdateApplyResult{}, err
	}
	change, err := stageExtensionDirReplacement(result.InstallPath, installDir)
	if err != nil {
		return marketplaceUpdateApplyResult{}, err
	}

	remoteVersion := firstNonEmpty(result.Version, latestVersion, manifest.Version)
	provenance := marketplaceUpdateProvenance(info, result, manifest, registryName, allowUnverified, installedBy, trust)
	if err := installMarketplaceExtensionUpdateRecord(
		registry,
		manifest,
		installDir,
		result.Checksum,
		slug,
		registryName,
		remoteVersion,
		provenance,
	); err != nil {
		return marketplaceUpdateApplyResult{}, errors.Join(err, change.Rollback())
	}
	if err := reloadMarketplaceExtensionUpdate(ctx, reload, registry, info, installDir, change); err != nil {
		return marketplaceUpdateApplyResult{}, err
	}
	out = marketplaceUpdateApplyResult{remoteVersion: remoteVersion, committed: true}
	if cleanupErr := cleanup.commitChange(change); cleanupErr != nil {
		out.warnings = append(out.warnings, marketplaceUpdateCleanupWarning(
			info.Name,
			marketplaceUpdateCleanupBackup,
			change.backupDir,
			cleanupErr,
		))
	}
	return out, nil
}

func reloadMarketplaceExtensionUpdate(
	ctx context.Context,
	reload MutationReload,
	registry LifecycleRegistry,
	info ExtensionInfo,
	installDir string,
	change *stagedExtensionDirChange,
) error {
	if reload == nil {
		return nil
	}
	if err := reload(ctx); err != nil {
		restoreErr := restoreUpdatedExtensionRecord(registry, info, installDir, change)
		if restoreErr == nil {
			restoreErr = reload(ctx)
		}
		return errors.Join(
			fmt.Errorf("extension: reload after update %q: %w", info.Name, err),
			restoreErr,
		)
	}
	return nil
}

func installMarketplaceExtensionUpdateRecord(
	registry LifecycleRegistry,
	manifest *Manifest,
	installDir string,
	checksum string,
	slug string,
	registryName string,
	remoteVersion string,
	provenance ExtensionProvenance,
) error {
	return registry.Install(
		manifest,
		installDir,
		checksum,
		WithInstallSource(SourceMarketplace),
		WithInstallRegistryMetadata(slug, registryName, remoteVersion),
		WithInstallProvenance(provenance),
		WithInstallReplaceExisting(),
	)
}

func installMarketplaceUpdateArchive(
	ctx context.Context,
	downloader registrypkg.Downloader,
	slug string,
	latestVersion string,
	stagingDir string,
	trust *MarketplaceTrustEvidence,
	observeDigestVerification MarketplaceDigestVerificationObserver,
) (*registrypkg.InstallResult, error) {
	version := strings.TrimSpace(latestVersion)
	expectedDigest := ""
	if trust != nil {
		version = strings.TrimSpace(trust.Version)
		expectedDigest = strings.TrimSpace(trust.ArchiveDigestSHA256)
	}
	result, err := registrypkg.NewInstaller(downloader).Install(ctx, slug, registrypkg.DownloadOpts{
		Version:        version,
		ExpectedSHA256: expectedDigest,
	}, stagingDir)
	if err != nil {
		err = wrapCuratedDigestMismatch(err, trust)
	}
	if observeDigestVerification != nil {
		observeDigestVerification(trust, err)
	}
	return result, err
}

func marketplaceUpdateProvenance(
	info ExtensionInfo,
	result *registrypkg.InstallResult,
	manifest *Manifest,
	registryName string,
	allowUnverified bool,
	installedBy string,
	trust *MarketplaceTrustEvidence,
) ExtensionProvenance {
	provenance := info.Provenance
	provenance.Slug = dereferenceOptionalString(info.RegistrySlug)
	provenance.InstalledFrom = ExtensionInstalledFromMarketplace
	provenance.ChecksumSHA256 = result.Checksum
	provenance.Permissions = extensionPermissions(manifest)
	provenance.InstalledBy = firstNonEmpty(installedBy, provenance.InstalledBy, extensionTrustInstalledByOperator)
	if trust != nil {
		registryTier := normalizedMarketplaceRegistryTier(trust.RegistryTier)
		provenance.CatalogEntryID = strings.TrimSpace(trust.CatalogEntryID)
		provenance.SourceURL = firstNonEmpty(curatedMarketplaceSourceURL(trust), provenance.SourceURL)
		provenance.ArchiveDigestSHA256 = result.ArchiveDigestSHA256
		provenance.ChecksumVerified = true
		provenance.RegistryTier = registryTier
		provenance.AllowUnverified = registryTier == ExtensionRegistryTierUnverified && allowUnverified
		provenance.Warnings = marketplaceTrustWarnings(trust)
		return provenance
	}
	provenance.CatalogEntryID = ""
	provenance.ArchiveDigestSHA256 = ""
	provenance.ChecksumVerified = false
	provenance.RegistryTier = registryTierForSource(SourceMarketplace, registryName)
	provenance.AllowUnverified = allowUnverified
	provenance.Warnings = []diagnosticcontract.DiagnosticItem{
		extensionChecksumUnverifiedDiagnostic(provenance.Slug, registryName, allowUnverified),
	}
	return provenance
}

func loadMarketplaceUpdatedExtensionManifest(installPath string, installedName string) (*Manifest, error) {
	manifest, err := LoadManifest(installPath)
	if err != nil {
		return nil, fmt.Errorf("extension: load updated extension manifest for %q: %w", installedName, err)
	}
	if manifest.Name != installedName {
		return nil, fmt.Errorf(
			"extension: extension update identity mismatch: installed %q, registry returned %q",
			installedName,
			manifest.Name,
		)
	}
	return manifest, nil
}

func selectMarketplaceExtensionsForUpdate(
	registry LifecycleRegistry,
	names []string,
	updateAll bool,
) ([]ExtensionInfo, error) {
	if registry == nil {
		return nil, errors.New("extension: registry is required")
	}
	if updateAll {
		infos, err := registry.List()
		if err != nil {
			return nil, err
		}
		items := make([]ExtensionInfo, 0, len(infos))
		for _, info := range infos {
			if marketplaceExtensionInstalled(info) {
				items = append(items, info)
			}
		}
		return items, nil
	}

	name := ""
	if len(names) > 0 {
		name = strings.TrimSpace(names[0])
	}
	if name == "" {
		return nil, errors.New("extension: extension name is required unless all is set")
	}
	info, err := registry.Get(name)
	if err != nil {
		return nil, err
	}
	if !marketplaceExtensionInstalled(*info) {
		return nil, fmt.Errorf("extension: extension %q is not a marketplace-installed extension", info.Name)
	}
	return []ExtensionInfo{*info}, nil
}

func marketplaceExtensionInstalled(info ExtensionInfo) bool {
	return info.Source == SourceMarketplace && dereferenceOptionalString(info.RegistrySlug) != ""
}
