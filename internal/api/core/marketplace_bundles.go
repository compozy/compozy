package core

import (
	"context"
	"encoding/base64"
	"errors"
	"sort"
	"strings"

	"github.com/compozy/agh/internal/api/contract"
	bundlepkg "github.com/compozy/agh/internal/bundles"
	settingspkg "github.com/compozy/agh/internal/settings"
)

const bundleEntryPrefix = "bundle_"

func (h *BaseHandlers) bundleMarketplaceKind(
	ctx context.Context,
	query string,
	offset int,
	limit int,
	scope marketplaceReadScope,
) (marketplaceKindPage, error) {
	if h == nil || h.Bundles == nil {
		return marketplaceKindPage{}, errors.Join(
			ErrMarketplaceUnavailable, errors.New("bundle catalog is not configured"),
		)
	}
	catalog, err := h.Bundles.Catalog(ctx)
	if err != nil {
		return marketplaceKindPage{}, err
	}
	activations, err := h.Bundles.ListActivations(ctx)
	if err != nil {
		return marketplaceKindPage{}, err
	}
	activationIndex := bundleActivationIndex(activations, scope)
	needle := strings.ToLower(strings.TrimSpace(query))
	filtered := make([]bundlepkg.CatalogEntry, 0, len(catalog))
	for _, entry := range catalog {
		if needle != "" && !strings.Contains(strings.ToLower(
			entry.ExtensionName+" "+entry.Bundle.Name+" "+entry.Bundle.Description,
		), needle) {
			continue
		}
		filtered = append(filtered, entry)
	}
	sort.SliceStable(filtered, func(i, j int) bool {
		left := strings.ToLower(filtered[i].Bundle.Name + "\x00" + filtered[i].ExtensionName)
		right := strings.ToLower(filtered[j].Bundle.Name + "\x00" + filtered[j].ExtensionName)
		return left < right
	})
	total := len(filtered)
	allItems := make([]contract.MarketplaceListingPayload, 0, len(filtered))
	for _, entry := range filtered {
		key := bundleIdentityKey(entry.ExtensionName, entry.Bundle.Name)
		activation, installed := activationIndex[key]
		managePath := ""
		if installed {
			managePath = marketplaceBundleActivationPath(activation.Activation.ID)
		}
		allItems = append(allItems, contract.MarketplaceListingPayload{
			Kind: contract.MarketplaceKindBundle, EntryID: encodeBundleEntryID(entry.ExtensionName, entry.Bundle.Name),
			Name: entry.Bundle.Name, Description: entry.Bundle.Description, Source: entry.ExtensionName,
			Installed: installed, UpdateAvailable: installed && activation.SpecDrift, ManagePath: managePath,
		})
	}
	items := []contract.MarketplaceListingPayload{}
	if offset < total {
		items = allItems[offset:min(offset+limit, total)]
	}
	fence := marketplaceBundleFence(allItems)
	return marketplaceKindPage{
		result: contract.MarketplaceKindResult{
			Kind: contract.MarketplaceKindBundle, Total: &total,
			Items: items,
		},
		currentFence: fence,
		nextFence:    fence,
		nextOffset:   offset + len(items),
		hasMore:      offset+len(items) < total,
	}, nil
}

func bundleActivationIndex(
	activations []bundlepkg.ActivationPreview,
	scope marketplaceReadScope,
) map[string]bundlepkg.ActivationPreview {
	sort.SliceStable(activations, func(i, j int) bool {
		return activations[i].Activation.ID < activations[j].Activation.ID
	})
	index := make(map[string]bundlepkg.ActivationPreview)
	for _, item := range activations {
		activation := item.Activation
		visible := activation.Scope == bundlepkg.ScopeGlobal ||
			scope.scope == settingspkg.ScopeWorkspace && activation.Scope == bundlepkg.ScopeWorkspace &&
				strings.TrimSpace(activation.WorkspaceID) == scope.workspaceID
		if !visible {
			continue
		}
		key := bundleIdentityKey(activation.ExtensionName, activation.BundleName)
		existing, exists := index[key]
		if !exists || activation.Scope == bundlepkg.ScopeWorkspace &&
			existing.Activation.Scope != bundlepkg.ScopeWorkspace {
			index[key] = item
		}
	}
	return index
}

func encodeBundleEntryID(extensionName string, bundleName string) string {
	return bundleEntryPrefix + base64.RawURLEncoding.EncodeToString(
		[]byte(bundleIdentityKey(extensionName, bundleName)),
	)
}

func decodeBundleEntryID(entryID string) (string, string, bool) {
	if !strings.HasPrefix(entryID, bundleEntryPrefix) {
		return "", "", false
	}
	decoded, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(entryID, bundleEntryPrefix))
	if err != nil {
		return "", "", false
	}
	parts := strings.Split(string(decoded), "\x00")
	if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" || strings.TrimSpace(parts[1]) == "" {
		return "", "", false
	}
	return parts[0], parts[1], true
}

func bundleIdentityKey(extensionName string, bundleName string) string {
	return strings.TrimSpace(extensionName) + "\x00" + strings.TrimSpace(bundleName)
}
