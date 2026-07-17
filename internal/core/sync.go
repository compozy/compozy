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
	"github.com/compozy/compozy/internal/core/tasks"
	"github.com/compozy/compozy/internal/core/workpackages"
	"github.com/compozy/compozy/internal/store/globaldb"
)

const authoredTaskListHeader = "| # | Title | Status | Complexity | Dependencies |"

func syncTaskMetadata(ctx context.Context, cfg SyncConfig) (*SyncResult, error) {
	if cfg.ExecutionScope != nil {
		return syncScopedWorkPackageDirect(ctx, cfg)
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

func syncScopedWorkPackageDirect(ctx context.Context, cfg SyncConfig) (*SyncResult, error) {
	scope := cfg.ExecutionScope
	result := &SyncResult{}
	if scope != nil {
		result.Target = scope.OperationalDir
	}
	workspaceRoot := strings.TrimSpace(cfg.WorkspaceRoot)
	if workspaceRoot == "" {
		return result, errors.New("work package execution scope requires workspace root")
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
	return syncScopedWorkPackage(ctx, db, workspace, cfg)
}

// SyncWithDB reconciles workflow artifacts into an already-open global.db.
func SyncWithDB(
	ctx context.Context,
	db *globaldb.GlobalDB,
	workspace globaldb.Workspace,
	cfg SyncConfig,
) (*SyncResult, error) {
	if cfg.ExecutionScope != nil {
		return syncScopedWorkPackage(ctx, db, workspace, cfg)
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

func syncScopedWorkPackage(
	ctx context.Context,
	db *globaldb.GlobalDB,
	workspace globaldb.Workspace,
	cfg SyncConfig,
) (*SyncResult, error) {
	if cfg.ExecutionScope == nil {
		return nil, errors.New("work package execution scope is required")
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
		return result, errors.New("work package execution scope workflow reference is required")
	}
	if !syncTargetBelongsToWorkspace(scope.SpecDir, workspace.RootDir) ||
		!syncTargetBelongsToWorkspace(scope.OperationalDir, workspace.RootDir) {
		return result, fmt.Errorf("mismatched workspace and work package execution scope: %s", scope.WorkflowRef)
	}
	if err := ensureCurrentExecutionScopeSpecifications(scope); err != nil {
		return result, err
	}

	target, err := (workpackages.TargetResolver{}).ResolvePackage(ctx, workspace.RootDir, scope.WorkflowRef)
	if err != nil {
		return result, fmt.Errorf("resolve work package execution scope %s: %w", scope.WorkflowRef, err)
	}
	currentScope, err := workpackages.BuildExecutionScope(target)
	if err != nil {
		return result, err
	}
	if !sameExecutionScope(scope, currentScope) {
		return result, fmt.Errorf("work package execution scope changed while syncing %s", scope.WorkflowRef)
	}
	if err := syncWorkPackageTarget(ctx, db, workspace, target, result); err != nil {
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
		if isWorkPackageOperationalDirectory(workspace.RootDir, target) {
			return result, WorkPackageRootOnlyError{Target: target}
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
	if target, ok := namedWorkPackageTarget(cfg.Name); ok {
		return "", false, WorkPackageRootOnlyError{Target: target}
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

func namedWorkPackageTarget(name string) (string, bool) {
	ref, err := workpackages.ParseRef(strings.TrimSpace(name))
	if err != nil || !ref.IsPackage() {
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

	initiativeTarget, resolveErr := (workpackages.TargetResolver{}).Resolve(
		ctx,
		workspace.RootDir,
		filepath.Base(tasksDir),
	)
	if resolveErr != nil {
		if errors.Is(resolveErr, workpackages.ErrInvalidPlan) {
			return fmt.Errorf("sync workflow %s: %w", tasksDir, resolveErr)
		}
		return fmt.Errorf("resolve sync workflow target %s: %w", tasksDir, resolveErr)
	}
	if initiativeTarget.Mode == workpackages.TargetModeInitiative {
		return syncWorkPackageInitiative(ctx, db, workspace, initiativeTarget, result)
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

func syncWorkPackageInitiative(
	ctx context.Context,
	db *globaldb.GlobalDB,
	workspace globaldb.Workspace,
	target workpackages.Target,
	result *SyncResult,
) error {
	parentSnapshots, parentChecksum, err := collectArtifactSnapshotsExcludingPackages(target.InitiativeDir)
	if err != nil {
		return err
	}

	collection, err := collectWorkPackageSyncChildren(ctx, workspace, target)
	if err != nil {
		return err
	}
	children, err := appendMissingWorkPackagePlaceholders(ctx, db, workspace, target, &collection)
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
		Children:                children,
		PreserveMissingChildren: len(collection.missingPackageIDs) > 0,
	})
	if err != nil {
		return fmt.Errorf("reconcile work package initiative %s: %w", target.Ref.Initiative, err)
	}

	applyWorkflowSyncResult(result, target.InitiativeDir, &aggregate.Parent)
	for childIndex := range aggregate.Children {
		child := &aggregate.Children[childIndex]
		applyWorkflowSyncResult(result, collection.childPaths[child.Workflow.PackageID], child)
		result.WorkPackageChildIDs = append(result.WorkPackageChildIDs, child.Workflow.ID)
	}
	result.WorkflowsPruned += len(aggregate.PrunedChildPackageIDs)
	result.PrunedWorkflows = append(result.PrunedWorkflows, aggregate.PrunedChildPackageIDs...)
	for _, skipped := range aggregate.SkippedChildPrunes {
		result.Warnings = append(result.Warnings, fmt.Sprintf(
			"%s: skipped stale work package prune: %s (%d active run(s))",
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
	if len(collection.missingPackageIDs) > 0 {
		sort.Strings(collection.missingPackageIDs)
		result.Partial = true
		result.MissingWorkPackages = append(result.MissingWorkPackages, collection.missingPackageIDs...)
		result.Warnings = append(result.Warnings, fmt.Sprintf(
			"%s: partial work package sync; missing package directories %s",
			target.Ref.Initiative,
			strings.Join(collection.missingPackageIDs, ", "),
		))
	}
	return nil
}

func syncWorkPackageTarget(
	ctx context.Context,
	db *globaldb.GlobalDB,
	workspace globaldb.Workspace,
	target workpackages.Target,
	result *SyncResult,
) error {
	if target.Mode != workpackages.TargetModePackage {
		return fmt.Errorf("sync work package target: %s is not a package", target.DisplayRef)
	}
	parentSnapshots, parentChecksum, err := collectArtifactSnapshotsExcludingPackages(target.InitiativeDir)
	if err != nil {
		return err
	}
	child, childPath, removed, err := collectWorkPackageSyncChildFromTarget(ctx, workspace, target)
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
		return fmt.Errorf("reconcile work package %s: %w", target.DisplayRef, err)
	}
	applyWorkflowSyncResult(result, target.InitiativeDir, &aggregate.Parent)
	for childIndex := range aggregate.Children {
		childResult := &aggregate.Children[childIndex]
		applyWorkflowSyncResult(result, childPath, childResult)
		result.WorkPackageChildIDs = append(result.WorkPackageChildIDs, childResult.Workflow.ID)
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

type workPackageSyncChildren struct {
	children               []globaldb.WorkflowSyncInput
	childPaths             map[string]string
	missingPackageIDs      []string
	removedLegacyArtifacts []string
}

func collectWorkPackageSyncChildren(
	ctx context.Context,
	workspace globaldb.Workspace,
	target workpackages.Target,
) (workPackageSyncChildren, error) {
	result := workPackageSyncChildren{
		children:   make([]globaldb.WorkflowSyncInput, 0, len(target.Plan.Packages)),
		childPaths: make(map[string]string, len(target.Plan.Packages)),
	}
	resolver := workpackages.TargetResolver{}
	for packageIndex := range target.Plan.Packages {
		pkg := &target.Plan.Packages[packageIndex]
		child, childPath, removed, err := collectWorkPackageSyncChild(ctx, resolver, workspace, target, pkg)
		if err != nil {
			if errors.Is(err, workpackages.ErrPackageNotFound) {
				result.missingPackageIDs = append(result.missingPackageIDs, pkg.ID)
				continue
			}
			return workPackageSyncChildren{}, err
		}
		result.children = append(result.children, child)
		result.childPaths[pkg.ID] = childPath
		result.removedLegacyArtifacts = append(result.removedLegacyArtifacts, removed...)
	}
	return result, nil
}

// appendMissingWorkPackagePlaceholders seeds a durable placeholder child for
// every declared package whose directory is absent and has no prior durable row.
// A first-ever partial sync would otherwise omit the node entirely, so any
// dependent package keeps a graph edge to a nonexistent node: DB-only read
// models that reconstruct the plan (workpackages.EvaluateReadiness) then reject
// the whole initiative and fail workflow listing, dashboard projection, and
// archive eligibility. The placeholder carries the declared identity and
// dependency edges but no artifacts and an incomplete lifecycle, so read models
// keep a complete graph and block start/archive with a clear reason instead of
// erroring or reporting a false archive-eligible state. Packages that already
// own a durable row are left untouched so PreserveMissingChildren keeps their
// prior projection intact rather than overwriting it with an empty placeholder.
func appendMissingWorkPackagePlaceholders(
	ctx context.Context,
	db *globaldb.GlobalDB,
	workspace globaldb.Workspace,
	target workpackages.Target,
	collection *workPackageSyncChildren,
) ([]globaldb.WorkflowSyncInput, error) {
	if len(collection.missingPackageIDs) == 0 {
		return collection.children, nil
	}
	missing := make(map[string]struct{}, len(collection.missingPackageIDs))
	for _, packageID := range collection.missingPackageIDs {
		missing[packageID] = struct{}{}
	}
	children := collection.children
	for packageIndex := range target.Plan.Packages {
		pkg := &target.Plan.Packages[packageIndex]
		if _, ok := missing[pkg.ID]; !ok {
			continue
		}
		slug := target.Ref.Initiative + "/" + pkg.ID
		if _, err := db.GetActiveWorkflowBySlug(ctx, workspace.ID, slug); err == nil {
			continue
		} else if !errors.Is(err, globaldb.ErrWorkflowNotFound) {
			return nil, fmt.Errorf("inspect durable work package %s: %w", pkg.ID, err)
		}
		children = append(children, globaldb.WorkflowSyncInput{
			WorkspaceID:        workspace.ID,
			WorkflowSlug:       slug,
			Kind:               globaldb.WorkflowKindWorkPackage,
			PackageID:          pkg.ID,
			DisplayTitle:       pkg.Title,
			Outcome:            pkg.Outcome,
			LifecycleCompleted: false,
			Dependencies:       packageDependencies(pkg),
			SyncedAt:           time.Now().UTC(),
			CheckpointScope:    "workflow",
		})
		collection.childPaths[pkg.ID] = filepath.Join(target.InitiativeDir, filepath.FromSlash(pkg.Directory))
	}
	return children, nil
}

func collectWorkPackageSyncChild(
	ctx context.Context,
	resolver workpackages.TargetResolver,
	workspace globaldb.Workspace,
	target workpackages.Target,
	pkg *workpackages.Package,
) (globaldb.WorkflowSyncInput, string, []string, error) {
	if err := ctx.Err(); err != nil {
		return globaldb.WorkflowSyncInput{}, "", nil, err
	}
	childTarget, err := resolver.Resolve(ctx, workspace.RootDir, target.Ref.Initiative+"/"+pkg.ID)
	if err != nil {
		return globaldb.WorkflowSyncInput{}, "", nil, fmt.Errorf("resolve work package %s: %w", pkg.ID, err)
	}
	return collectWorkPackageSyncChildFromTarget(ctx, workspace, childTarget)
}

func collectWorkPackageSyncChildFromTarget(
	ctx context.Context,
	workspace globaldb.Workspace,
	childTarget workpackages.Target,
) (globaldb.WorkflowSyncInput, string, []string, error) {
	if err := ctx.Err(); err != nil {
		return globaldb.WorkflowSyncInput{}, "", nil, err
	}
	if childTarget.Mode != workpackages.TargetModePackage {
		return globaldb.WorkflowSyncInput{}, "", nil, fmt.Errorf(
			"resolve work package target: %s is not a package",
			childTarget.DisplayRef,
		)
	}
	removed, err := cleanupLegacyWorkflowMetadata(childTarget.PackageDir)
	if err != nil {
		return globaldb.WorkflowSyncInput{}, "", nil, fmt.Errorf(
			"cleanup work package %s metadata: %w",
			childTarget.Package.ID,
			err,
		)
	}
	snapshots, checksum, err := collectArtifactSnapshots(childTarget.PackageDir)
	if err != nil {
		return globaldb.WorkflowSyncInput{}, "", nil, fmt.Errorf(
			"collect work package %s artifacts: %w",
			childTarget.Package.ID,
			err,
		)
	}
	taskItems, err := collectTaskItems(childTarget.PackageDir)
	if err != nil {
		return globaldb.WorkflowSyncInput{}, "", nil, fmt.Errorf(
			"collect work package %s tasks: %w",
			childTarget.Package.ID,
			err,
		)
	}
	reviewRounds, err := collectReviewRounds(childTarget.PackageDir)
	if err != nil {
		return globaldb.WorkflowSyncInput{}, "", nil, fmt.Errorf(
			"collect work package %s reviews: %w",
			childTarget.Package.ID,
			err,
		)
	}
	removedWithPackageID := make([]string, 0, len(removed))
	for _, path := range removed {
		removedWithPackageID = append(removedWithPackageID, childTarget.Package.ID+"/"+path)
	}
	return globaldb.WorkflowSyncInput{
		WorkspaceID:        workspace.ID,
		WorkflowSlug:       childTarget.DisplayRef,
		Kind:               globaldb.WorkflowKindWorkPackage,
		PackageID:          childTarget.Package.ID,
		DisplayTitle:       childTarget.Package.Title,
		Outcome:            childTarget.Package.Outcome,
		LifecycleCompleted: childTarget.Package.Completed,
		Dependencies:       packageDependencies(&childTarget.Package),
		SyncedAt:           time.Now().UTC(),
		CheckpointScope:    "workflow",
		CheckpointChecksum: checksum,
		ArtifactSnapshots:  snapshots,
		TaskItems:          taskItems,
		ReviewRounds:       reviewRounds,
	}, childTarget.PackageDir, removedWithPackageID, nil
}

func packageDependencies(pkg *workpackages.Package) []globaldb.WorkflowDependency {
	dependencies := make([]globaldb.WorkflowDependency, 0, len(pkg.Dependencies))
	for _, dependency := range pkg.Dependencies {
		dependencies = append(dependencies, globaldb.WorkflowDependency{
			PackageID: dependency.From,
			Rationale: dependency.Rationale,
		})
	}
	sort.Slice(dependencies, func(i, j int) bool {
		if dependencies[i].PackageID == dependencies[j].PackageID {
			return dependencies[i].Rationale < dependencies[j].Rationale
		}
		return dependencies[i].PackageID < dependencies[j].PackageID
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

func collectArtifactSnapshotsExcludingPackages(tasksDir string) ([]globaldb.ArtifactSnapshotInput, string, error) {
	return collectArtifactSnapshotsWithOptions(tasksDir, true)
}

func collectArtifactSnapshotsWithOptions(
	tasksDir string,
	excludePackages bool,
) ([]globaldb.ArtifactSnapshotInput, string, error) {
	root, err := os.OpenRoot(strings.TrimSpace(tasksDir))
	if err != nil {
		return nil, "", fmt.Errorf("open workflow root for artifact scan: %w", err)
	}
	defer root.Close()

	collector := artifactSnapshotCollector{
		root:            root,
		tasksDir:        tasksDir,
		excludePackages: excludePackages,
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
	root            *os.Root
	tasksDir        string
	excludePackages bool
	snapshots       []globaldb.ArtifactSnapshotInput
	checksumParts   []string
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
	if c.excludePackages && path != c.tasksDir && entry.Name() == "_packages" {
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
	case clean == workpackages.ManifestFileName:
		return "work_package_plan"
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
