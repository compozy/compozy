package core

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
)

func (h *BaseHandlers) installedExtensionMarketplaceEntry(
	ctx context.Context,
	entryID string,
	scope marketplaceReadScope,
) (contract.MarketplaceEntryResponse, error) {
	if h == nil {
		return contract.MarketplaceEntryResponse{}, marketplaceInstalledEntryNotFound(
			entryID,
		)
	}
	items, err := h.marketplaceExtensions(ctx, scope)
	if err != nil {
		return contract.MarketplaceEntryResponse{}, err
	}
	for index := range items {
		item := &items[index]
		name := strings.TrimSpace(item.Name)
		if name != entryID {
			continue
		}
		if err := h.joinInstalledExtensionMarketplace(ctx, items[index:index+1]); err != nil {
			h.Logger.Warn("api: installed extension marketplace enrichment failed", "error", err)
		}
		source := strings.TrimSpace(item.Source)
		if item.Provenance != nil && strings.TrimSpace(item.Provenance.InstalledFrom) != "" {
			source = strings.TrimSpace(item.Provenance.InstalledFrom)
		}
		listing := contract.MarketplaceListingPayload{
			EntryID: entryID,
			Name:    name,
			Version: strings.TrimSpace(
				item.Version,
			),
			Source:           source,
			Installed:        true,
			Format:           strings.TrimSpace(item.Format),
			InstalledName:    name,
			InstalledVersion: strings.TrimSpace(item.Version),
			ManagePath:       marketplaceExtensionsInstalledPath,
			Trust:            item.Trust,
		}
		detail := &contract.MarketplaceExtensionDetailPayload{
			Contents:   item.Contents,
			MCPServers: item.MCPServers,
			Inputs:     []contract.MarketplaceInputPayload{},
		}
		if detail.MCPServers == nil {
			detail.MCPServers = []contract.MarketplaceServerPayload{}
		}
		if item.Origin != nil {
			listing.Source = item.Origin.Source
			listing.SourceRef = item.Origin.SourceRef
			listing.EntryID = item.Origin.EntryID
		}
		if item.Provenance != nil {
			listing.DigestSHA256 = item.Provenance.ArchiveDigestSHA256
			detail.DigestSHA256 = item.Provenance.ArchiveDigestSHA256
			detail.ResolvedRef = item.Provenance.ResolvedRef
			detail.Layout = item.Provenance.Layout
			detail.InstallSlug = item.Provenance.Slug
		}
		if item.Marketplace != nil {
			listing = *item.Marketplace
		}
		return contract.MarketplaceEntryResponse{Entry: listing, Extension: detail}, nil
	}
	return contract.MarketplaceEntryResponse{}, marketplaceInstalledEntryNotFound(
		entryID,
	)
}

func marketplaceInstalledEntryNotFound(entryID string) error {
	return errors.Join(
		ErrMarketplaceNotFound,
		fmt.Errorf("installed extension %q not found", entryID),
	)
}
