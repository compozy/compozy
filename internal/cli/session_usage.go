package cli

import (
	"strconv"

	"github.com/spf13/cobra"
)

func newSessionUsageCommand(deps commandDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "usage <session-id>",
		Short: "Show aggregated session token usage and cost provenance",
		Example: `  # Show truthful token and cost totals
  agh session usage sess_1234

  # Read the usage contract as JSON for scripts
  agh session usage sess_1234 -o json`,
		Args: exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}
			record, err := client.GetSessionUsage(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, sessionUsageBundle(record))
		},
	}
}

func sessionUsageBundle(record SessionUsageRecord) outputBundle {
	cost := formatCostProvenance(record.TotalCost, record.CostCurrency, record.CostStatus)
	return outputBundle{
		jsonValue: record,
		human: func() (string, error) {
			return renderHumanSection("Session Usage", []keyValue{
				{Label: cliInputTokensValue, Value: stringOrDash(formatInt64Ptr(record.InputTokens))},
				{Label: cliOutputTokensValue, Value: stringOrDash(formatInt64Ptr(record.OutputTokens))},
				{Label: "Total Tokens", Value: stringOrDash(formatInt64Ptr(record.TotalTokens))},
				{Label: "Total Cost", Value: stringOrDash(cost)},
				{Label: "Cost Status", Value: stringOrDash(string(record.CostStatus))},
				{Label: "Cost Source", Value: stringOrDash(string(record.CostSource))},
				{Label: cliTurnsValue, Value: strconv.FormatInt(record.TurnCount, 10)},
			}), nil
		},
		toon: func() (string, error) {
			return renderToonObject("session_usage", []string{
				cliInputTokensKey,
				cliOutputTokensKey,
				"total_tokens",
				"total_cost",
				"cost_status",
				"cost_source",
				"turn_count",
			}, []string{
				formatInt64Ptr(record.InputTokens),
				formatInt64Ptr(record.OutputTokens),
				formatInt64Ptr(record.TotalTokens),
				cost,
				string(record.CostStatus),
				string(record.CostSource),
				strconv.FormatInt(record.TurnCount, 10),
			}), nil
		},
	}
}
