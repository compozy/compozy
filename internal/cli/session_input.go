package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

type sessionInputIdentityFlags struct {
	messageID      string
	idempotencyKey string
}

type sessionInputSteerFlags struct {
	sessionInputIdentityFlags
	expectedTurnID string
}

func newSessionInputCommand(deps commandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "input",
		Short: "Manage pending session input",
		Args:  cobra.NoArgs,
	}
	cmd.AddCommand(newSessionInputListCommand(deps))
	cmd.AddCommand(newSessionInputEditCommand(deps))
	cmd.AddCommand(newSessionInputSteerCommand(deps))
	cmd.AddCommand(newSessionInputCancelCommand(deps))
	return cmd
}

func newSessionInputListCommand(deps commandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "list <session-id>",
		Short:   "List pending input for a session",
		Args:    exactOneNonBlankArg(),
		Example: "  compozy session input list sess_1234",
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}
			record, err := client.ListSessionInputs(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, sessionInputListBundle(record))
		},
	}
	configureProfileReadCommand(cmd, deps)
	return cmd
}

func newSessionInputEditCommand(deps commandDeps) *cobra.Command {
	flags := sessionInputIdentityFlags{}
	cmd := &cobra.Command{
		Use:     "edit <session-id> <input-id> <message>",
		Short:   "Replace one queued session input",
		Args:    exactSessionInputMutationArgs(),
		Example: "  compozy session input edit sess_1234 queue_entry_1234 \"Run the focused test first.\"",
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}
			messageID, idempotencyKey, err := sessionInputIdentity(cmd, flags)
			if err != nil {
				return err
			}
			record, err := client.ReplaceSessionInput(cmd.Context(), args[0], args[1], ReplaceSessionInputRequest{
				Text: args[2], MessageID: messageID, IdempotencyKey: idempotencyKey,
			})
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, sessionInputRecordBundle(record))
		},
	}
	bindSessionInputIdentityFlags(cmd, &flags)
	return cmd
}

func newSessionInputSteerCommand(deps commandDeps) *cobra.Command {
	flags := sessionInputSteerFlags{}
	cmd := &cobra.Command{
		Use:   "steer <session-id> <input-id> <message>",
		Short: "Promote queued input to fenced steering guidance",
		Args:  exactSessionInputMutationArgs(),
		Example: "  compozy session input steer sess_1234 queue_entry_1234 " +
			"\"Prefer the smaller patch.\" --expected-turn-id turn_1234",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := warnExpectedTurnAlias(cmd); err != nil {
				return err
			}
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}
			messageID, idempotencyKey, err := sessionInputIdentity(cmd, flags.sessionInputIdentityFlags)
			if err != nil {
				return err
			}
			expectedTurnID := strings.TrimSpace(flags.expectedTurnID)
			if (cmd.Flags().Changed("expected-turn") || cmd.Flags().Changed("expected-turn-id")) &&
				expectedTurnID == "" {
				return fmt.Errorf("cli: --expected-turn cannot be empty")
			}
			record, err := client.PromoteSessionInput(cmd.Context(), args[0], args[1], PromoteSessionInputRequest{
				Text: args[2], MessageID: messageID, IdempotencyKey: idempotencyKey, ExpectedTurnID: expectedTurnID,
			})
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, sessionPromptBundle(record))
		},
	}
	bindSessionInputIdentityFlags(cmd, &flags.sessionInputIdentityFlags)
	cmd.Flags().
		StringVar(&flags.expectedTurnID, "expected-turn-id", "", "Active turn id that this steer request must match")
	cmd.Flags().StringVar(&flags.expectedTurnID, "expected-turn", "", "Optional strict active turn fence")
	mustMarkFlagHidden(cmd, "expected-turn-id")
	return cmd
}

func newSessionInputCancelCommand(deps commandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "cancel <session-id> <input-id>",
		Short:   "Cancel one queued session input",
		Args:    exactTwoNonBlankArgs(),
		Example: "  compozy session input cancel sess_1234 queue_entry_1234",
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}
			record, err := client.CancelSessionInput(cmd.Context(), args[0], args[1])
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, sessionPromptBundle(record))
		},
	}
	return cmd
}

func bindSessionInputIdentityFlags(cmd *cobra.Command, flags *sessionInputIdentityFlags) {
	cmd.Flags().StringVar(&flags.messageID, "message-id", "", "Stable authored message id for an explicit retry")
	cmd.Flags().StringVar(
		&flags.idempotencyKey,
		"idempotency-key",
		"",
		"Stable command idempotency key for an explicit retry",
	)
}

func sessionInputIdentity(cmd *cobra.Command, flags sessionInputIdentityFlags) (string, string, error) {
	messageID, idempotencyKey, err := promptIdentityFromFlags(cmd, flags.messageID, flags.idempotencyKey)
	if err != nil {
		return "", "", err
	}
	return strings.TrimSpace(messageID), strings.TrimSpace(idempotencyKey), nil
}

func exactSessionInputMutationArgs() cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		if err := cobra.ExactArgs(3)(cmd, args); err != nil {
			return err
		}
		for index, arg := range args {
			if strings.TrimSpace(arg) == "" {
				return fmt.Errorf("cli: argument %d for %q cannot be blank", index+1, cmd.CommandPath())
			}
		}
		return nil
	}
}
