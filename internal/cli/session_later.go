package cli

import (
	"fmt"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/spf13/cobra"
)

func newSessionLaterCommand(deps commandDeps) *cobra.Command {
	var reason string
	var returnAt string
	command := &cobra.Command{
		Use:   "later <id>",
		Short: "Move a session and its children to Later",
		Long: "Move a session to Later. Its active turn and runtime stop, its monitors and pending wakes are " +
			"canceled, its processes and terminals are terminated, and its child sessions are parked with it. " +
			"History, transcript, and worktree stay. With --return-at the session comes back into Needs you " +
			"with a notification at that time.",
		Example: `  # Park a session with a note
  compozy session later sess-123 --reason "revisit after the billing spec"

  # Park it until a date (local midnight) or for a duration
  compozy session later sess-123 --return-at 2026-11-03
  compozy session later sess-123 --return-at 72h`,
		Args: exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			at, err := parseLaterReturnAt(returnAt, deps.now())
			if err != nil {
				return withCommandExitCode(2, err)
			}
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}
			record, err := client.MoveSessionToLater(cmd.Context(), args[0], contract.MoveSessionToLaterRequest{
				Reason: reason, ReturnAt: at,
			})
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, sessionBundle(&record, deps.now))
		},
	}
	command.Flags().StringVar(&reason, "reason", "", "One-line note shown on the Later card")
	command.Flags().StringVar(
		&returnAt,
		"return-at",
		"",
		"When the session returns to Needs you: RFC 3339 time, YYYY-MM-DD (local midnight), or a duration like 72h",
	)
	return command
}

func newSessionResumeLaterCommand(deps commandDeps) *cobra.Command {
	var withChildren bool
	command := &cobra.Command{
		Use:   "resume-later <id>",
		Short: "Bring a session back from Later without starting it",
		Args:  exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}
			record, err := client.ResumeSessionFromLater(cmd.Context(), args[0], withChildren)
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, sessionBundle(&record, deps.now))
		},
	}
	command.Flags().BoolVar(&withChildren, "with-children", false, "Also bring back parked child sessions")
	return command
}

func parseLaterReturnAt(value string, now time.Time) (*time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}
	if at, err := time.Parse(time.RFC3339, value); err == nil {
		return &at, nil
	}
	if day, err := time.ParseInLocation(time.DateOnly, value, time.Local); err == nil {
		return &day, nil
	}
	if duration, err := time.ParseDuration(value); err == nil && duration >= 0 {
		return new(now.Add(duration)), nil
	}
	return nil, fmt.Errorf("invalid --return-at %q: use RFC 3339, YYYY-MM-DD, or a duration like 72h", value)
}
