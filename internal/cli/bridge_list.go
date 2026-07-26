package cli

import (
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/compozy/agh/internal/api/contract"
	bridgepkg "github.com/compozy/agh/internal/bridges"
	"github.com/spf13/cobra"
)

type bridgeListJSONLItemRecord struct {
	Type   string                       `json:"type"`
	Bridge contract.BridgePayload       `json:"bridge"`
	Health contract.BridgeHealthPayload `json:"health"`
}

func newBridgeListCommand(deps commandDeps) *cobra.Command {
	query := BridgeListQuery{}
	cmd := &cobra.Command{
		Use:   bridgeListKey,
		Short: "List bridge instances",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if strings.TrimSpace(query.WorkspaceID) != "" && strings.TrimSpace(query.Workspace) != "" {
				return errors.New("cli: --workspace-id and --workspace cannot be combined")
			}
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}
			result, err := client.ListBridges(cmd.Context(), query)
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, bridgeListBundle(result, deps.now))
		},
	}
	cmd.Flags().StringVar(&query.Scope, bridgeScopeKey, "", "Filter by scope: all, global, or workspace")
	cmd.Flags().StringVar(&query.WorkspaceID, "workspace-id", "", "Filter by active workspace ID")
	cmd.Flags().StringVar(&query.Workspace, "workspace", "", "Filter by workspace ID, name, or path")
	cmd.Flags().StringVar(&query.Search, "q", "", "Search display name, platform, extension, or status")
	cmd.Flags().StringVar(&query.Platform, "platform", "", "Filter by messaging platform")
	cmd.Flags().StringVar(&query.Status, bridgeStatusKey, "", "Filter by effective runtime status")
	cmd.Flags().StringVar(&query.Sort, "sort", bridgepkg.BridgeCatalogSortName, "Stable ordering: name")
	cmd.Flags().StringVar(&query.Cursor, "cursor", "", "Continue from a bridge catalog cursor")
	cmd.Flags().IntVar(&query.Limit, "limit", 0, "Maximum bridges to return (default 50, max 200)")
	return cmd
}

func bridgeListBundle(result BridgeListRecord, now func() time.Time) outputBundle {
	bundle := bridgeListTableBundle(result, now)
	bundle.jsonl = bridgeListJSONLWriter(result)
	bundle.human = bridgeListHumanRenderer(bundle.human, result.Page)
	bundle.toon = bridgeListToonRenderer(bundle.toon, result)
	return bundle
}

func bridgeListTableBundle(result BridgeListRecord, now func() time.Time) outputBundle {
	return listBundle(
		result,
		result.Bridges,
		"Bridges",
		[]string{
			"ID",
			automationNameValue,
			bundlePlatformValue,
			bridgeExtensionValue,
			automationScopeValue,
			configWorkspaceValue,
			automationStatusValue,
			"Routing",
			authoredContextUpdatedValue,
		},
		"bridges",
		[]string{
			"id",
			bridgeDisplayNameKey,
			bridgePlatformKey,
			bridgeSetupExtensionNameKey,
			bridgeScopeKey,
			bridgeWorkspaceIDKey,
			bridgeStatusKey,
			"routing",
			bridgeUpdatedAtKey,
		},
		func(item contract.BridgePayload) []string {
			return []string{
				stringOrDash(item.ID),
				stringOrDash(item.DisplayName),
				stringOrDash(item.Platform),
				stringOrDash(item.ExtensionName),
				stringOrDash(string(item.Scope)),
				stringOrDash(item.WorkspaceID),
				stringOrDash(string(bridgeListEffectiveStatus(result, item))),
				stringOrDash(bridgeRoutingPolicyLabel(item.RoutingPolicy)),
				stringOrDash(formatAge(now, item.UpdatedAt)),
			}
		},
		func(item contract.BridgePayload) []string {
			return []string{
				item.ID,
				item.DisplayName,
				item.Platform,
				item.ExtensionName,
				string(item.Scope),
				item.WorkspaceID,
				string(bridgeListEffectiveStatus(result, item)),
				bridgeRoutingPolicyLabel(item.RoutingPolicy),
				formatTime(item.UpdatedAt),
			}
		},
	)
}

func bridgeListJSONLWriter(result BridgeListRecord) func(*cobra.Command) error {
	return func(cmd *cobra.Command) error {
		for _, item := range result.Bridges {
			health, ok := result.BridgeHealth[item.ID]
			if !ok {
				health = contract.BridgeHealthPayload{
					BridgeInstanceID: item.ID,
					Status:           item.Status,
				}
			}
			if err := writeJSONLine(cmd, bridgeListJSONLItemRecord{
				Type:   bridgeBridgeKey,
				Bridge: item,
				Health: health,
			}); err != nil {
				return err
			}
		}
		return writeJSONLine(cmd, struct {
			Type   string                              `json:"type"`
			Page   contract.CountedCursorPagePayload   `json:"page"`
			Facets contract.BridgeCatalogFacetsPayload `json:"facets"`
		}{Type: listPageRecordType, Page: result.Page, Facets: result.Facets})
	}
}

func bridgeListHumanRenderer(
	base func() (string, error),
	page contract.CountedCursorPagePayload,
) func() (string, error) {
	return func() (string, error) {
		table, err := base()
		if err != nil {
			return "", err
		}
		return renderHumanBlocks(table, bridgePageHuman(page)), nil
	}
}

func bridgeListToonRenderer(
	base func() (string, error),
	result BridgeListRecord,
) func() (string, error) {
	return func() (string, error) {
		itemsToon, err := base()
		if err != nil {
			return "", err
		}
		pageToon := renderToonObject(
			"page",
			[]string{
				listReturnedField,
				listTotalField,
				listLimitField,
				listHasMoreField,
				listNextCursorField,
			},
			[]string{
				strconv.Itoa(len(result.Bridges)), strconv.Itoa(result.Page.Total),
				strconv.Itoa(result.Page.Limit), strconv.FormatBool(result.Page.HasMore),
				result.Page.NextCursor,
			},
		)
		return itemsToon + "\n" + pageToon, nil
	}
}

func bridgePageHuman(page contract.CountedCursorPagePayload) string {
	return renderHumanSection("Page", []keyValue{
		{Label: listTotalLabel, Value: strconv.Itoa(page.Total)},
		{Label: listLimitLabel, Value: strconv.Itoa(page.Limit)},
		{Label: listHasMoreLabel, Value: strconv.FormatBool(page.HasMore)},
		{Label: listNextCursorLabel, Value: stringOrDash(page.NextCursor)},
	})
}

func bridgeListEffectiveStatus(
	result BridgeListRecord,
	item contract.BridgePayload,
) bridgepkg.BridgeStatus {
	if health, ok := result.BridgeHealth[item.ID]; ok && health.Status.Normalize() != "" {
		return health.Status.Normalize()
	}
	return item.Status.Normalize()
}
