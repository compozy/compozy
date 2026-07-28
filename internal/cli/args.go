package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func exactOneNonBlankArg() cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		if err := cobra.ExactArgs(1)(cmd, args); err != nil {
			return err
		}
		for i, arg := range args {
			if strings.TrimSpace(arg) == "" {
				return fmt.Errorf("cli: argument %d for %q cannot be blank", i+1, cmd.CommandPath())
			}
		}
		return nil
	}
}

func exactTwoNonBlankArgs() cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		if err := cobra.ExactArgs(2)(cmd, args); err != nil {
			return err
		}
		for i, arg := range args {
			if strings.TrimSpace(arg) == "" {
				return fmt.Errorf("cli: argument %d for %q cannot be blank", i+1, cmd.CommandPath())
			}
		}
		return nil
	}
}
