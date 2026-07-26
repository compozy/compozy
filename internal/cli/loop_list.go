package cli

import (
	"strconv"
	"strings"

	"github.com/compozy/agh/internal/api/contract"
	looppkg "github.com/compozy/agh/internal/loop"
	"github.com/spf13/cobra"
)

func newLoopListCommand(deps commandDeps) *cobra.Command {
	var workspaceRef string
	query := LoopListQuery{}
	cmd := &cobra.Command{
		Use:   loopListKey,
		Short: "List Loop definitions",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, workspaceID, err := loopClientAndWorkspace(cmd, deps, workspaceRef)
			if err != nil {
				return err
			}
			response, err := client.ListLoops(cmd.Context(), workspaceID, query)
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, loopListBundle(response))
		},
	}
	cmd.Flags().StringVar(&workspaceRef, loopWorkspaceKey, "", "Workspace path, name, or ID")
	cmd.Flags().StringVar(&query.Search, "query", "", "Search Loop name or contract goal")
	cmd.Flags().StringVar(&query.Kind, "kind", "", "Filter by kind: read_only or workspace")
	cmd.Flags().StringVar(&query.Category, "category", "", "Filter by exact catalog category")
	cmd.Flags().StringVar(&query.Status, loopStatusKey, "", "Filter by exact latest-run status")
	cmd.Flags().StringVar(&query.Sort, "sort", looppkg.CatalogSortName, "Stable ordering: name")
	cmd.Flags().StringVar(&query.Cursor, "cursor", "", "Continue from an opaque Loop catalog cursor")
	cmd.Flags().IntVar(&query.Limit, "limit", 0, "Maximum Loops to return (default 50, max 200)")
	mustMarkFlagRequired(cmd, loopWorkspaceKey)
	return cmd
}

func loopListBundle(response contract.LoopsResponse) outputBundle {
	bundle := listBundle(
		response,
		response.Loops,
		"Loops",
		[]string{bundleNameValue, bundleKindValue, "Category", "Last Run", "Runs 30d", "Success 30d"},
		"loops",
		[]string{automationNameKey, bundleKindKey, "category", "last_run_status", "runs_30d", "success_rate_30d"},
		func(item contract.LoopCatalogEntryPayload) []string {
			return []string{
				item.Name,
				string(loopListKind(item)),
				stringOrDash(item.Catalog.Category),
				stringOrDash(loopListLastStatus(item)),
				strconv.Itoa(item.Aggregate30d.Runs),
				strconv.FormatFloat(item.SuccessRate30*100, 'f', 1, 64) + "%",
			}
		},
		func(item contract.LoopCatalogEntryPayload) []string {
			return []string{
				item.Name,
				string(loopListKind(item)),
				item.Catalog.Category,
				loopListLastStatus(item),
				strconv.Itoa(item.Aggregate30d.Runs),
				strconv.FormatFloat(item.SuccessRate30, 'f', -1, 64),
			}
		},
	)
	bundle.jsonl = func(cmd *cobra.Command) error {
		for _, item := range response.Loops {
			if err := writeJSONLine(cmd, item); err != nil {
				return err
			}
		}
		return writeJSONLine(cmd, struct {
			Type   string                            `json:"type"`
			Page   contract.CountedCursorPagePayload `json:"page"`
			Facets contract.LoopCatalogFacetsPayload `json:"facets"`
		}{Type: listPageRecordType, Page: response.Page, Facets: response.Facets})
	}
	baseHuman := bundle.human
	bundle.human = func() (string, error) {
		table, err := baseHuman()
		if err != nil {
			return "", err
		}
		return renderHumanBlocks(table, loopCatalogPageHuman(response.Page)), nil
	}
	baseToon := bundle.toon
	bundle.toon = func() (string, error) {
		items, err := baseToon()
		if err != nil {
			return "", err
		}
		page := renderToonObject(
			listPageRecordType,
			[]string{
				listReturnedField,
				listTotalField,
				listLimitField,
				listHasMoreField,
				listNextCursorField,
			},
			[]string{
				strconv.Itoa(len(response.Loops)),
				strconv.Itoa(response.Page.Total),
				strconv.Itoa(response.Page.Limit),
				strconv.FormatBool(response.Page.HasMore),
				response.Page.NextCursor,
			},
		)
		return items + "\n" + page, nil
	}
	return bundle
}

func loopCatalogPageHuman(page contract.CountedCursorPagePayload) string {
	return renderHumanSection("Page", []keyValue{
		{Label: listTotalLabel, Value: strconv.Itoa(page.Total)},
		{Label: listLimitLabel, Value: strconv.Itoa(page.Limit)},
		{Label: listHasMoreLabel, Value: strconv.FormatBool(page.HasMore)},
		{Label: listNextCursorLabel, Value: stringOrDash(page.NextCursor)},
	})
}

func loopListKind(item contract.LoopCatalogEntryPayload) looppkg.CatalogKind {
	if item.Source == contract.LoopSourceWorkspace {
		return looppkg.CatalogKindWorkspace
	}
	return looppkg.CatalogKindReadOnly
}

func loopListLastStatus(item contract.LoopCatalogEntryPayload) string {
	if item.LastRun == nil {
		return ""
	}
	return strings.TrimSpace(string(item.LastRun.Status))
}
