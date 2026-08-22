package cli

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/spf13/cobra"
)

const (
	marketplaceDefaultLimit = 20
	marketplaceInstalledKey = "installed"
	marketplaceScopeKey     = "scope"
	outputStaleKey          = "stale"
	outputStaleValue        = "Stale"
)

func newMarketplaceCommand(deps commandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   marketplaceSkillSource,
		Short: "Discover installable CompozyOS capabilities",
	}
	cmd.AddCommand(newMarketplaceSearchCommand(deps))
	cmd.AddCommand(newMarketplaceInfoCommand(deps))
	cmd.AddCommand(newMarketplaceRefreshCommand(deps))
	return cmd
}

func newMarketplaceSearchCommand(deps commandDeps) *cobra.Command {
	var kind string
	var cursor string
	var readScope marketplaceReadFlagValues
	limit := marketplaceDefaultLimit
	cmd := &cobra.Command{
		Use:   "search [query]",
		Short: "Search or browse the marketplace",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if limit <= 0 {
				return fmt.Errorf("cli: marketplace limit must be positive: %d", limit)
			}
			if strings.TrimSpace(kind) == "" && strings.TrimSpace(cursor) != "" {
				return errors.New("cli: --cursor requires --kind")
			}
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}
			scope, err := readScope.resolve(cmd, deps, client)
			if err != nil {
				return err
			}
			query := ""
			if len(args) > 0 {
				query = args[0]
			}
			if strings.TrimSpace(kind) == "" {
				response, err := client.SearchMarketplace(cmd.Context(), query, limit, scope)
				if err != nil {
					return err
				}
				return writeCommandOutput(cmd, marketplaceGroupedBundle(response))
			}
			response, err := client.BrowseMarketplace(cmd.Context(), kind, query, limit, cursor, scope)
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, marketplaceKindBundle(response))
		},
	}
	cmd.Flags().StringVar(&kind, "kind", "", "Limit results to mcp, extension, or skill")
	cmd.Flags().StringVar(&cursor, "cursor", "", "Continue a single-kind search from an opaque cursor")
	cmd.Flags().IntVar(&limit, "limit", marketplaceDefaultLimit, "Maximum results per marketplace kind")
	addMarketplaceReadFlags(cmd, &readScope)
	return cmd
}

func newMarketplaceInfoCommand(deps commandDeps) *cobra.Command {
	var installedName string
	var readScope marketplaceReadFlagValues
	cmd := &cobra.Command{
		Use:   "info <kind> <entry_id>",
		Short: "Show one marketplace entry",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}
			scope, err := readScope.resolve(cmd, deps, client)
			if err != nil {
				return err
			}
			response, err := client.MarketplaceInfo(
				cmd.Context(), args[0], args[1], installedName, scope,
			)
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, marketplaceEntryBundle(response))
		},
	}
	cmd.Flags().StringVar(
		&installedName,
		"installed-name",
		"",
		"Resolve an exact installed MCP, extension, or skill identity",
	)
	addMarketplaceReadFlags(cmd, &readScope)
	return cmd
}

type marketplaceReadFlagValues struct {
	scope       string
	workspaceID string
}

func (v marketplaceReadFlagValues) resolve(
	cmd *cobra.Command,
	deps commandDeps,
	client workspaceLookupClient,
) (MarketplaceReadScope, error) {
	result := MarketplaceReadScope{
		Scope:       contract.SettingsLayeredScopeKind(strings.TrimSpace(v.scope)),
		WorkspaceID: strings.TrimSpace(v.workspaceID),
	}
	if result.Scope == contract.SettingsLayeredScopeWorkspace {
		resolution, err := resolveCommandWorkspace(
			cmd.Context(),
			cmd,
			deps,
			client,
			workspaceResolutionRequest{FlagRef: result.WorkspaceID},
		)
		if err != nil {
			return MarketplaceReadScope{}, err
		}
		result.WorkspaceID = resolution.ID
	}
	if result.Scope == contract.SettingsLayeredScopeProfile {
		profiles, ok := client.(profileClientAPI)
		if !ok {
			return MarketplaceReadScope{}, errors.New("cli: profile catalog is unavailable")
		}
		resolution, err := resolveCommandProfile(cmd.Context(), cmd, deps, profiles, client)
		if err != nil {
			return MarketplaceReadScope{}, err
		}
		result.Profile = strings.TrimSpace(resolution.Profile.Name)
	}
	if _, err := result.queryValues(); err != nil {
		return MarketplaceReadScope{}, err
	}
	return result, nil
}

func addMarketplaceReadFlags(cmd *cobra.Command, values *marketplaceReadFlagValues) {
	cmd.Flags().StringVar(
		&values.scope,
		marketplaceScopeKey,
		string(contract.SettingsLayeredScopeUser),
		"Installed-state scope: user, profile, or workspace",
	)
	cmd.Flags().
		StringVar(&values.workspaceID, "workspace", "", "Override workspace scope (ID, name, or path)")
}

func newMarketplaceRefreshCommand(deps commandDeps) *cobra.Command {
	var kind string
	cmd := &cobra.Command{
		Use:   "refresh",
		Short: "Refresh curated marketplace catalogs",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}
			response, err := client.RefreshMarketplace(cmd.Context(), kind)
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, marketplaceRefreshBundle(response))
		},
	}
	cmd.Flags().StringVar(&kind, "kind", "", "Refresh only mcp, extension, or skill")
	return cmd
}

func marketplaceGroupedBundle(response MarketplaceSearchRecord) outputBundle {
	items := make([]MarketplaceListingRecord, 0)
	for _, result := range response.Kinds {
		items = append(items, result.Items...)
	}
	bundle := marketplaceListingsBundle(response, items)
	bundle.jsonl = func(cmd *cobra.Command) error {
		return writeJSONLines(cmd, response.Kinds)
	}
	return bundle
}

func marketplaceListingsBundle(jsonValue any, items []MarketplaceListingRecord) outputBundle {
	bundle := listBundle(
		jsonValue,
		items,
		"Marketplace Results",
		[]string{
			cliKindValue,
			"Entry ID",
			automationNameValue,
			versionValue,
			cliInstalledValue,
			authoredContextSourceValue,
		},
		marketplaceSkillSource,
		[]string{
			networkKindKey,
			"entry_id",
			automationNameKey,
			versionKey,
			marketplaceInstalledKey,
			automationSourceKey,
		},
		func(item MarketplaceListingRecord) []string {
			return []string{
				string(item.Kind),
				item.EntryID,
				item.Name,
				stringOrDash(item.Version),
				strconv.FormatBool(item.Installed),
				item.Source,
			}
		},
		func(item MarketplaceListingRecord) []string {
			return []string{
				string(item.Kind),
				item.EntryID,
				item.Name,
				item.Version,
				strconv.FormatBool(item.Installed),
				item.Source,
			}
		},
	)
	bundle.json = func(cmd *cobra.Command) error {
		return writeJSONWithoutWorkspaceResolution(cmd, jsonValue)
	}
	return bundle
}

type marketplaceKindPageRecord struct {
	Type       string `json:"type"`
	Kind       string `json:"kind"`
	Returned   int    `json:"returned"`
	Total      *int   `json:"total,omitempty"`
	NextCursor string `json:"next_cursor,omitempty"`
	Stale      bool   `json:"stale"`
	ErrorClass string `json:"error_class,omitempty"`
	Error      string `json:"error,omitempty"`
}

func marketplaceKindBundle(response MarketplaceKindRecord) outputBundle {
	bundle := marketplaceListingsBundle(response, response.Items)
	page := marketplaceKindPageRecord{
		Type: listPageRecordType, Kind: string(response.Kind), Returned: len(response.Items),
		Total: response.Total, NextCursor: response.NextCursor, Stale: response.Stale,
		ErrorClass: response.ErrorClass, Error: response.Error,
	}
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
		return renderHumanBlocks(table, marketplaceKindPageHuman(page)), nil
	}
	baseToon := bundle.toon
	bundle.toon = func() (string, error) {
		items, err := baseToon()
		if err != nil {
			return "", err
		}
		return items + "\n" + marketplaceKindPageToon(page), nil
	}
	return bundle
}

func marketplaceKindPageHuman(page marketplaceKindPageRecord) string {
	return renderHumanSection("Page", []keyValue{
		{Label: cliKindValue, Value: page.Kind},
		{Label: "Returned", Value: strconv.Itoa(page.Returned)},
		{Label: listTotalLabel, Value: optionalMarketplaceTotal(page.Total)},
		{Label: listNextCursorLabel, Value: stringOrDash(page.NextCursor)},
		{Label: outputStaleValue, Value: strconv.FormatBool(page.Stale)},
		{Label: "Error Class", Value: stringOrDash(page.ErrorClass)},
		{Label: "Error", Value: stringOrDash(page.Error)},
	})
}

func marketplaceKindPageToon(page marketplaceKindPageRecord) string {
	return renderToonObject(
		listPageRecordType,
		[]string{
			networkKindKey, listReturnedField, listTotalField, listNextCursorField,
			outputStaleKey, "error_class", automationErrorKey,
		},
		[]string{
			page.Kind, strconv.Itoa(page.Returned), optionalMarketplaceTotal(page.Total), page.NextCursor,
			strconv.FormatBool(page.Stale), page.ErrorClass, page.Error,
		},
	)
}

func optionalMarketplaceTotal(total *int) string {
	if total == nil {
		return "-"
	}
	return strconv.Itoa(*total)
}

func marketplaceEntryBundle(response MarketplaceEntryRecord) outputBundle {
	entry := response.Entry
	return outputBundle{
		jsonValue: response,
		json: func(cmd *cobra.Command) error {
			return writeJSONWithoutWorkspaceResolution(cmd, response)
		},
		jsonl: func(cmd *cobra.Command) error {
			return writeJSONLine(cmd, response)
		},
		human: func() (string, error) {
			return renderHumanSection("Marketplace Entry", []keyValue{
				{Label: cliKindValue, Value: string(entry.Kind)},
				{Label: "Entry ID", Value: entry.EntryID},
				{Label: automationNameValue, Value: entry.Name},
				{Label: "Description", Value: entry.Description},
				{Label: versionValue, Value: stringOrDash(entry.Version)},
				{Label: authoredContextSourceValue, Value: entry.Source},
				{Label: "Installed", Value: strconv.FormatBool(entry.Installed)},
			}), nil
		},
		toon: func() (string, error) {
			return renderToonObject(
				"marketplace_entry",
				[]string{
					networkKindKey,
					"entry_id",
					automationNameKey,
					extensionMarketplaceDescriptionKey,
					versionKey,
					automationSourceKey,
					marketplaceInstalledKey,
				},
				[]string{
					string(entry.Kind),
					entry.EntryID,
					entry.Name,
					entry.Description,
					entry.Version,
					entry.Source,
					strconv.FormatBool(entry.Installed),
				},
			), nil
		},
	}
}

func marketplaceRefreshBundle(response MarketplaceRefreshRecord) outputBundle {
	return listBundle(
		response,
		response.Kinds,
		"Marketplace Refresh",
		[]string{cliKindValue, "Outcome", "Entries", outputStaleValue, "Error Class"},
		"marketplace_refresh",
		[]string{networkKindKey, cliOutcomeKey, "entry_count", outputStaleKey, "error_class"},
		func(item contract.MarketplaceRefreshKindPayload) []string {
			return []string{
				item.Kind,
				item.Outcome,
				strconv.Itoa(item.EntryCount),
				strconv.FormatBool(item.Stale),
				stringOrDash(item.ErrorClass),
			}
		},
		func(item contract.MarketplaceRefreshKindPayload) []string {
			return []string{
				item.Kind,
				item.Outcome,
				strconv.Itoa(item.EntryCount),
				strconv.FormatBool(item.Stale),
				item.ErrorClass,
			}
		},
	)
}
