package cli

import (
	"errors"
	"strings"

	"github.com/spf13/cobra"
)

const (
	sessionReparentParentFlag   = "parent"
	sessionReparentTopLevelFlag = "top-level"
)

func newSessionReparentCommand(deps commandDeps) *cobra.Command {
	var parentID string
	var topLevel bool
	cmd := &cobra.Command{
		Use:   "reparent <id>",
		Short: "Nest a top-level session under another session, or make it top-level again",
		Long: "Adopt a session that has no parent as a child of another user session in the same " +
			"workspace and profile, or make a previously adopted child top-level again. " +
			"--top-level on a stopped spawned worker takes it over as a normal user session: its spawn " +
			"TTL, parent auto-stop, creator wake, budget, and permission narrowing end. " +
			"Subagents and continued or forked sessions keep their lineage.",
		Example: `  # Nest a top-level session under another session
  compozy session reparent sess_child --parent sess_parent

  # Make an adopted child top-level again, or take over a stopped spawned worker
  compozy session reparent sess_child --top-level`,
		Args: exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			parent := strings.TrimSpace(parentID)
			if parent == "" && !topLevel {
				return errors.New("set --parent <session-id> or --top-level")
			}
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}
			var record SessionRecord
			if topLevel {
				record, err = client.ClearSessionParent(cmd.Context(), args[0])
			} else {
				record, err = client.SetSessionParent(
					cmd.Context(), args[0], SetSessionParentRequest{ParentSessionID: parent},
				)
			}
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, sessionBundle(&record, deps.now))
		},
	}
	cmd.Flags().StringVar(&parentID, sessionReparentParentFlag, "", "Parent session ID to nest the session under")
	cmd.Flags().BoolVar(
		&topLevel, sessionReparentTopLevelFlag, false, "Make an adopted child or stopped spawned worker top-level",
	)
	mustMarkFlagsMutuallyExclusive(cmd, sessionReparentParentFlag, sessionReparentTopLevelFlag)
	return cmd
}
