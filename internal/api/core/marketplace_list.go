package core

import (
	"context"
	"net/http"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	extensionpkg "github.com/compozy/compozy/internal/extension"
	marketplacepkg "github.com/compozy/compozy/internal/marketplace"
	registrypkg "github.com/compozy/compozy/internal/registry"
)

func (h *BaseHandlers) marketplaceCatalogDiagnostic(diagnostic string) string {
	if strings.TrimSpace(diagnostic) == "" || h == nil || !h.MaskInternalErrors {
		return diagnostic
	}
	return http.StatusText(http.StatusInternalServerError)
}

type marketplaceInstall struct {
	extension  *contract.ExtensionPayload
	name       string
	version    string
	managePath string
	// format is the installed instance's own truth, recorded at ingestion.
	format string
}

type marketplaceInstallIndex struct {
	byOrigin map[marketplacepkg.Origin]marketplaceInstall
}

func newMarketplaceInstallIndex() marketplaceInstallIndex {
	return marketplaceInstallIndex{
		byOrigin: make(map[marketplacepkg.Origin]marketplaceInstall),
	}
}

func (h *BaseHandlers) extensionInstallIndex(
	ctx context.Context,
	scope marketplaceReadScope,
) (marketplaceInstallIndex, error) {
	items, err := h.marketplaceExtensions(ctx, scope)
	if err != nil {
		return marketplaceInstallIndex{}, err
	}
	index := newMarketplaceInstallIndex()
	for itemIndex := range items {
		item := &items[itemIndex]
		if item.Origin == nil || item.Origin.SourceRef == "" || item.Origin.EntryID == "" {
			continue
		}
		installation := marketplaceInstall{
			extension:  item,
			name:       strings.TrimSpace(item.Name),
			version:    strings.TrimSpace(item.Version),
			managePath: marketplaceExtensionsInstalledPath,
			format:     strings.TrimSpace(item.Format),
		}
		index.byOrigin[marketplacepkg.Origin{SourceRef: item.Origin.SourceRef, EntryID: item.Origin.EntryID}] = installation
	}
	return index, nil
}

func (h *BaseHandlers) catalogMarketplaceListing(
	ctx context.Context,
	entry marketplacepkg.Entry,
	installed marketplaceInstallIndex,
) (contract.MarketplaceListingPayload, error) {
	details, err := marketplacepkg.ProjectEntry(entry)
	if err != nil {
		return contract.MarketplaceListingPayload{}, err
	}
	installation, isInstalled := installed.byOrigin[marketplacepkg.Origin{
		SourceRef: details.SourceRef, EntryID: entry.EntryID,
	}]
	updateAvailable := isInstalled && registrypkg.VersionIsNewer(installation.version, entry.Version)
	result := contract.MarketplaceListingPayload{
		EntryID:          entry.EntryID,
		Name:             entry.Name,
		Description:      entry.Description,
		Version:          entry.Version,
		Author:           details.Author,
		Source:           details.Source,
		InstallSlug:      entry.InstallSlug,
		Tier:             entry.Tier,
		Format:           marketplaceListingFormat(details, installation, isInstalled),
		PublishedAt:      entry.PublishedAt,
		UpdatedAt:        entry.UpdatedAt,
		Installed:        isInstalled,
		InstalledName:    installation.name,
		InstalledVersion: installation.version,
		UpdateAvailable:  updateAvailable,
		ManagePath:       installation.managePath,
	}
	trust, err := h.Extensions.MarketplaceTrust(ctx, extensionpkg.MarketplaceTrustEvidence{
		CatalogEntryID:      entry.EntryID,
		Version:             entry.Version,
		ArchiveDigestSHA256: entry.DigestSHA256,
		RegistryTier:        entry.Tier,
	})
	if err != nil {
		return contract.MarketplaceListingPayload{}, err
	}
	result.SourceRef = details.SourceRef
	result.DigestSHA256 = entry.DigestSHA256
	result.Icon = entry.Icon
	result.Layout = entry.Layout
	result.Installable = entry.InstallBlocker == ""
	result.InstallBlocker = entry.InstallBlocker
	result.Trust = &trust
	return result, nil
}

// marketplaceListingFormat resolves the one format a surface may render. Before install the curated
// marker is all we have; once an instance exists its recorded format is authoritative, because
// detection — not the feed — decided what was ingested.
func marketplaceListingFormat(
	details marketplacepkg.EntryDetails,
	installation marketplaceInstall,
	isInstalled bool,
) string {
	if isInstalled {
		if format := strings.TrimSpace(installation.format); format != "" {
			return format
		}
	}
	if details.Extension == nil {
		return ""
	}
	return strings.TrimSpace(details.Extension.Format)
}
