package cli

import "github.com/spf13/cobra"

const (
	profileFlagName = "profile"
	profileEnvName  = "COMPOZY_PROFILE"
)

func configureRootProfileFlag(cmd *cobra.Command) {
	cmd.PersistentFlags().String(profileFlagName, "", "Act as a profile for this command")
}

func commandProfileFlag(cmd *cobra.Command) (string, error) {
	return cmd.Flags().GetString(profileFlagName)
}
