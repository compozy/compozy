package core

import (
	"context"
	"errors"
	"fmt"
	"strings"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/core/model"
	"github.com/compozy/compozy/internal/core/reviews"
	"github.com/compozy/compozy/internal/core/tasks"
	"github.com/compozy/compozy/internal/core/workpackages"
	"github.com/compozy/compozy/internal/store/globaldb"
)

// WorkPackageCompletionRequest contains only evidence the hidden final-review
// bridge can provide after its clean-review and verification gates.
type WorkPackageCompletionRequest struct {
	WorkspaceRoot      string
	Reference          string
	VerificationPassed bool
}

// WorkPackageCompletionResult keeps review success separate from durable plan
// mutation and subsequent catalog synchronization.
type WorkPackageCompletionResult struct {
	Reference          string `json:"reference"`
	ReviewClean        bool   `json:"review_clean"`
	CompletionRecorded bool   `json:"completion_recorded"`
	AlreadyCompleted   bool   `json:"already_completed"`
	SyncPending        bool   `json:"sync_pending"`
}

type workPackageCompletionStore interface {
	MarkComplete(context.Context, string, string) (workpackages.CompletionResult, error)
}

type packageTargetResolver interface {
	ResolvePackage(context.Context, string, string) (workpackages.Target, error)
}

// WorkPackageCompletionService owns the hidden final-review completion bridge.
// It deliberately has no Git dependency.
type WorkPackageCompletionService struct {
	resolver packageTargetResolver
	store    workPackageCompletionStore
	sync     func(context.Context, string, model.ExecutionScope) error
}

// NewWorkPackageCompletionService constructs the production completion bridge.
func NewWorkPackageCompletionService() *WorkPackageCompletionService {
	return &WorkPackageCompletionService{
		resolver: workpackages.TargetResolver{},
		store:    workpackages.NewStore(),
		sync:     syncCompletedWorkPackageInitiative,
	}
}

// Complete records one package checkbox only after package-local task and
// review evidence plus final verification satisfy the completion gate.
func (s *WorkPackageCompletionService) Complete(
	ctx context.Context,
	request WorkPackageCompletionRequest,
) (WorkPackageCompletionResult, error) {
	if err := context.Cause(ctx); err != nil {
		return WorkPackageCompletionResult{}, err
	}
	service := usableWorkPackageCompletionService(s)
	result := WorkPackageCompletionResult{Reference: strings.TrimSpace(request.Reference)}

	paths, reviewClean, err := validateCompletionEvidence(ctx, request)
	result.ReviewClean = reviewClean
	if err != nil {
		return result, err
	}

	target, err := service.resolver.ResolvePackage(ctx, request.WorkspaceRoot, result.Reference)
	if err != nil {
		return result, fmt.Errorf("resolve current work package plan: %w", err)
	}
	scope, err := workpackages.BuildExecutionScope(target)
	if err != nil {
		return result, err
	}
	if err := ensureCurrentExecutionScopeSpecifications(scope); err != nil {
		return result, err
	}
	if !sameCompletionOperationalPaths(paths, scope) {
		return result, fmt.Errorf("work package operational paths changed while completing %s", result.Reference)
	}
	readiness, err := workpackages.EvaluateReadiness(target.Plan, target.Package.ID)
	if err != nil {
		return result, err
	}
	if !readiness.Eligible {
		return result, completionDependencyError(target.Package.ID, readiness)
	}

	completed, err := service.store.MarkComplete(ctx, target.InitiativeDir, target.Package.ID)
	if err != nil {
		return result, err
	}
	result.CompletionRecorded = completed.CompletionRecorded
	result.AlreadyCompleted = completed.AlreadyCompleted
	if !result.CompletionRecorded && !result.AlreadyCompleted {
		return result, errors.New("work package completion was not recorded")
	}
	if err := service.sync(ctx, request.WorkspaceRoot, scope); err != nil {
		result.SyncPending = true
		return result, fmt.Errorf("sync completed work package %s: %w", result.Reference, err)
	}
	return result, nil
}

func validateCompletionEvidence(
	ctx context.Context,
	request WorkPackageCompletionRequest,
) (workpackages.OperationalPaths, bool, error) {
	paths, evidence, err := completionEvidence(ctx, request.WorkspaceRoot, request.Reference)
	// ReviewClean reports only the final-verification and review outcome; terminal-task
	// readiness is an independent completion precondition enforced below and must not
	// corrupt a genuinely clean review result.
	reviewClean := request.VerificationPassed && err == nil && evidence.reviewsResolved
	if err != nil {
		return paths, reviewClean, fmt.Errorf("inspect work package completion evidence: %w", err)
	}
	eligibility := workpackages.CanRecordCompletion(workpackages.CompletionPreconditions{
		VerificationPassed: request.VerificationPassed,
		ReviewInterrupted:  false,
		NewIssues:          false,
		PriorIssueStatuses: evidence.issueStatuses,
		HeadingExists:      true,
	})
	if !eligibility.Eligible || !evidence.tasksTerminal {
		return paths, reviewClean, completionBlockedError(eligibility, evidence.tasksTerminal)
	}
	return paths, reviewClean, nil
}

func usableWorkPackageCompletionService(service *WorkPackageCompletionService) *WorkPackageCompletionService {
	if service == nil {
		return NewWorkPackageCompletionService()
	}
	if service.resolver == nil {
		service.resolver = workpackages.TargetResolver{}
	}
	if service.store == nil {
		service.store = workpackages.NewStore()
	}
	if service.sync == nil {
		service.sync = syncCompletedWorkPackageInitiative
	}
	return service
}

type completionReviewEvidence struct {
	tasksTerminal   bool
	reviewsResolved bool
	issueStatuses   []string
}

func completionEvidence(
	ctx context.Context,
	workspaceRoot string,
	reference string,
) (workpackages.OperationalPaths, completionReviewEvidence, error) {
	paths, err := workpackages.ResolveOperationalPaths(ctx, workspaceRoot, reference)
	if err != nil {
		return workpackages.OperationalPaths{}, completionReviewEvidence{}, err
	}
	taskMeta, err := tasks.SnapshotTaskMeta(paths.PackageDir)
	if err != nil {
		return workpackages.OperationalPaths{}, completionReviewEvidence{}, err
	}
	evidence := completionReviewEvidence{
		tasksTerminal:   taskMeta.Total > 0 && taskMeta.Pending == 0,
		reviewsResolved: true,
	}
	rounds, err := reviews.DiscoverRounds(paths.PackageDir)
	if err != nil {
		return workpackages.OperationalPaths{}, completionReviewEvidence{}, err
	}
	for _, round := range rounds {
		entries, readErr := reviews.ReadReviewEntries(reviews.ReviewDirectory(paths.PackageDir, round))
		if readErr != nil {
			return workpackages.OperationalPaths{}, completionReviewEvidence{}, readErr
		}
		for _, entry := range entries {
			resolved, parseErr := reviews.IsReviewResolved(entry.Content)
			if parseErr != nil {
				return workpackages.OperationalPaths{}, completionReviewEvidence{}, parseErr
			}
			if resolved {
				evidence.issueStatuses = append(evidence.issueStatuses, "resolved")
				continue
			}
			evidence.issueStatuses = append(evidence.issueStatuses, "pending")
			evidence.reviewsResolved = false
		}
	}
	return paths, evidence, nil
}

func sameCompletionOperationalPaths(paths workpackages.OperationalPaths, scope model.ExecutionScope) bool {
	return strings.TrimSpace(paths.InitiativeDir) == strings.TrimSpace(scope.SpecDir) &&
		strings.TrimSpace(paths.PackageDir) == strings.TrimSpace(scope.OperationalDir)
}

func completionBlockedError(
	eligibility workpackages.CompletionEligibility,
	tasksTerminal bool,
) error {
	if !tasksTerminal {
		return errors.New("work package completion requires all package tasks to be terminal")
	}
	if eligibility.Reason == "" {
		return errors.New("work package completion is not eligible")
	}
	return fmt.Errorf("work package completion blocked: %s", eligibility.Reason)
}

func completionDependencyError(packageID string, readiness workpackages.Readiness) error {
	blockers := make([]string, 0, len(readiness.DirectUnmet))
	for _, dependency := range readiness.DirectUnmet {
		blockers = append(blockers, dependency.From)
	}
	if len(blockers) == 0 {
		for _, path := range readiness.TransitiveUnmet {
			blockers = append(blockers, strings.Join(path.PackageIDs, " -> "))
		}
	}
	return fmt.Errorf(
		"%w: %s requires %s",
		workpackages.ErrDependenciesUnmet,
		packageID,
		strings.Join(blockers, ", "),
	)
}

func syncCompletedWorkPackageInitiative(
	ctx context.Context,
	workspaceRoot string,
	scope model.ExecutionScope,
) error {
	homePaths, err := compozyconfig.ResolveHomePaths()
	if err != nil {
		return fmt.Errorf("resolve compozy home paths: %w", err)
	}
	if err := compozyconfig.EnsureHomeLayout(homePaths); err != nil {
		return fmt.Errorf("ensure compozy home layout: %w", err)
	}
	db, err := globaldb.Open(ctx, homePaths.GlobalDBPath)
	if err != nil {
		return fmt.Errorf("open global completion database: %w", err)
	}
	defer func() {
		_ = db.Close()
	}()
	workspace, err := db.ResolveOrRegister(ctx, workspaceRoot)
	if err != nil {
		return fmt.Errorf("resolve completion workspace: %w", err)
	}
	if _, err := SyncWithDB(ctx, db, workspace, SyncConfig{ExecutionScope: &scope}); err != nil {
		return err
	}
	return nil
}

// CompleteWorkPackage invokes the production hidden completion bridge.
func CompleteWorkPackage(
	ctx context.Context,
	request WorkPackageCompletionRequest,
) (WorkPackageCompletionResult, error) {
	return NewWorkPackageCompletionService().Complete(ctx, request)
}
