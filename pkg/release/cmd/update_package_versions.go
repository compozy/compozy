package cmd

import (
	"github.com/compozy/compozy/pkg/release/internal/usecase"
	"github.com/spf13/cobra"
)

func NewUpdatePackageVersionsCmd(uc *usecase.UpdatePackageVersionsUseCase) *cobra.Command {
	var version string

	cmd := &cobra.Command{
		Use:   "update-package-versions",
		Short: "Update the version of all NPM packages under the tools/ directory",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return uc.Execute(cmd.Context(), version)
		},
	}

	cmd.Flags().StringVar(&version, "version", "", "The new version to set")
	if err := cmd.MarkFlagRequired("version"); err != nil {
		cmd.Printf("Warning: failed to mark flag as required: %v\n", err)
	}

	return cmd
}

// func init() {
// 	cfg, err := config.LoadConfig()
// 	if err != nil {
// 		panic(err)
// 	}
//
// 	uc := &usecase.UpdatePackageVersionsUseCase{
// 		FsRepo:   repository.FileSystemRepository(afero.NewOsFs()),
// 		ToolsDir: cfg.ToolsDir,
// 	}
// 	rootCmd.AddCommand(NewUpdatePackageVersionsCmd(uc))
// }
