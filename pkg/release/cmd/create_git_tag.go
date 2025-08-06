package cmd

import (
	"github.com/compozy/compozy/pkg/release/internal/usecase"
	"github.com/spf13/cobra"
)

func NewCreateGitTagCmd(uc *usecase.CreateGitTagUseCase) *cobra.Command {
	var tagName, message string

	cmd := &cobra.Command{
		Use:   "create-git-tag",
		Short: "Create a Git tag for a new release",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return uc.Execute(cmd.Context(), tagName, message)
		},
	}

	cmd.Flags().StringVar(&tagName, "tag-name", "", "The name of the git tag")
	cmd.Flags().StringVar(&message, "message", "", "The message for the git tag")
	if err := cmd.MarkFlagRequired("tag-name"); err != nil {
		cmd.Printf("Warning: failed to mark flag as required: %v\n", err)
	}

	return cmd
}

// func init() {
// 	uc := &usecase.CreateGitTagUseCase{}
// 	rootCmd.AddCommand(NewCreateGitTagCmd(uc))
// }
