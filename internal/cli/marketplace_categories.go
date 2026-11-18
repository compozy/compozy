package cli

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/marketplace"
	"github.com/spf13/cobra"
)

const marketplaceCategoryCountKey = "count"

func unknownMarketplaceCategory(value string) error {
	ids := []string{}
	for _, category := range marketplace.Categories() {
		ids = append(ids, string(category.ID))
	}
	return withCommandExitCode(
		2,
		fmt.Errorf(
			"unknown category %q (marketplace_category_unknown)\nValid categories: %s",
			value,
			strings.Join(ids, ", "),
		),
	)
}
func newMarketplaceCategoriesCommand(deps commandDeps) *cobra.Command {
	var flags marketplaceReadFlagValues
	cmd := &cobra.Command{
		Use:   "categories [query]",
		Short: "List Marketplace categories and counts",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}
			scope, err := flags.resolve(cmd, deps, client)
			if err != nil {
				return err
			}
			query := ""
			if len(args) > 0 {
				query = args[0]
			}
			page, err := client.SearchMarketplace(cmd.Context(), query, 1, "", scope)
			if err != nil {
				return err
			}
			response := contract.MarketplaceCategoriesResponse{Revision: page.Revision, Categories: page.Categories}
			humanItems := []contract.MarketplaceCategoryCount{}
			for _, category := range response.Categories {
				if category.Count > 0 {
					humanItems = append(humanItems, category)
				}
			}
			bundle := listBundle(
				response,
				humanItems,
				"Marketplace Categories",
				[]string{"ID", agentCategoryValue, "Count"},
				"marketplace_categories",
				[]string{"id", "label", marketplaceCategoryCountKey},
				func(c contract.MarketplaceCategoryCount) []string {
					return []string{c.ID, c.Label, strconv.Itoa(c.Count)}
				},
				func(c contract.MarketplaceCategoryCount) []string {
					return []string{c.ID, c.Label, strconv.Itoa(c.Count)}
				},
			)
			bundle.jsonl = func(cmd *cobra.Command) error { return writeJSONLines(cmd, response.Categories) }
			return writeCommandOutput(cmd, bundle)
		},
	}
	addMarketplaceReadFlags(cmd, &flags)
	return cmd
}
