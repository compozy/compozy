package extensionpkg

import (
	"net/http"
	"strings"

	registrypkg "github.com/compozy/agh/internal/registry"
)

func hasCuratedMarketplaceArtifact(trust *MarketplaceTrustEvidence) bool {
	return trust != nil && strings.TrimSpace(trust.ArtifactURL) != ""
}

func newCuratedMarketplaceArtifactDownloader(
	trust *MarketplaceTrustEvidence,
	client *http.Client,
) (registrypkg.Downloader, error) {
	return registrypkg.NewHTTPArchiveDownloader(
		strings.TrimSpace(trust.ArtifactURL),
		strings.TrimSpace(trust.Version),
		client,
	)
}

func curatedMarketplaceArtifactDetail(
	slug string,
	trust *MarketplaceTrustEvidence,
) *registrypkg.Detail {
	return &registrypkg.Detail{
		Listing: registrypkg.Listing{
			Slug:    strings.TrimSpace(slug),
			Name:    strings.TrimSpace(slug),
			Version: strings.TrimSpace(trust.Version),
			Source:  MarketplaceCatalogRegistryName,
			Type:    registrypkg.PackageTypeExtension,
		},
		Repository: strings.TrimSpace(trust.Repository),
	}
}

func curatedMarketplaceSourceURL(trust *MarketplaceTrustEvidence) string {
	if trust == nil {
		return ""
	}
	if repository := strings.TrimSpace(trust.Repository); repository != "" {
		return repository
	}
	return strings.TrimSpace(trust.ArtifactURL)
}
