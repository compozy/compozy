package daemon

import (
	"context"
	"errors"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	extensionpkg "github.com/compozy/compozy/internal/extension"
	marketplacepkg "github.com/compozy/compozy/internal/marketplace"
)

// InspectCatalogExtension uses the exact entry already selected by the read surface, without resolving a newer listing.
func (s *daemonExtensionService) InspectCatalogExtension(
	ctx context.Context,
	entry marketplacepkg.Entry,
	profileName string,
) (contract.MarketplaceExtensionDetailPayload, error) {
	if err := s.checkReady(); err != nil {
		return contract.MarketplaceExtensionDetailPayload{}, err
	}
	details, err := marketplacepkg.ProjectEntry(entry)
	if err != nil {
		return contract.MarketplaceExtensionDetailPayload{}, err
	}
	if details.Extension == nil {
		return contract.MarketplaceExtensionDetailPayload{}, errors.New(
			"daemon: catalog inspection requires an extension entry",
		)
	}
	if strings.TrimSpace(profileName) == "" {
		profileName = daemonDefaultProfileName
	}
	return extensionpkg.InspectMarketplacePackage(ctx, s.homePaths, extensionpkg.MarketplaceInstallRequest{
		Slug: entry.InstallSlug, Version: entry.Version, ExpectedDigest: entry.DigestSHA256,
		Trust: &extensionpkg.MarketplaceTrustEvidence{
			CatalogEntryID:      entry.EntryID,
			Version:             entry.Version,
			ArchiveDigestSHA256: entry.DigestSHA256,
			RegistryTier:        entry.Tier,
			ArtifactURL:         details.Extension.ArtifactURL,
			Repository:          details.Extension.Repository,
		},
	}, profileName)
}
