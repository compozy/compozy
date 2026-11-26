package cli

import (
	"errors"
	"fmt"
	"slices"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

var removedGatewayVerbs = []string{"connect", "gateway", "pair", "device"}

func isRemovedGatewayCommand(name string) bool {
	return slices.Contains(removedGatewayVerbs, name)
}

// The 0.8.0 tombstones are removed in 0.9.0.
func registerRemovedGatewayCommands(root *cobra.Command) {
	for _, name := range removedGatewayVerbs {
		root.AddCommand(&cobra.Command{
			Use: name, Hidden: true, DisableFlagParsing: true,
			RunE: func(_ *cobra.Command, _ []string) error {
				return removedGatewayUsage("compozy " + name)
			},
		})
	}
	root.SetFlagErrorFunc(func(_ *cobra.Command, err error) error {
		flag, ok := errors.AsType[*pflag.NotExistError](err)
		if ok && flag.GetSpecifiedName() == "confirm-gateway-requirement" {
			return removedGatewayUsage("--confirm-gateway-requirement")
		}
		return err
	})
}

type removedGatewayError string

func (e removedGatewayError) Error() string { return string(e) }

func (removedGatewayError) cliExitCode() int { return 2 }

func removedGatewayUsage(name string) error {
	return removedGatewayError(fmt.Sprintf(
		"%s was removed in 0.8.0; use compozy computer — see https://compozy.com/docs/computers", name,
	))
}
