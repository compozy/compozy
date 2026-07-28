package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// mustMarkFlagRequired makes command-construction bugs fail loudly at startup.
func mustMarkFlagRequired(cmd *cobra.Command, name string) {
	if err := cmd.MarkFlagRequired(name); err != nil {
		panic(fmt.Sprintf("cli: mark required flag %q: %v", name, err))
	}
}

// mustMarkFlagHidden makes command-construction bugs fail loudly at startup.
func mustMarkFlagHidden(cmd *cobra.Command, name string) {
	if err := cmd.Flags().MarkHidden(name); err != nil {
		panic(fmt.Sprintf("cli: mark hidden flag %q: %v", name, err))
	}
}
