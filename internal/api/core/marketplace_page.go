package core

import (
	"crypto/sha256"
	"encoding/base64"
	"strconv"
	"time"

	"github.com/compozy/agh/internal/api/contract"
	marketplacepkg "github.com/compozy/agh/internal/marketplace"
	registrypkg "github.com/compozy/agh/internal/registry"
)

type marketplaceKindPage struct {
	result       contract.MarketplaceKindResult
	currentFence string
	nextFence    string
	nextOffset   int
	hasMore      bool
}

func marketplaceCatalogFence(kind marketplacepkg.Kind, state marketplacepkg.KindState) string {
	return marketplaceStringFence([]string{
		"curated",
		string(kind),
		strconv.Itoa(state.ManifestVersion),
		state.GeneratedAt.UTC().Format(time.RFC3339Nano),
		state.FetchedAt.UTC().Format(time.RFC3339Nano),
		strconv.Itoa(state.EntryCount),
	})
}

func marketplaceRemoteSkillFence(listings []registrypkg.Listing) string {
	parts := make([]string, 1, 1+len(listings)*8)
	parts[0] = "remote-skill"
	for _, listing := range listings {
		parts = append(
			parts,
			listing.Slug,
			listing.Name,
			listing.Description,
			listing.Author,
			listing.Version,
			strconv.Itoa(listing.Downloads),
			listing.Source,
			string(listing.Type),
		)
	}
	return marketplaceStringFence(parts)
}

func marketplaceBundleFence(items []contract.MarketplaceListingPayload) string {
	parts := make([]string, 1, 1+len(items)*7)
	parts[0] = "bundle"
	for _, item := range items {
		parts = append(
			parts,
			item.EntryID,
			item.Name,
			item.Description,
			item.Source,
			strconv.FormatBool(item.Installed),
			strconv.FormatBool(item.UpdateAvailable),
			item.ManagePath,
		)
	}
	return marketplaceStringFence(parts)
}

func marketplaceStringFence(parts []string) string {
	payload := make([]byte, 0, len(parts)*16)
	for _, part := range parts {
		payload = strconv.AppendInt(payload, int64(len(part)), 10)
		payload = append(payload, ':')
		payload = append(payload, part...)
	}
	digest := sha256.Sum256(payload)
	return base64.RawURLEncoding.EncodeToString(digest[:])
}
