package cli

import (
	"fmt"
	"strings"

	registrypkg "github.com/compozy/agh/internal/registry"
)

func skillMarketplaceListingsFromRecords(
	items []MarketplaceListingRecord,
) ([]registrypkg.Listing, error) {
	listings := make([]registrypkg.Listing, 0, len(items))
	for _, item := range items {
		installSlug := strings.TrimSpace(item.InstallSlug)
		if installSlug == "" {
			return nil, fmt.Errorf("cli: marketplace skill %q has no install slug", item.EntryID)
		}
		downloads := 0
		if item.Downloads != nil {
			downloads = *item.Downloads
		}
		listings = append(listings, registrypkg.Listing{
			Slug:        installSlug,
			Name:        item.Name,
			Description: item.Description,
			Author:      item.Author,
			Version:     item.Version,
			Downloads:   downloads,
			Source:      item.Source,
			Type:        registrypkg.PackageTypeSkill,
		})
	}
	return listings, nil
}
