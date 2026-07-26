package extensionpkg

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"path/filepath"
	"strings"

	aghconfig "github.com/compozy/agh/internal/config"
	diagnosticcontract "github.com/compozy/agh/internal/diagnosticcontract"
	registrypkg "github.com/compozy/agh/internal/registry"
)

const (
	// MarketplaceUpdateStatusCurrent reports that no remote update is available.
	MarketplaceUpdateStatusCurrent = "current"
	// MarketplaceUpdateStatusAvailable reports that a remote update exists but was not applied.
	MarketplaceUpdateStatusAvailable = "available"
	// MarketplaceUpdateStatusUpdated reports that a remote update was applied.
	MarketplaceUpdateStatusUpdated = "updated"
)

// LifecycleRegistry is the installed-extension persistence surface required by
// managed lifecycle helpers.
type LifecycleRegistry interface {
	Get(name string) (*ExtensionInfo, error)
	List() ([]ExtensionInfo, error)
	Install(manifest *Manifest, path string, checksum string, opts ...InstallOption) error
	Disable(name string) error
	Uninstall(name string) error
}

// MarketplaceSourceLoader resolves configured marketplace sources. The
// optional source filter is an already-normalized operator/tool input.
type MarketplaceSourceLoader func(context.Context) ([]registrypkg.Source, error)

// ErrMarketplaceSourceUnavailable reports that a marketplace source cannot be resolved or used.
var ErrMarketplaceSourceUnavailable = errors.New("extension: marketplace source unavailable")

// MutationReload is called after a registry/on-disk mutation and before the
// lifecycle helper commits any staged filesystem backup.
type MutationReload func(context.Context) error

// MarketplaceInstallRequest describes one marketplace-backed extension install.
type MarketplaceInstallRequest struct {
	Slug                      string
	SourceFilter              string
	Version                   string
	Asset                     string
	PolicyAllowsUnverified    bool
	AllowUnverified           bool
	InstalledBy               string
	Trust                     *MarketplaceTrustEvidence
	ArtifactHTTPClient        *http.Client
	ObserveDigestVerification MarketplaceDigestVerificationObserver
}

// ManagedRemoveResult describes one removed managed extension.
type ManagedRemoveResult struct {
	Name     string                              `json:"name"`
	Path     string                              `json:"path"`
	Status   string                              `json:"status"`
	Warnings []diagnosticcontract.DiagnosticItem `json:"warnings,omitempty"`
}

// MarketplaceUpdateRequest describes one marketplace update batch.
type MarketplaceUpdateRequest struct {
	Names                     []string
	All                       bool
	CheckOnly                 bool
	Version                   string
	PolicyAllowsUnverified    bool
	AllowUnverified           bool
	InstalledBy               string
	ResolveTrust              MarketplaceTrustResolver
	ArtifactHTTPClient        *http.Client
	ObserveDigestVerification MarketplaceDigestVerificationObserver
	commitChange              func(*stagedExtensionDirChange) error
	removeStaging             func(string) error
}

// MarketplaceUpdateResult describes one marketplace update outcome.
type MarketplaceUpdateResult struct {
	Name           string                              `json:"name"`
	Slug           string                              `json:"slug"`
	Registry       string                              `json:"registry"`
	CurrentVersion string                              `json:"current_version,omitempty"`
	LatestVersion  string                              `json:"latest_version,omitempty"`
	Path           string                              `json:"path"`
	Status         string                              `json:"status"`
	Warnings       []diagnosticcontract.DiagnosticItem `json:"warnings,omitempty"`
}

type stagedExtensionDirChange struct {
	targetDir    string
	backupDir    string
	removeBackup func(string) error
}

type marketplaceManagedInstall struct {
	slug          string
	detail        *registrypkg.Detail
	manifest      *Manifest
	finalDir      string
	checksum      string
	archiveDigest string
	remoteVersion string
	trust         *MarketplaceTrustEvidence
}

// InstallMarketplaceManaged installs one extension through the configured
// marketplace registry into the managed extension root and records marketplace
// provenance in the installed-extension registry.
func InstallMarketplaceManaged(
	ctx context.Context,
	homePaths aghconfig.HomePaths,
	registry LifecycleRegistry,
	loader MarketplaceSourceLoader,
	req MarketplaceInstallRequest,
) (_ *ExtensionInfo, err error) {
	if registry == nil {
		return nil, errors.New("extension: registry is required")
	}
	prepared, err := prepareMarketplaceManagedInstall(ctx, homePaths, registry, loader, req)
	if err != nil {
		return nil, err
	}
	provenance := marketplaceInstallProvenance(prepared, req)
	if err := registry.Install(
		prepared.manifest,
		prepared.finalDir,
		prepared.checksum,
		WithInstallSource(SourceMarketplace),
		WithInstallRegistryMetadata(prepared.slug, strings.TrimSpace(prepared.detail.Source), prepared.remoteVersion),
		WithInstallProvenance(provenance),
	); err != nil {
		return nil, errors.Join(err, removeExtensionDir(prepared.finalDir))
	}

	info, err := registry.Get(prepared.manifest.Name)
	if err != nil {
		return nil, err
	}
	return info, nil
}

func prepareMarketplaceManagedInstall(
	ctx context.Context,
	homePaths aghconfig.HomePaths,
	registry LifecycleRegistry,
	loader MarketplaceSourceLoader,
	req MarketplaceInstallRequest,
) (_ marketplaceManagedInstall, err error) {
	slug := strings.TrimSpace(req.Slug)
	if slug == "" {
		return marketplaceManagedInstall{}, errors.New("extension: marketplace slug is required")
	}
	if err := validateMarketplaceTrustGate(
		slug,
		req.SourceFilter,
		req.PolicyAllowsUnverified,
		req.AllowUnverified,
		req.Trust,
	); err != nil {
		return marketplaceManagedInstall{}, err
	}
	var downloader registrypkg.Downloader
	var closeDownloader func() error
	var detail *registrypkg.Detail
	if hasCuratedMarketplaceArtifact(req.Trust) {
		downloader, err = newCuratedMarketplaceArtifactDownloader(req.Trust, req.ArtifactHTTPClient)
		if err != nil {
			return marketplaceManagedInstall{}, err
		}
		detail = curatedMarketplaceArtifactDetail(slug, req.Trust)
	} else {
		multi, registryErr := newExtensionMarketplaceRegistry(ctx, loader, req.SourceFilter)
		if registryErr != nil {
			return marketplaceManagedInstall{}, registryErr
		}
		closeDownloader = multi.Close
		defer func() {
			if closeDownloader != nil {
				err = errors.Join(err, closeDownloader())
			}
		}()
		detail, err = multi.Info(ctx, slug)
		if err != nil {
			return marketplaceManagedInstall{}, err
		}
		downloader = multi
	}
	stagingDir, err := NewManagedInstallStagingDir(homePaths)
	if err != nil {
		return marketplaceManagedInstall{}, err
	}
	defer joinRemoveAll(&err, stagingDir, "extension: remove staged extension directory")

	result, err := installMarketplaceArchive(ctx, downloader, slug, req, stagingDir)
	if err != nil {
		return marketplaceManagedInstall{}, err
	}
	if closeDownloader != nil {
		closeSource := closeDownloader
		closeDownloader = nil
		if closeErr := closeSource(); closeErr != nil {
			return marketplaceManagedInstall{}, fmt.Errorf("extension: close marketplace registry source: %w", closeErr)
		}
	}
	manifest, finalDir, err := moveMarketplaceInstallIntoPlace(homePaths, registry, slug, result)
	if err != nil {
		return marketplaceManagedInstall{}, err
	}
	return marketplaceManagedInstall{
		slug:          slug,
		detail:        detail,
		manifest:      manifest,
		finalDir:      finalDir,
		checksum:      result.Checksum,
		archiveDigest: result.ArchiveDigestSHA256,
		remoteVersion: firstNonEmpty(result.Version, detail.Version, manifest.Version),
		trust:         req.Trust,
	}, nil
}

func newExtensionMarketplaceRegistry(
	ctx context.Context,
	loader MarketplaceSourceLoader,
	sourceFilter string,
) (*registrypkg.MultiRegistry, error) {
	sources, err := LoadMarketplaceSources(ctx, loader, sourceFilter)
	if err != nil {
		return nil, err
	}
	return registrypkg.NewMultiRegistry(slog.Default(), sources...), nil
}

func installMarketplaceArchive(
	ctx context.Context,
	downloader registrypkg.Downloader,
	slug string,
	req MarketplaceInstallRequest,
	stagingDir string,
) (*registrypkg.InstallResult, error) {
	version := strings.TrimSpace(req.Version)
	expectedDigest := ""
	if req.Trust != nil {
		version = strings.TrimSpace(req.Trust.Version)
		expectedDigest = strings.TrimSpace(req.Trust.ArchiveDigestSHA256)
	}
	result, err := registrypkg.NewInstaller(downloader).Install(ctx, slug, registrypkg.DownloadOpts{
		Version:        version,
		Asset:          strings.TrimSpace(req.Asset),
		ExpectedSHA256: expectedDigest,
	}, stagingDir)
	if err != nil {
		err = wrapCuratedDigestMismatch(err, req.Trust)
	}
	if req.ObserveDigestVerification != nil {
		req.ObserveDigestVerification(req.Trust, err)
	}
	return result, err
}

func moveMarketplaceInstallIntoPlace(
	homePaths aghconfig.HomePaths,
	registry LifecycleRegistry,
	slug string,
	result *registrypkg.InstallResult,
) (*Manifest, string, error) {
	manifest, err := LoadManifest(result.InstallPath)
	if err != nil {
		return nil, "", fmt.Errorf("extension: load installed extension manifest for %q: %w", slug, err)
	}
	if err := ensureExtensionNotInstalled(registry, manifest.Name); err != nil {
		return nil, "", err
	}
	finalDir, err := ManagedInstallPathChecked(homePaths, manifest.Name)
	if err != nil {
		return nil, "", err
	}
	if err := registrypkg.MoveInstalledDir(result.InstallPath, finalDir, false); err != nil {
		return nil, "", fmt.Errorf("extension: move %q into managed install path: %w", manifest.Name, err)
	}
	return manifest, finalDir, nil
}

func marketplaceInstallProvenance(
	prepared marketplaceManagedInstall,
	req MarketplaceInstallRequest,
) ExtensionProvenance {
	if prepared.trust != nil {
		registryTier := normalizedMarketplaceRegistryTier(prepared.trust.RegistryTier)
		allowUnverified := registryTier == ExtensionRegistryTierUnverified && req.AllowUnverified
		return ExtensionProvenance{
			Slug:           prepared.slug,
			CatalogEntryID: strings.TrimSpace(prepared.trust.CatalogEntryID),
			InstalledFrom:  ExtensionInstalledFromMarketplace,
			SourceURL: firstNonEmpty(
				curatedMarketplaceSourceURL(prepared.trust),
				strings.TrimSpace(prepared.detail.Repository),
			),
			ChecksumSHA256:      prepared.checksum,
			ArchiveDigestSHA256: prepared.archiveDigest,
			ChecksumVerified:    true,
			RegistryTier:        registryTier,
			Permissions:         extensionPermissions(prepared.manifest),
			InstalledBy:         firstNonEmpty(req.InstalledBy, extensionTrustInstalledByOperator),
			AllowUnverified:     allowUnverified,
			Warnings:            marketplaceTrustWarnings(prepared.trust),
		}
	}
	return ExtensionProvenance{
		Slug:             prepared.slug,
		InstalledFrom:    ExtensionInstalledFromMarketplace,
		SourceURL:        strings.TrimSpace(prepared.detail.Repository),
		ChecksumSHA256:   prepared.checksum,
		ChecksumVerified: false,
		RegistryTier:     registryTierForSource(SourceMarketplace, strings.TrimSpace(prepared.detail.Source)),
		Permissions:      extensionPermissions(prepared.manifest),
		InstalledBy:      firstNonEmpty(req.InstalledBy, extensionTrustInstalledByOperator),
		AllowUnverified:  req.AllowUnverified,
		Warnings: []diagnosticcontract.DiagnosticItem{
			extensionChecksumUnverifiedDiagnostic(prepared.slug, prepared.detail.Source, true),
		},
	}
}

// RemoveManagedExtension removes one installed extension and rolls back the
// registry and on-disk state if the caller's reload hook fails.
func RemoveManagedExtension(
	ctx context.Context,
	registry LifecycleRegistry,
	name string,
	reload MutationReload,
) (_ ManagedRemoveResult, err error) {
	if registry == nil {
		return ManagedRemoveResult{}, errors.New("extension: registry is required")
	}
	info, err := registry.Get(name)
	if err != nil {
		return ManagedRemoveResult{}, err
	}
	installDir, err := InstalledExtensionDir(*info)
	if err != nil {
		return ManagedRemoveResult{}, err
	}
	change, err := stageExtensionDirRemoval(installDir)
	if err != nil {
		return ManagedRemoveResult{}, err
	}

	if err := registry.Uninstall(info.Name); err != nil {
		return ManagedRemoveResult{}, errors.Join(err, change.Rollback())
	}
	if reload != nil {
		if err := reload(ctx); err != nil {
			restoreErr := restoreRemovedExtensionRecord(registry, *info, installDir, change)
			if restoreErr == nil {
				restoreErr = reload(ctx)
			}
			return ManagedRemoveResult{}, errors.Join(
				fmt.Errorf("extension: reload after remove %q: %w", info.Name, err),
				restoreErr,
			)
		}
	}
	return finalizeManagedExtensionRemoval(info.Name, installDir, change), nil
}

// InstalledExtensionDir returns the root directory for a persisted extension
// registry row after validating the manifest path shape.
func InstalledExtensionDir(info ExtensionInfo) (string, error) {
	manifestPath := filepath.Clean(strings.TrimSpace(info.ManifestPath))
	if manifestPath == "" || manifestPath == "." {
		return "", fmt.Errorf("extension: extension %q has an invalid manifest path %q", info.Name, info.ManifestPath)
	}
	if !filepath.IsAbs(manifestPath) {
		return "", fmt.Errorf(
			"extension: extension %q has a non-absolute manifest path %q",
			info.Name,
			info.ManifestPath,
		)
	}
	switch filepath.Base(manifestPath) {
	case "extension.toml", "extension.json":
	default:
		return "", fmt.Errorf("extension: extension %q has an invalid manifest path %q", info.Name, info.ManifestPath)
	}
	installDir := filepath.Dir(manifestPath)
	if installDir == "." || installDir == string(filepath.Separator) {
		return "", fmt.Errorf("extension: extension %q has an invalid install directory %q", info.Name, installDir)
	}
	return installDir, nil
}
