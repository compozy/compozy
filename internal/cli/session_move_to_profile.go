package cli

import (
	"time"

	"github.com/spf13/cobra"
)

func newSessionMoveToProfileCommand(deps commandDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "move-to-profile <id> <profile>",
		Short: "Copy a stopped session into another profile and archive the original",
		Long: "Copy a stopped user session into another profile and archive the original. The copy is a " +
			"fork owned by the target profile and carries the conversation; the original keeps its profile " +
			"and history, and past usage stays with it. A worktree is released and its branch checked out " +
			"again in a new worktree owned by the target profile. Refused for a running, archived, or child " +
			"session, a session with children, or a worktree with uncommitted changes. To place a session " +
			"in a folder, use session move.",
		Example: `  # Move a stopped session to the "work" profile
  compozy session move-to-profile sess_abc work`,
		Args: exactTwoNonBlankArgs(),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}
			response, err := client.MoveSessionToProfile(
				cmd.Context(), args[0], MoveSessionToProfileRequest{TargetProfile: args[1]},
			)
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, sessionProfileMoveBundle(&response, deps.now))
		},
	}
}

func sessionProfileMoveBundle(response *MoveSessionToProfileResponse, now func() time.Time) outputBundle {
	return outputBundle{
		jsonValue: response,
		human: func() (string, error) {
			copyText, err := renderSessionHuman(&response.Session, now)
			if err != nil {
				return "", err
			}
			rows := []keyValue{{Label: "Original", Value: stringOrDash(response.Original.ID) + " (archived)"}}
			if response.WorktreeID != "" {
				rows = append(rows,
					keyValue{Label: worktreeLabel, Value: response.WorktreeID},
					keyValue{Label: worktreeBranchLabel, Value: stringOrDash(response.Branch)},
				)
			}
			return copyText + "\n" + renderHumanSection("Moved", rows), nil
		},
		toon: func() (string, error) { return renderSessionToon(&response.Session) },
	}
}
