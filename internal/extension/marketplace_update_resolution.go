package extensionpkg

import (
	"context"
	"errors"
	"strings"

	marketplacepkg "github.com/compozy/compozy/internal/marketplace"
	registrypkg "github.com/compozy/compozy/internal/registry"
)

type marketplaceUpdateResolution struct {
	downloader      registrypkg.Downloader
	trust           *MarketplaceTrustEvidence
	plugin          *MarketplacePluginAcquisition
	slug            string
	expectedDigest  string
	latestVersion   string
	hasUpdate       bool
	registryName    string
	closeDownloader func() error
}

func resolveMarketplaceUpdate(
	ctx context.Context,
	loader MarketplaceSourceLoader,
	info *ExtensionInfo,
	slug string,
	registryName string,
	currentVersion string,
	req MarketplaceUpdateRequest,
) (marketplaceUpdateResolution, error) {
	requestedVersion := strings.TrimSpace(req.Version)
	if info.Provenance.SourceRef != "" && info.Provenance.SourceRef != marketplacepkg.CompozyCatalogRef {
		return resolvePluginMarketplaceUpdate(ctx, info, req)
	}
	trust, err := resolveCurrentMarketplaceTrust(ctx, info, req.ResolveTrust, slug, requestedVersion)
	if err != nil {
		return marketplaceUpdateResolution{}, err
	}
	if hasCuratedMarketplaceArtifact(trust) {
		downloader, err := newCuratedMarketplaceArtifactDownloader(trust, req.ArtifactHTTPClient)
		if err != nil {
			return marketplaceUpdateResolution{}, err
		}
		latestVersion := strings.TrimSpace(trust.Version)
		hasUpdate := registrypkg.VersionIsNewer(currentVersion, latestVersion)
		if requestedVersion != "" {
			hasUpdate = requestedVersion != currentVersion
		}
		return marketplaceUpdateResolution{
			downloader: downloader, trust: trust, latestVersion: latestVersion, hasUpdate: hasUpdate,
			registryName: MarketplaceCatalogRegistryName, slug: slug, expectedDigest: trust.ArchiveDigestSHA256,
		}, nil
	}

	multi, err := newExtensionMarketplaceRegistry(ctx, loader, registryName)
	if err != nil {
		return marketplaceUpdateResolution{}, err
	}
	updateInfo, err := multi.CheckUpdate(ctx, slug, currentVersion)
	if err != nil {
		return marketplaceUpdateResolution{}, errors.Join(err, multi.Close())
	}
	if requestedVersion != "" {
		updateInfo.HasUpdate = requestedVersion != currentVersion
		updateInfo.LatestVersion = requestedVersion
	}
	return marketplaceUpdateResolution{
		downloader: multi, trust: trust, latestVersion: updateInfo.LatestVersion,
		hasUpdate: updateInfo.HasUpdate, registryName: registryName, closeDownloader: multi.Close, slug: slug,
	}, nil
}

func resolvePluginMarketplaceUpdate(
	ctx context.Context, info *ExtensionInfo, req MarketplaceUpdateRequest,
) (marketplaceUpdateResolution, error) {
	if req.ResolvePlugin == nil {
		return marketplaceUpdateResolution{}, errors.New("extension: plugin marketplace resolver is unavailable")
	}
	plugin, err := req.ResolvePlugin(
		ctx,
		info.Provenance.SourceRef,
		info.Provenance.EntryID,
		strings.TrimSpace(req.Version),
	)
	if err != nil {
		return marketplaceUpdateResolution{}, err
	}
	if plugin == nil || plugin.Record.SourceRef != info.Provenance.SourceRef ||
		plugin.Record.EntryID != info.Provenance.EntryID {
		return marketplaceUpdateResolution{}, errors.New(
			"extension: plugin update origin differs from the installed origin",
		)
	}
	slug := plugin.SourceName + "/" + plugin.Record.EntryID
	if err := plugin.validate(MarketplaceInstallRequest{
		Slug: slug, ExpectedDigest: plugin.Record.DigestSHA256, Version: req.Version,
	}); err != nil {
		return marketplaceUpdateResolution{}, err
	}
	return marketplaceUpdateResolution{
		downloader: &pluginMarketplaceDownloader{acquisition: plugin}, plugin: plugin, slug: slug,
		expectedDigest: plugin.Record.DigestSHA256, latestVersion: plugin.Record.Version,
		hasUpdate: info.Provenance.ArchiveDigestSHA256 != plugin.Record.DigestSHA256, registryName: plugin.SourceName,
	}, nil
}

func resolveCurrentMarketplaceTrust(
	ctx context.Context,
	info *ExtensionInfo,
	resolver MarketplaceTrustResolver,
	slug string,
	requestedVersion string,
) (*MarketplaceTrustEvidence, error) {
	if strings.TrimSpace(info.Provenance.CatalogEntryID) == "" {
		return nil, nil
	}
	return resolveMarketplaceUpdateTrust(ctx, resolver, slug, requestedVersion)
}
