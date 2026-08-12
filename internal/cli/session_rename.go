package cli

import "github.com/spf13/cobra"

func newSessionRenameCommand(deps commandDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "rename <id> <name>",
		Short: "Rename a user session",
		Example: `  # Rename a session
  compozy session rename sess_1234 "Checkout retries"`,
		Args: exactTwoNonBlankArgs(),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}
			record, err := client.RenameSession(
				cmd.Context(),
				args[0],
				RenameSessionRequest{Name: args[1]},
			)
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, sessionBundle(record, deps.now))
		},
	}
}
