package cli

import "github.com/spf13/cobra"

func newSessionArchiveCommand(deps commandDeps) *cobra.Command {
	return newSessionArchiveActionCommand(deps, true)
}

func newSessionUnarchiveCommand(deps commandDeps) *cobra.Command {
	return newSessionArchiveActionCommand(deps, false)
}

func newSessionArchiveActionCommand(deps commandDeps, archived bool) *cobra.Command {
	action := "archive"
	short := "Archive a stopped session"
	if !archived {
		action = "unarchive"
		short = "Restore an archived session"
	}
	return &cobra.Command{
		Use:   action + " <id>",
		Short: short,
		Args:  exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}
			var record SessionRecord
			if archived {
				record, err = client.ArchiveSession(cmd.Context(), args[0])
			} else {
				record, err = client.UnarchiveSession(cmd.Context(), args[0])
			}
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, sessionBundle(record, deps.now))
		},
	}
}
