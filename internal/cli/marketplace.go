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
			response, err := client.SearchMarketplace(cmd.Context(), query, limit, cursor, scope)
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, marketplaceCatalogBundle(response))
		},
	}
	cmd.Flags().StringVar(&cursor, "cursor", "", "Continue catalog search from an opaque cursor")
	cmd.Flags().IntVar(&limit, "limit", marketplaceDefaultLimit, "Maximum catalog results")
	addMarketplaceReadFlags(cmd, &readScope)
	return cmd
}

func newMarketplaceInfoCommand(deps commandDeps) *cobra.Command {
	var installedName string
	var source string
	var readScope marketplaceReadFlagValues
	cmd := &cobra.Command{
		Use:   "info <entry_id>",
		Short: "Show one marketplace entry",
		Args:  exactOneNonBlankArg(),
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
				cmd.Context(), args[0], source, installedName, scope,
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
		"Resolve an exact installed extension identity",
	)
	cmd.Flags().StringVar(&source, "source", "", "Catalog source name")
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
		profiles, ok := client.(profileResolutionClient)
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
	cmd := &cobra.Command{
		Use:   "refresh",
		Short: "Refresh the Marketplace catalog",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}
			response, err := client.RefreshMarketplace(cmd.Context())
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, marketplaceRefreshBundle(response))
		},
	}
	return cmd
}

func marketplaceListingsBundle(jsonValue any, items []MarketplaceListingRecord) outputBundle {
	bundle := listBundle(
		jsonValue, items, "Marketplace Results",
		[]string{authoredContextSourceValue, "Entry", versionValue, cliInstalledValue, "Description"},
		marketplaceSkillSource,
		[]string{automationSourceKey, "entry_id", versionKey, marketplaceInstalledKey, "description"},
		func(item MarketplaceListingRecord) []string {
			return []string{
				item.Source,
				item.EntryID,
				stringOrDash(item.Version),
				strconv.FormatBool(item.Installed),
				item.Description,
			}
		},
		func(item MarketplaceListingRecord) []string {
			return []string{
				item.Source,
				item.EntryID,
				item.Version,
				strconv.FormatBool(item.Installed),
				item.Description,
			}
		},
	)
	bundle.json = func(cmd *cobra.Command) error {
		return writeJSONWithoutWorkspaceResolution(cmd, jsonValue)
	}
	return bundle
}

type marketplaceCatalogPageRecord struct {
	Type       string `json:"type"`
	Revision   string `json:"revision"`
	Returned   int    `json:"returned"`
	Total      int    `json:"total"`
	NextCursor string `json:"next_cursor,omitempty"`
	Stale      bool   `json:"stale"`
	ErrorClass string `json:"error_class,omitempty"`
	Error      string `json:"error,omitempty"`
}

func marketplaceCatalogBundle(response MarketplaceListRecord) outputBundle {
	bundle := marketplaceListingsBundle(response, response.Items)
	page := marketplaceCatalogPageRecord{
		Type: listPageRecordType, Revision: response.Revision, Returned: len(response.Items),
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
		return renderHumanBlocks(table, marketplaceCatalogPageHuman(page)), nil
	}
	baseToon := bundle.toon
	bundle.toon = func() (string, error) {
		items, err := baseToon()
		if err != nil {
			return "", err
		}
		return items + "\n" + marketplaceCatalogPageToon(page), nil
	}
	return bundle
}

func marketplaceCatalogPageHuman(page marketplaceCatalogPageRecord) string {
	return renderHumanSection("Page", []keyValue{
		{Label: "Revision", Value: page.Revision},
		{Label: "Returned", Value: strconv.Itoa(page.Returned)},
		{Label: listTotalLabel, Value: strconv.Itoa(page.Total)},
		{Label: listNextCursorLabel, Value: stringOrDash(page.NextCursor)},
		{Label: outputStaleValue, Value: strconv.FormatBool(page.Stale)},
		{Label: "Error Class", Value: stringOrDash(page.ErrorClass)},
		{Label: "Error", Value: stringOrDash(page.Error)},
	})
}

func marketplaceCatalogPageToon(page marketplaceCatalogPageRecord) string {
	return renderToonObject(
		listPageRecordType,
		[]string{
			"revision", listReturnedField, listTotalField, listNextCursorField,
			outputStaleKey, "error_class", automationErrorKey,
		},
		[]string{
			page.Revision, strconv.Itoa(page.Returned), strconv.Itoa(page.Total), page.NextCursor,
			strconv.FormatBool(page.Stale), page.ErrorClass, page.Error,
		},
	)
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
					"entry_id",
					automationNameKey,
					extensionMarketplaceDescriptionKey,
					versionKey,
					automationSourceKey,
					marketplaceInstalledKey,
				},
				[]string{
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
		response.Sources,
		"Marketplace Refresh",
		[]string{"Source", "Outcome", "Entries", outputStaleValue, "Error Class"},
		"marketplace_refresh",
		[]string{"source", cliOutcomeKey, "entry_count", outputStaleKey, "error_class"},
		func(item contract.MarketplaceRefreshSourcePayload) []string {
			return []string{
				item.Source,
				item.Outcome,
				strconv.Itoa(item.EntryCount),
				strconv.FormatBool(item.Stale),
				stringOrDash(item.ErrorClass),
			}
		},
		func(item contract.MarketplaceRefreshSourcePayload) []string {
			return []string{
				item.Source,
				item.Outcome,
				strconv.Itoa(item.EntryCount),
				strconv.FormatBool(item.Stale),
				item.ErrorClass,
			}
		},
	)
}
