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
	short := "Archive a stopped session, or stop and archive a live one with --stop"
	if !archived {
		action = "unarchive"
		short = "Restore an archived session"
	}
	var stop, withChildren bool
	flags := processPolicyFlags{}
	command := &cobra.Command{
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
				record, err = client.ArchiveSession(cmd.Context(), args[0], stop, withChildren, flags.policy())
			} else {
				record, err = client.UnarchiveSession(cmd.Context(), args[0])
			}
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, sessionBundle(&record, deps.now))
		},
	}
	if archived {
		flags.bind(command)
		command.Flags().BoolVar(
			&stop,
			"stop",
			false,
			"Stop a live session through the normal stop path (canceling any active turn) before archiving it",
		)
		command.Flags().BoolVar(
			&withChildren,
			"with-children",
			false,
			"Also archive the session's child sessions, recursively; with --stop a failed stop archives nothing",
		)
	}
	return command
}
