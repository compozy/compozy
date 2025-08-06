package cmd

import (
	"fmt"

	"github.com/compozy/compozy/pkg/release/internal/usecase"
	"github.com/spf13/cobra"
)

func NewCheckChangesCmd(uc *usecase.CheckChangesUseCase) *cobra.Command {
	var ciOutput bool
	cmd := &cobra.Command{
		Use:   "check-changes",
		Short: "Check if there are pending changes for a new release",
		RunE: func(cmd *cobra.Command, _ []string) error {
			hasChanges, latestTag, err := uc.Execute(cmd.Context())
			if err != nil {
				return err
			}

			if ciOutput {
				// GitHub Actions output format
				fmt.Fprintf(cmd.OutOrStdout(), "has_changes=%t\n", hasChanges)
				fmt.Fprintf(cmd.OutOrStdout(), "latest_tag=%s\n", latestTag)
			} else {
				// Human-readable output
				fmt.Fprintf(cmd.OutOrStdout(), "Has changes: %t\n", hasChanges)
				fmt.Fprintf(cmd.OutOrStdout(), "Latest tag: %s\n", latestTag)
			}

			return nil
		},
	}
	cmd.Flags().BoolVar(&ciOutput, "ci-output", false, "Output in CI-friendly format for GitHub Actions")
	return cmd
}

// init will be called during the initialization of the package, and we can add the command to the root command here
// func init() {
// 	// We need to instantiate the use case with its dependencies.
// 	// For now, we'll use nil, but we'll replace this with the actual dependencies later.
// 	uc := &usecase.CheckChangesUseCase{}
// 	rootCmd.AddCommand(NewCheckChangesCmd(uc))
// }
