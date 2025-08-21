package orchestrator

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"slices"
	"strings"

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
	ForceRelease   bool
	DryRun         bool
	CIOutput       bool
	SkipPR         bool   // For testing without PR creation
	EnableRollback bool   // Enable saga-based rollback support
	Rollback       bool   // Perform rollback of failed session
	SessionID      string // Session ID for rollback operations
}

// PRReleaseOrchestrator orchestrates the entire PR release workflow.
type PRReleaseOrchestrator struct {
	gitRepo    repository.GitExtendedRepository
	githubRepo repository.GithubExtendedRepository
	fsRepo     repository.FileSystemRepository
	cliffSvc   service.CliffService
	npmSvc     service.NpmService
	toolsDir   string
	stateRepo  repository.StateRepository
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
	// Initialize state repository for rollback support
	stateRepo := repository.NewJSONStateRepository(fsRepo, ".release-state")
	return &PRReleaseOrchestrator{
		gitRepo:    gitRepo,
		githubRepo: githubRepo,
		fsRepo:     fsRepo,
		cliffSvc:   cliffSvc,
		npmSvc:     npmSvc,
		toolsDir:   toolsDir,
		stateRepo:  stateRepo,
	}
}

// Execute runs the complete PR release workflow.
func (o *PRReleaseOrchestrator) Execute(ctx context.Context, cfg PRReleaseConfig) error {
	// Handle rollback operation
	if cfg.Rollback {
		return o.performRollback(ctx, cfg.SessionID)
	}

	// Normal execution with optional rollback support
	if cfg.EnableRollback {
		return o.executeWithSaga(ctx, cfg)
	}

	// Legacy execution without rollback support
	return o.executeLegacy(ctx, cfg)
}

// executeLegacy runs the workflow without rollback support (original implementation)
func (o *PRReleaseOrchestrator) executeLegacy(ctx context.Context, cfg PRReleaseConfig) error {
	// Add timeout to match workflow (default 60 minutes for jobs)
	ctx, cancel := context.WithTimeout(ctx, DefaultWorkflowTimeout)
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

	// Dry-run: stop here so no commit, push or PR is made.
	if cfg.DryRun {
		o.printStatus(cfg.CIOutput,
			fmt.Sprintf("🛈 Dry-run complete – release %s prepared locally (no commit/push/PR).", version))
		return nil
	}

	if err := o.commitChanges(ctx, version); err != nil {
		return fmt.Errorf("failed to commit changes: %w", err)
	}
	if err := o.gitRepo.PushBranch(ctx, branchName); err != nil {
		return fmt.Errorf("failed to push branch: %w", err)
	}
	if !cfg.SkipPR {
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
	if os.Getenv("GITHUB_ACTIONS") == githubActionsTrue {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to update root package.json: %w", err)
		}
	} else {
		if output, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("failed to update root package.json: %w\nOutput: %s", err, string(output))
		}
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
	if err := afero.WriteFile(o.fsRepo, "CHANGELOG.md", []byte(changelog), FilePermissionsReadWrite); err != nil {
		return "", fmt.Errorf("failed to write changelog: %w", err)
	}
	// Also create release notes
	if err := afero.WriteFile(o.fsRepo, "RELEASE_NOTES.md", []byte(changelog), FilePermissionsReadWrite); err != nil {
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
	return retry.Do(
		ctx,
		retry.WithMaxRetries(DefaultRetryCount, retry.NewExponential(DefaultRetryDelay)),
		func(ctx context.Context) error {
			return o.githubRepo.CreateOrUpdatePR(ctx, branchName, "main", title, body, labels)
		},
	)
}

// executeWithSaga runs the workflow with saga-based rollback support
func (o *PRReleaseOrchestrator) executeWithSaga(ctx context.Context, cfg PRReleaseConfig) error {
	// Add timeout to match workflow (default 60 minutes for jobs)
	ctx, cancel := context.WithTimeout(ctx, DefaultWorkflowTimeout)
	defer cancel()

	// Validate required environment variables
	if err := ValidateEnvironmentVariables([]string{"GITHUB_TOKEN"}); err != nil {
		return fmt.Errorf("environment validation failed: %w", err)
	}

	// Initialize saga with current branch info
	saga, err := o.initializeSaga(ctx)
	if err != nil {
		return err
	}

	// Build and execute workflow steps
	if err := o.buildAndExecuteWorkflow(ctx, saga, cfg); err != nil {
		return err
	}

	return nil
}

// initializeSaga creates and configures the saga executor
func (o *PRReleaseOrchestrator) initializeSaga(ctx context.Context) (*SagaExecutor, error) {
	saga := NewSagaExecutor(o.stateRepo, true)

	// Get current branch for rollback
	originalBranch, err := o.gitRepo.GetCurrentBranch(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get current branch: %w", err)
	}
	saga.SetOriginalBranch(originalBranch)
	return saga, nil
}

// buildAndExecuteWorkflow builds all workflow steps and executes the saga
func (o *PRReleaseOrchestrator) buildAndExecuteWorkflow(
	ctx context.Context,
	saga *SagaExecutor,
	cfg PRReleaseConfig,
) error {
	compensator := NewCompensatingActions(o.gitRepo, o.githubRepo)
	originalBranch := saga.GetState().OriginalBranch

	// Shared workflow context
	wctx := &workflowContext{}

	// Add all workflow steps
	o.addCheckChangesStep(saga, cfg, compensator, wctx)
	o.addCalculateVersionStep(saga, cfg, compensator, wctx)
	o.addCreateBranchStep(saga, compensator, wctx, originalBranch)
	o.addUpdatePackagesStep(saga, compensator, wctx)
	o.addGenerateChangelogStep(saga, compensator, wctx)
	o.addCommitChangesStep(saga, compensator, wctx)
	o.addPushBranchStep(saga, cfg, compensator, wctx)
	o.addCreatePRStep(saga, cfg, compensator, wctx)

	// Execute the saga
	if err := saga.Execute(ctx); err != nil {
		return fmt.Errorf("workflow failed: %w", err)
	}

	o.printStatus(cfg.CIOutput, fmt.Sprintf("✅ Release PR workflow completed for version %s", wctx.version))
	return nil
}

// workflowContext holds shared state for workflow execution
type workflowContext struct {
	version          string
	branchName       string
	hasChanges       bool
	latestTag        string
	prNumber         int
	createdInSession bool
}

// Workflow step methods
func (o *PRReleaseOrchestrator) addCheckChangesStep(
	saga *SagaExecutor,
	cfg PRReleaseConfig,
	compensator *CompensatingActions,
	wctx *workflowContext,
) {
	saga.AddStep(SagaStep{
		Name: "Check Changes",
		Type: domain.OperationTypeCheckChanges,
		Execute: func(ctx context.Context) (map[string]any, error) {
			var err error
			wctx.hasChanges, wctx.latestTag, err = o.checkChanges(ctx)
			if err != nil {
				return nil, fmt.Errorf("failed to check changes: %w", err)
			}
			o.printCIOutput(cfg.CIOutput, "has_changes=%t\n", wctx.hasChanges)
			o.printCIOutput(cfg.CIOutput, "latest_tag=%s\n", wctx.latestTag)
			return map[string]any{
				"has_changes": wctx.hasChanges,
				"latest_tag":  wctx.latestTag,
			}, nil
		},
		Compensate: compensator.NoOp,
	})
}

func (o *PRReleaseOrchestrator) addCalculateVersionStep(
	saga *SagaExecutor,
	cfg PRReleaseConfig,
	compensator *CompensatingActions,
	wctx *workflowContext,
) {
	saga.AddStep(SagaStep{
		Name: "Calculate Version",
		Type: domain.OperationTypeCalculateVersion,
		Execute: func(ctx context.Context) (map[string]any, error) {
			if !wctx.hasChanges && !cfg.ForceRelease {
				o.printStatus(cfg.CIOutput, "No changes detected since last release")
				return map[string]any{"skip": true}, nil
			}
			var err error
			wctx.version, err = o.calculateVersion(ctx, wctx.latestTag)
			if err != nil {
				return nil, fmt.Errorf("failed to calculate version: %w", err)
			}
			if err := ValidateVersion(wctx.version); err != nil {
				return nil, fmt.Errorf("invalid version: %w", err)
			}
			o.printCIOutput(cfg.CIOutput, "version=%s\n", wctx.version)
			saga.SetVersion(wctx.version)
			return map[string]any{"version": wctx.version}, nil
		},
		Compensate: compensator.NoOp,
	})
}

func (o *PRReleaseOrchestrator) addCreateBranchStep(
	saga *SagaExecutor,
	compensator *CompensatingActions,
	wctx *workflowContext,
	originalBranch string,
) {
	saga.AddStep(SagaStep{
		Name: "Create Release Branch",
		Type: domain.OperationTypeCreateBranch,
		Execute: func(ctx context.Context) (map[string]any, error) {
			if wctx.version == "" {
				return map[string]any{"skip": true}, nil
			}
			wctx.branchName = fmt.Sprintf("release/%s", wctx.version)
			if err := ValidateBranchName(wctx.branchName); err != nil {
				return nil, fmt.Errorf("invalid branch name: %w", err)
			}
			saga.SetBranchName(wctx.branchName)

			// Check if branch already exists
			branches, err := o.gitRepo.ListLocalBranches(ctx)
			if err != nil {
				return nil, fmt.Errorf("failed to list local branches: %w", err)
			}

			branchExists := slices.Contains(branches, wctx.branchName)

			wctx.createdInSession = !branchExists
			if wctx.createdInSession {
				if err := o.createReleaseBranch(ctx, wctx.branchName); err != nil {
					return nil, fmt.Errorf("failed to create release branch: %w", err)
				}
			}

			if err := o.gitRepo.CheckoutBranch(ctx, wctx.branchName); err != nil {
				return nil, fmt.Errorf("failed to checkout release branch: %w", err)
			}

			return map[string]any{
				"branch_name":        wctx.branchName,
				"original_branch":    originalBranch,
				"created_in_session": wctx.createdInSession,
			}, nil
		},
		Compensate: compensator.DeleteBranch,
	})
}

func (o *PRReleaseOrchestrator) addUpdatePackagesStep(
	saga *SagaExecutor,
	compensator *CompensatingActions,
	wctx *workflowContext,
) {
	saga.AddStep(SagaStep{
		Name: "Update Package Versions",
		Type: domain.OperationTypeUpdatePackages,
		Execute: func(ctx context.Context) (map[string]any, error) {
			if wctx.version == "" {
				return map[string]any{"skip": true}, nil
			}
			if err := o.updatePackageVersions(ctx, wctx.version); err != nil {
				return nil, fmt.Errorf("failed to update package versions: %w", err)
			}
			return map[string]any{
				"modified_files": []string{
					"package.json",
					"package-lock.json",
					"tools/*/package.json",
				},
			}, nil
		},
		Compensate: compensator.RestoreFiles,
	})
}

func (o *PRReleaseOrchestrator) addGenerateChangelogStep(
	saga *SagaExecutor,
	compensator *CompensatingActions,
	wctx *workflowContext,
) {
	saga.AddStep(SagaStep{
		Name: "Generate Changelog",
		Type: domain.OperationTypeGenerateChangelog,
		Execute: func(ctx context.Context) (map[string]any, error) {
			if wctx.version == "" {
				return map[string]any{"skip": true}, nil
			}
			changelog, err := o.generateChangelog(ctx, wctx.version, "unreleased")
			if err != nil {
				return nil, fmt.Errorf("failed to generate changelog: %w", err)
			}
			return map[string]any{
				"modified_files": []string{
					"CHANGELOG.md",
					"RELEASE_NOTES.md",
				},
				"changelog": changelog,
			}, nil
		},
		Compensate: compensator.RestoreFiles,
	})
}

func (o *PRReleaseOrchestrator) addCommitChangesStep(
	saga *SagaExecutor,
	compensator *CompensatingActions,
	wctx *workflowContext,
) {
	saga.AddStep(SagaStep{
		Name: "Commit Changes",
		Type: domain.OperationTypeCommitChanges,
		Execute: func(ctx context.Context) (map[string]any, error) {
			if wctx.version == "" {
				return map[string]any{"skip": true}, nil
			}
			if err := o.commitChanges(ctx, wctx.version); err != nil {
				return nil, fmt.Errorf("failed to commit changes: %w", err)
			}
			return map[string]any{
				"commit_sha": "HEAD",
			}, nil
		},
		Compensate: compensator.ResetCommit,
	})
}

func (o *PRReleaseOrchestrator) addPushBranchStep(
	saga *SagaExecutor,
	cfg PRReleaseConfig,
	compensator *CompensatingActions,
	wctx *workflowContext,
) {
	saga.AddStep(SagaStep{
		Name: "Push Branch",
		Type: domain.OperationTypePushBranch,
		Execute: func(ctx context.Context) (map[string]any, error) {
			if wctx.version == "" || cfg.DryRun {
				return map[string]any{"skip": true}, nil
			}
			// Use force push for existing branches to handle non-fast-forward updates
			var err error
			if wctx.createdInSession {
				err = o.gitRepo.PushBranch(ctx, wctx.branchName)
			} else {
				err = o.gitRepo.PushBranchForce(ctx, wctx.branchName)
			}
			if err != nil {
				return nil, fmt.Errorf("failed to push branch: %w", err)
			}
			return map[string]any{
				"pushed":      true,
				"branch_name": wctx.branchName,
			}, nil
		},
		Compensate: compensator.DeleteBranch,
	})
}

func (o *PRReleaseOrchestrator) addCreatePRStep(
	saga *SagaExecutor,
	cfg PRReleaseConfig,
	compensator *CompensatingActions,
	wctx *workflowContext,
) {
	saga.AddStep(SagaStep{
		Name: "Create Pull Request",
		Type: domain.OperationTypeCreatePR,
		Execute: func(ctx context.Context) (map[string]any, error) {
			if wctx.version == "" || cfg.SkipPR || cfg.DryRun {
				return map[string]any{"skip": true}, nil
			}

			changelog, err := o.generateChangelog(ctx, wctx.version, "unreleased")
			if err != nil {
				return nil, fmt.Errorf("failed to get changelog for PR: %w", err)
			}

			ver, err := domain.NewVersion(wctx.version)
			if err != nil {
				return nil, fmt.Errorf("failed to parse version: %w", err)
			}
			release := &domain.Release{
				Version:   ver,
				Changelog: changelog,
			}
			uc := &usecase.PreparePRBodyUseCase{}
			body, err := uc.Execute(ctx, release)
			if err != nil {
				return nil, fmt.Errorf("failed to prepare PR body: %w", err)
			}

			title := fmt.Sprintf("ci(release): Release %s", wctx.version)
			labels := []string{"release-pending", "automated"}

			err = retry.Do(
				ctx,
				retry.WithMaxRetries(DefaultRetryCount, retry.NewExponential(DefaultRetryDelay)),
				func(ctx context.Context) error {
					return o.githubRepo.CreateOrUpdatePR(ctx, wctx.branchName, "main", title, body, labels)
				},
			)
			if err != nil {
				return nil, err
			}

			wctx.prNumber = 0 // Placeholder since CreateOrUpdatePR doesn't return PR number
			return map[string]any{
				"pr_number": wctx.prNumber,
			}, nil
		},
		Compensate: compensator.ClosePullRequest,
	})
}

// performRollback rolls back a failed release session
func (o *PRReleaseOrchestrator) performRollback(ctx context.Context, sessionID string) error {
	if sessionID == "" {
		// Load the latest session if no ID provided
		state, err := o.stateRepo.LoadLatest(ctx)
		if err != nil {
			return fmt.Errorf("failed to load latest session: %w", err)
		}
		sessionID = state.SessionID
	}

	// Load the saga from state
	saga, err := LoadExistingSaga(o.stateRepo, sessionID)
	if err != nil {
		return fmt.Errorf("failed to load saga: %w", err)
	}

	// Create compensating actions handler
	compensator := NewCompensatingActions(o.gitRepo, o.githubRepo)

	// Rebuild saga steps with compensating actions
	// This is needed because the loaded saga doesn't have the function pointers
	o.rebuildSagaSteps(saga, compensator)

	// Perform rollback
	if err := saga.Rollback(ctx); err != nil {
		return fmt.Errorf("rollback failed: %w", err)
	}

	fmt.Println("✅ Rollback completed successfully")
	return nil
}

// rebuildSagaSteps rebuilds the saga steps with compensating actions
func (o *PRReleaseOrchestrator) rebuildSagaSteps(saga *SagaExecutor, compensator *CompensatingActions) {
	// Map operation types to compensating actions
	compensateMap := map[domain.OperationType]func(context.Context, map[string]any) error{
		domain.OperationTypeCheckChanges:      compensator.NoOp,
		domain.OperationTypeCalculateVersion:  compensator.NoOp,
		domain.OperationTypeCreateBranch:      compensator.DeleteBranch,
		domain.OperationTypeUpdatePackages:    compensator.RestoreFiles,
		domain.OperationTypeGenerateChangelog: compensator.RestoreFiles,
		domain.OperationTypeCommitChanges:     compensator.ResetCommit,
		domain.OperationTypePushBranch:        compensator.DeleteBranch,
		domain.OperationTypeCreatePR:          compensator.ClosePullRequest,
	}

	// Rebuild steps with compensating actions
	for _, op := range saga.GetState().Operations {
		if compensate, ok := compensateMap[op.Type]; ok {
			saga.AddStep(SagaStep{
				Name:       string(op.Type),
				Type:       op.Type,
				Execute:    nil, // Not needed for rollback
				Compensate: compensate,
			})
		}
	}
}
