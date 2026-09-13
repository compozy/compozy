package extensionpkg

import (
	"context"
	"errors"
	"fmt"
	"strings"

	compozyconfig "github.com/compozy/compozy/internal/config"
	diagnosticcontract "github.com/compozy/compozy/internal/diagnosticcontract"
	diagnosticspkg "github.com/compozy/compozy/internal/diagnostics"
	marketplacepkg "github.com/compozy/compozy/internal/marketplace"
	registrypkg "github.com/compozy/compozy/internal/registry"
)

// UpdateMarketplaceManaged updates marketplace extensions with rollback on reload failure.
func UpdateMarketplaceManaged(
	ctx context.Context,
	homePaths compozyconfig.HomePaths,
	registry LifecycleRegistry,
	loader MarketplaceSourceLoader,
	req MarketplaceUpdateRequest,
	reload MutationReload,
) ([]MarketplaceUpdateResult, error) {
	targets, err := SelectMarketplaceUpdateTargets(registry, req.Names, req.All)
	if err != nil {
		return nil, err
	}

	items := make([]MarketplaceUpdateResult, 0, len(targets))
	for infoIndex := range targets {
		item, err := updateMarketplaceExtension(ctx, homePaths, registry, loader, &targets[infoIndex], req, reload)
		if err != nil {
			item = failedMarketplaceUpdateResult(&targets[infoIndex], item, err)
			items = append(items, item)
			return items, newMarketplaceUpdateBatchError(targets[infoIndex].Name, items, err)
		}
		items = append(items, item)
	}
	return items, nil
}

func failedMarketplaceUpdateResult(
	info *ExtensionInfo,
	item MarketplaceUpdateResult,
	cause error,
) MarketplaceUpdateResult {
	if strings.TrimSpace(item.Name) == "" {
		item.Name = strings.TrimSpace(info.Name)
	}
	if strings.TrimSpace(item.Slug) == "" {
		item.Slug = dereferenceOptionalString(info.RegistrySlug)
	}
	if strings.TrimSpace(item.Registry) == "" {
		item.Registry = dereferenceOptionalString(info.RegistryName)
	}
	if strings.TrimSpace(item.CurrentVersion) == "" {
		item.CurrentVersion = firstNonEmpty(dereferenceOptionalString(info.RemoteVersion), info.Version)
	}
	if strings.TrimSpace(item.LatestVersion) == "" {
		item.LatestVersion = item.CurrentVersion
	}
	if strings.TrimSpace(item.Path) == "" {
		if manifestPath := strings.TrimSpace(info.ManifestPath); manifestPath != "" {
			item.Path = PackageRootFromManifest(manifestPath)
		}
	}
	item.Status = MarketplaceUpdateStatusFailed
	item.Error = marketplaceUpdateFailureDiagnostic(item.Name, cause)
	return item
}

func marketplaceUpdateFailureDiagnostic(name string, cause error) *diagnosticcontract.DiagnosticItem {
	message := fmt.Sprintf("Extension %q could not be updated.", strings.TrimSpace(name))
	if errors.Is(cause, context.Canceled) || errors.Is(cause, context.DeadlineExceeded) {
		message = fmt.Sprintf("Extension %q update was interrupted.", strings.TrimSpace(name))
	}
	item := diagnosticspkg.NewItem(diagnosticspkg.ItemSpec{
		ID:            "extension-update-failed:" + strings.TrimSpace(name),
		Code:          diagnosticcontract.CodeExtensionUpdateFailed,
		Category:      diagnosticcontract.CategoryExtension,
		Title:         "Extension update failed",
		Message:       message,
		Severity:      diagnosticcontract.SeverityError,
		DataFreshness: diagnosticcontract.FreshnessLive,
	},
		diagnosticspkg.WithSuggestedCommand("compozy extension status "+strings.TrimSpace(name)),
	)
	return &item
}

func updateMarketplaceExtension(
	ctx context.Context,
	homePaths compozyconfig.HomePaths,
	registry LifecycleRegistry,
	loader MarketplaceSourceLoader,
	info *ExtensionInfo,
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

func marketplaceUpdateMetadata(info *ExtensionInfo) (string, string, error) {
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
	info *ExtensionInfo,
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
	homePaths compozyconfig.HomePaths,
	registry LifecycleRegistry,
	info *ExtensionInfo,
	req MarketplaceUpdateRequest,
	resolution marketplaceUpdateResolution,
	reload MutationReload,
) (out marketplaceUpdateApplyResult, err error) {
	installDir, err := InstalledExtensionDir(info)
	if err != nil {
		return marketplaceUpdateApplyResult{}, err
	}
	stagingDir, err := NewManagedInstallStagingDir(homePaths)
	if err != nil {
		return marketplaceUpdateApplyResult{}, err
	}
	cleanup := marketplaceUpdateCleanupForRequest(req)
	defer finalizeMarketplaceUpdateStagingCleanup(&out, &err, cleanup, info.Name, stagingDir)
	result, err := installMarketplaceUpdateArchive(ctx, resolution, stagingDir, req.ObserveDigestVerification)
	if err != nil {
		return marketplaceUpdateApplyResult{}, err
	}
	manifest, err := loadMarketplaceUpdatedExtensionManifest(result.InstallPath, info.Name)
	if err != nil {
		return marketplaceUpdateApplyResult{}, err
	}
	return applyMarketplaceUpdateCandidate(ctx, &marketplaceUpdateCommitInput{
		registry: registry, info: *info, installDir: installDir, result: result, manifest: manifest,
		slug: resolution.slug, registryName: resolution.registryName, latestVersion: resolution.latestVersion,
		provenance:      marketplaceUpdateProvenance(info, result, manifest, req, resolution),
		commitCandidate: req.CommitCandidate, rollbackCandidate: req.RollbackCandidate, reload: reload,
	}, req.PreflightCandidate, cleanup)
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
	resolution marketplaceUpdateResolution,
	stagingDir string,
	observeDigestVerification MarketplaceDigestVerificationObserver,
) (*registrypkg.InstallResult, error) {
	result, err := registrypkg.NewInstaller(resolution.downloader).
		Install(ctx, resolution.slug, registrypkg.DownloadOpts{
			Version: strings.TrimSpace(resolution.latestVersion), ExpectedSHA256: resolution.expectedDigest,
		}, stagingDir)
	if err != nil {
		err = wrapCuratedDigestMismatch(err, resolution.trust)
	}
	if observeDigestVerification != nil {
		observeDigestVerification(resolution.trust, err)
	}
	return result, err
}

func marketplaceUpdateProvenance(
	info *ExtensionInfo,
	result *registrypkg.InstallResult,
	manifest *Manifest,
	req MarketplaceUpdateRequest,
	resolution marketplaceUpdateResolution,
) ExtensionProvenance {
	trust := resolution.trust
	provenance := info.Provenance
	provenance.Slug = resolution.slug
	provenance.ChecksumSHA256 = result.Checksum
	provenance.Permissions = extensionPermissions(manifest)
	provenance.InstalledBy = firstNonEmpty(req.InstalledBy, provenance.InstalledBy, extensionTrustInstalledByOperator)
	if trust != nil {
		registryTier := normalizedMarketplaceRegistryTier(trust.RegistryTier)
		provenance.CatalogEntryID = strings.TrimSpace(trust.CatalogEntryID)
		provenance.SourceName = marketplacepkg.CompozyCatalogSource
		provenance.SourceRef = marketplacepkg.CompozyCatalogRef
		provenance.EntryID = strings.TrimSpace(trust.CatalogEntryID)
		provenance.SourceURL = firstNonEmpty(curatedMarketplaceSourceURL(trust), provenance.SourceURL)
		provenance.ArchiveDigestSHA256 = result.ArchiveDigestSHA256
		provenance.DigestMatched = result.DigestMatched
		provenance.ChecksumVerified = true
		provenance.RegistryTier = registryTier
		provenance.AllowUnverified = registryTier == ExtensionRegistryTierUnverified && req.AllowUnverified
		provenance.Warnings = marketplaceTrustWarnings(trust)
		return provenance
	}
	provenance.CatalogEntryID = ""
	provenance.SourceName = ""
	provenance.SourceRef = ""
	provenance.EntryID = ""
	provenance.ResolvedRef = ""
	provenance.Layout = manifest.Layout
	if plugin := resolution.plugin; plugin != nil {
		provenance.SourceName = plugin.SourceName
		provenance.SourceRef = plugin.Record.SourceRef
		provenance.EntryID = plugin.Record.EntryID
		provenance.ResolvedRef = plugin.Record.ResolvedRef
		provenance.Layout = plugin.Record.Layout
		provenance.SourceURL = plugin.Record.SourceRef
	}
	provenance.ArchiveDigestSHA256 = result.ArchiveDigestSHA256
	provenance.DigestMatched = result.DigestMatched
	provenance.ChecksumVerified = false
	provenance.RegistryTier = ExtensionRegistryTierUnverified
	provenance.AllowUnverified = req.AllowUnverified
	provenance.Warnings = []diagnosticcontract.DiagnosticItem{
		extensionChecksumUnverifiedDiagnostic(provenance.Slug, resolution.registryName, req.AllowUnverified),
	}
	return provenance
}

func loadMarketplaceUpdatedExtensionManifest(installPath string, installedName string) (*Manifest, error) {
	manifest, err := LoadManifest(installPath)
	if err != nil {
		return nil, fmt.Errorf("extension: load updated extension manifest for %q: %w", installedName, err)
	}
	if manifest.Name != installedName {
		return nil, &ManifestValidationError{
			Field:   manifestNameKey,
			Value:   manifest.Name,
			Message: fmt.Sprintf("extension update identity mismatch: installed %q", installedName),
		}
	}
	return manifest, nil
}

// SelectMarketplaceUpdateTargets resolves the complete managed update selection before acquisition.
func SelectMarketplaceUpdateTargets(
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
		for infoIndex := range infos {
			if marketplaceExtensionInstalled(&infos[infoIndex]) {
				items = append(items, infos[infoIndex])
			}
		}
		return items, nil
	}

	if len(names) == 0 {
		return nil, errors.New("extension: extension name is required unless all is set")
	}
	items := make([]ExtensionInfo, 0, len(names))
	seen := make(map[string]struct{}, len(names))
	for _, name := range names {
		name = strings.TrimSpace(name)
		if name == "" {
			return nil, errors.New("extension: extension name must not be blank")
		}
		if _, found := seen[name]; found {
			continue
		}
		info, err := registry.Get(name)
		if err != nil {
			return nil, err
		}
		if !marketplaceExtensionInstalled(info) {
			return nil, fmt.Errorf("extension: extension %q is not a marketplace-installed extension", info.Name)
		}
		seen[name] = struct{}{}
		items = append(items, *info)
	}
	return items, nil
}

func marketplaceExtensionInstalled(info *ExtensionInfo) bool {
	return info.Source == SourceMarketplace && dereferenceOptionalString(info.RegistrySlug) != ""
}
