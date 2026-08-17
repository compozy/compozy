package cli

import (
	"strconv"

	"github.com/spf13/cobra"
)

const sessionAttentionFinishedKey = "finished"

func sessionAttentionSummaryBundle(summary SessionAttentionSummaryRecord) outputBundle {
	return outputBundle{
		jsonValue: summary,
		jsonl: func(cmd *cobra.Command) error {
			return writeJSONLine(cmd, summary)
		},
		human: func() (string, error) {
			rows := make([][]string, 0, len(summary.ByWorkspace))
			for _, workspace := range summary.ByWorkspace {
				rows = append(rows, []string{
					workspace.WorkspaceID,
					strconv.Itoa(workspace.NeedsYou),
					strconv.Itoa(workspace.Finished),
				})
			}
			return renderHumanBlocks(
				renderHumanSection("Attention Summary", []keyValue{
					{Label: "Needs You", Value: strconv.Itoa(summary.NeedsYou)},
					{Label: "Finished", Value: strconv.Itoa(summary.Finished)},
				}),
				renderHumanTable("Workspaces", []string{"WORKSPACE", "NEEDS YOU", "FINISHED"}, rows),
			), nil
		},
		toon: func() (string, error) {
			rows := make([][]string, 0, len(summary.ByWorkspace))
			for _, workspace := range summary.ByWorkspace {
				rows = append(rows, []string{
					workspace.WorkspaceID,
					strconv.Itoa(workspace.NeedsYou),
					strconv.Itoa(workspace.Finished),
				})
			}
			return renderToonObject(
				"attention_summary",
				[]string{"needs_you", sessionAttentionFinishedKey},
				[]string{strconv.Itoa(summary.NeedsYou), strconv.Itoa(summary.Finished)},
			) + "\n" + renderToonArray(
				"by_workspace",
				[]string{automationWorkspaceIDKey, "needs_you", sessionAttentionFinishedKey},
				rows,
			), nil
		},
	}
}
