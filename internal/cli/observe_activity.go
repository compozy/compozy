package cli

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/spf13/cobra"
)

func newObserveActivityCommand(deps commandDeps) *cobra.Command {
	var query PermissionRollupQuery
	cmd := &cobra.Command{
		Use: "activity", Short: "Show daily permission decisions by agent",
		Example: "  compozy observe activity --days 7 --unattended -o json",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if query.Days < 1 || query.Days > 90 {
				return fmt.Errorf("--days must be between 1 and 90")
			}
			query.Agent = strings.TrimSpace(query.Agent)
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}
			response, err := client.PermissionRollup(cmd.Context(), query)
			if err != nil {
				return err
			}
			row := func(item contract.PermissionDailyRollupPayload) []string {
				return []string{item.Day, item.AgentName, item.Decision, strconv.FormatInt(item.Count, 10)}
			}
			return writeCommandOutput(
				cmd,
				listBundle(
					response,
					response.Rollups,
					"Daily Permission Decisions",
					[]string{"DAY", "AGENT", "DECISION", "COUNT"},
					"rollups",
					[]string{"day", "agent_name", "decision", "count"},
					row,
					row,
				),
			)
		},
	}
	cmd.Flags().IntVar(&query.Days, "days", 30, "UTC calendar days including today (1–90)")
	cmd.Flags().StringVar(&query.Agent, "agent", "", "Filter by agent name")
	cmd.Flags().
		BoolVar(&query.Unattended, "unattended", false, "Include only automation, daemon and extension initiators")
	configureProfileReadCommand(cmd, deps)
	return cmd
}
