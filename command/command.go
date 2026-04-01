package command

import (
	"github.com/compozy/compozy/internal/cli"
	"github.com/spf13/cobra"
)

// New returns the reusable compozy Cobra command.
func New() *cobra.Command {
	return cli.NewRootCommand()
}
