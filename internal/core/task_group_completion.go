package core

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/core/model"
	"github.com/compozy/compozy/internal/core/reviews"
	"github.com/compozy/compozy/internal/core/taskgroups"
	"github.com/compozy/compozy/internal/core/tasks"
	"github.com/compozy/compozy/internal/store/globaldb"
)

// TaskGroupCompletionRequest contains only evidence the hidden final-review
// bridge can provide after its clean-review and verification gates.
type TaskGroupCompletionRequest struct {
	WorkspaceRoot      string
	Reference          string
	VerificationPassed bool
}

// TaskGroupCompletionResult keeps review success separate from durable plan
// mutation and subsequent catalog synchronization.
type TaskGroupCompletionResult struct {
	Reference          string `json:"reference"`
	ReviewClean        bool   `json:"review_clean"`
	CompletionRecorded bool   `json:"completion_recorded"`
	AlreadyCompleted   bool   `json:"already_completed"`
	SyncPending        bool   `json:"sync_pending"`
}

type taskGroupCompletionStore interface {
	MarkCompleteValidated(
		context.Context,
		string,
		string,
		taskgroups.CompletionValidator,
	) (taskgroups.CompletionResult, error)
}

type taskGroupTargetResolver interface {
	ResolveTaskGroup(context.Context, string, string) (taskgroups.Target, error)
}

// TaskGroupCompletionService owns the hidden final-review completion bridge.
// It deliberately has no Git dependency.
type TaskGroupCompletionService struct {
	resolver taskGroupTargetResolver
	store    taskGroupCompletionStore
	sync     func(context.Context, string, model.ExecutionScope) error
}

// NewTaskGroupCompletionService constructs the production completion bridge.
func NewTaskGroupCompletionService() *TaskGroupCompletionService {
	return &TaskGroupCompletionService{
		resolver: taskgroups.TargetResolver{},
		store:    taskgroups.NewStore(),
		sync:     syncCompletedTaskGroupInitiative,
	}
}

// completionGate is the fully re-derived evidence required to record one task group.
type completionGate struct {
	target      taskgroups.Target
	scope       model.ExecutionScope
	reviewClean bool
}

// Complete records one task group checkbox only after task-group-local task and
// review evidence plus final verification satisfy the completion gate.
func (s *TaskGroupCompletionService) Complete(
	ctx context.Context,
	request TaskGroupCompletionRequest,
) (TaskGroupCompletionResult, error) {
	if err := context.Cause(ctx); err != nil {
		return TaskGroupCompletionResult{}, err
	}
	service := usableTaskGroupCompletionService(s)
	result := TaskGroupCompletionResult{Reference: strings.TrimSpace(request.Reference)}

	gate, err := service.evaluateCompletionGate(ctx, request, result.Reference)
	result.ReviewClean = gate.reviewClean
	if err != nil {
		return result, err
	}

	completed, err := service.store.MarkCompleteValidated(
		ctx,
		gate.target.InitiativeDir,
		gate.target.TaskGroup.ID,
		func(validationCtx context.Context) error {
			currentGate, validationErr := service.evaluateCompletionGate(validationCtx, request, result.Reference)
			result.ReviewClean = currentGate.reviewClean
			if validationErr == nil {
				gate = currentGate
			}
			return validationErr
		},
	)
	if err != nil {
		return result, err
	}
	result.CompletionRecorded = completed.CompletionRecorded
	result.AlreadyCompleted = completed.AlreadyCompleted
	if !result.CompletionRecorded && !result.AlreadyCompleted {
		return result, errors.New("task group completion was not recorded")
	}
	if err := service.sync(ctx, request.WorkspaceRoot, gate.scope); err != nil {
		result.SyncPending = true
		return result, fmt.Errorf("sync completed task group %s: %w", result.Reference, err)
	}
	return result, nil
}

// evaluateCompletionGate re-reads current task, review, verification, operational
// path, and dependency evidence and only succeeds when a completion may be
// recorded from that current evidence. reviewClean reports the review/verification
// outcome even when a later completion precondition blocks the checkbox.
func (s *TaskGroupCompletionService) evaluateCompletionGate(
	ctx context.Context,
	request TaskGroupCompletionRequest,
	reference string,
) (completionGate, error) {
	paths, reviewClean, err := validateCompletionEvidence(ctx, request, reference)
	gate := completionGate{reviewClean: reviewClean}
	if err != nil {
		return gate, err
	}
	target, err := s.resolver.ResolveTaskGroup(ctx, request.WorkspaceRoot, reference)
	if err != nil {
		return gate, fmt.Errorf("resolve current task group plan: %w", err)
	}
	gate.target = target
	scope, err := taskgroups.BuildExecutionScope(target)
	if err != nil {
		return gate, err
	}
	gate.scope = scope
	if err := ensureCurrentExecutionScopeSpecifications(scope); err != nil {
		return gate, err
	}
	if !sameCompletionOperationalPaths(paths, scope) {
		return gate, fmt.Errorf("task group operational paths changed while completing %s", reference)
	}
	readiness, err := taskgroups.EvaluateReadiness(target.Plan, target.TaskGroup.ID)
	if err != nil {
		return gate, err
	}
	if !readiness.Eligible {
		return gate, completionDependencyError(target.TaskGroup.ID, readiness)
	}
	return gate, nil
}

func validateCompletionEvidence(
	ctx context.Context,
	request TaskGroupCompletionRequest,
	reference string,
) (taskgroups.OperationalPaths, bool, error) {
	paths, review, err := reviewCompletionEvidence(ctx, request.WorkspaceRoot, reference)
	// ReviewClean is derived only from final verification and the independent review
	// scan. A task-inspection failure below is a separate completion blocker and must
	// never flip a genuinely clean, fully resolved review result.
	reviewClean := request.VerificationPassed && err == nil && review.reviewsResolved
	if err != nil {
		return paths, reviewClean, fmt.Errorf("inspect task group review evidence: %w", err)
	}
	tasksTerminal, err := taskCompletionEvidence(ctx, paths.TaskGroupDir, reference)
	if err != nil {
		return paths, reviewClean, fmt.Errorf("inspect task group task evidence: %w", err)
	}
	eligibility := taskgroups.CanRecordCompletion(taskgroups.CompletionPreconditions{
		VerificationPassed: request.VerificationPassed,
		ReviewInterrupted:  false,
		NewIssues:          false,
		PriorIssueStatuses: review.issueStatuses,
		HeadingExists:      true,
	})
	if !eligibility.Eligible || !tasksTerminal {
		return paths, reviewClean, completionBlockedError(eligibility, tasksTerminal)
	}
	return paths, reviewClean, nil
}

func usableTaskGroupCompletionService(service *TaskGroupCompletionService) *TaskGroupCompletionService {
	if service == nil {
		return NewTaskGroupCompletionService()
	}
	if service.resolver == nil {
		service.resolver = taskgroups.TargetResolver{}
	}
	if service.store == nil {
		service.store = taskgroups.NewStore()
	}
	if service.sync == nil {
		service.sync = syncCompletedTaskGroupInitiative
	}
	return service
}

// reviewCompletionOutcome captures the review-scan result that drives ReviewClean.
// It is deliberately independent of task-terminal state so a task-inspection
// failure cannot corrupt the review outcome.
type reviewCompletionOutcome struct {
	reviewsResolved bool
	issueStatuses   []string
}

// reviewCompletionEvidence resolves the task group directory and scans every review
// round. It never inspects task metadata, so its error surface is limited to the
// operational-path and review-scan failures that genuinely make the review state
// unknowable.
func reviewCompletionEvidence(
	ctx context.Context,
	workspaceRoot string,
	reference string,
) (taskgroups.OperationalPaths, reviewCompletionOutcome, error) {
	paths, err := taskgroups.ResolveOperationalPaths(ctx, workspaceRoot, reference)
	if err != nil {
		return taskgroups.OperationalPaths{}, reviewCompletionOutcome{}, err
	}
	outcome := reviewCompletionOutcome{reviewsResolved: true}
	rounds, err := reviews.DiscoverRounds(paths.TaskGroupDir)
	if err != nil {
		return paths, reviewCompletionOutcome{}, err
	}
	for _, round := range rounds {
		entries, readErr := reviews.ReadReviewEntries(reviews.ReviewDirectory(paths.TaskGroupDir, round))
		if readErr != nil {
			return paths, reviewCompletionOutcome{}, readErr
		}
		for _, entry := range entries {
			resolved, parseErr := reviews.IsReviewResolved(entry.Content)
			if parseErr != nil {
				return paths, reviewCompletionOutcome{}, parseErr
			}
			if resolved {
				outcome.issueStatuses = append(outcome.issueStatuses, "resolved")
				continue
			}
			outcome.issueStatuses = append(outcome.issueStatuses, "pending")
			outcome.reviewsResolved = false
		}
	}
	return paths, outcome, nil
}

// taskCompletionEvidence validates the selected task graph and reports whether
// every manifest-owned task is terminal. Its failures are completion blockers
// that the caller keeps separate from the review outcome.
func taskCompletionEvidence(ctx context.Context, taskGroupDir, reference string) (bool, error) {
	_, taskFiles, err := tasks.LoadValidatedTaskGraphManifest(ctx, taskGroupDir, reference)
	if err != nil {
		return false, err
	}
	if len(taskFiles) == 0 {
		return false, nil
	}
	for index := range taskFiles {
		if !tasks.IsTaskCompleted(taskFiles[index].Entry) {
			return false, nil
		}
	}
	return true, nil
}

func sameCompletionOperationalPaths(paths taskgroups.OperationalPaths, scope model.ExecutionScope) bool {
	return strings.TrimSpace(paths.InitiativeDir) == strings.TrimSpace(scope.SpecDir) &&
		strings.TrimSpace(paths.TaskGroupDir) == strings.TrimSpace(scope.OperationalDir)
}

func completionBlockedError(
	eligibility taskgroups.CompletionEligibility,
	tasksTerminal bool,
) error {
	if !tasksTerminal {
		return errors.New("task group completion requires all task group tasks to be terminal")
	}
	if eligibility.Reason == "" {
		return errors.New("task group completion is not eligible")
	}
	return fmt.Errorf("task group completion blocked: %s", eligibility.Reason)
}

func completionDependencyError(taskGroupID string, readiness taskgroups.Readiness) error {
	blockers := make([]string, 0, len(readiness.DirectUnmet))
	for _, dependency := range readiness.DirectUnmet {
		blockers = append(blockers, dependency.From)
	}
	if len(blockers) == 0 {
		for _, path := range readiness.TransitiveUnmet {
			blockers = append(blockers, strings.Join(path.TaskGroupIDs, " -> "))
		}
	}
	return fmt.Errorf(
		"%w: %s requires %s",
		taskgroups.ErrDependenciesUnmet,
		taskGroupID,
		strings.Join(blockers, ", "),
	)
}

func syncCompletedTaskGroupInitiative(
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

// HydratePlanCompletion projects authoritative globaldb completion into one
// workspace plan without clearing any existing checkbox.
func HydratePlanCompletion(
	ctx context.Context,
	workspaceRoot, initiative string,
) (marked []string, err error) {
	homePaths, err := compozyconfig.ResolveHomePaths()
	if err != nil {
		return nil, fmt.Errorf("resolve compozy home paths: %w", err)
	}
	if err := compozyconfig.EnsureHomeLayout(homePaths); err != nil {
		return nil, fmt.Errorf("ensure compozy home layout: %w", err)
	}
	db, err := globaldb.Open(ctx, homePaths.GlobalDBPath)
	if err != nil {
		return nil, fmt.Errorf("open completion hydration database: %w", err)
	}
	defer func() {
		if closeErr := db.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("close completion hydration database: %w", closeErr))
		}
	}()
	return HydratePlanCompletionWithDB(ctx, db, workspaceRoot, initiative)
}

// HydratePlanCompletionWithDB projects completion using an existing globaldb
// handle. Daemon callers use it so hydration shares their authoritative store.
func HydratePlanCompletionWithDB(
	ctx context.Context,
	db *globaldb.GlobalDB,
	workspaceRoot, initiative string,
) ([]string, error) {
	if err := context.Cause(ctx); err != nil {
		return nil, fmt.Errorf("hydrate task group completion: %w", err)
	}
	if db == nil {
		return nil, errors.New("hydrate task group completion: global database is required")
	}
	workspaceRoot = strings.TrimSpace(workspaceRoot)
	initiative = strings.TrimSpace(initiative)
	if workspaceRoot == "" {
		return nil, errors.New("hydrate task group completion: workspace root is required")
	}
	if initiative == "" {
		return nil, errors.New("hydrate task group completion: initiative is required")
	}

	initiativeDir := filepath.Join(model.TasksBaseDirForWorkspace(workspaceRoot), initiative)
	planPath := filepath.Join(initiativeDir, taskgroups.ManifestFileName)
	if _, statErr := os.Stat(planPath); statErr != nil {
		if errors.Is(statErr, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("inspect task group plan before hydration: %w", statErr)
	}

	completed, err := CompletedTaskGroupIDsWithDB(ctx, db, workspaceRoot, initiative)
	if err != nil {
		return nil, err
	}
	marked, err := taskgroups.NewStore().HydrateCompletion(ctx, initiativeDir, completed)
	if err != nil {
		return nil, fmt.Errorf("hydrate task group plan %s: %w", initiative, err)
	}
	return marked, nil
}

// CompletedTaskGroupIDsWithDB reads the authoritative completed IDs for one
// initiative in one registered workspace.
func CompletedTaskGroupIDsWithDB(
	ctx context.Context,
	db *globaldb.GlobalDB,
	workspaceRoot, initiative string,
) ([]string, error) {
	if err := context.Cause(ctx); err != nil {
		return nil, fmt.Errorf("read task group completion authority: %w", err)
	}
	if db == nil {
		return nil, errors.New("read task group completion authority: global database is required")
	}
	workspaceRoot = strings.TrimSpace(workspaceRoot)
	initiative = strings.TrimSpace(initiative)
	if workspaceRoot == "" {
		return nil, errors.New("read task group completion authority: workspace root is required")
	}
	if initiative == "" {
		return nil, errors.New("read task group completion authority: initiative is required")
	}

	workspace, err := db.Get(ctx, workspaceRoot)
	if errors.Is(err, globaldb.ErrWorkspaceNotFound) {
		workspace, err = db.ResolveOrRegister(ctx, workspaceRoot)
	}
	if err != nil {
		return nil, fmt.Errorf("resolve completion hydration workspace: %w", err)
	}
	workflows, err := db.ListWorkflows(ctx, globaldb.ListWorkflowsOptions{WorkspaceID: workspace.ID})
	if err != nil {
		return nil, fmt.Errorf("list completion hydration workflows: %w", err)
	}
	completed, err := completedTaskGroupIDsForInitiative(workflows, initiative)
	if err != nil {
		return nil, err
	}
	return completed, nil
}

func completedTaskGroupIDsForInitiative(
	workflows []globaldb.Workflow,
	initiative string,
) ([]string, error) {
	completed := make([]string, 0)
	for index := range workflows {
		workflow := &workflows[index]
		if workflow.Kind != globaldb.WorkflowKindTaskGroup || !workflow.LifecycleCompleted {
			continue
		}
		ref, err := taskgroups.ParseTaskGroupRef(workflow.Slug)
		if err != nil {
			return nil, fmt.Errorf("parse completed task group workflow %q: %w", workflow.Slug, err)
		}
		if ref.Initiative != initiative {
			continue
		}
		if taskGroupID := strings.TrimSpace(workflow.TaskGroupID); taskGroupID != "" &&
			taskGroupID != ref.TaskGroupID {
			return nil, fmt.Errorf(
				"completed workflow %q task group id %q does not match slug",
				workflow.Slug,
				taskGroupID,
			)
		}
		completed = append(completed, ref.TaskGroupID)
	}
	return completed, nil
}

// CompleteTaskGroup invokes the production hidden completion bridge.
func CompleteTaskGroup(
	ctx context.Context,
	request TaskGroupCompletionRequest,
) (TaskGroupCompletionResult, error) {
	return NewTaskGroupCompletionService().Complete(ctx, request)
}
