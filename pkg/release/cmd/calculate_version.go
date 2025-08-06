package cmd

import (
	"fmt"

	"github.com/compozy/compozy/pkg/release/internal/usecase"
	"github.com/spf13/cobra"
)

func NewCalculateVersionCmd(uc *usecase.CalculateVersionUseCase) *cobra.Command {
	var ciOutput bool
	cmd := &cobra.Command{
		Use:   "calculate-version",
		Short: "Calculate the next semantic version based on conventional commit messages",
		RunE: func(cmd *cobra.Command, _ []string) error {
			version, err := uc.Execute(cmd.Context())
			if err != nil {
				return err
			}

			if ciOutput {
				// GitHub Actions output format
				fmt.Fprintf(cmd.OutOrStdout(), "version=%s\n", version.String())
			} else {
				// Human-readable output
				fmt.Fprintf(cmd.OutOrStdout(), "Next version: %s\n", version.String())
			}

			return nil
		},
	}
	cmd.Flags().BoolVar(&ciOutput, "ci-output", false, "Output in CI-friendly format for GitHub Actions")
	return cmd
}

// func init() {
// 	uc := &usecase.CalculateVersionUseCase{}
// 	rootCmd.AddCommand(NewCalculateVersionCmd(uc))
// }
