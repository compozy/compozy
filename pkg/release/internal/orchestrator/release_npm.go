package orchestrator

import (
	"context"
	"fmt"
	"os"

	"github.com/compozy/compozy/pkg/release/internal/repository"
	"github.com/compozy/compozy/pkg/release/internal/service"
	"github.com/compozy/compozy/pkg/release/internal/usecase"
	"github.com/sethvargo/go-retry"
)

// ReleaseNPMConfig contains configuration for the NPM release workflow.
type ReleaseNPMConfig struct {
	CIOutput bool
	DryRun   bool
}

// ReleaseNPMOrchestrator orchestrates the NPM package publishing workflow.
type ReleaseNPMOrchestrator struct {
	fsRepo   repository.FileSystemRepository
	npmSvc   service.NpmService
	toolsDir string
}

// NewReleaseNPMOrchestrator creates a new NPM release orchestrator.
func NewReleaseNPMOrchestrator(
	fsRepo repository.FileSystemRepository,
	npmSvc service.NpmService,
) *ReleaseNPMOrchestrator {
	toolsDir := os.Getenv("TOOLS_DIR")
	if toolsDir == "" {
		toolsDir = defaultToolsDir
	}
	return &ReleaseNPMOrchestrator{
		fsRepo:   fsRepo,
		npmSvc:   npmSvc,
		toolsDir: toolsDir,
	}
}

// Execute runs the NPM publishing workflow.
func (o *ReleaseNPMOrchestrator) Execute(ctx context.Context, cfg ReleaseNPMConfig) error {
	// Add timeout for NPM operations
	ctx, cancel := context.WithTimeout(ctx, DefaultWorkflowTimeout)
	defer cancel()
	// Validate required environment variables
	requiredVars := []string{"NPM_TOKEN"}
	if err := ValidateEnvironmentVariables(requiredVars); err != nil {
		return fmt.Errorf("environment validation failed: %w", err)
	}
	// Publish NPM packages
	if !cfg.DryRun {
		if err := o.publishNPM(ctx); err != nil {
			return fmt.Errorf("failed to publish NPM packages: %w", err)
		}
	}
	o.printStatus(cfg.CIOutput, "✅ NPM packages published successfully")
	return nil
}

// publishNPM publishes NPM packages with retry logic
func (o *ReleaseNPMOrchestrator) publishNPM(ctx context.Context) error {
	uc := &usecase.PublishNpmPackagesUseCase{
		FsRepo:   o.fsRepo,
		NpmSvc:   o.npmSvc,
		ToolsDir: o.toolsDir,
	}
	// Publish NPM with retry for network failures
	return retry.Do(
		ctx,
		retry.WithMaxRetries(DefaultRetryCount, retry.NewExponential(DefaultRetryDelay)),
		func(ctx context.Context) error {
			return uc.Execute(ctx)
		},
	)
}

// printStatus prints status messages when not in CI mode
func (o *ReleaseNPMOrchestrator) printStatus(ciOutput bool, message string) {
	if !ciOutput {
		fmt.Println(message)
	}
}
