package cmd

import (
	"github.com/compozy/compozy/pkg/release/internal/usecase"
	"github.com/spf13/cobra"
)

func NewUpdateMainChangelogCmd(uc *usecase.UpdateMainChangelogUseCase) *cobra.Command {
	var changelog, changelogPath string

	cmd := &cobra.Command{
		Use:   "update-main-changelog",
		Short: "Update the main CHANGELOG.md file with the new release's changelog",
		RunE: func(cmd *cobra.Command, _ []string) error {
			uc.ChangelogPath = changelogPath
			return uc.Execute(cmd.Context(), changelog)
		},
	}

	cmd.Flags().StringVar(&changelog, "changelog", "", "The changelog for the new release")
	cmd.Flags().StringVar(&changelogPath, "changelog-path", "CHANGELOG.md", "The path to the main changelog file")
	// MarkFlagRequired should not fail for valid flag names, but handle it gracefully
	if err := cmd.MarkFlagRequired("changelog"); err != nil {
		// This should never happen with a valid flag name, but log and continue
		cmd.Printf("Warning: failed to mark flag as required: %v\n", err)
	}
	return cmd
}

// func init() {
// 	uc := &usecase.UpdateMainChangelogUseCase{
// 		FsRepo: repository.FileSystemRepository(afero.NewOsFs()),
// 	}
// 	rootCmd.AddCommand(NewUpdateMainChangelogCmd(uc))
// }
