package cli

import (
	"errors"

	"github.com/spf13/cobra"
)

func newBundleUpdateCommand(deps commandDeps) *cobra.Command {
	var confirmNetworkRequirement bool
	var expectedVersion int64
	cmd := &cobra.Command{
		Use:   "update <activation-id>",
		Short: "Reapply a bundle activation from its installed profile",
		Args:  exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			if expectedVersion <= 0 {
				return errors.New("cli: --expected-version must be positive")
			}
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}
			item, err := client.UpdateBundleActivation(
				cmd.Context(),
				args[0],
				UpdateBundleActivationRequest{
					ExpectedVersion:           expectedVersion,
					ConfirmNetworkRequirement: confirmNetworkRequirement,
				},
			)
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, bundleActivationBundle(item))
		},
	}
	bindConfirmNetworkRequirementFlag(cmd, &confirmNetworkRequirement)
	cmd.Flags().Int64Var(&expectedVersion, "expected-version", 0, "Current activation version for optimistic update")
	return cmd
}
