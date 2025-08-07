package orchestrator

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/compozy/compozy/pkg/release/internal/domain"
	"github.com/compozy/compozy/pkg/release/internal/repository"
	"github.com/compozy/compozy/pkg/release/internal/service"
	"github.com/compozy/compozy/pkg/release/internal/usecase"
	"github.com/sethvargo/go-retry"
	"github.com/spf13/afero"
)

const (
	// defaultToolsDir is the default directory for tools.
	defaultToolsDir = "tools"
)

// PRReleaseConfig contains configuration for PR release workflow.
type PRReleaseConfig struct {
	ForceRelease bool
	DryRun       bool
	CIOutput     bool
	SkipPR       bool // For testing without PR creation
}

// PRReleaseOrchestrator orchestrates the entire PR release workflow.
type PRReleaseOrchestrator struct {
	gitRepo    repository.GitExtendedRepository
	githubRepo repository.GithubExtendedRepository
	fsRepo     repository.FileSystemRepository
	cliffSvc   service.CliffService
	npmSvc     service.NpmService
	toolsDir   string
}

// NewPRReleaseOrchestrator creates a new PR release orchestrator.
func NewPRReleaseOrchestrator(
	gitRepo repository.GitExtendedRepository,
	githubRepo repository.GithubExtendedRepository,
	fsRepo repository.FileSystemRepository,
	cliffSvc service.CliffService,
	npmSvc service.NpmService,
) *PRReleaseOrchestrator {
	toolsDir := os.Getenv("TOOLS_DIR")
	if toolsDir == "" {
		toolsDir = defaultToolsDir
	}
	return &PRReleaseOrchestrator{
		gitRepo:    gitRepo,
		githubRepo: githubRepo,
		fsRepo:     fsRepo,
		cliffSvc:   cliffSvc,
		npmSvc:     npmSvc,
		toolsDir:   toolsDir,
	}
}

// Execute runs the complete PR release workflow.
func (o *PRReleaseOrchestrator) Execute(ctx context.Context, cfg PRReleaseConfig) error {
	// Add timeout to match workflow (default 60 minutes for jobs)
	ctx, cancel := context.WithTimeout(ctx, 60*time.Minute)
	defer cancel()
	// Validate required environment variables for GitHub operations
	if err := ValidateEnvironmentVariables([]string{"GITHUB_TOKEN"}); err != nil {
		return fmt.Errorf("environment validation failed: %w", err)
	}
	// Step 1: Check for changes
	hasChanges, latestTag, err := o.checkChanges(ctx)
	if err != nil {
		return fmt.Errorf("failed to check changes: %w", err)
	}
	o.printCIOutput(cfg.CIOutput, "has_changes=%t\n", hasChanges)
	o.printCIOutput(cfg.CIOutput, "latest_tag=%s\n", latestTag)
	if !hasChanges && !cfg.ForceRelease {
		o.printStatus(cfg.CIOutput, "No changes detected since last release")
		return nil
	}
	// Step 2: Calculate version and prepare branch
	version, branchName, err := o.prepareRelease(ctx, latestTag, cfg.CIOutput)
	if err != nil {
		return err
	}
	// Step 3: Update code and create PR
	return o.updateAndCreatePR(ctx, version, branchName, cfg)
}

// prepareRelease calculates version and creates the release branch
func (o *PRReleaseOrchestrator) prepareRelease(
	ctx context.Context,
	latestTag string,
	ciOutput bool,
) (string, string, error) {
	version, err := o.calculateVersion(ctx, latestTag)
	if err != nil {
		return "", "", fmt.Errorf("failed to calculate version: %w", err)
	}
	// Validate version format
	if err := ValidateVersion(version); err != nil {
		return "", "", fmt.Errorf("invalid version: %w", err)
	}
	o.printCIOutput(ciOutput, "version=%s\n", version)
	branchName := fmt.Sprintf("release/%s", version)
	// Validate branch name
	if err := ValidateBranchName(branchName); err != nil {
		return "", "", fmt.Errorf("invalid branch name: %w", err)
	}
	if err := o.createReleaseBranch(ctx, branchName); err != nil {
		return "", "", fmt.Errorf("failed to create release branch: %w", err)
	}
	if err := o.gitRepo.CheckoutBranch(ctx, branchName); err != nil {
		return "", "", fmt.Errorf("failed to checkout release branch: %w", err)
	}
	return version, branchName, nil
}

// updateAndCreatePR updates versions, changelog and creates the PR
func (o *PRReleaseOrchestrator) updateAndCreatePR(
	ctx context.Context,
	version, branchName string,
	cfg PRReleaseConfig,
) error {
	if err := o.updatePackageVersions(ctx, version); err != nil {
		return fmt.Errorf("failed to update package versions: %w", err)
	}
	changelog, err := o.generateChangelog(ctx, version, "unreleased")
	if err != nil {
		return fmt.Errorf("failed to generate changelog: %w", err)
	}
	if err := o.commitChanges(ctx, version); err != nil {
		return fmt.Errorf("failed to commit changes: %w", err)
	}
	if err := o.gitRepo.PushBranch(ctx, branchName); err != nil {
		return fmt.Errorf("failed to push branch: %w", err)
	}
	if !cfg.SkipPR && !cfg.DryRun {
		if err := o.createPullRequest(ctx, version, changelog, branchName); err != nil {
			return fmt.Errorf("failed to create pull request: %w", err)
		}
	}
	o.printStatus(cfg.CIOutput, fmt.Sprintf("✅ Release PR workflow completed for version %s", version))
	return nil
}

// printCIOutput prints output in CI format if enabled
func (o *PRReleaseOrchestrator) printCIOutput(ciOutput bool, format string, args ...any) {
	if ciOutput {
		fmt.Printf(format, args...)
	}
}

// printStatus prints status messages when not in CI mode
func (o *PRReleaseOrchestrator) printStatus(ciOutput bool, message string) {
	if !ciOutput {
		fmt.Println(message)
	}
}

func (o *PRReleaseOrchestrator) checkChanges(ctx context.Context) (bool, string, error) {
	uc := &usecase.CheckChangesUseCase{
		GitRepo:  o.gitRepo,
		CliffSvc: o.cliffSvc,
	}
	return uc.Execute(ctx)
}

func (o *PRReleaseOrchestrator) calculateVersion(ctx context.Context, _ string) (string, error) {
	uc := &usecase.CalculateVersionUseCase{
		GitRepo:  o.gitRepo,
		CliffSvc: o.cliffSvc,
	}
	version, err := uc.Execute(ctx)
	if err != nil {
		return "", err
	}
	return version.String(), nil
}

func (o *PRReleaseOrchestrator) createReleaseBranch(ctx context.Context, branchName string) error {
	uc := &usecase.CreateReleaseBranchUseCase{
		GitRepo: o.gitRepo,
	}
	return uc.Execute(ctx, branchName)
}

func (o *PRReleaseOrchestrator) updatePackageVersions(ctx context.Context, version string) error {
	// Update root package.json version
	versionWithoutV := strings.TrimPrefix(version, "v")
	cmd := exec.CommandContext(ctx, "npm", "version", versionWithoutV, "--no-git-tag-version", "--allow-same-version")
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to update root package.json: %w\nOutput: %s", err, string(output))
	}
	// Update tool packages
	uc := &usecase.UpdatePackageVersionsUseCase{
		FsRepo:   o.fsRepo,
		ToolsDir: o.toolsDir,
	}
	return uc.Execute(ctx, versionWithoutV)
}

func (o *PRReleaseOrchestrator) generateChangelog(ctx context.Context, version, mode string) (string, error) {
	uc := &usecase.GenerateChangelogUseCase{
		CliffSvc: o.cliffSvc,
	}
	changelog, err := uc.Execute(ctx, version, mode)
	if err != nil {
		return "", err
	}
	// Write changelog to file using filesystem repository
	if err := afero.WriteFile(o.fsRepo, "CHANGELOG.md", []byte(changelog), 0644); err != nil {
		return "", fmt.Errorf("failed to write changelog: %w", err)
	}
	// Also create release notes
	if err := afero.WriteFile(o.fsRepo, "RELEASE_NOTES.md", []byte(changelog), 0644); err != nil {
		return "", fmt.Errorf("failed to write release notes: %w", err)
	}
	return changelog, nil
}

func (o *PRReleaseOrchestrator) commitChanges(ctx context.Context, version string) error {
	// Configure git
	user := "github-actions[bot]"
	email := "github-actions[bot]@users.noreply.github.com"
	if err := o.gitRepo.ConfigureUser(ctx, user, email); err != nil {
		return fmt.Errorf("failed to configure git user: %w", err)
	}
	// Add files
	filesToAdd := []string{
		"CHANGELOG.md",
		"package.json",
		"package-lock.json",
		"tools/*/package.json",
	}
	for _, pattern := range filesToAdd {
		// Use git add with pattern, ignore errors for missing files
		if err := o.gitRepo.AddFiles(ctx, pattern); err != nil {
			return fmt.Errorf("failed to add files: %w", err)
		}
	}
	// Commit if there are changes
	message := fmt.Sprintf("ci(release): prepare release %s", version)
	return o.gitRepo.Commit(ctx, message)
}

func (o *PRReleaseOrchestrator) createPullRequest(ctx context.Context, version, changelog, branchName string) error {
	// Create domain version object
	ver, err := domain.NewVersion(version)
	if err != nil {
		return fmt.Errorf("failed to parse version: %w", err)
	}
	// Create domain release object for PR body preparation
	release := &domain.Release{
		Version:   ver,
		Changelog: changelog,
	}
	uc := &usecase.PreparePRBodyUseCase{}
	body, err := uc.Execute(ctx, release)
	if err != nil {
		return fmt.Errorf("failed to prepare PR body: %w", err)
	}
	title := fmt.Sprintf("ci(release): Release %s", version)
	labels := []string{"release-pending", "automated"}
	// Create/Update PR with retry for network failures
	return retry.Do(ctx, retry.WithMaxRetries(3, retry.NewExponential(1*time.Second)), func(ctx context.Context) error {
		return o.githubRepo.CreateOrUpdatePR(ctx, branchName, "main", title, body, labels)
	})
}
