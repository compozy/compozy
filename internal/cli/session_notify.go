package cli

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

func newNotifyCommand(deps commandDeps) *cobra.Command {
	var body string
	cmd := &cobra.Command{
		Use:   "notify <title>",
		Short: "Send the operator a notification",
		Args:  exactOneNonBlankArg(),
		Example: `  # Notify the operator from the current managed agent session
  compozy notify "Deps audit done" --body "3 findings, 1 high severity"

  # Inspect the daemon-provable delivery outcome
  compozy notify "Deps audit done" -o json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}
			credentials, err := requireAgentCommandIdentity(
				cmd.Context(), deps, client, agentActionCLI("notify"),
			)
			if err != nil {
				return err
			}
			result, err := client.AgentNotify(cmd.Context(), AgentNotifyRequest{
				Title: strings.TrimSpace(args[0]),
				Body:  body,
			}, credentials)
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, agentNotifyBundle(result))
		},
	}
	cmd.Flags().StringVar(&body, "body", "", "Optional notification body")
	return cmd
}

func agentNotifyBundle(result AgentNotifyRecord) outputBundle {
	return outputBundle{
		jsonValue: result,
		human: func() (string, error) {
			return fmt.Sprintf("OUTCOME   %s", result.Outcome), nil
		},
		toon: func() (string, error) {
			return renderToonObject(
				"notify",
				[]string{cliOutcomeKey, "retry_after_ms"},
				[]string{result.Outcome, strconv.FormatInt(result.RetryAfterMS, 10)},
			), nil
		},
	}
}
