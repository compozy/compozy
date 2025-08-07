package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/compozy/compozy/pkg/release/internal/config"
	"github.com/compozy/compozy/pkg/release/internal/orchestrator"
	"github.com/compozy/compozy/pkg/release/internal/repository"
	"github.com/compozy/compozy/pkg/release/internal/service"
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

// InitCommands initializes all commands with their dependencies
func InitCommands() error {
	c, err := newContainer()
	if err != nil {
		return err
	}

	// Individual commands have been replaced by orchestrator commands

	// Add orchestrator-based commands
	if err := addOrchestratorCommands(c); err != nil {
		return err
	}

	return nil
}

// addOrchestratorCommands adds the new consolidated commands
func addOrchestratorCommands(c *container) error {
	// Initialize extended repositories for orchestrators
	gitExtRepo, err := repository.NewGitExtendedRepository()
	if err != nil {
		return fmt.Errorf("failed to initialize git extended repository: %w", err)
	}

	// Get GitHub configuration from environment for orchestrator commands
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		token = os.Getenv("RELEASE_TOKEN")
	}
	if token == "" {
		// GitHub commands are optional - only add if token is available
		return nil
	}

	owner := os.Getenv("GITHUB_REPOSITORY_OWNER")
	if owner == "" {
		// Try to extract from GITHUB_REPOSITORY (format: owner/repo)
		repoEnv := os.Getenv("GITHUB_REPOSITORY")
		if repoEnv != "" {
			if idx := strings.Index(repoEnv, "/"); idx > 0 {
				owner = repoEnv[:idx]
			}
		}
	}
	if owner == "" {
		// GitHub commands are optional
		return nil
	}

	repo := os.Getenv("GITHUB_REPOSITORY_NAME")
	if repo == "" {
		// Try to extract from GITHUB_REPOSITORY (format: owner/repo)
		repoEnv := os.Getenv("GITHUB_REPOSITORY")
		if repoEnv != "" {
			if idx := strings.Index(repoEnv, "/"); idx > 0 && idx < len(repoEnv)-1 {
				repo = repoEnv[idx+1:]
			}
		}
	}
	if repo == "" {
		// Default to "compozy"
		repo = "compozy"
	}

	githubExtRepo, err := repository.NewGithubExtendedRepository(token, owner, repo)
	if err != nil {
		return fmt.Errorf("failed to initialize GitHub extended repository: %w", err)
	}

	// Create PR Release orchestrator
	prOrch := orchestrator.NewPRReleaseOrchestrator(
		gitExtRepo,
		githubExtRepo,
		c.fsRepo,
		c.cliffSvc,
		c.npmSvc,
	)
	rootCmd.AddCommand(NewPRReleaseCmd(prOrch))

	// Create Release orchestrator
	releaseOrch := orchestrator.NewReleaseOrchestrator(
		gitExtRepo,
		githubExtRepo,
		c.fsRepo,
		c.cliffSvc,
		c.npmSvc,
		nil, // GoReleaser service is optional
	)
	rootCmd.AddCommand(NewReleaseCmd(releaseOrch))

	// Create Dry Run orchestrator
	goreleaserSvc := service.NewGoReleaserService()
	dryRunOrch := orchestrator.NewDryRunOrchestrator(
		gitExtRepo,
		githubExtRepo,
		c.cliffSvc,
		goreleaserSvc,
		c.fsRepo,
		c.cfg.ToolsDir,
	)
	rootCmd.AddCommand(NewDryRunCmd(dryRunOrch))

	// Create NPM Release orchestrator
	npmReleaseOrch := orchestrator.NewReleaseNPMOrchestrator(
		c.fsRepo,
		c.npmSvc,
	)
	rootCmd.AddCommand(NewReleaseNPMCmd(npmReleaseOrch))

	return nil
}
