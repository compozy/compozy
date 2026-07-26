package extensionpkg

import (
	"context"
	"errors"
	"strings"

	registrypkg "github.com/compozy/agh/internal/registry"
)

type marketplaceUpdateResolution struct {
	downloader      registrypkg.Downloader
	trust           *MarketplaceTrustEvidence
	latestVersion   string
	hasUpdate       bool
	registryName    string
	closeDownloader func() error
}

func resolveMarketplaceUpdate(
	ctx context.Context,
	loader MarketplaceSourceLoader,
	info ExtensionInfo,
	slug string,
	registryName string,
	currentVersion string,
	req MarketplaceUpdateRequest,
) (marketplaceUpdateResolution, error) {
	requestedVersion := strings.TrimSpace(req.Version)
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
			registryName: MarketplaceCatalogRegistryName,
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
	if trust == nil && updateInfo.HasUpdate && !req.CheckOnly {
		trust, err = resolveMarketplaceUpdateTrust(ctx, req.ResolveTrust, slug, updateInfo.LatestVersion)
		if err != nil {
			return marketplaceUpdateResolution{}, errors.Join(err, multi.Close())
		}
	}
	return marketplaceUpdateResolution{
		downloader: multi, trust: trust, latestVersion: updateInfo.LatestVersion,
		hasUpdate: updateInfo.HasUpdate, registryName: registryName, closeDownloader: multi.Close,
	}, nil
}

func resolveCurrentMarketplaceTrust(
	ctx context.Context,
	info ExtensionInfo,
	resolver MarketplaceTrustResolver,
	slug string,
	requestedVersion string,
) (*MarketplaceTrustEvidence, error) {
	if strings.TrimSpace(info.Provenance.CatalogEntryID) == "" {
		return nil, nil
	}
	return resolveMarketplaceUpdateTrust(ctx, resolver, slug, requestedVersion)
}
