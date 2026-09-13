package core

import (
	"context"

	"github.com/compozy/compozy/internal/api/contract"
	marketplacepkg "github.com/compozy/compozy/internal/marketplace"
)

type marketplaceExtensionInspector interface {
	InspectCatalogExtension(
		context.Context,
		marketplacepkg.Entry,
		string,
	) (contract.MarketplaceExtensionDetailPayload, error)
}

func (h *BaseHandlers) populateMarketplaceExtensionDetail(
	ctx context.Context,
	entry marketplacepkg.Entry,
	installed marketplaceInstallIndex,
	scope marketplaceReadScope,
	detail *contract.MarketplaceExtensionDetailPayload,
) error {
	if detail == nil {
		return nil
	}
	origin := marketplacepkg.Origin{SourceRef: marketplacepkg.CompozyCatalogRef, EntryID: entry.EntryID}
	if item := installed.byOrigin[origin].extension; item != nil {
		detail.Contents, detail.MCPServers = item.Contents, item.MCPServers
		if detail.MCPServers == nil {
			detail.MCPServers = []contract.MarketplaceServerPayload{}
		}
		return nil
	}
	if inspector, ok := h.Extensions.(marketplaceExtensionInspector); ok {
		inspected, err := inspector.InspectCatalogExtension(ctx, entry, scope.profileName)
		if err != nil {
			return err
		}
		detail.Contents, detail.MCPServers, detail.Inputs = inspected.Contents, inspected.MCPServers, inspected.Inputs
	}
	return nil
}
