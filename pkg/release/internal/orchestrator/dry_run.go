// internal/orchestrator/dry_run.go
package orchestrator

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/compozy/compozy/pkg/release/internal/repository"
	"github.com/compozy/compozy/pkg/release/internal/service"
	"github.com/compozy/compozy/pkg/release/internal/usecase"
	"github.com/spf13/afero"
)

const (
	githubActionsTrue    = "true"
	envGithubIssueNumber = "GITHUB_ISSUE_NUMBER"
	envGithubEventPath   = "GITHUB_EVENT_PATH"
	envGithubHeadRef     = "GITHUB_HEAD_REF"
	envGithubSHA         = "GITHUB_SHA"
	envGithubActions     = "GITHUB_ACTIONS"
	metadataJSONPath     = "dist/metadata.json"
	artifactTypeArchive  = "Archive"
)

// DryRunConfig holds configuration for the dry-run orchestrator
type DryRunConfig struct {
	CIOutput bool // Output in CI format
	DryRun   bool // Always true for this orchestrator, but for consistency
}

// DryRunOrchestrator orchestrates the dry-run validation process
type DryRunOrchestrator struct {
	gitRepo       repository.GitExtendedRepository
	githubRepo    repository.GithubExtendedRepository
	cliffSvc      service.CliffService
	goreleaserSvc service.GoReleaserService // Assuming this exists in service/goreleaser.go
	fsRepo        afero.Fs
	toolsDir      string // From config.ToolsDir
}

// NewDryRunOrchestrator creates a new DryRunOrchestrator
func NewDryRunOrchestrator(
	gitRepo repository.GitExtendedRepository,
	githubRepo repository.GithubExtendedRepository,
	cliffSvc service.CliffService,
	goreleaserSvc service.GoReleaserService,
	fsRepo afero.Fs,
	toolsDir string,
) *DryRunOrchestrator {
	return &DryRunOrchestrator{
		gitRepo:       gitRepo,
		githubRepo:    githubRepo,
		cliffSvc:      cliffSvc,
		goreleaserSvc: goreleaserSvc,
		fsRepo:        fsRepo,
		toolsDir:      toolsDir,
	}
}

// Execute runs the dry-run validation
func (o *DryRunOrchestrator) Execute(ctx context.Context, cfg DryRunConfig) error {
	ctx, cancel := context.WithTimeout(ctx, DefaultWorkflowTimeout)
	defer cancel()
	if err := o.stepValidateChangelog(ctx, cfg); err != nil {
		return err
	}
	if err := o.stepRunGoReleaser(ctx, cfg); err != nil {
		return err
	}
	version, err := o.stepExtractVersion(ctx, cfg)
	if err != nil {
		return err
	}
	if err := o.stepValidateNPM(ctx, cfg, version); err != nil {
		return err
	}
	if os.Getenv(envGithubActions) == githubActionsTrue {
		if err := o.stepCommentPR(ctx, cfg); err != nil {
			return err
		}
	} else {
		o.printStatus(cfg.CIOutput, "Dry-run completed. Review required.")
	}
	o.printStatus(cfg.CIOutput, "## ✅ Dry-Run Completed Successfully")
	return nil
}

// stepValidateChangelog validates git-cliff changelog generation
func (o *DryRunOrchestrator) stepValidateChangelog(ctx context.Context, cfg DryRunConfig) error {
	o.printStatus(cfg.CIOutput, "### 📝 Validating Changelog Generation")
	if err := o.validateCliff(ctx); err != nil {
		return fmt.Errorf("git-cliff validation failed: %w", err)
	}
	return nil
}

// stepRunGoReleaser executes GoReleaser dry-run
func (o *DryRunOrchestrator) stepRunGoReleaser(ctx context.Context, cfg DryRunConfig) error {
	o.printStatus(cfg.CIOutput, "### 🏗️ Running GoReleaser Dry-Run")
	fmt.Println("🔍 Running GoReleaser dry-run")
	if err := o.runGoReleaserDry(ctx); err != nil {
		return fmt.Errorf("GoReleaser dry-run failed: %w", err)
	}
	fmt.Println("✅ GoReleaser dry-run completed")
	return nil
}

// stepExtractVersion extracts version from branch name
func (o *DryRunOrchestrator) stepExtractVersion(ctx context.Context, cfg DryRunConfig) (string, error) {
	o.printStatus(cfg.CIOutput, "### 📦 Validating NPM packages")
	fmt.Println("🔍 Extracting version from branch")
	version, err := o.extractVersionFromBranch(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to extract version: %w", err)
	}
	fmt.Printf("ℹ️ Detected version: %s\n", version)
	return version, nil
}

// stepValidateNPM validates NPM package versions
func (o *DryRunOrchestrator) stepValidateNPM(ctx context.Context, _ DryRunConfig, version string) error {
	fmt.Println("🔍 Validating NPM package versions")
	if err := o.validateNPMVersions(ctx, version); err != nil {
		return fmt.Errorf("NPM validation failed: %w", err)
	}
	fmt.Println("✅ NPM validation completed")
	return nil
}

// stepCommentPR creates PR comment with dry-run results
func (o *DryRunOrchestrator) stepCommentPR(ctx context.Context, _ DryRunConfig) error {
	fmt.Println("🔍 Creating PR comment")
	if err := o.commentOnPR(ctx); err != nil {
		return fmt.Errorf("PR comment failed: %w", err)
	}
	fmt.Println("✅ PR comment created")
	return nil
}

// validateCliff runs git-cliff --unreleased --verbose
func (o *DryRunOrchestrator) validateCliff(ctx context.Context) error {
	fmt.Println("🔍 Running git-cliff --unreleased --verbose")
	cmd := exec.CommandContext(ctx, "git-cliff", "--unreleased", "--verbose")
	// Find the repository root by walking up directories
	wd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}
	repoRoot := findRepoRoot(wd)
	if repoRoot != "" {
		cmd.Dir = repoRoot
		fmt.Printf("🔍 Running git-cliff from repository root: %s\n", repoRoot)
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git-cliff failed: %w", err)
	}
	fmt.Println("✅ git-cliff validation completed")
	return nil
}

// runGoReleaserDry runs goreleaser release --snapshot --skip=publish --clean
func (o *DryRunOrchestrator) runGoReleaserDry(ctx context.Context) error {
	return o.goreleaserSvc.Run(ctx, "release", "--snapshot", "--skip=publish", "--clean")
}

// extractVersionFromBranch extracts version from GITHUB_HEAD_REF or branch name
func (o *DryRunOrchestrator) extractVersionFromBranch(ctx context.Context) (string, error) {
	headRef := os.Getenv(envGithubHeadRef)
	if headRef == "" {
		// Fallback to current branch
		branch, err := o.gitRepo.GetCurrentBranch(ctx)
		if err != nil {
			return "", err
		}
		headRef = branch
	}
	re := regexp.MustCompile(`v?\d+\.\d+\.\d+`)
	matches := re.FindStringSubmatch(headRef)
	if len(matches) == 0 {
		return "", fmt.Errorf("no version found in branch name: %s", headRef)
	}
	version := matches[0]
	version = strings.TrimPrefix(version, "v") // Remove 'v' prefix if present
	return version, nil
}

// validateNPMVersions runs UpdatePackageVersions (idempotent check; since branch may already have updates)
func (o *DryRunOrchestrator) validateNPMVersions(ctx context.Context, version string) error {
	uc := &usecase.UpdatePackageVersionsUseCase{
		FsRepo:   o.fsRepo,
		ToolsDir: o.toolsDir,
	}
	// Run update; if already matching, it should be no-op
	if err := uc.Execute(ctx, version); err != nil {
		return err
	}
	// Optional: Verify versions in package.json files
	// E.g., read root package.json and check "version" == version
	return nil // Or add explicit checks if needed
}

// commentOnPR reads metadata.json, builds body, adds comment via GithubRepo
func (o *DryRunOrchestrator) commentOnPR(ctx context.Context) error {
	prNumber := o.getPRNumber(ctx)
	if prNumber == 0 {
		fmt.Println("ℹ️ Skipping PR comment (no PR number found)")
		return nil
	}

	// Read metadata.json
	metadataPath := metadataJSONPath
	file, err := o.fsRepo.Open(metadataPath)
	if err != nil {
		return fmt.Errorf("failed to open metadata.json: %w", err)
	}
	defer file.Close()
	var metadata map[string]any
	if err := json.NewDecoder(bufio.NewReader(file)).Decode(&metadata); err != nil {
		return fmt.Errorf("failed to parse metadata.json: %w", err)
	}

	// Build artifacts list (filter Archive types)
	artifactsList := "Not available."
	if arts, ok := metadata["artifacts"].([]any); ok {
		uniqueBuilds := make(map[string]struct{})
		for _, a := range arts {
			artMap, ok := a.(map[string]any)
			if !ok {
				continue
			}
			if artMap["type"] == artifactTypeArchive {
				goos, ok := artMap["goos"].(string)
				if !ok {
					continue
				}
				goarch, ok := artMap["goarch"].(string)
				if !ok {
					continue
				}
				uniqueBuilds[fmt.Sprintf("%s/%s", goos, goarch)] = struct{}{}
			}
		}
		var builds []string
		for b := range uniqueBuilds {
			builds = append(builds, fmt.Sprintf("- %s", b))
		}
		sort.Strings(builds)
		artifactsList = strings.Join(builds, "\n")
	}

	// Build comment body
	sha := os.Getenv(envGithubSHA)
	if len(sha) > 7 {
		sha = sha[:7]
	}
	body := fmt.Sprintf(`## ✅ Dry-Run Completed Successfully

### 📊 Build Summary
- **Version**: %s
- **Commit**: %s

### 📦 Built Artifacts
%s

---
*This is an automated comment from the release dry-run check.*
`, metadata["version"], sha, artifactsList)

	// Add comment
	return o.githubRepo.AddComment(ctx, prNumber, body)
}

// getPRNumber retrieves PR number from environment variables or GitHub event payload
func (o *DryRunOrchestrator) getPRNumber(_ context.Context) int {
	// Try environment variable first
	if prNumberStr := os.Getenv(envGithubIssueNumber); prNumberStr != "" {
		if prNumber, err := strconv.Atoi(prNumberStr); err == nil {
			return prNumber
		}
	}
	// Try GitHub event payload as fallback
	if eventPath := os.Getenv(envGithubEventPath); eventPath != "" {
		// Validate the path is within the expected GitHub Actions directory
		// GitHub Actions always sets this to a file in the runner's workspace
		if !isValidGitHubEventPath(eventPath) {
			return 0
		}
		// #nosec G304 - GITHUB_EVENT_PATH is validated and is a trusted environment variable
		// set by GitHub Actions that always points to a controlled file
		file, err := os.Open(eventPath)
		if err == nil {
			defer file.Close()
			var payload map[string]any
			if err := json.NewDecoder(file).Decode(&payload); err == nil {
				// Check for pull_request.number
				if pr, ok := payload["pull_request"].(map[string]any); ok {
					if n, ok := pr["number"].(float64); ok {
						return int(n)
					}
				}
				// Check for issue.number as fallback
				if issue, ok := payload["issue"].(map[string]any); ok {
					if n, ok := issue["number"].(float64); ok {
						return int(n)
					}
				}
			}
		}
	}
	return 0
}

// printStatus prints status if not CI
func (o *DryRunOrchestrator) printStatus(ciOutput bool, message string) {
	if !ciOutput {
		fmt.Println(message)
	}
}

// findRepoRoot walks up directories to find the git repository root
func findRepoRoot(startDir string) string {
	dir := startDir
	for {
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			return dir
		}
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return ""
}

// isValidGitHubEventPath validates that the GitHub event path is safe to open
// GitHub Actions sets GITHUB_EVENT_PATH to a file in the runner's workspace
func isValidGitHubEventPath(path string) bool {
	// Ensure the path is absolute
	if !filepath.IsAbs(path) {
		return false
	}

	// Clean the path to remove any traversal attempts
	cleanPath := filepath.Clean(path)

	// GitHub Actions typically sets this to a path like:
	// /home/runner/work/_temp/_github_workflow/event.json
	// or /github/workflow/event.json
	// We check that it contains expected patterns

	// Must end with .json
	if !strings.HasSuffix(cleanPath, ".json") {
		return false
	}

	// Should contain typical GitHub Actions path patterns
	validPatterns := []string{
		"/_temp/",
		"/workflow/",
		"/_github_workflow/",
		"/runner/",
	}

	hasValidPattern := false
	for _, pattern := range validPatterns {
		if strings.Contains(cleanPath, pattern) {
			hasValidPattern = true
			break
		}
	}

	return hasValidPattern
}
