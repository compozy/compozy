package cmd

import (
	"github.com/compozy/compozy/pkg/release/internal/usecase"
	"github.com/spf13/cobra"
)

func NewPublishNpmPackagesCmd(uc *usecase.PublishNpmPackagesUseCase) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "publish-npm-packages",
		Short: "Publish all NPM packages under the tools/ directory",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return uc.Execute(cmd.Context())
		},
	}

	return cmd
}

// func init() {
// 	cfg, err := config.LoadConfig()
// 	if err != nil {
// 		panic(err)
// 	}
//
// 	uc := &usecase.PublishNpmPackagesUseCase{
// 		FsRepo:   repository.FileSystemRepository(afero.NewOsFs()),
// 		ToolsDir: cfg.ToolsDir,
// 	}
// 	rootCmd.AddCommand(NewPublishNpmPackagesCmd(uc))
// }
