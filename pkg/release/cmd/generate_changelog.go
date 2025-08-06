package cmd

import (
	"fmt"

	"github.com/compozy/compozy/pkg/release/internal/usecase"
	"github.com/spf13/cobra"
)

func NewGenerateChangelogCmd(uc *usecase.GenerateChangelogUseCase) *cobra.Command {
	var version, mode string

	cmd := &cobra.Command{
		Use:   "generate-changelog",
		Short: "Generate a changelog for a new release",
		RunE: func(cmd *cobra.Command, _ []string) error {
			changelog, err := uc.Execute(cmd.Context(), version, mode)
			if err != nil {
				return err
			}

			fmt.Fprintln(cmd.OutOrStdout(), changelog)

			return nil
		},
	}

	cmd.Flags().StringVar(&version, "version", "", "The version to generate the changelog for")
	cmd.Flags().StringVar(&mode, "mode", "unreleased", "The mode to generate the changelog in (unreleased or release)")
	if err := cmd.MarkFlagRequired("version"); err != nil {
		cmd.Printf("Warning: failed to mark flag as required: %v\n", err)
	}

	return cmd
}

// func init() {
// 	uc := &usecase.GenerateChangelogUseCase{}
// 	rootCmd.AddCommand(NewGenerateChangelogCmd(uc))
// }
