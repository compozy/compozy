package cli

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/compozy/agh/internal/api/contract"
	extensionpkg "github.com/compozy/agh/internal/extension"
)

const (
	extensionMarketplaceSlugValue = "Slug"
	extensionMarketplaceSlugKey   = "slug"
)

const (
	skillOutputRegistryKey = "registry"
)

const (
	extensionMarketplaceDescriptionValue  = "Description"
	extensionMarketplacePathValue         = "Path"
	extensionMarketplaceStatusValue       = "Status"
	extensionMarketplaceCurrentVersionKey = "current_version"
	extensionMarketplaceDescriptionKey    = "description"
	extensionMarketplacePathKey           = "path"
)

const (
	defaultExtensionRegistrySearchLimit = 20
)

type extensionRemoveItem = ManagedExtensionRemoveRecord

type extensionUpdateItem = ExtensionUpdateRecord

type marketplaceExtensionSearchItem struct {
	Slug        string `json:"slug"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Author      string `json:"author"`
	Version     string `json:"version"`
	Downloads   int    `json:"downloads"`
	Source      string `json:"source"`
}

func searchExtensions(
	ctx context.Context,
	deps commandDeps,
	query string,
	limit int,
) ([]marketplaceExtensionSearchItem, error) {
	if limit <= 0 {
		return nil, fmt.Errorf("cli: search limit must be positive: %d", limit)
	}
	client, err := requireExtensionDaemonClient(ctx, deps)
	if err != nil {
		return nil, err
	}
	response, err := client.BrowseMarketplace(
		ctx,
		"extension",
		query,
		limit,
		"",
		MarketplaceReadScope{Scope: contract.SettingsWorkspaceScopeGlobal},
	)
	if err != nil {
		return nil, err
	}
	items := make([]marketplaceExtensionSearchItem, 0, len(response.Items))
	for _, listing := range response.Items {
		installSlug := strings.TrimSpace(listing.InstallSlug)
		if installSlug == "" {
			return nil, fmt.Errorf("cli: marketplace extension %q has no install slug", listing.EntryID)
		}
		downloads := 0
		if listing.Downloads != nil {
			downloads = *listing.Downloads
		}
		items = append(items, marketplaceExtensionSearchItem{
			Slug: installSlug, Name: listing.Name, Description: listing.Description,
			Author: listing.Author, Version: listing.Version, Downloads: downloads, Source: listing.Source,
		})
	}
	return items, nil
}

func installMarketplaceExtension(
	ctx context.Context,
	deps commandDeps,
	slug string,
	sourceFilter string,
	version string,
	asset string,
	allowUnverified bool,
) (ExtensionRecord, error) {
	client, err := requireExtensionDaemonClient(ctx, deps)
	if err != nil {
		return ExtensionRecord{}, err
	}
	item, err := client.InstallExtension(ctx, InstallExtensionRequest{
		Slug:            strings.TrimSpace(slug),
		Source:          strings.TrimSpace(sourceFilter),
		Version:         strings.TrimSpace(version),
		Asset:           strings.TrimSpace(asset),
		AllowUnverified: allowUnverified,
	})
	if err != nil {
		return ExtensionRecord{}, err
	}
	return item, nil
}

func removeInstalledExtension(
	ctx context.Context,
	deps commandDeps,
	name string,
) (extensionRemoveItem, error) {
	client, err := requireExtensionDaemonClient(ctx, deps)
	if err != nil {
		return extensionRemoveItem{}, err
	}
	return client.RemoveExtension(ctx, name)
}

func updateMarketplaceExtensions(
	ctx context.Context,
	deps commandDeps,
	args []string,
	updateAll bool,
	checkOnly bool,
	version string,
	allowUnverified bool,
) ([]extensionUpdateItem, error) {
	client, err := requireExtensionDaemonClient(ctx, deps)
	if err != nil {
		return nil, err
	}
	targets, err := selectDaemonMarketplaceExtensionsForUpdate(ctx, client, args, updateAll)
	if err != nil {
		return nil, err
	}
	items := make([]extensionUpdateItem, 0, len(targets))
	for _, target := range targets {
		updated, err := client.UpdateExtension(ctx, target.Name, UpdateExtensionRequest{
			Version:         strings.TrimSpace(version),
			CheckOnly:       checkOnly,
			AllowUnverified: allowUnverified && !checkOnly,
		})
		if err != nil {
			return nil, err
		}
		items = append(items, daemonExtensionUpdateItem(target, updated, checkOnly))
	}
	return items, nil
}

func selectDaemonMarketplaceExtensionsForUpdate(
	ctx context.Context,
	client DaemonClient,
	args []string,
	updateAll bool,
) ([]ExtensionRecord, error) {
	if updateAll {
		items, err := client.ListExtensions(ctx)
		if err != nil {
			return nil, err
		}
		selected := make([]ExtensionRecord, 0, len(items))
		for _, item := range items {
			if extensionRecordMarketplaceSlug(item) != "" {
				selected = append(selected, item)
			}
		}
		return selected, nil
	}
	name := ""
	if len(args) > 0 {
		name = strings.TrimSpace(args[0])
	}
	if name == "" {
		return nil, errors.New("cli: extension name is required unless --all is set")
	}
	item, err := client.ExtensionStatus(ctx, name)
	if err != nil {
		return nil, err
	}
	if extensionRecordMarketplaceSlug(item) == "" {
		return nil, fmt.Errorf("cli: extension %q is not a marketplace-installed extension", item.Name)
	}
	return []ExtensionRecord{item}, nil
}

func daemonExtensionUpdateItem(
	before ExtensionRecord,
	after ExtensionUpdateRecord,
	checkOnly bool,
) extensionUpdateItem {
	status := after.Status
	if status == "" && checkOnly {
		status = skillUpdateStatusCurrent
	}
	if status == "" {
		status = skillUpdateStatusUpdated
	}
	return extensionUpdateItem{
		Name:           firstNonEmpty(after.Name, before.Name),
		Slug:           firstNonEmpty(after.Slug, extensionRecordMarketplaceSlug(before)),
		Registry:       firstNonEmpty(after.Registry, extensionRecordMarketplaceRegistry(before)),
		CurrentVersion: firstNonEmpty(after.CurrentVersion, before.Version),
		LatestVersion:  firstNonEmpty(after.LatestVersion, before.Version),
		Path:           after.Path,
		Status:         status,
		Warnings:       append([]contract.DiagnosticItem(nil), after.Warnings...),
	}
}

func extensionRecordMarketplaceSlug(item ExtensionRecord) string {
	if item.Provenance == nil || item.Provenance.InstalledFrom != extensionpkg.ExtensionInstalledFromMarketplace {
		return ""
	}
	return strings.TrimSpace(item.Provenance.Slug)
}

func extensionRecordMarketplaceRegistry(item ExtensionRecord) string {
	if item.Provenance == nil {
		return ""
	}
	return strings.TrimSpace(item.Provenance.RegistryTier)
}

func extensionSearchBundle(items []marketplaceExtensionSearchItem) outputBundle {
	return listBundle(
		items,
		items,
		"Extension Registry Results",
		[]string{
			extensionMarketplaceSlugValue,
			automationNameValue,
			extensionMarketplaceDescriptionValue,
			"Author",
			versionValue,
			"Downloads",
			authoredContextSourceValue,
		},
		"extensions",
		[]string{
			extensionMarketplaceSlugKey,
			automationNameKey,
			extensionMarketplaceDescriptionKey,
			"author",
			versionKey,
			"downloads",
			automationSourceKey,
		},
		func(item marketplaceExtensionSearchItem) []string {
			return []string{
				stringOrDash(item.Slug),
				stringOrDash(item.Name),
				stringOrDash(item.Description),
				stringOrDash(item.Author),
				stringOrDash(item.Version),
				strconv.Itoa(item.Downloads),
				stringOrDash(item.Source),
			}
		},
		func(item marketplaceExtensionSearchItem) []string {
			return []string{
				item.Slug,
				item.Name,
				item.Description,
				item.Author,
				item.Version,
				strconv.Itoa(item.Downloads),
				item.Source,
			}
		},
	)
}

func extensionRemoveBundle(item extensionRemoveItem) outputBundle {
	return outputBundle{
		jsonValue: item,
		human: func() (string, error) {
			return renderHumanSection("Extension Remove", []keyValue{
				{Label: automationNameValue, Value: stringOrDash(item.Name)},
				{Label: extensionMarketplacePathValue, Value: stringOrDash(item.Path)},
				{Label: extensionMarketplaceStatusValue, Value: stringOrDash(item.Status)},
			}), nil
		},
		toon: func() (string, error) {
			return renderToonObject(
				"extension_remove",
				[]string{automationNameKey, extensionMarketplacePathKey, automationStatusKey},
				[]string{
					item.Name,
					item.Path,
					item.Status,
				},
			), nil
		},
	}
}

func extensionUpdateBundle(items []extensionUpdateItem) outputBundle {
	return listBundle(
		items,
		items,
		"Extension Updates",
		[]string{
			automationNameValue,
			extensionMarketplaceSlugValue,
			"Registry",
			"Current",
			"Latest",
			extensionMarketplacePathValue,
			extensionMarketplaceStatusValue,
			"Warnings",
		},
		"extension_updates",
		[]string{
			automationNameKey,
			extensionMarketplaceSlugKey,
			skillOutputRegistryKey,
			extensionMarketplaceCurrentVersionKey,
			"latest_version",
			extensionMarketplacePathKey,
			automationStatusKey,
			"warnings",
		},
		func(item extensionUpdateItem) []string {
			return []string{
				stringOrDash(item.Name),
				stringOrDash(item.Slug),
				stringOrDash(item.Registry),
				stringOrDash(item.CurrentVersion),
				stringOrDash(item.LatestVersion),
				stringOrDash(item.Path),
				stringOrDash(item.Status),
				stringOrDash(extensionUpdateWarningCodes(item.Warnings)),
			}
		},
		func(item extensionUpdateItem) []string {
			return []string{
				item.Name,
				item.Slug,
				item.Registry,
				item.CurrentVersion,
				item.LatestVersion,
				item.Path,
				item.Status,
				extensionUpdateWarningCodes(item.Warnings),
			}
		},
	)
}

func extensionUpdateWarningCodes(warnings []contract.DiagnosticItem) string {
	codes := make([]string, 0, len(warnings))
	for _, warning := range warnings {
		code := strings.TrimSpace(warning.Code)
		if code != "" {
			codes = append(codes, code)
		}
	}
	return strings.Join(codes, ", ")
}
