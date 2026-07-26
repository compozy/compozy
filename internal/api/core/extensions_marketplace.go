package core

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/agh/internal/api/contract"
	marketplacepkg "github.com/compozy/agh/internal/marketplace"
)

func (h *BaseHandlers) joinInstalledExtensionMarketplace(
	ctx context.Context,
	items []contract.ExtensionPayload,
) error {
	if h == nil || h.MarketplaceCatalog == nil {
		return nil
	}

	joinErrors := make([]error, 0)
	for index := range items {
		item := &items[index]
		if item.Provenance == nil {
			continue
		}
		entryID := strings.TrimSpace(item.Provenance.CatalogEntryID)
		if entryID == "" {
			continue
		}
		entry, err := h.MarketplaceCatalog.Detail(ctx, marketplacepkg.KindExtension, entryID)
		if onlyMarketplaceEntryNotFound(err) {
			continue
		}
		if err != nil {
			joinErrors = append(
				joinErrors,
				fmt.Errorf("resolve installed extension marketplace entry %q: %w", entryID, err),
			)
			continue
		}
		if entry == nil {
			joinErrors = append(
				joinErrors,
				fmt.Errorf("resolve installed extension marketplace entry %q: empty result", entryID),
			)
			continue
		}

		installed := newMarketplaceInstallIndex()
		installation := marketplaceInstall{
			name:       strings.TrimSpace(item.Name),
			version:    strings.TrimSpace(item.Version),
			managePath: marketplaceExtensionsInstalledPath,
		}
		installed.byEntryID[strings.TrimSpace(entry.EntryID)] = installation
		if slug := strings.TrimSpace(entry.InstallSlug); slug != "" {
			installed.bySlug[slug] = installation
		}
		listing, err := h.curatedMarketplaceListing(ctx, *entry, installed)
		if err != nil {
			joinErrors = append(
				joinErrors,
				fmt.Errorf("project installed extension marketplace entry %q: %w", entryID, err),
			)
			continue
		}
		item.Marketplace = &listing
	}
	return errors.Join(joinErrors...)
}

func onlyMarketplaceEntryNotFound(err error) bool {
	if err == nil {
		return false
	}
	if joined, ok := err.(interface{ Unwrap() []error }); ok {
		children := joined.Unwrap()
		if len(children) == 0 {
			return false
		}
		for _, child := range children {
			if !onlyMarketplaceEntryNotFound(child) {
				return false
			}
		}
		return true
	}
	if wrapped, ok := err.(interface{ Unwrap() error }); ok {
		return onlyMarketplaceEntryNotFound(wrapped.Unwrap())
	}
	return errors.Is(err, marketplacepkg.ErrEntryNotFound)
}
