package cli

import (
	"strconv"

	"github.com/spf13/cobra"
)

func newSessionUsageCommand(deps commandDeps) *cobra.Command {
	var turns bool
	command := &cobra.Command{
		Use:   "usage <session-id>",
		Short: "Show session tokens, context, and cost provenance",
		Example: `  # Show truthful token and cost totals
  compozy session usage sess_1234

  # Read the usage contract as JSON for scripts
  compozy session usage sess_1234 -o json

  # Inspect each turn and replay compaction span
  compozy session usage sess_1234 --turns`,
		Args: exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}
			if turns {
				record, err := client.GetSessionUsageTurns(cmd.Context(), args[0])
				if err != nil {
					return err
				}
				return writeCommandOutput(cmd, sessionUsageTurnsBundle(record))
			}
			record, err := client.GetSessionUsage(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, sessionUsageBundle(record))
		},
	}
	command.Flags().BoolVar(&turns, "turns", false, "Show per-turn usage, deliveries, and replay compaction spans")
	return command
}

func sessionUsageBundle(record SessionUsageRecord) outputBundle {
	cost := formatCostProvenance(record.TotalCost, record.CostCurrency, record.CostStatus)
	return outputBundle{
		jsonValue: record,
		human: func() (string, error) {
			summary := renderHumanSection("Session Usage", []keyValue{
				{Label: cliInputTokensValue, Value: stringOrDash(formatInt64Ptr(record.InputTokens))},
				{Label: cliOutputTokensValue, Value: stringOrDash(formatInt64Ptr(record.OutputTokens))},
				{Label: "Cache Read", Value: stringOrDash(formatInt64Ptr(record.CacheReadTokens))},
				{Label: "Cache Write", Value: stringOrDash(formatInt64Ptr(record.CacheWriteTokens))},
				{Label: "Total Tokens", Value: stringOrDash(formatInt64Ptr(record.TotalTokens))},
				{Label: "Total Cost", Value: stringOrDash(cost)},
				{Label: "Cost Status", Value: stringOrDash(string(record.CostStatus))},
				{Label: "Cost Source", Value: stringOrDash(string(record.CostSource))},
				{Label: cliTurnsValue, Value: strconv.FormatInt(record.TurnCount, 10)},
			})
			return renderHumanBlocks(summary, sessionContextHuman(record.Context)), nil
		},
		toon: func() (string, error) { return sessionUsageToon(record, cost), nil },
	}
}
