package command

import (
	"github.com/compozy/looper/internal/cli"
	"github.com/spf13/cobra"
)

// New returns the reusable looper Cobra command.
func New() *cobra.Command {
	return cli.NewRootCommand()
}
