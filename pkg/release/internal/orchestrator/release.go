package orchestrator

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"

	"github.com/compozy/compozy/pkg/release/internal/repository"
	"github.com/compozy/compozy/pkg/release/internal/service"
	"github.com/compozy/compozy/pkg/release/internal/usecase"
	"github.com/sethvargo/go-retry"
	"github.com/spf13/afero"
)

// ReleaseConfig contains configuration for the release workflow.
type ReleaseConfig struct {
	Version       string // Can be extracted or provided
	SkipTag       bool
	SkipNPM       bool
	SkipChangelog bool
	CIOutput      bool
	DryRun        bool
}

// ReleaseOrchestrator orchestrates the production release workflow.
type ReleaseOrchestrator struct {
	gitRepo       repository.GitExtendedRepository
	githubRepo    repository.GithubExtendedRepository
	fsRepo        repository.FileSystemRepository
	cliffSvc      service.CliffService
	npmSvc        service.NpmService
	goreleaserSvc service.GoReleaserService
	toolsDir      string
}

// NewReleaseOrchestrator creates a new release orchestrator.
func NewReleaseOrchestrator(
	gitRepo repository.GitExtendedRepository,
	githubRepo repository.GithubExtendedRepository,
	fsRepo repository.FileSystemRepository,
	cliffSvc service.CliffService,
	npmSvc service.NpmService,
	goreleaserSvc service.GoReleaserService,
) *ReleaseOrchestrator {
	toolsDir := os.Getenv("TOOLS_DIR")
	if toolsDir == "" {
		toolsDir = defaultToolsDir
	}
	return &ReleaseOrchestrator{
		gitRepo:       gitRepo,
		githubRepo:    githubRepo,
		fsRepo:        fsRepo,
		cliffSvc:      cliffSvc,
		npmSvc:        npmSvc,
		goreleaserSvc: goreleaserSvc,
		toolsDir:      toolsDir,
	}
}

// Execute runs the complete release workflow.
func (o *ReleaseOrchestrator) Execute(ctx context.Context, cfg ReleaseConfig) error {
	// Add timeout to match workflow (120 minutes for release job as per YAML)
	ctx, cancel := context.WithTimeout(ctx, ReleaseWorkflowTimeout)
	defer cancel()
	// Validate required environment variables
	requiredVars := []string{"GITHUB_TOKEN"}
	if !cfg.SkipNPM {
		requiredVars = append(requiredVars, "NPM_TOKEN")
	}
	if err := ValidateEnvironmentVariables(requiredVars); err != nil {
		return fmt.Errorf("environment validation failed: %w", err)
	}
	// Step 1: Extract/validate version
	version, err := o.extractVersion(ctx, cfg.Version)
	if err != nil {
		return fmt.Errorf("failed to extract version: %w", err)
	}
	o.printCIOutput(cfg.CIOutput, "version=%s\n", version)
	// Step 2: Check if release should proceed
	if shouldSkip, err := o.checkIdempotency(ctx, version, cfg); err != nil {
		return err
	} else if shouldSkip {
		return nil
	}
	// Step 3: Execute release steps
	if err := o.executeReleaseSteps(ctx, version, cfg); err != nil {
		return err
	}
	o.printStatus(cfg.CIOutput, fmt.Sprintf("✅ Release %s completed successfully", version))
	return nil
}

// checkIdempotency checks if release should be skipped due to existing tag
func (o *ReleaseOrchestrator) checkIdempotency(ctx context.Context, version string, cfg ReleaseConfig) (bool, error) {
	exists, err := o.tagExists(ctx, version)
	if err != nil {
		return false, fmt.Errorf("failed to check tag existence: %w", err)
	}
	if exists && !cfg.SkipTag {
		o.printStatus(cfg.CIOutput, fmt.Sprintf("Tag %s already exists, skipping release", version))
		return true, nil
	}
	return false, nil
}

// executeReleaseSteps runs all release steps
func (o *ReleaseOrchestrator) executeReleaseSteps(ctx context.Context, version string, cfg ReleaseConfig) error {
	// Create git tag
	if !cfg.SkipTag && !cfg.DryRun {
		if err := o.createTag(ctx, version); err != nil {
			return fmt.Errorf("failed to create tag: %w", err)
		}
	}
	// Generate changelog and run release tools
	changelog, err := o.processChangelog(ctx, version, cfg)
	if err != nil {
		return err
	}
	// Run release tools
	return o.runReleaseTools(ctx, version, changelog, cfg)
}

// processChangelog handles changelog generation
func (o *ReleaseOrchestrator) processChangelog(ctx context.Context, version string, cfg ReleaseConfig) (string, error) {
	if cfg.SkipChangelog {
		return "", nil
	}
	changelog, err := o.generateChangelog(ctx, version, "release")
	if err != nil {
		return "", fmt.Errorf("failed to generate changelog: %w", err)
	}
	return changelog, nil
}

// runReleaseTools executes GoReleaser, NPM publish, and changelog update
func (o *ReleaseOrchestrator) runReleaseTools(ctx context.Context, version, changelog string, cfg ReleaseConfig) error {
	if !cfg.DryRun {
		if err := o.runGoReleaser(ctx, version, changelog); err != nil {
			return fmt.Errorf("failed to run GoReleaser: %w", err)
		}
		if !cfg.SkipNPM {
			if err := o.publishNPM(ctx); err != nil {
				return fmt.Errorf("failed to publish NPM packages: %w", err)
			}
		}
		if !cfg.SkipChangelog && changelog != "" {
			if err := o.updateMainChangelog(ctx, changelog); err != nil {
				return fmt.Errorf("failed to update main changelog: %w", err)
			}
		}
	}
	return nil
}

// printCIOutput prints output in CI format if enabled
func (o *ReleaseOrchestrator) printCIOutput(ciOutput bool, format string, args ...any) {
	if ciOutput {
		fmt.Printf(format, args...)
	}
}

// printStatus prints status messages when not in CI mode
func (o *ReleaseOrchestrator) printStatus(ciOutput bool, message string) {
	if !ciOutput {
		fmt.Println(message)
	}
}

func (o *ReleaseOrchestrator) extractVersion(ctx context.Context, providedVersion string) (string, error) {
	// If version is provided, validate and use it
	if providedVersion != "" {
		if err := ValidateVersion(providedVersion); err != nil {
			return "", fmt.Errorf("invalid provided version: %w", err)
		}
		return providedVersion, nil
	}
	// Extract from commit message
	commitMsg := os.Getenv("GITHUB_EVENT_HEAD_COMMIT_MESSAGE")
	if commitMsg == "" {
		// Try to get from latest commit
		cmd := exec.CommandContext(ctx, "git", "log", "-1", "--pretty=%B")
		if os.Getenv("GITHUB_ACTIONS") == githubActionsTrue {
			cmd.Stderr = os.Stderr
		}
		output, err := cmd.Output()
		if err != nil {
			return "", fmt.Errorf("failed to get commit message: %w", err)
		}
		commitMsg = string(output)
	}
	// Extract version using regex
	re := regexp.MustCompile(`v?\d+\.\d+\.\d+`)
	matches := re.FindStringSubmatch(commitMsg)
	if len(matches) == 0 {
		return "", fmt.Errorf("no version found in commit message")
	}
	version := matches[0]
	// Ensure version has 'v' prefix
	if !strings.HasPrefix(version, "v") {
		version = "v" + version
	}
	// Validate extracted version
	if err := ValidateVersion(version); err != nil {
		return "", fmt.Errorf("invalid extracted version: %w", err)
	}
	return version, nil
}

func (o *ReleaseOrchestrator) tagExists(ctx context.Context, tag string) (bool, error) {
	return o.gitRepo.TagExists(ctx, tag)
}

func (o *ReleaseOrchestrator) createTag(ctx context.Context, tag string) error {
	uc := &usecase.CreateGitTagUseCase{
		GitRepo: o.gitRepo,
	}
	message := fmt.Sprintf("Release %s", tag)
	// Create and push tag with retry for network failures
	return retry.Do(
		ctx,
		retry.WithMaxRetries(DefaultRetryCount, retry.NewExponential(DefaultRetryDelay)),
		func(ctx context.Context) error {
			return uc.Execute(ctx, tag, message)
		},
	)
}

func (o *ReleaseOrchestrator) generateChangelog(ctx context.Context, version, mode string) (string, error) {
	uc := &usecase.GenerateChangelogUseCase{
		CliffSvc: o.cliffSvc,
	}
	changelog, err := uc.Execute(ctx, version, mode)
	if err != nil {
		return "", err
	}
	// Write to RELEASE_NOTES.md for GoReleaser using filesystem repository
	if err := afero.WriteFile(o.fsRepo, "RELEASE_NOTES.md", []byte(changelog), FilePermissionsReadWrite); err != nil {
		return "", fmt.Errorf("failed to write release notes: %w", err)
	}
	return changelog, nil
}

func (o *ReleaseOrchestrator) runGoReleaser(ctx context.Context, _, changelog string) error {
	// GoReleaser is called externally via GitHub Actions
	// This is just a placeholder for the integration
	if o.goreleaserSvc != nil {
		// Write changelog to file for GoReleaser to use
		if changelog != "" {
			if err := os.WriteFile("RELEASE_NOTES.md", []byte(changelog), FilePermissionsSecure); err != nil {
				return fmt.Errorf("failed to write release notes for GoReleaser: %w", err)
			}
		}
		// Run GoReleaser with appropriate arguments
		return o.goreleaserSvc.Run(ctx, "release", "--clean", "--release-notes=RELEASE_NOTES.md")
	}
	// For now, we assume GoReleaser is called separately in the workflow
	return nil
}

func (o *ReleaseOrchestrator) publishNPM(ctx context.Context) error {
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

func (o *ReleaseOrchestrator) updateMainChangelog(ctx context.Context, changelog string) error {
	uc := &usecase.UpdateMainChangelogUseCase{
		FsRepo:        o.fsRepo,
		ChangelogPath: "CHANGELOG.md",
	}
	return uc.Execute(ctx, changelog)
}
