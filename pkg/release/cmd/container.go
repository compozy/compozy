package cmd

import (
	"github.com/compozy/compozy/pkg/release/internal/config"
	"github.com/compozy/compozy/pkg/release/internal/repository"
	"github.com/compozy/compozy/pkg/release/internal/service"
	"github.com/compozy/compozy/pkg/release/internal/usecase"
	"github.com/spf13/afero"
)

// container holds all the dependencies for the application.

type container struct {
	cfg *config.Config

	fsRepo   repository.FileSystemRepository
	gitRepo  repository.GitRepository
	ghRepo   repository.GithubRepository
	cliffSvc service.CliffService
	npmSvc   service.NpmService
}

// newContainer creates a new container with all the dependencies.
func newContainer() (*container, error) {
	cfg, err := config.LoadConfig()
	if err != nil {
		return nil, err
	}

	fsRepo := repository.FileSystemRepository(afero.NewOsFs())
	gitRepo, err := repository.NewGitRepository()
	if err != nil {
		return nil, err
	}

	// GitHub repository is optional - only create if token is provided
	var ghRepo repository.GithubRepository
	if cfg.GithubToken != "" {
		ghRepo, err = repository.NewGithubRepository(cfg.GithubToken, cfg.GithubOwner, cfg.GithubRepo)
		if err != nil {
			return nil, err
		}
	}

	cliffSvc := service.NewCliffService()
	npmSvc := service.NewNpmService()

	return &container{
		cfg:      cfg,
		fsRepo:   fsRepo,
		gitRepo:  gitRepo,
		ghRepo:   ghRepo,
		cliffSvc: cliffSvc,
		npmSvc:   npmSvc,
	}, nil
}

// func init() {
// 	c, err := newContainer()
// 	if err != nil {
// 		panic(err)
// 	}
//
// 	rootCmd.AddCommand(NewCheckChangesCmd(&usecase.CheckChangesUseCase{
// 		GitRepo:  c.gitRepo,
// 		CliffSvc: c.cliffSvc,
// 	}))
//
// 	rootCmd.AddCommand(NewCalculateVersionCmd(&usecase.CalculateVersionUseCase{
// 		GitRepo:  c.gitRepo,
// 		CliffSvc: c.cliffSvc,
// 	}))
//
// 	rootCmd.AddCommand(NewGenerateChangelogCmd(&usecase.GenerateChangelogUseCase{
// 		CliffSvc: c.cliffSvc,
// 	}))
//
// 	rootCmd.AddCommand(NewCreateReleaseBranchCmd(&usecase.CreateReleaseBranchUseCase{
// 		GitRepo: c.gitRepo,
// 	}))
//
// 	rootCmd.AddCommand(NewCreateGitTagCmd(&usecase.CreateGitTagUseCase{
// 		GitRepo: c.gitRepo,
// 	}))
//
// 	rootCmd.AddCommand(NewUpdatePackageVersionsCmd(&usecase.UpdatePackageVersionsUseCase{
// 		FsRepo:   c.fsRepo,
// 		ToolsDir: c.cfg.ToolsDir,
// 	}))
//
// 	rootCmd.AddCommand(NewPublishNpmPackagesCmd(&usecase.PublishNpmPackagesUseCase{
// 		FsRepo:   c.fsRepo,
// 		NpmSvc:   c.npmSvc,
// 		ToolsDir: c.cfg.ToolsDir,
// 	}))
//
// 	rootCmd.AddCommand(NewPreparePRBodyCmd(&usecase.PreparePRBodyUseCase{}))
//
// 	rootCmd.AddCommand(NewUpdateMainChangelogCmd(&usecase.UpdateMainChangelogUseCase{
// 		FsRepo: c.fsRepo,
// 	}))
// }

// InitCommands initializes all commands with their dependencies
func InitCommands() error {
	c, err := newContainer()
	if err != nil {
		return err
	}

	rootCmd.AddCommand(NewCheckChangesCmd(&usecase.CheckChangesUseCase{
		GitRepo:  c.gitRepo,
		CliffSvc: c.cliffSvc,
	}))

	rootCmd.AddCommand(NewCalculateVersionCmd(&usecase.CalculateVersionUseCase{
		GitRepo:  c.gitRepo,
		CliffSvc: c.cliffSvc,
	}))

	rootCmd.AddCommand(NewGenerateChangelogCmd(&usecase.GenerateChangelogUseCase{
		CliffSvc: c.cliffSvc,
	}))

	rootCmd.AddCommand(NewCreateReleaseBranchCmd(&usecase.CreateReleaseBranchUseCase{
		GitRepo: c.gitRepo,
	}))

	rootCmd.AddCommand(NewCreateGitTagCmd(&usecase.CreateGitTagUseCase{
		GitRepo: c.gitRepo,
	}))

	rootCmd.AddCommand(NewUpdatePackageVersionsCmd(&usecase.UpdatePackageVersionsUseCase{
		FsRepo:   c.fsRepo,
		ToolsDir: c.cfg.ToolsDir,
	}))

	rootCmd.AddCommand(NewPublishNpmPackagesCmd(&usecase.PublishNpmPackagesUseCase{
		FsRepo:   c.fsRepo,
		NpmSvc:   c.npmSvc,
		ToolsDir: c.cfg.ToolsDir,
	}))

	rootCmd.AddCommand(NewPreparePRBodyCmd(&usecase.PreparePRBodyUseCase{}))

	rootCmd.AddCommand(NewUpdateMainChangelogCmd(&usecase.UpdateMainChangelogUseCase{
		FsRepo: c.fsRepo,
	}))

	return nil
}
