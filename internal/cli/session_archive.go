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
	stoppedFlags := sessionArchiveStoppedFlags{}
	args := exactOneNonBlankArg()
	if archived {
		args = sessionArchiveArgs(&stoppedFlags)
	}
	command := &cobra.Command{
		Use:   action + " <id>",
		Short: short,
		Args:  args,
		RunE: func(cmd *cobra.Command, args []string) error {
			if archived {
				if stoppedFlags.stopped {
					return runSessionArchiveStopped(cmd.Context(), cmd, deps, stoppedFlags)
				}
				if err := sessionArchiveStoppedOnlyFlagError(cmd); err != nil {
					return err
				}
			}
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
		command.Use = action + " (<id> | --stopped)"
		command.Long = "Archive one session by ID, or with --stopped archive every stopped thread of a project " +
			"(or of every project with --all-workspaces). Archiving is reversible with `compozy session unarchive`."
		command.Example = `  # Preview, then archive, every stopped thread of a project
  compozy session archive --stopped --workspace platform --dry-run
  compozy session archive --stopped --workspace platform`
		bindSessionArchiveStoppedFlags(command, &stoppedFlags)
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
