package command

import (
	"errors"

	"github.com/compozy/compozy/internal/cli"
	"github.com/spf13/cobra"
)

// New returns the reusable compozy Cobra command.
func New() *cobra.Command {
	return cli.NewRootCommand()
}

// ExitCode extracts a command-specific exit code from an execution error.
func ExitCode(err error) int {
	if err == nil {
		return 0
	}
	var exitErr interface{ ExitCode() int }
	if errors.As(err, &exitErr) && exitErr.ExitCode() > 0 {
		return exitErr.ExitCode()
	}
	return 1
}
