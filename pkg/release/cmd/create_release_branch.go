package cmd

import (
	"github.com/compozy/compozy/pkg/release/internal/usecase"
	"github.com/spf13/cobra"
)

func NewCreateReleaseBranchCmd(uc *usecase.CreateReleaseBranchUseCase) *cobra.Command {
	var branchName string

	cmd := &cobra.Command{
		Use:   "create-release-branch",
		Short: "Create a release branch",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return uc.Execute(cmd.Context(), branchName)
		},
	}

	cmd.Flags().StringVar(&branchName, "branch-name", "", "The name of the release branch")
	if err := cmd.MarkFlagRequired("branch-name"); err != nil {
		cmd.Printf("Warning: failed to mark flag as required: %v\n", err)
	}

	return cmd
}

// func init() {
// 	uc := &usecase.CreateReleaseBranchUseCase{}
// 	rootCmd.AddCommand(NewCreateReleaseBranchCmd(uc))
// }
