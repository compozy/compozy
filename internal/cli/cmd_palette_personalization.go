package cli

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/spf13/cobra"
)

func newCmdPalettePersonalizationCommand(deps commandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "personalization",
		Short: "Inspect or reset palette personalization",
	}
	cmd.AddCommand(
		newCmdPalettePersonalizationShowCommand(deps),
		newCmdPalettePersonalizationResetCommand(deps),
	)
	return cmd
}

func newCmdPalettePersonalizationShowCommand(deps commandDeps) *cobra.Command {
	var scope cmdPaletteScopeFlags
	cmd := &cobra.Command{
		Use:   cliShowVerb,
		Short: "Show workspace palette personalization",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, workspace, err := cmdPaletteClientAndWorkspace(cmd, deps, scope.workspace)
			if err != nil {
				return err
			}
			summary, err := client.GetCmdPalettePersonalization(cmd.Context(), workspace)
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, cmdPalettePersonalizationOutput(summary))
		},
	}
	scope.add(cmd, false)
	return cmd
}

func newCmdPalettePersonalizationResetCommand(deps commandDeps) *cobra.Command {
	var scope cmdPaletteScopeFlags
	cmd := &cobra.Command{
		Use:   cliResetVerb,
		Short: "Reset workspace palette personalization",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, workspace, err := cmdPaletteClientAndWorkspace(cmd, deps, scope.workspace)
			if err != nil {
				return err
			}
			response, err := client.ResetCmdPalettePersonalization(cmd.Context(), workspace)
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, cmdPalettePersonalizationResetOutput(workspace, response))
		},
	}
	scope.add(cmd, false)
	return cmd
}

func cmdPalettePersonalizationOutput(summary contract.CmdPalettePersonalizationResponse) outputBundle {
	pins := make([]string, 0, len(summary.Pins))
	for _, pin := range summary.Pins {
		pins = append(pins, string(pin))
	}
	return outputBundle{
		jsonValue: summary,
		jsonl:     func(cmd *cobra.Command) error { return writeJSONLine(cmd, summary) },
		human: func() (string, error) {
			return renderHumanSectionResult("Palette personalization", []keyValue{
				{Label: authoredContextWorkspaceValue, Value: string(summary.Workspace)},
				{Label: "Pins", Value: strings.Join(pins, ", ")},
				{Label: "Recents", Value: strconv.Itoa(summary.Recents)},
				{Label: "Frecency entries", Value: strconv.Itoa(summary.FrecencyEntries)},
				{Label: "Query associations", Value: strconv.Itoa(summary.QueryAssociations)},
			})
		},
		toon: func() (string, error) {
			return renderToonObject(
				"personalization",
				[]string{cmdPaletteWorkspaceFlag, "pins", "recents", "frecency_entries", "query_associations"},
				[]string{
					string(summary.Workspace), strings.Join(pins, ","), strconv.Itoa(summary.Recents),
					strconv.Itoa(summary.FrecencyEntries), strconv.Itoa(summary.QueryAssociations),
				},
			), nil
		},
	}
}

func cmdPalettePersonalizationResetOutput(
	workspace string,
	response contract.CmdPalettePersonalizationResetResponse,
) outputBundle {
	return outputBundle{
		jsonValue: response,
		jsonl:     func(cmd *cobra.Command) error { return writeJSONLine(cmd, response) },
		human: func() (string, error) {
			return fmt.Sprintf(
				"Reset palette personalization for workspace %s (pins, recents, frecency, query learning).",
				workspace,
			), nil
		},
		toon: func() (string, error) {
			return renderToonObject(
				"personalization_reset",
				[]string{automationStatusKey, cmdPaletteWorkspaceFlag},
				[]string{response.Status, workspace},
			), nil
		},
	}
}
