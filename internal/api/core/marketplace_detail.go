package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/compozy/compozy/internal/api/contract"
	marketplacepkg "github.com/compozy/compozy/internal/marketplace"
)

func (h *BaseHandlers) catalogMarketplaceEntry(
	ctx context.Context,
	source, entryID string,
	scope marketplaceReadScope,
) (contract.MarketplaceEntryResponse, error) {
	if h == nil || h.MarketplaceCatalog == nil {
		return contract.MarketplaceEntryResponse{}, errors.Join(
			ErrMarketplaceUnavailable, errors.New("catalog is not configured"),
		)
	}
	entry, err := h.MarketplaceCatalog.Detail(ctx, source, entryID)
	if err != nil {
		return contract.MarketplaceEntryResponse{}, normalizeCuratedMarketplaceError(err)
	}
	if entry == nil {
		return contract.MarketplaceEntryResponse{}, errors.Join(
			ErrMarketplaceNotFound, fmt.Errorf("marketplace entry %q not found", entryID),
		)
	}
	installed, err := h.extensionInstallIndex(ctx, scope)
	if err != nil {
		return contract.MarketplaceEntryResponse{}, err
	}
	listing, err := h.catalogMarketplaceListing(ctx, *entry, installed)
	if err != nil {
		return contract.MarketplaceEntryResponse{}, err
	}
	details, err := marketplacepkg.ProjectEntry(*entry)
	if err != nil {
		return contract.MarketplaceEntryResponse{}, err
	}
	response := contract.MarketplaceEntryResponse{Entry: listing}
	response.Extension = marketplaceExtensionDetail(details)
	if err := h.populateMarketplaceExtensionDetail(ctx, *entry, installed, scope, response.Extension); err != nil {
		return contract.MarketplaceEntryResponse{}, err
	}
	return response, nil
}

func marketplaceExtensionDetail(
	details marketplacepkg.EntryDetails,
) *contract.MarketplaceExtensionDetailPayload {
	if details.Extension == nil {
		return nil
	}
	result := &contract.MarketplaceExtensionDetailPayload{
		Inputs:       []contract.MarketplaceInputPayload{},
		MCPServers:   []contract.MarketplaceServerPayload{},
		InstallSlug:  details.Extension.InstallSlug,
		ArtifactURL:  details.Extension.ArtifactURL,
		DigestSHA256: details.Extension.DigestSHA256,
		Repository:   details.Extension.Repository,
	}
	for _, input := range details.Extension.Inputs {
		result.Inputs = append(result.Inputs, contract.MarketplaceInputPayload{
			ID: input.ID, Prompt: input.Prompt, Type: input.Type, Required: input.Required,
			Binding: contract.MarketplaceInputBindingPayload{Type: input.Binding.Type, Name: input.Binding.Name},
			Default: append(json.RawMessage(nil), input.Default...),
		})
	}
	return result
}
