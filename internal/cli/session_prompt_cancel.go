package cli

import (
	"errors"

	"github.com/compozy/compozy/internal/session"
	"github.com/spf13/cobra"
)

const sessionPromptCancelExitNothingInFlight = 66

func newSessionPromptCancelCommand(deps commandDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "prompt-cancel <id>",
		Short: "Cancel the active session prompt",
		Args:  exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}
			result, err := client.CancelSessionPrompt(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			if err := writeCommandOutput(cmd, sessionPromptCancelBundle(result)); err != nil {
				return err
			}
			if result.Outcome == string(session.PromptCancelOutcomeNothingInFlight) {
				return withCommandExitCode(
					sessionPromptCancelExitNothingInFlight,
					errors.New("session prompt has nothing in flight"),
				)
			}
			return nil
		},
	}
}
