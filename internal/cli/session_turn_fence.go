package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// Keep compatibility warnings away from machine-readable stdout. Cobra's
// built-in flag deprecation output shares the command output stream.
func warnExpectedTurnAlias(cmd *cobra.Command) error {
	if !cmd.Flags().Changed("expected-turn-id") {
		return nil
	}
	_, err := fmt.Fprintln(cmd.ErrOrStderr(),
		"Warning: --expected-turn-id is deprecated; use --expected-turn (removed in v0.5.0)")
	return err
}
