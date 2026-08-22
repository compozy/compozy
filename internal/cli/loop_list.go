package cli

import (
	"strconv"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	looppkg "github.com/compozy/compozy/internal/loop"
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
	cmd.Flags().StringVar(&workspaceRef, loopWorkspaceKey, "", "Override workspace (ID, name, or path)")
	cmd.Flags().StringVar(&query.Search, "query", "", "Search Loop name or contract goal")
	cmd.Flags().StringVar(&query.Kind, "kind", "", "Filter by kind: read_only or workspace")
	cmd.Flags().StringVar(&query.Category, "category", "", "Filter by exact catalog category")
	cmd.Flags().StringVar(&query.Status, loopStatusKey, "", "Filter by exact latest-run status")
	cmd.Flags().StringVar(&query.Sort, "sort", looppkg.CatalogSortName, "Stable ordering: name")
	cmd.Flags().StringVar(&query.Cursor, "cursor", "", "Continue from an opaque Loop catalog cursor")
	cmd.Flags().IntVar(&query.Limit, "limit", 0, "Maximum Loops to return (default 50, max 200)")
	configureProfileReadCommand(cmd, deps)
	return cmd
}

func loopListBundle(response contract.LoopsResponse) outputBundle {
	bundle := listBundle(
		response,
		response.Loops,
		"Loops",
		loopListHumanHeaders(),
		"loops",
		loopListToonFields(),
		loopListHumanRow,
		loopListToonRow,
	)
	bundle.jsonl = loopListJSONLRenderer(response)
	bundle.human = loopListHumanRenderer(bundle.human, response.Page)
	bundle.toon = loopListToonRenderer(bundle.toon, response)
	return bundle
}

func loopListJSONLRenderer(response contract.LoopsResponse) func(*cobra.Command) error {
	return func(cmd *cobra.Command) error {
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
}

func loopListHumanRenderer(
	baseHuman func() (string, error),
	page contract.CountedCursorPagePayload,
) func() (string, error) {
	return func() (string, error) {
		table, err := baseHuman()
		if err != nil {
			return "", err
		}
		return renderHumanBlocks(table, loopCatalogPageHuman(page)), nil
	}
}

func loopListToonRenderer(
	baseToon func() (string, error),
	response contract.LoopsResponse,
) func() (string, error) {
	return func() (string, error) {
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
}

func loopListHumanHeaders() []string {
	return []string{
		cliNameValue,
		cliKindValue,
		"Category",
		"Last Run",
		"Best Gen",
		"Best Score",
		"Runs 30d",
		"Success 30d",
	}
}

func loopListToonFields() []string {
	return []string{
		automationNameKey,
		cliKindKey,
		"category",
		"last_run_status",
		"best_generation",
		"best_score",
		"runs_30d",
		"success_rate_30d",
	}
}

func loopListHumanRow(item contract.LoopCatalogEntryPayload) []string {
	return []string{
		item.Name,
		string(loopListKind(item)),
		stringOrDash(item.Catalog.Category),
		stringOrDash(loopListLastStatus(item)),
		formatOptionalInt64(loopListBestGeneration(item)),
		formatOptionalFloat64(loopListBestScore(item)),
		strconv.Itoa(item.Aggregate30d.Runs),
		strconv.FormatFloat(item.SuccessRate30*100, 'f', 1, 64) + "%",
	}
}

func loopListToonRow(item contract.LoopCatalogEntryPayload) []string {
	return []string{
		item.Name,
		string(loopListKind(item)),
		item.Catalog.Category,
		loopListLastStatus(item),
		formatOptionalInt64(loopListBestGeneration(item)),
		formatOptionalFloat64(loopListBestScore(item)),
		strconv.Itoa(item.Aggregate30d.Runs),
		strconv.FormatFloat(item.SuccessRate30, 'f', -1, 64),
	}
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

func loopListBestGeneration(item contract.LoopCatalogEntryPayload) *int64 {
	if item.LastRun == nil {
		return nil
	}
	return item.LastRun.BestGeneration
}

func loopListBestScore(item contract.LoopCatalogEntryPayload) *float64 {
	if item.LastRun == nil {
		return nil
	}
	return item.LastRun.BestScore
}
