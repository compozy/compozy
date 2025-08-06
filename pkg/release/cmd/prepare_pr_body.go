package cmd

import (
	"fmt"

	"github.com/compozy/compozy/pkg/release/internal/domain"
	"github.com/compozy/compozy/pkg/release/internal/usecase"
	"github.com/spf13/cobra"
)

func NewPreparePRBodyCmd(uc *usecase.PreparePRBodyUseCase) *cobra.Command {
	var version, changelog string

	cmd := &cobra.Command{
		Use:   "prepare-pr-body",
		Short: "Prepare the body for a release pull request",
		RunE: func(cmd *cobra.Command, _ []string) error {
			v, err := domain.NewVersion(version)
			if err != nil {
				return err
			}

			release := &domain.Release{
				Version:   v,
				Changelog: changelog,
			}

			body, err := uc.Execute(cmd.Context(), release)
			if err != nil {
				return err
			}

			fmt.Fprintln(cmd.OutOrStdout(), body)

			return nil
		},
	}

	cmd.Flags().StringVar(&version, "version", "", "The version of the release")
	cmd.Flags().StringVar(&changelog, "changelog", "", "The changelog for the release")
	if err := cmd.MarkFlagRequired("version"); err != nil {
		cmd.Printf("Warning: failed to mark flag as required: %v\n", err)
	}
	if err := cmd.MarkFlagRequired("changelog"); err != nil {
		cmd.Printf("Warning: failed to mark flag as required: %v\n", err)
	}

	return cmd
}

// func init() {
// 	uc := &usecase.PreparePRBodyUseCase{}
// 	rootCmd.AddCommand(NewPreparePRBodyCmd(uc))
// }
