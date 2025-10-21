package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func main() {
	cmd := newRootCommand()
	if err := cmd.Execute(); err != nil {
		if errors.Is(err, errFuncLengthViolations) ||
			errors.Is(err, errFuncSpacingViolations) ||
			errors.Is(err, errFuncCommentViolations) {
			os.Exit(1)
		}
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func newRootCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:           "func-tools",
		Short:         "Utility checks for Go function style constraints",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	cmd.AddCommand(newLengthCommand())
	cmd.AddCommand(newSpacingCommand())
	cmd.AddCommand(newCommentsCommand())
	return cmd
}

func newLengthCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "length [path]",
		Short: "Report functions that exceed the configured line limit",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			root := "."
			if len(args) == 1 {
				root = args[0]
			}
			return runFuncLengthCheck(root)
		},
	}
	return cmd
}

func newSpacingCommand() *cobra.Command {
	var fix bool
	cmd := &cobra.Command{
		Use:   "spacing [path]",
		Short: "Detect or fix unnecessary blank lines inside function bodies",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			root := "."
			if len(args) == 1 {
				root = args[0]
			}
			return runFuncSpacingCheck(root, fix)
		},
	}
	cmd.Flags().BoolVar(&fix, "fix", false, "Automatically remove blank lines between statements")
	return cmd
}

func newCommentsCommand() *cobra.Command {
	var fix bool
	cmd := &cobra.Command{
		Use:   "comments [path]",
		Short: "Detect or remove non-TODO comments inside function bodies",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			root := "."
			if len(args) == 1 {
				root = args[0]
			}
			return runFuncCommentCleanup(root, fix)
		},
	}
	cmd.Flags().BoolVar(&fix, "fix", false, "Delete removable comments inside function bodies")
	return cmd
}
