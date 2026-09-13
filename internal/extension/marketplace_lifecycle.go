package extensionpkg

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	compozyconfig "github.com/compozy/compozy/internal/config"
	diagnosticcontract "github.com/compozy/compozy/internal/diagnosticcontract"
	marketplacepkg "github.com/compozy/compozy/internal/marketplace"
	registrypkg "github.com/compozy/compozy/internal/registry"
)

const (
	// MarketplaceUpdateStatusCurrent reports that no remote update is available.
	MarketplaceUpdateStatusCurrent = "current"
	// MarketplaceUpdateStatusAvailable reports that a remote update exists but was not applied.
	MarketplaceUpdateStatusAvailable = "available"
	// MarketplaceUpdateStatusUpdated reports that a remote update was applied.
	MarketplaceUpdateStatusUpdated = "updated"
	// MarketplaceUpdateStatusFailed reports one failed item in a batch.
	MarketplaceUpdateStatusFailed = "failed"
)

// LifecycleRegistry is the installed-extension persistence surface required by
// managed lifecycle helpers.
type LifecycleRegistry interface {
	Get(name string) (*ExtensionInfo, error)
	List() ([]ExtensionInfo, error)
	Install(manifest *Manifest, path string, checksum string, opts ...InstallOption) error
	Disable(name string) error
	Uninstall(name string) error
	RestoreNetworkConfirmation(InstanceKey, NetworkConfirmation) error
}

var _ LifecycleRegistry = (*Registry)(nil)

// MarketplaceSourceLoader resolves configured marketplace sources. The
// optional source filter is an already-normalized operator/tool input.
type MarketplaceSourceLoader func(context.Context) ([]registrypkg.Source, error)

// ErrMarketplaceSourceUnavailable reports that a marketplace source cannot be resolved or used.
var ErrMarketplaceSourceUnavailable = errors.New("extension: marketplace source unavailable")

// MutationReload is called after a registry/on-disk mutation and before the
// lifecycle helper commits any staged filesystem backup.
type MutationReload func(context.Context) error

// MarketplaceUpdatePreflight validates a candidate before it replaces the installed files.
type MarketplaceUpdatePreflight func(ExtensionInfo, *Manifest) error

// MarketplaceUpdateCommit runs after registry persistence; failure restores the prior installation.
type MarketplaceUpdateCommit func(ExtensionInfo, *Manifest) error

// MarketplaceUpdateRollback restores candidate-owned state after the previous package and registry
// record have been restored successfully, and before that installation is reloaded.
type MarketplaceUpdateRollback func(context.Context, ExtensionInfo) error

// MarketplaceInstallRequest describes one marketplace-backed extension install.
type MarketplaceInstallRequest struct {
	Plugin                    *MarketplacePluginAcquisition
	Scope                     InstallationScope
	ExpectedDigest            string
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
	Name           string                              `json:"name"`
	Path           string                              `json:"path"`
	DataPath       string                              `json:"data_path,omitempty"`
	QuarantinePath string                              `json:"quarantine_path,omitempty"`
	Status         string                              `json:"status"`
	Warnings       []diagnosticcontract.DiagnosticItem `json:"warnings,omitempty"`
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
	ResolvePlugin             MarketplacePluginResolver
	ArtifactHTTPClient        *http.Client
	ObserveDigestVerification MarketplaceDigestVerificationObserver
	PreflightCandidate        MarketplaceUpdatePreflight
	CommitCandidate           MarketplaceUpdateCommit
	RollbackCandidate         MarketplaceUpdateRollback
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
	Error          *diagnosticcontract.DiagnosticItem  `json:"error,omitempty"`
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
	stagingDir    string
	installPath   string
	finalDir      string
	checksum      string
	archiveDigest string
	digestMatched bool
	remoteVersion string
	trust         *MarketplaceTrustEvidence
	cleanup       []registrypkg.CleanupDiagnostic
}

// InstallMarketplaceManaged installs one extension through the configured
// marketplace registry into the managed extension root and records marketplace
// provenance in the installed-extension registry.
func InstallMarketplaceManaged(
	ctx context.Context,
	homePaths compozyconfig.HomePaths,
	registry LifecycleRegistry,
	loader MarketplaceSourceLoader,
	req MarketplaceInstallRequest,
) (_ *ExtensionInfo, err error) {
	prepared, err := PrepareMarketplaceManagedInstall(ctx, homePaths, registry, loader, req)
	if err != nil {
		return nil, err
	}
	defer func() {
		err = errors.Join(err, prepared.Close())
	}()
	return prepared.Commit(req.Scope)
}

func prepareMarketplaceManagedInstall(
	ctx context.Context,
	homePaths compozyconfig.HomePaths,
	loader MarketplaceSourceLoader,
	req MarketplaceInstallRequest,
) (_ marketplaceManagedInstall, err error) {
	slug, err := validateMarketplaceManagedInstallRequest(req)
	if err != nil {
		return marketplaceManagedInstall{}, err
	}
	return prepareMarketplacePackage(ctx, homePaths, loader, req, slug)
}

// The caller validates either installation consent or the read-only pinned-artifact contract.
func prepareMarketplacePackage(ctx context.Context, homePaths compozyconfig.HomePaths, loader MarketplaceSourceLoader,
	req MarketplaceInstallRequest, slug string,
) (_ marketplaceManagedInstall, err error) {
	var downloader registrypkg.Downloader
	var closeDownloader func() error
	var detail *registrypkg.Detail
	switch {
	case req.Plugin != nil:
		downloader = &pluginMarketplaceDownloader{acquisition: req.Plugin}
		detail = &registrypkg.Detail{
			Listing: registrypkg.Listing{
				Slug:    slug,
				Source:  req.Plugin.SourceName,
				Version: req.Plugin.Record.Version,
			},
			Repository: req.Plugin.Record.SourceRef,
		}
	case hasCuratedMarketplaceArtifact(req.Trust):
		downloader, err = newCuratedMarketplaceArtifactDownloader(req.Trust, req.ArtifactHTTPClient)
		if err != nil {
			return marketplaceManagedInstall{}, err
		}
		detail = curatedMarketplaceArtifactDetail(slug, req.Trust)
	default:
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
	cleanupStaging := true
	defer func() {
		if cleanupStaging {
			joinRemoveAll(&err, stagingDir, "extension: remove staged extension directory")
		}
	}()

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
	manifest, finalDir, err := inspectMarketplaceInstall(homePaths, slug, result)
	if err != nil {
		return marketplaceManagedInstall{}, err
	}
	cleanupStaging = false
	return marketplaceManagedInstall{
		slug:          slug,
		detail:        detail,
		manifest:      manifest,
		stagingDir:    stagingDir,
		installPath:   result.InstallPath,
		finalDir:      finalDir,
		checksum:      result.Checksum,
		archiveDigest: result.ArchiveDigestSHA256,
		digestMatched: result.DigestMatched,
		remoteVersion: firstNonEmpty(result.Version, detail.Version, manifest.Version),
		trust:         req.Trust,
		cleanup:       append([]registrypkg.CleanupDiagnostic(nil), result.CleanupDiagnostics...),
	}, nil
}

func validateMarketplaceManagedInstallRequest(req MarketplaceInstallRequest) (string, error) {
	if err := ValidateExpectedDigest(req.ExpectedDigest); err != nil {
		return "", err
	}
	if req.Plugin != nil {
		if err := req.Plugin.validate(req); err != nil {
			return "", err
		}
	}
	if req.Trust != nil {
		if err := CheckExpectedDigest(req.ExpectedDigest, req.Trust.ArchiveDigestSHA256); err != nil {
			return "", err
		}
	}
	slug := strings.TrimSpace(req.Slug)
	if slug == "" {
		return "", errors.New("extension: marketplace slug is required")
	}
	if err := validateMarketplaceTrustGate(
		slug,
		req.SourceFilter,
		req.PolicyAllowsUnverified,
		req.AllowUnverified,
		req.Trust,
	); err != nil {
		return "", err
	}
	return slug, nil
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
	expectedDigest := strings.ToLower(strings.TrimSpace(req.ExpectedDigest))
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
		if mismatch, ok := errors.AsType[*registrypkg.ArchiveDigestMismatchError](err); ok && req.ExpectedDigest != "" {
			err = &SourceChangedError{
				ListedDigest:  mismatch.ExpectedSHA256,
				FetchedDigest: mismatch.ActualSHA256,
				Cause:         err,
			}
		}
	}
	if req.ObserveDigestVerification != nil {
		req.ObserveDigestVerification(req.Trust, err)
	}
	return result, err
}

func inspectMarketplaceInstall(
	homePaths compozyconfig.HomePaths,
	slug string,
	result *registrypkg.InstallResult,
) (*Manifest, string, error) {
	manifest, err := LoadManifest(result.InstallPath)
	if err != nil {
		return nil, "", fmt.Errorf("extension: load installed extension manifest for %q: %w", slug, err)
	}
	finalDir, err := ManagedInstallPathChecked(homePaths, manifest.Name)
	if err != nil {
		return nil, "", err
	}
	return manifest, finalDir, nil
}

func marketplaceInstallProvenance(
	prepared marketplaceManagedInstall,
	req MarketplaceInstallRequest,
) ExtensionProvenance {
	var provenance ExtensionProvenance
	if prepared.trust != nil {
		registryTier := normalizedMarketplaceRegistryTier(prepared.trust.RegistryTier)
		allowUnverified := registryTier == ExtensionRegistryTierUnverified && req.AllowUnverified
		provenance = ExtensionProvenance{
			Slug:           prepared.slug,
			CatalogEntryID: strings.TrimSpace(prepared.trust.CatalogEntryID),
			SourceName:     marketplacepkg.CompozyCatalogSource,
			SourceRef:      marketplacepkg.CompozyCatalogRef,
			EntryID:        strings.TrimSpace(prepared.trust.CatalogEntryID),
			InstalledFrom:  ExtensionInstalledFromMarketplace,
			SourceURL: firstNonEmpty(
				curatedMarketplaceSourceURL(prepared.trust),
				strings.TrimSpace(prepared.detail.Repository),
			),
			ChecksumSHA256:      prepared.checksum,
			ArchiveDigestSHA256: prepared.archiveDigest,
			DigestMatched:       prepared.digestMatched,
			ChecksumVerified:    true,
			RegistryTier:        registryTier,
			Permissions:         extensionPermissions(prepared.manifest),
			InstalledBy:         firstNonEmpty(req.InstalledBy, extensionTrustInstalledByOperator),
			AllowUnverified:     allowUnverified,
			Warnings:            marketplaceTrustWarnings(prepared.trust),
		}
	} else {
		provenance = ExtensionProvenance{
			Slug:                prepared.slug,
			InstalledFrom:       installedFromForRegistrySource(prepared.detail.Source),
			SourceURL:           strings.TrimSpace(prepared.detail.Repository),
			ChecksumSHA256:      prepared.checksum,
			ArchiveDigestSHA256: prepared.archiveDigest,
			DigestMatched:       prepared.digestMatched,
			ChecksumVerified:    false,
			RegistryTier:        ExtensionRegistryTierUnverified,
			Permissions:         extensionPermissions(prepared.manifest),
			InstalledBy:         firstNonEmpty(req.InstalledBy, extensionTrustInstalledByOperator),
			AllowUnverified:     req.AllowUnverified,
			Warnings: []diagnosticcontract.DiagnosticItem{
				extensionChecksumUnverifiedDiagnostic(prepared.slug, prepared.detail.Source, true),
			},
		}
	}
	if req.Plugin != nil {
		provenance.SourceName = req.Plugin.SourceName
		provenance.SourceRef = req.Plugin.Record.SourceRef
		provenance.EntryID = req.Plugin.Record.EntryID
		provenance.ResolvedRef = req.Plugin.Record.ResolvedRef
		provenance.Layout = req.Plugin.Record.Layout
		provenance.InstalledFrom = ExtensionInstalledFromMarketplace
	}
	provenance.Warnings = appendExtensionInstallCleanupWarnings(
		provenance.Warnings,
		prepared.slug,
		prepared.cleanup,
	)
	return provenance
}
