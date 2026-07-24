package core

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/core/frontmatter"
	"github.com/compozy/compozy/internal/core/model"
	"github.com/compozy/compozy/internal/core/reviews"
	"github.com/compozy/compozy/internal/core/taskgroups"
	"github.com/compozy/compozy/internal/core/tasks"
	"github.com/compozy/compozy/internal/store/globaldb"
)

const authoredTaskListHeader = "| # | Title | Status | Complexity | Dependencies |"

func syncTaskMetadata(ctx context.Context, cfg SyncConfig) (*SyncResult, error) {
	if cfg.ExecutionScope != nil {
		return syncScopedTaskGroupDirect(ctx, cfg)
	}
	target, singleWorkflow, err := resolveSyncTarget(cfg)
	result := &SyncResult{Target: target}
	if err != nil {
		return result, err
	}

	db, workspace, err := openWorkflowGlobalDB(ctx, target)
	if err != nil {
		return result, err
	}
	defer func() {
		_ = db.Close()
	}()

	return syncResolvedTarget(ctx, db, workspace, target, singleWorkflow, result)
}

func syncScopedTaskGroupDirect(ctx context.Context, cfg SyncConfig) (*SyncResult, error) {
	scope := cfg.ExecutionScope
	result := &SyncResult{}
	if scope != nil {
		result.Target = scope.OperationalDir
	}
	workspaceRoot := strings.TrimSpace(cfg.WorkspaceRoot)
	if workspaceRoot == "" {
		return result, errors.New("task group execution scope requires workspace root")
	}
	homePaths, err := compozyconfig.ResolveHomePaths()
	if err != nil {
		return result, fmt.Errorf("resolve compozy home paths: %w", err)
	}
	if err := compozyconfig.EnsureHomeLayout(homePaths); err != nil {
		return result, fmt.Errorf("ensure compozy home layout: %w", err)
	}
	db, err := globaldb.Open(ctx, homePaths.GlobalDBPath)
	if err != nil {
		return result, fmt.Errorf("open scoped sync database: %w", err)
	}
	defer func() {
		_ = db.Close()
	}()
	workspace, err := db.ResolveOrRegister(ctx, workspaceRoot)
	if err != nil {
		return result, fmt.Errorf("resolve scoped sync workspace: %w", err)
	}
	return syncScopedTaskGroup(ctx, db, workspace, cfg)
}

// SyncWithDB reconciles workflow artifacts into an already-open global.db.
func SyncWithDB(
	ctx context.Context,
	db *globaldb.GlobalDB,
	workspace globaldb.Workspace,
	cfg SyncConfig,
) (*SyncResult, error) {
	if cfg.ExecutionScope != nil {
		return syncScopedTaskGroup(ctx, db, workspace, cfg)
	}
	target, singleWorkflow, err := resolveSyncTarget(cfg)
	result := &SyncResult{Target: target}
	if err != nil {
		return result, err
	}
	if db == nil {
		return result, errors.New("sync database is required")
	}
	if strings.TrimSpace(workspace.ID) == "" {
		return result, errors.New("sync workspace id is required")
	}
	if !syncTargetBelongsToWorkspace(target, workspace.RootDir) {
		return result, fmt.Errorf("mismatched workspace and sync target: %s is outside %s", target, workspace.RootDir)
	}
	return syncResolvedTarget(ctx, db, workspace, target, singleWorkflow, result)
}

func syncScopedTaskGroup(
	ctx context.Context,
	db *globaldb.GlobalDB,
	workspace globaldb.Workspace,
	cfg SyncConfig,
) (*SyncResult, error) {
	if cfg.ExecutionScope == nil {
		return nil, errors.New("task group execution scope is required")
	}
	if db == nil {
		return nil, errors.New("sync database is required")
	}
	if strings.TrimSpace(workspace.ID) == "" {
		return nil, errors.New("sync workspace id is required")
	}
	scope := *cfg.ExecutionScope
	result := &SyncResult{Target: scope.OperationalDir}
	if strings.TrimSpace(scope.WorkflowRef) == "" {
		return result, errors.New("task group execution scope workflow reference is required")
	}
	if !syncTargetBelongsToWorkspace(scope.SpecDir, workspace.RootDir) ||
		!syncTargetBelongsToWorkspace(scope.OperationalDir, workspace.RootDir) {
		return result, fmt.Errorf("mismatched workspace and task group execution scope: %s", scope.WorkflowRef)
	}
	if err := ensureCurrentExecutionScopeSpecifications(scope); err != nil {
		return result, err
	}

	target, err := (taskgroups.TargetResolver{}).ResolveTaskGroup(ctx, workspace.RootDir, scope.WorkflowRef)
	if err != nil {
		return result, fmt.Errorf("resolve task group execution scope %s: %w", scope.WorkflowRef, err)
	}
	currentScope, err := taskgroups.BuildExecutionScope(target)
	if err != nil {
		return result, err
	}
	if !sameExecutionScope(scope, currentScope) {
		return result, fmt.Errorf("task group execution scope changed while syncing %s", scope.WorkflowRef)
	}
	if err := syncTaskGroupTarget(ctx, db, workspace, target, result); err != nil {
		return result, err
	}
	return result, nil
}

func sameExecutionScope(left, right model.ExecutionScope) bool {
	return canonicalWorkflowScopePath(left.SpecDir) == canonicalWorkflowScopePath(right.SpecDir) &&
		canonicalWorkflowScopePath(left.OperationalDir) == canonicalWorkflowScopePath(right.OperationalDir) &&
		strings.TrimSpace(left.WorkflowRef) == strings.TrimSpace(right.WorkflowRef) &&
		canonicalWorkflowScopePath(left.TasksDir) == canonicalWorkflowScopePath(right.TasksDir) &&
		canonicalWorkflowScopePath(left.ReviewsDir) == canonicalWorkflowScopePath(right.ReviewsDir) &&
		canonicalWorkflowScopePath(left.MemoryDir) == canonicalWorkflowScopePath(right.MemoryDir)
}

func syncTargetBelongsToWorkspace(target string, workspaceRoot string) bool {
	root := strings.TrimSpace(workspaceRoot)
	if root == "" {
		return false
	}
	tasksRoot := canonicalWorkflowScopePath(model.TasksBaseDirForWorkspace(root))
	cleanTarget := canonicalWorkflowScopePath(strings.TrimSpace(target))
	rel, err := filepath.Rel(tasksRoot, cleanTarget)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)))
}

func canonicalWorkflowScopePath(path string) string {
	cleaned := filepath.Clean(strings.TrimSpace(path))
	resolved, err := filepath.EvalSymlinks(cleaned)
	if err != nil {
		return cleaned
	}
	return filepath.Clean(resolved)
}

func syncResolvedTarget(
	ctx context.Context,
	db *globaldb.GlobalDB,
	workspace globaldb.Workspace,
	target string,
	singleWorkflow bool,
	result *SyncResult,
) (*SyncResult, error) {
	if singleWorkflow {
		if isTaskGroupOperationalDirectory(workspace.RootDir, target) {
			return result, TaskGroupRootOnlyError{Target: target}
		}
		if err := syncWorkspaceWorkflow(ctx, db, workspace, target, result); err != nil {
			return result, err
		}
		sortSyncResult(result)
		return result, nil
	}

	entries, err := os.ReadDir(target)
	if err != nil {
		return result, fmt.Errorf("read sync target: %w", err)
	}
	presentSlugs := make([]string, 0, len(entries))
	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return result, err
		}
		if !entry.IsDir() || !model.IsActiveWorkflowDirName(entry.Name()) {
			continue
		}
		presentSlugs = append(presentSlugs, entry.Name())
		if err := syncWorkspaceWorkflow(ctx, db, workspace, filepath.Join(target, entry.Name()), result); err != nil {
			return result, err
		}
	}
	if err := pruneMissingWorkflowRows(ctx, db, workspace.ID, presentSlugs, result); err != nil {
		return result, err
	}

	sortSyncResult(result)
	return result, nil
}

func pruneMissingWorkflowRows(
	ctx context.Context,
	db *globaldb.GlobalDB,
	workspaceID string,
	presentSlugs []string,
	result *SyncResult,
) error {
	pruned, err := db.PruneMissingActiveWorkflows(ctx, workspaceID, presentSlugs)
	if err != nil {
		return fmt.Errorf("prune missing workflow rows: %w", err)
	}
	result.WorkflowsPruned += len(pruned.PrunedSlugs)
	result.PrunedWorkflows = append(result.PrunedWorkflows, pruned.PrunedSlugs...)
	for _, skipped := range pruned.Skipped {
		result.Warnings = append(
			result.Warnings,
			fmt.Sprintf(
				"%s: skipped stale workflow prune: %s (%d active run(s))",
				skipped.Slug,
				skipped.Reason,
				skipped.ActiveRuns,
			),
		)
	}
	return nil
}

func openWorkflowGlobalDB(
	ctx context.Context,
	targetPath string,
) (*globaldb.GlobalDB, globaldb.Workspace, error) {
	homePaths, err := compozyconfig.ResolveHomePaths()
	if err != nil {
		return nil, globaldb.Workspace{}, fmt.Errorf("resolve compozy home paths: %w", err)
	}
	if err := compozyconfig.EnsureHomeLayout(homePaths); err != nil {
		return nil, globaldb.Workspace{}, fmt.Errorf("ensure compozy home layout: %w", err)
	}

	db, err := globaldb.Open(ctx, homePaths.GlobalDBPath)
	if err != nil {
		return nil, globaldb.Workspace{}, fmt.Errorf("open global sync database: %w", err)
	}

	workspace, err := db.ResolveOrRegister(ctx, targetPath)
	if err != nil {
		_ = db.Close()
		return nil, globaldb.Workspace{}, fmt.Errorf("resolve workspace for sync target: %w", err)
	}
	return db, workspace, nil
}

func resolveSyncTarget(cfg SyncConfig) (string, bool, error) {
	if target, ok := namedTaskGroupTarget(cfg.Name); ok {
		return "", false, TaskGroupRootOnlyError{Target: target}
	}
	resolved, err := resolveWorkflowTarget(workflowTargetOptions{
		command:       "sync",
		workspaceRoot: cfg.WorkspaceRoot,
		rootDir:       cfg.RootDir,
		name:          cfg.Name,
		tasksDir:      cfg.TasksDir,
		selectorFlags: "--name or --tasks-dir",
	})
	if err != nil {
		return "", false, err
	}
	return resolved.target, resolved.specificTarget, nil
}

func namedTaskGroupTarget(name string) (string, bool) {
	ref, err := taskgroups.ParseRef(strings.TrimSpace(name))
	if err != nil || !ref.IsTaskGroup() {
		return "", false
	}
	return ref.String(), true
}

func syncWorkspaceWorkflow(
	ctx context.Context,
	db *globaldb.GlobalDB,
	workspace globaldb.Workspace,
	tasksDir string,
	result *SyncResult,
) error {
	if db == nil {
		return errors.New("sync database is required")
	}
	if result == nil {
		return errors.New("sync result is required")
	}

	initiativeTarget, resolveErr := (taskgroups.TargetResolver{}).Resolve(
		ctx,
		workspace.RootDir,
		filepath.Base(tasksDir),
	)
	if resolveErr != nil {
		if errors.Is(resolveErr, taskgroups.ErrInvalidPlan) {
			return fmt.Errorf("sync workflow %s: %w", tasksDir, resolveErr)
		}
		return fmt.Errorf("resolve sync workflow target %s: %w", tasksDir, resolveErr)
	}
	if initiativeTarget.Mode == taskgroups.TargetModeInitiative {
		return syncTaskGroupInitiative(ctx, db, workspace, initiativeTarget, result)
	}
	return syncWorkflow(ctx, db, workspace.ID, tasksDir, result)
}

func syncWorkflow(
	ctx context.Context,
	db *globaldb.GlobalDB,
	workspaceID string,
	tasksDir string,
	result *SyncResult,
) error {
	if db == nil {
		return errors.New("sync database is required")
	}
	if result == nil {
		return errors.New("sync result is required")
	}

	removedLegacyArtifacts, err := cleanupLegacyWorkflowMetadata(tasksDir)
	if err != nil {
		return err
	}

	artifactSnapshots, checkpointChecksum, err := collectArtifactSnapshots(tasksDir)
	if err != nil {
		return err
	}
	taskItems, err := collectTaskItems(tasksDir)
	if err != nil {
		return err
	}
	reviewRounds, err := collectReviewRounds(tasksDir)
	if err != nil {
		return err
	}

	syncedAt := time.Now().UTC()
	syncState, err := db.ReconcileWorkflowSync(ctx, globaldb.WorkflowSyncInput{
		WorkspaceID:        workspaceID,
		WorkflowSlug:       filepath.Base(tasksDir),
		SyncedAt:           syncedAt,
		CheckpointScope:    "workflow",
		CheckpointChecksum: checkpointChecksum,
		ArtifactSnapshots:  artifactSnapshots,
		TaskItems:          taskItems,
		ReviewRounds:       reviewRounds,
	})
	if err != nil {
		return fmt.Errorf("sync workflow %s: %w", tasksDir, err)
	}

	result.WorkflowsScanned++
	result.SnapshotsUpserted += syncState.SnapshotsUpserted
	result.TaskItemsUpserted += syncState.TaskItemsUpserted
	result.ReviewRoundsUpserted += syncState.ReviewRoundsUpserted
	result.ReviewIssuesUpserted += syncState.ReviewIssuesUpserted
	result.CheckpointsUpdated += syncState.CheckpointsUpdated
	result.LegacyArtifactsRemoved += len(removedLegacyArtifacts)
	result.SyncedPaths = append(result.SyncedPaths, tasksDir)
	if len(removedLegacyArtifacts) > 0 {
		sort.Strings(removedLegacyArtifacts)
		result.Warnings = append(
			result.Warnings,
			fmt.Sprintf(
				"%s: removed legacy generated artifacts %s",
				filepath.Base(tasksDir),
				strings.Join(removedLegacyArtifacts, ", "),
			),
		)
	}
	return nil
}

func syncTaskGroupInitiative(
	ctx context.Context,
	db *globaldb.GlobalDB,
	workspace globaldb.Workspace,
	target taskgroups.Target,
	result *SyncResult,
) error {
	parentSnapshots, parentChecksum, err := collectArtifactSnapshotsExcludingTaskGroups(target.InitiativeDir)
	if err != nil {
		return err
	}

	collection, err := collectTaskGroupSyncChildren(ctx, workspace, target)
	if err != nil {
		return err
	}
	children := appendMissingTaskGroupPlaceholders(workspace, target, &collection)

	aggregate, err := db.ReconcileAggregateWorkflowSync(ctx, globaldb.AggregateWorkflowSyncInput{
		Parent: globaldb.WorkflowSyncInput{
			WorkspaceID:        workspace.ID,
			WorkflowSlug:       target.Ref.Initiative,
			Kind:               globaldb.WorkflowKindInitiative,
			SyncedAt:           time.Now().UTC(),
			CheckpointScope:    "workflow",
			CheckpointChecksum: parentChecksum,
			ArtifactSnapshots:  parentSnapshots,
		},
		Children: children,
		// The child set is complete: appendMissingTaskGroupPlaceholders emits a
		// Missing placeholder for every declared task group whose directory is absent,
		// so those task group IDs stay in the reconcile's seen set. Full-initiative sync
		// must therefore prune children the plan no longer declares (the pruner still
		// skips and reports any child that owns an active run). Suppressing pruning
		// here would strand a task group dropped from the plan as a ghost child whenever
		// a sibling directory happened to be absent. Only the deliberately scoped
		// single-task-group sync (syncTaskGroupTarget), which collects one child
		// without its siblings, sets PreserveMissingChildren=true.
		PreserveMissingChildren: false,
	})
	if err != nil {
		return fmt.Errorf("reconcile task group initiative %s: %w", target.Ref.Initiative, err)
	}

	applyWorkflowSyncResult(result, target.InitiativeDir, &aggregate.Parent)
	for childIndex := range aggregate.Children {
		child := &aggregate.Children[childIndex]
		applyWorkflowSyncResult(result, collection.childPaths[child.Workflow.TaskGroupID], child)
		result.TaskGroupChildIDs = append(result.TaskGroupChildIDs, child.Workflow.ID)
	}
	result.WorkflowsPruned += len(aggregate.PrunedChildTaskGroupIDs)
	result.PrunedWorkflows = append(result.PrunedWorkflows, aggregate.PrunedChildTaskGroupIDs...)
	for _, skipped := range aggregate.SkippedChildPrunes {
		result.Warnings = append(result.Warnings, fmt.Sprintf(
			"%s: skipped stale task group prune: %s (%d active run(s))",
			skipped.Slug,
			skipped.Reason,
			skipped.ActiveRuns,
		))
	}
	result.LegacyArtifactsRemoved += len(collection.removedLegacyArtifacts)
	if len(collection.removedLegacyArtifacts) > 0 {
		sort.Strings(collection.removedLegacyArtifacts)
		result.Warnings = append(result.Warnings, fmt.Sprintf(
			"%s: removed legacy generated artifacts %s",
			target.Ref.Initiative,
			strings.Join(collection.removedLegacyArtifacts, ", "),
		))
	}
	if len(collection.missingTaskGroupIDs) > 0 {
		sort.Strings(collection.missingTaskGroupIDs)
		result.Partial = true
		result.MissingTaskGroups = append(result.MissingTaskGroups, collection.missingTaskGroupIDs...)
		result.Warnings = append(result.Warnings, fmt.Sprintf(
			"%s: partial task group sync; missing task group directories %s",
			target.Ref.Initiative,
			strings.Join(collection.missingTaskGroupIDs, ", "),
		))
	}
	return nil
}

func syncTaskGroupTarget(
	ctx context.Context,
	db *globaldb.GlobalDB,
	workspace globaldb.Workspace,
	target taskgroups.Target,
	result *SyncResult,
) error {
	if target.Mode != taskgroups.TargetModeTaskGroup {
		return fmt.Errorf("sync task group target: %s is not a task group", target.DisplayRef)
	}
	parentSnapshots, parentChecksum, err := collectArtifactSnapshotsExcludingTaskGroups(target.InitiativeDir)
	if err != nil {
		return err
	}
	child, childPath, removed, err := collectTaskGroupSyncChildFromTarget(ctx, workspace, target)
	if err != nil {
		return err
	}
	aggregate, err := db.ReconcileAggregateWorkflowSync(ctx, globaldb.AggregateWorkflowSyncInput{
		Parent: globaldb.WorkflowSyncInput{
			WorkspaceID:        workspace.ID,
			WorkflowSlug:       target.Ref.Initiative,
			Kind:               globaldb.WorkflowKindInitiative,
			SyncedAt:           time.Now().UTC(),
			CheckpointScope:    "workflow",
			CheckpointChecksum: parentChecksum,
			ArtifactSnapshots:  parentSnapshots,
		},
		Children:                []globaldb.WorkflowSyncInput{child},
		PreserveMissingChildren: true,
	})
	if err != nil {
		return fmt.Errorf("reconcile task group %s: %w", target.DisplayRef, err)
	}
	applyWorkflowSyncResult(result, target.InitiativeDir, &aggregate.Parent)
	for childIndex := range aggregate.Children {
		childResult := &aggregate.Children[childIndex]
		applyWorkflowSyncResult(result, childPath, childResult)
		result.TaskGroupChildIDs = append(result.TaskGroupChildIDs, childResult.Workflow.ID)
	}
	result.LegacyArtifactsRemoved += len(removed)
	if len(removed) > 0 {
		sort.Strings(removed)
		result.Warnings = append(result.Warnings, fmt.Sprintf(
			"%s: removed legacy generated artifacts %s",
			target.DisplayRef,
			strings.Join(removed, ", "),
		))
	}
	return nil
}

type taskGroupSyncChildren struct {
	children               []globaldb.WorkflowSyncInput
	childPaths             map[string]string
	missingTaskGroupIDs    []string
	removedLegacyArtifacts []string
}

func collectTaskGroupSyncChildren(
	ctx context.Context,
	workspace globaldb.Workspace,
	target taskgroups.Target,
) (taskGroupSyncChildren, error) {
	result := taskGroupSyncChildren{
		children:   make([]globaldb.WorkflowSyncInput, 0, len(target.Plan.TaskGroups)),
		childPaths: make(map[string]string, len(target.Plan.TaskGroups)),
	}
	resolver := taskgroups.TargetResolver{}
	for taskGroupIndex := range target.Plan.TaskGroups {
		taskGroup := &target.Plan.TaskGroups[taskGroupIndex]
		child, childPath, removed, err := collectTaskGroupSyncChild(ctx, resolver, workspace, target, taskGroup)
		if err != nil {
			if errors.Is(err, taskgroups.ErrTaskGroupNotFound) {
				result.missingTaskGroupIDs = append(result.missingTaskGroupIDs, taskGroup.ID)
				continue
			}
			return taskGroupSyncChildren{}, err
		}
		result.children = append(result.children, child)
		result.childPaths[taskGroup.ID] = childPath
		result.removedLegacyArtifacts = append(result.removedLegacyArtifacts, removed...)
	}
	return result, nil
}

// appendMissingTaskGroupPlaceholders reconciles the durable child row for
// every declared task group whose directory is absent so Missing always tracks
// current source availability. A first-ever partial sync would otherwise omit
// the node entirely, so any dependent task group keeps a graph edge to a
// nonexistent node: DB-only read models that reconstruct the plan
// (taskgroups.EvaluateReadiness) then reject the whole initiative and fail
// workflow listing, dashboard projection, and archive eligibility. A task group
// with no durable row yet is seeded fresh with the declared identity and
// dependency edges but no artifacts, so read models keep a complete graph and
// block start/archive with a clear reason. A task group that already owns a durable
// row -- whether it was materialized before its directory vanished, is a
// placeholder written by an earlier partial sync, or was materialized by a scoped
// sync racing this one -- keeps its retained snapshots, tasks, reviews, and
// checkpoint via a metadata-only reconcile that only refreshes identity columns
// (flipping Missing to true and tracking taskGroup.Completed plus later plan edits);
// otherwise the preserved row would advertise an absent directory as runnable or
// archive-eligible. Each placeholder sets MetadataOnlyIfExisting so the reconcile
// resolves "preserve vs seed" against the transaction's own snapshot: keying on
// row existence rather than the row's current Missing flag is essential (a repeat
// partial sync already reads Missing=true, and reseeding empty projections would
// delete the retained history), and resolving it inside the write transaction
// rather than via an out-of-transaction precheck closes a TOCTOU race where a
// concurrent scoped sync commits real projections in the gap. Missing clears
// automatically once the directory returns and a full projection is collected.
func appendMissingTaskGroupPlaceholders(
	workspace globaldb.Workspace,
	target taskgroups.Target,
	collection *taskGroupSyncChildren,
) []globaldb.WorkflowSyncInput {
	if len(collection.missingTaskGroupIDs) == 0 {
		return collection.children
	}
	missing := make(map[string]struct{}, len(collection.missingTaskGroupIDs))
	for _, taskGroupID := range collection.missingTaskGroupIDs {
		missing[taskGroupID] = struct{}{}
	}
	children := collection.children
	for taskGroupIndex := range target.Plan.TaskGroups {
		taskGroup := &target.Plan.TaskGroups[taskGroupIndex]
		if _, ok := missing[taskGroup.ID]; !ok {
			continue
		}
		slug := target.Ref.Initiative + "/" + taskGroup.ID
		placeholder := globaldb.WorkflowSyncInput{
			WorkspaceID:  workspace.ID,
			WorkflowSlug: slug,
			Kind:         globaldb.WorkflowKindTaskGroup,
			TaskGroupID:  taskGroup.ID,
			DisplayTitle: taskGroup.Title,
			Outcome:      taskGroup.Outcome,
			// Seed the canonical manifest checkbox so a first-ever missing child
			// reflects completion truthfully instead of always projecting false.
			LifecycleCompleted: taskGroup.Completed,
			Missing:            true,
			Dependencies:       taskGroupDependencies(taskGroup),
			SyncedAt:           time.Now().UTC(),
			CheckpointScope:    "workflow",
			// Preserve any durable row for this task group -- whether it was materialized
			// before its directory vanished, written by an earlier partial sync, or
			// created by a scoped sync racing this one -- via a metadata-only reconcile,
			// and seed a fresh node only when no row exists. The reconcile resolves that
			// existence inside its own write transaction, so a concurrent materialized
			// sync that commits just before this one cannot be clobbered by a stale
			// out-of-transaction precheck.
			MetadataOnlyIfExisting: true,
		}
		children = append(children, placeholder)
		collection.childPaths[taskGroup.ID] = filepath.Join(
			target.InitiativeDir,
			filepath.FromSlash(taskGroup.Directory),
		)
	}
	return children
}

func collectTaskGroupSyncChild(
	ctx context.Context,
	resolver taskgroups.TargetResolver,
	workspace globaldb.Workspace,
	target taskgroups.Target,
	taskGroup *taskgroups.TaskGroup,
) (globaldb.WorkflowSyncInput, string, []string, error) {
	if err := ctx.Err(); err != nil {
		return globaldb.WorkflowSyncInput{}, "", nil, err
	}
	childTarget, err := resolver.Resolve(ctx, workspace.RootDir, target.Ref.Initiative+"/"+taskGroup.ID)
	if err != nil {
		return globaldb.WorkflowSyncInput{}, "", nil, fmt.Errorf("resolve task group %s: %w", taskGroup.ID, err)
	}
	return collectTaskGroupSyncChildFromTarget(ctx, workspace, childTarget)
}

func collectTaskGroupSyncChildFromTarget(
	ctx context.Context,
	workspace globaldb.Workspace,
	childTarget taskgroups.Target,
) (globaldb.WorkflowSyncInput, string, []string, error) {
	if err := ctx.Err(); err != nil {
		return globaldb.WorkflowSyncInput{}, "", nil, err
	}
	if childTarget.Mode != taskgroups.TargetModeTaskGroup {
		return globaldb.WorkflowSyncInput{}, "", nil, fmt.Errorf(
			"resolve task group target: %s is not a task group",
			childTarget.DisplayRef,
		)
	}
	removed, err := cleanupLegacyWorkflowMetadata(childTarget.TaskGroupDir)
	if err != nil {
		return globaldb.WorkflowSyncInput{}, "", nil, fmt.Errorf(
			"cleanup task group %s metadata: %w",
			childTarget.TaskGroup.ID,
			err,
		)
	}
	snapshots, checksum, err := collectArtifactSnapshots(childTarget.TaskGroupDir)
	if err != nil {
		return globaldb.WorkflowSyncInput{}, "", nil, fmt.Errorf(
			"collect task group %s artifacts: %w",
			childTarget.TaskGroup.ID,
			err,
		)
	}
	taskItems, err := collectTaskItems(childTarget.TaskGroupDir)
	if err != nil {
		return globaldb.WorkflowSyncInput{}, "", nil, fmt.Errorf(
			"collect task group %s tasks: %w",
			childTarget.TaskGroup.ID,
			err,
		)
	}
	reviewRounds, err := collectReviewRounds(childTarget.TaskGroupDir)
	if err != nil {
		return globaldb.WorkflowSyncInput{}, "", nil, fmt.Errorf(
			"collect task group %s reviews: %w",
			childTarget.TaskGroup.ID,
			err,
		)
	}
	removedWithTaskGroupID := make([]string, 0, len(removed))
	for _, path := range removed {
		removedWithTaskGroupID = append(removedWithTaskGroupID, childTarget.TaskGroup.ID+"/"+path)
	}
	return globaldb.WorkflowSyncInput{
		WorkspaceID:        workspace.ID,
		WorkflowSlug:       childTarget.DisplayRef,
		Kind:               globaldb.WorkflowKindTaskGroup,
		TaskGroupID:        childTarget.TaskGroup.ID,
		DisplayTitle:       childTarget.TaskGroup.Title,
		Outcome:            childTarget.TaskGroup.Outcome,
		LifecycleCompleted: childTarget.TaskGroup.Completed,
		Dependencies:       taskGroupDependencies(&childTarget.TaskGroup),
		SyncedAt:           time.Now().UTC(),
		CheckpointScope:    "workflow",
		CheckpointChecksum: checksum,
		ArtifactSnapshots:  snapshots,
		TaskItems:          taskItems,
		ReviewRounds:       reviewRounds,
	}, childTarget.TaskGroupDir, removedWithTaskGroupID, nil
}

func taskGroupDependencies(taskGroup *taskgroups.TaskGroup) []globaldb.WorkflowDependency {
	dependencies := make([]globaldb.WorkflowDependency, 0, len(taskGroup.Dependencies))
	for _, dependency := range taskGroup.Dependencies {
		dependencies = append(dependencies, globaldb.WorkflowDependency{
			TaskGroupID: dependency.From,
			Rationale:   dependency.Rationale,
		})
	}
	sort.Slice(dependencies, func(i, j int) bool {
		if dependencies[i].TaskGroupID == dependencies[j].TaskGroupID {
			return dependencies[i].Rationale < dependencies[j].Rationale
		}
		return dependencies[i].TaskGroupID < dependencies[j].TaskGroupID
	})
	return dependencies
}

func applyWorkflowSyncResult(result *SyncResult, path string, state *globaldb.WorkflowSyncResult) {
	if result == nil {
		return
	}
	result.WorkflowsScanned++
	result.SnapshotsUpserted += state.SnapshotsUpserted
	result.TaskItemsUpserted += state.TaskItemsUpserted
	result.ReviewRoundsUpserted += state.ReviewRoundsUpserted
	result.ReviewIssuesUpserted += state.ReviewIssuesUpserted
	result.CheckpointsUpdated += state.CheckpointsUpdated
	result.SyncedPaths = append(result.SyncedPaths, path)
}

func collectArtifactSnapshots(tasksDir string) ([]globaldb.ArtifactSnapshotInput, string, error) {
	return collectArtifactSnapshotsWithOptions(tasksDir, false)
}

func collectArtifactSnapshotsExcludingTaskGroups(tasksDir string) ([]globaldb.ArtifactSnapshotInput, string, error) {
	return collectArtifactSnapshotsWithOptions(tasksDir, true)
}

func collectArtifactSnapshotsWithOptions(
	tasksDir string,
	excludeTaskGroups bool,
) ([]globaldb.ArtifactSnapshotInput, string, error) {
	root, err := os.OpenRoot(strings.TrimSpace(tasksDir))
	if err != nil {
		return nil, "", fmt.Errorf("open workflow root for artifact scan: %w", err)
	}
	defer root.Close()

	collector := artifactSnapshotCollector{
		root:              root,
		tasksDir:          tasksDir,
		excludeTaskGroups: excludeTaskGroups,
	}
	err = filepath.WalkDir(tasksDir, collector.visit)
	if err != nil {
		return nil, "", fmt.Errorf("walk workflow artifacts: %w", err)
	}

	sort.SliceStable(collector.snapshots, func(i, j int) bool {
		left := collector.snapshots[i].ArtifactKind + "\x00" + collector.snapshots[i].RelativePath
		right := collector.snapshots[j].ArtifactKind + "\x00" + collector.snapshots[j].RelativePath
		return left < right
	})
	sort.Strings(collector.checksumParts)
	return collector.snapshots, checksumHex([]byte(strings.Join(collector.checksumParts, "\n"))), nil
}

type artifactSnapshotCollector struct {
	root              *os.Root
	tasksDir          string
	excludeTaskGroups bool
	snapshots         []globaldb.ArtifactSnapshotInput
	checksumParts     []string
}

func (c *artifactSnapshotCollector) visit(path string, entry fs.DirEntry, walkErr error) error {
	if walkErr != nil {
		return walkErr
	}
	if entry.IsDir() {
		return c.visitDirectory(path, entry)
	}
	if !entry.Type().IsRegular() || !strings.HasSuffix(strings.ToLower(entry.Name()), ".md") {
		return nil
	}
	return c.collectArtifact(path, entry)
}

func (c *artifactSnapshotCollector) visitDirectory(path string, entry fs.DirEntry) error {
	if c.excludeTaskGroups && path != c.tasksDir && entry.Name() == "_task_groups" {
		return filepath.SkipDir
	}
	if path != c.tasksDir && strings.HasPrefix(entry.Name(), ".") {
		return filepath.SkipDir
	}
	return nil
}

func (c *artifactSnapshotCollector) collectArtifact(path string, entry fs.DirEntry) error {
	relativePath, err := filepath.Rel(c.tasksDir, path)
	if err != nil {
		return fmt.Errorf("resolve relative artifact path for %s: %w", path, err)
	}
	relativePath = filepath.ToSlash(relativePath)
	if relativePath == "_meta.md" || isReviewRoundMetaPath(relativePath) {
		return nil
	}
	info, err := entry.Info()
	if err != nil {
		return fmt.Errorf("stat artifact %s: %w", path, err)
	}
	content, err := c.root.ReadFile(filepath.FromSlash(relativePath))
	if err != nil {
		return fmt.Errorf("read artifact %s: %w", path, err)
	}
	frontmatterJSON, bodyText, err := snapshotArtifactContent(string(content))
	if err != nil {
		return fmt.Errorf("parse artifact %s: %w", path, err)
	}
	checksum := checksumHex(content)
	artifactKind := classifyArtifactKind(relativePath)
	c.snapshots = append(c.snapshots, globaldb.ArtifactSnapshotInput{
		ArtifactKind:    artifactKind,
		RelativePath:    relativePath,
		Checksum:        checksum,
		FrontmatterJSON: frontmatterJSON,
		BodyText:        bodyText,
		SourceMTime:     info.ModTime().UTC(),
	})
	c.checksumParts = append(c.checksumParts, artifactKind+"\x00"+relativePath+"\x00"+checksum)
	return nil
}

func snapshotArtifactContent(content string) (string, string, error) {
	metadata := make(map[string]any)
	body, err := frontmatter.Parse(content, &metadata)
	if err == nil {
		encoded, marshalErr := json.Marshal(metadata)
		if marshalErr != nil {
			return "", "", fmt.Errorf("marshal artifact front matter: %w", marshalErr)
		}
		return string(encoded), body, nil
	}
	if errors.Is(err, frontmatter.ErrHeaderNotFound) {
		return "{}", content, nil
	}
	return "", "", err
}

func classifyArtifactKind(relativePath string) string {
	clean := filepath.ToSlash(strings.TrimSpace(relativePath))
	base := filepath.Base(clean)

	switch {
	case clean == "_prd.md":
		return "prd"
	case clean == "_techspec.md":
		return "techspec"
	case clean == "_tasks.md":
		return "tasks_index"
	case clean == taskgroups.ManifestFileName:
		return "task_group_plan"
	case tasks.ExtractTaskNumber(base) > 0 && !strings.Contains(clean, "/"):
		return "task"
	case strings.HasPrefix(clean, "adrs/"):
		return "adr"
	case strings.HasPrefix(clean, "memory/"):
		return "memory"
	case isReviewIssuePath(clean):
		return "review_issue"
	case strings.HasPrefix(clean, "qa/"):
		return "qa"
	case strings.HasPrefix(clean, "prompt/"), strings.HasPrefix(clean, "prompts/"):
		return "prompt"
	case strings.HasPrefix(clean, "protocol/"), strings.HasPrefix(clean, "protocols/"):
		return "protocol"
	default:
		return "artifact"
	}
}

func isReviewRoundMetaPath(relativePath string) bool {
	clean := filepath.ToSlash(strings.TrimSpace(relativePath))
	if filepath.Base(clean) != "_meta.md" {
		return false
	}
	dir := filepath.Dir(clean)
	return strings.HasPrefix(dir, "reviews-")
}

func isReviewIssuePath(relativePath string) bool {
	clean := filepath.ToSlash(strings.TrimSpace(relativePath))
	dir := filepath.Dir(clean)
	if !strings.HasPrefix(dir, "reviews-") {
		return false
	}
	return reviews.ExtractIssueNumber(filepath.Base(clean)) > 0
}

func collectTaskItems(tasksDir string) ([]globaldb.TaskItemInput, error) {
	entries, err := tasks.ReadTaskEntries(tasksDir, true)
	if err != nil {
		return nil, fmt.Errorf("read task entries: %w", err)
	}

	taskItems := make([]globaldb.TaskItemInput, 0, len(entries))
	for _, entry := range entries {
		task, err := tasks.ParseTaskFile(entry.Content)
		if err != nil {
			return nil, tasks.WrapParseError(entry.AbsPath, err)
		}

		taskNumber := tasks.ExtractTaskNumber(entry.Name)
		if taskNumber == 0 {
			return nil, fmt.Errorf("invalid task file name %q", entry.Name)
		}
		sourcePath := filepath.ToSlash(entry.Name)
		taskID := strings.TrimSuffix(filepath.Base(sourcePath), filepath.Ext(sourcePath))
		taskItems = append(taskItems, globaldb.TaskItemInput{
			TaskNumber: taskNumber,
			TaskID:     taskID,
			Title:      task.Title,
			Status:     strings.ToLower(strings.TrimSpace(task.Status)),
			Kind:       task.TaskType,
			DependsOn:  append([]string(nil), task.Dependencies...),
			SourcePath: sourcePath,
		})
	}
	return taskItems, nil
}

func collectReviewRounds(tasksDir string) ([]globaldb.ReviewRoundInput, error) {
	roundNumbers, err := reviews.DiscoverRounds(tasksDir)
	if err != nil {
		return nil, fmt.Errorf("discover review rounds: %w", err)
	}

	rounds := make([]globaldb.ReviewRoundInput, 0, len(roundNumbers))
	for _, roundNumber := range roundNumbers {
		reviewDir := reviews.ReviewDirectory(tasksDir, roundNumber)
		reviewEntries, err := reviews.ReadReviewEntries(reviewDir)
		if err != nil {
			return nil, fmt.Errorf("read review entries %s: %w", reviewDir, err)
		}
		if len(reviewEntries) == 0 {
			continue
		}

		round, err := collectReviewRound(tasksDir, reviewDir, roundNumber, reviewEntries)
		if err != nil {
			return nil, err
		}
		rounds = append(rounds, round)
	}

	return rounds, nil
}

func collectReviewRound(
	tasksDir string,
	reviewDir string,
	roundNumber int,
	reviewEntries []model.IssueEntry,
) (globaldb.ReviewRoundInput, error) {
	resolvedCount := 0
	issues := make([]globaldb.ReviewIssueInput, 0, len(reviewEntries))
	var provider, prRef string

	for _, entry := range reviewEntries {
		reviewCtx, err := reviews.ParseReviewContext(entry.Content)
		if err != nil {
			return globaldb.ReviewRoundInput{}, reviews.WrapParseError(entry.AbsPath, err)
		}
		if reviewCtx.Round != 0 && reviewCtx.Round != roundNumber {
			return globaldb.ReviewRoundInput{}, fmt.Errorf(
				"review issue %s declares round=%d but directory %s is round=%d",
				entry.AbsPath,
				reviewCtx.Round,
				filepath.Base(reviewDir),
				roundNumber,
			)
		}
		nextProvider := strings.TrimSpace(reviewCtx.Provider)
		if nextProvider != "" {
			if provider != "" && provider != nextProvider {
				return globaldb.ReviewRoundInput{}, fmt.Errorf(
					"review issue %s has provider %q but round %s already uses provider %q",
					entry.AbsPath,
					nextProvider,
					filepath.Base(reviewDir),
					provider,
				)
			}
			provider = nextProvider
		}
		nextPR := strings.TrimSpace(reviewCtx.PR)
		if nextPR != "" {
			if prRef != "" && prRef != nextPR {
				return globaldb.ReviewRoundInput{}, fmt.Errorf(
					"review issue %s has pr %q but round %s already uses pr %q",
					entry.AbsPath,
					nextPR,
					filepath.Base(reviewDir),
					prRef,
				)
			}
			prRef = nextPR
		}
		if strings.EqualFold(strings.TrimSpace(reviewCtx.Status), "resolved") {
			resolvedCount++
		}

		relativePath, err := filepath.Rel(tasksDir, entry.AbsPath)
		if err != nil {
			return globaldb.ReviewRoundInput{}, fmt.Errorf("resolve review issue path %s: %w", entry.AbsPath, err)
		}
		issues = append(issues, globaldb.ReviewIssueInput{
			IssueNumber: reviews.ExtractIssueNumber(entry.Name),
			Severity:    strings.TrimSpace(reviewCtx.Severity),
			Status:      strings.ToLower(strings.TrimSpace(reviewCtx.Status)),
			SourcePath:  filepath.ToSlash(relativePath),
		})
	}

	return globaldb.ReviewRoundInput{
		RoundNumber:     roundNumber,
		Provider:        provider,
		PRRef:           prRef,
		ResolvedCount:   resolvedCount,
		UnresolvedCount: len(issues) - resolvedCount,
		Issues:          issues,
	}, nil
}

func cleanupLegacyWorkflowMetadata(tasksDir string) ([]string, error) {
	removed := make([]string, 0, 2)

	if deleted, err := removeFileIfPresent(filepath.Join(tasksDir, "_meta.md")); err != nil {
		return nil, fmt.Errorf("remove legacy workflow metadata: %w", err)
	} else if deleted {
		removed = append(removed, "_meta.md")
	}

	taskListPath := filepath.Join(tasksDir, "_tasks.md")
	taskListBody, err := os.ReadFile(taskListPath)
	switch {
	case err == nil:
		if shouldRemoveLegacyTaskList(string(taskListBody)) {
			if err := os.Remove(taskListPath); err != nil {
				return nil, fmt.Errorf("remove legacy task list %s: %w", taskListPath, err)
			}
			removed = append(removed, "_tasks.md")
		}
	case errors.Is(err, os.ErrNotExist):
		// Nothing to clean.
	default:
		return nil, fmt.Errorf("read task list %s: %w", taskListPath, err)
	}

	return removed, nil
}

func shouldRemoveLegacyTaskList(content string) bool {
	return !isAuthoredTaskList(content) && !isTaskGraphManifest(content)
}

func isAuthoredTaskList(content string) bool {
	return strings.Contains(content, authoredTaskListHeader)
}

func isTaskGraphManifest(content string) bool {
	var manifest tasks.TaskGraphManifest
	if _, err := frontmatter.Parse(content, &manifest); err != nil {
		return !errors.Is(err, frontmatter.ErrHeaderNotFound)
	}
	return strings.TrimSpace(manifest.SchemaVersion) != "" ||
		strings.TrimSpace(manifest.Workflow) != "" ||
		len(manifest.Graph.Nodes) > 0 ||
		len(manifest.Graph.Edges) > 0
}

func removeFileIfPresent(path string) (bool, error) {
	if err := os.Remove(path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func checksumHex(content []byte) string {
	sum := sha256.Sum256(content)
	return hex.EncodeToString(sum[:])
}

func sortSyncResult(result *SyncResult) {
	if result == nil {
		return
	}
	sort.Strings(result.SyncedPaths)
	sort.Strings(result.PrunedWorkflows)
	sort.Strings(result.Warnings)
}
