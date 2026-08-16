package cli

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	extensionpkg "github.com/compozy/compozy/internal/extension"
	"github.com/spf13/cobra"
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
	extensionMarketplaceTierValue         = "Tier"
	extensionMarketplaceCurrentVersionKey = "current_version"
	extensionMarketplaceDescriptionKey    = "description"
	extensionMarketplacePathKey           = "path"
	extensionMarketplaceTierKey           = "tier"
)

const (
	defaultExtensionRegistrySearchLimit = 20
)

type extensionRemoveItem = ManagedExtensionRemoveRecord

type extensionUpdateItem = ExtensionUpdateRecord

type extensionSearchItem = contract.ExtensionSearchItem

type extensionSearchPageRecord struct {
	Type            string   `json:"type"`
	Returned        int      `json:"returned"`
	NextCursor      string   `json:"next_cursor,omitempty"`
	SourcesDegraded []string `json:"sources_degraded,omitempty"`
}

type extensionUpdateOptions struct {
	Names                []string
	All                  bool
	CheckOnly            bool
	Version              string
	AllowUnverified      bool
	ConfirmNetworkDigest string
}

type extensionPortableUpdateOutput struct {
	Update   extensionUpdateItem
	Report   *extensionpkg.ValidationReport
	DataPath string
}

func extensionPortableUpdatePresentation(
	deps commandDeps,
	item extensionUpdateItem,
) (extensionPortableUpdateOutput, bool, error) {
	report, err := extensionpkg.ValidateBundleReport(item.Path)
	if err != nil {
		return extensionPortableUpdateOutput{}, false, nil
	}
	if report.Format != string(extensionpkg.FormatAgentPlugin) {
		return extensionPortableUpdateOutput{}, false, nil
	}
	homePaths, err := deps.resolveHome()
	if err != nil {
		return extensionPortableUpdateOutput{}, false, nil
	}
	dataPath, err := homePaths.ExtensionDataPath(item.Name, "")
	if err != nil {
		return extensionPortableUpdateOutput{}, false, nil
	}
	return extensionPortableUpdateOutput{Update: item, Report: report, DataPath: dataPath}, true, nil
}

func searchExtensionsPage(
	ctx context.Context,
	deps commandDeps,
	query string,
	sources []string,
	limit int,
	cursor string,
) (ExtensionSearchRecord, error) {
	if limit <= 0 {
		return ExtensionSearchRecord{}, fmt.Errorf("cli: search limit must be positive: %d", limit)
	}
	client, err := requireExtensionDaemonClient(ctx, deps)
	if err != nil {
		return ExtensionSearchRecord{}, err
	}
	response, err := client.SearchExtensions(ctx, ExtensionSearchRequest{
		Query: strings.TrimSpace(query), Sources: sources, Limit: limit, Cursor: strings.TrimSpace(cursor),
	})
	if err != nil {
		return ExtensionSearchRecord{}, err
	}
	response.Items = append([]extensionSearchItem(nil), response.Items...)
	response.SourcesDegraded = append([]string(nil), response.SourcesDegraded...)
	return response, nil
}

func updateMarketplaceExtensions(
	ctx context.Context,
	deps commandDeps,
	options extensionUpdateOptions,
) ([]extensionUpdateItem, error) {
	client, err := requireExtensionDaemonClient(ctx, deps)
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(options.Names))
	for _, name := range options.Names {
		if name = strings.TrimSpace(name); name != "" {
			names = append(names, name)
		}
	}
	if !options.All && len(names) == 1 {
		item, err := client.UpdateExtension(ctx, names[0], UpdateExtensionRequest{
			Version: strings.TrimSpace(options.Version), CheckOnly: options.CheckOnly,
			AllowUnverified:      options.AllowUnverified && !options.CheckOnly,
			ConfirmNetworkDigest: strings.TrimSpace(options.ConfirmNetworkDigest),
		})
		if err != nil {
			return nil, err
		}
		return []extensionUpdateItem{item}, nil
	}
	items, err := client.UpdateExtensions(ctx, UpdateExtensionsRequest{
		Names: names, All: options.All, Version: strings.TrimSpace(options.Version), CheckOnly: options.CheckOnly,
		AllowUnverified: options.AllowUnverified && !options.CheckOnly,
	})
	if err != nil {
		return nil, err
	}
	return append([]extensionUpdateItem(nil), items...), nil
}

func extensionSearchBundle(response ExtensionSearchRecord) outputBundle {
	bundle := listBundle(
		response.Items,
		response.Items,
		"Extension Registry Results",
		[]string{
			extensionMarketplaceSlugValue,
			automationNameValue,
			extensionMarketplaceDescriptionValue,
			"Author",
			versionValue,
			extensionUpdateValue,
			"Downloads",
			authoredContextSourceValue,
			extensionMarketplaceTierValue,
			"Integrity",
		},
		"extensions",
		[]string{
			extensionMarketplaceSlugKey,
			automationNameKey,
			extensionMarketplaceDescriptionKey,
			"author",
			versionKey,
			"update_available",
			"downloads",
			automationSourceKey,
			extensionMarketplaceTierKey,
			"integrity",
		},
		func(item extensionSearchItem) []string {
			return []string{
				stringOrDash(item.Slug),
				stringOrDash(item.Name),
				stringOrDash(item.Description),
				stringOrDash(item.Author),
				stringOrDash(item.Version),
				stringOrDash(extensionUpdateLabel(item.UpdateAvailable, item.Version)),
				strconv.Itoa(item.Downloads),
				stringOrDash(item.Source),
				stringOrDash(item.Tier),
				stringOrDash(item.Integrity),
			}
		},
		func(item extensionSearchItem) []string {
			return []string{
				item.Slug,
				item.Name,
				item.Description,
				item.Author,
				item.Version,
				extensionUpdateLabel(item.UpdateAvailable, item.Version),
				strconv.Itoa(item.Downloads),
				item.Source,
				item.Tier,
				item.Integrity,
			}
		},
	)
	return withExtensionSearchPage(bundle, response)
}

func withExtensionSearchPage(bundle outputBundle, response ExtensionSearchRecord) outputBundle {
	page := extensionSearchPageRecord{
		Type: listPageRecordType, Returned: len(response.Items), NextCursor: response.NextCursor,
		SourcesDegraded: append([]string(nil), response.SourcesDegraded...),
	}
	bundle.jsonValue = response
	bundle.jsonl = func(cmd *cobra.Command) error {
		if err := writeJSONLines(cmd, response.Items); err != nil {
			return err
		}
		return writeJSONLine(cmd, page)
	}
	baseHuman := bundle.human
	bundle.human = func() (string, error) {
		table, err := baseHuman()
		if err != nil {
			return "", err
		}
		return renderHumanBlocks(table, extensionSearchPageHuman(page)), nil
	}
	baseToon := bundle.toon
	bundle.toon = func() (string, error) {
		items, err := baseToon()
		if err != nil {
			return "", err
		}
		return items + "\n" + extensionSearchPageToon(page), nil
	}
	return bundle
}

func extensionSearchPageHuman(page extensionSearchPageRecord) string {
	return renderHumanSection("Page", []keyValue{
		{Label: "Returned", Value: strconv.Itoa(page.Returned)},
		{Label: listNextCursorLabel, Value: stringOrDash(page.NextCursor)},
		{Label: "Sources Degraded", Value: stringOrDash(strings.Join(page.SourcesDegraded, ", "))},
	})
}

func extensionSearchPageToon(page extensionSearchPageRecord) string {
	return renderToonObject(
		listPageRecordType,
		[]string{listReturnedField, listNextCursorField, "sources_degraded"},
		[]string{strconv.Itoa(page.Returned), page.NextCursor, strings.Join(page.SourcesDegraded, ",")},
	)
}

func extensionRemoveBundle(item extensionRemoveItem) outputBundle {
	return outputBundle{
		jsonValue: item,
		jsonl: func(cmd *cobra.Command) error {
			return writeJSONLine(cmd, item)
		},
		human: func() (string, error) {
			blocks := []string{
				fmt.Sprintf("✓ remove %s", strings.TrimSpace(item.Name)),
				"removed: " + strings.TrimSpace(item.Path),
			}
			if strings.TrimSpace(item.DataPath) != "" && strings.TrimSpace(item.QuarantinePath) == "" {
				blocks = append(blocks, "removed: "+strings.TrimSpace(item.DataPath))
			}
			if strings.TrimSpace(item.QuarantinePath) != "" {
				blocks = append(blocks, fmt.Sprintf(
					"warning: data directory could not be removed; quarantined as %s — "+
						"a fresh install of %s starts empty",
					strings.TrimSpace(item.QuarantinePath),
					strings.TrimSpace(item.Name),
				))
			}
			return renderHumanBlocks(blocks...), nil
		},
		toon: func() (string, error) {
			return renderToonObject(
				"extension_remove",
				[]string{
					automationNameKey,
					extensionMarketplacePathKey,
					"data_path",
					"quarantine_path",
					automationStatusKey,
				},
				[]string{
					item.Name,
					item.Path,
					item.DataPath,
					item.QuarantinePath,
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
