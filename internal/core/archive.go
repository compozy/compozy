package core

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/core/model"
	"github.com/compozy/compozy/internal/core/reviews"
	"github.com/compozy/compozy/internal/core/tasks"
	"github.com/compozy/compozy/internal/core/workpackages"
	"github.com/compozy/compozy/internal/store/globaldb"
)

const workflowStateNotSyncedReason = "workflow state not synced"

var (
	ErrWorkflowForceRequired      = errors.New("core: workflow force required")
	ErrWorkPackageRootOnly        = errors.New("core: work package sync or archive requires initiative root")
	ErrArchiveDatabaseRequired    = errors.New("core: archive database is required")
	ErrArchiveWorkspaceIDRequired = errors.New("core: archive workspace id is required")
)

// WorkPackageRootOnlyError rejects a package-local sync or archive target.
type WorkPackageRootOnlyError struct {
	Target string
}

func (e WorkPackageRootOnlyError) Error() string {
	return fmt.Sprintf("core: work package target %q cannot be synchronized or archived independently", e.Target)
}

func (e WorkPackageRootOnlyError) Is(target error) bool {
	return target == ErrWorkPackageRootOnly
}

// ArchiveWorkspaceMismatchError reports that the archive target is outside the
// injected workspace boundary.
type ArchiveWorkspaceMismatchError struct {
	Target        string
	WorkspaceRoot string
}

func (e ArchiveWorkspaceMismatchError) Error() string {
	return fmt.Sprintf(
		"core: mismatched workspace and archive target: %s is outside %s",
		e.Target,
		e.WorkspaceRoot,
	)
}

// WorkflowArchiveForceRequiredError reports a workflow archive conflict that
// can be resolved locally by completing tasks and resolving review issues.
type WorkflowArchiveForceRequiredError struct {
	WorkspaceID      string
	WorkflowID       string
	Slug             string
	Reason           string
	TaskTotal        int
	TaskNonTerminal  int
	ReviewTotal      int
	ReviewUnresolved int
}

func (e WorkflowArchiveForceRequiredError) Error() string {
	name := strings.TrimSpace(e.Slug)
	if name == "" {
		name = strings.TrimSpace(e.WorkflowID)
	}
	if name == "" {
		name = "workflow"
	}
	if strings.TrimSpace(e.Reason) == "" {
		return fmt.Sprintf("core: workflow %q requires force archive confirmation", name)
	}
	return fmt.Sprintf("core: workflow %q requires force archive confirmation: %s", name, e.Reason)
}

func (e WorkflowArchiveForceRequiredError) Is(target error) bool {
	return target == ErrWorkflowForceRequired
}

func archiveTaskWorkflows(ctx context.Context, cfg ArchiveConfig) (*ArchiveResult, error) {
	target, rootDir, singleWorkflow, err := resolveArchiveTarget(ctx, cfg)
	result := &ArchiveResult{
		Target:         target,
		ArchiveRoot:    model.ArchivedTasksDir(rootDir),
		SkippedReasons: make(map[string]string),
	}
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
	return archiveResolvedTarget(ctx, db, workspace, cfg, target, singleWorkflow, result)
}

// ArchiveWithDB archives workflow artifacts using an already-open global.db.
func ArchiveWithDB(
	ctx context.Context,
	db *globaldb.GlobalDB,
	workspace globaldb.Workspace,
	cfg ArchiveConfig,
) (*ArchiveResult, error) {
	target, rootDir, singleWorkflow, err := resolveArchiveTarget(ctx, cfg)
	result := &ArchiveResult{
		Target:         target,
		ArchiveRoot:    model.ArchivedTasksDir(rootDir),
		SkippedReasons: make(map[string]string),
	}
	if err != nil {
		return result, err
	}
	if db == nil {
		return result, ErrArchiveDatabaseRequired
	}
	if strings.TrimSpace(workspace.ID) == "" {
		return result, ErrArchiveWorkspaceIDRequired
	}
	if !syncTargetBelongsToWorkspace(target, workspace.RootDir) {
		return result, ArchiveWorkspaceMismatchError{
			Target:        target,
			WorkspaceRoot: workspace.RootDir,
		}
	}
	return archiveResolvedTarget(ctx, db, workspace, cfg, target, singleWorkflow, result)
}

func archiveResolvedTarget(
	ctx context.Context,
	db *globaldb.GlobalDB,
	workspace globaldb.Workspace,
	cfg ArchiveConfig,
	target string,
	singleWorkflow bool,
	result *ArchiveResult,
) (*ArchiveResult, error) {
	if singleWorkflow {
		if isWorkPackageOperationalDirectory(workspace.RootDir, target) {
			return result, WorkPackageRootOnlyError{Target: target}
		}
		if err := archiveWorkflow(ctx, db, workspace, target, cfg.Force, result, true); err != nil {
			return result, err
		}
		sortArchiveResult(result)
		return result, nil
	}

	entries, err := os.ReadDir(target)
	if err != nil {
		return result, fmt.Errorf("read archive target: %w", err)
	}
	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return result, err
		}
		if !entry.IsDir() || !model.IsActiveWorkflowDirName(entry.Name()) {
			continue
		}
		if err := archiveWorkflow(
			ctx,
			db,
			workspace,
			filepath.Join(target, entry.Name()),
			false,
			result,
			false,
		); err != nil {
			return result, err
		}
	}

	sortArchiveResult(result)
	return result, nil
}

func isWorkPackageOperationalDirectory(workspaceRoot string, path string) bool {
	tasksRoot := canonicalWorkflowScopePath(model.TasksBaseDirForWorkspace(workspaceRoot))
	target := canonicalWorkflowScopePath(path)
	relative, err := filepath.Rel(tasksRoot, target)
	if err != nil || relative == "." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return false
	}
	for _, component := range strings.Split(filepath.Clean(relative), string(filepath.Separator)) {
		if component == "_packages" {
			return true
		}
	}
	return false
}

func resolveArchiveTarget(ctx context.Context, cfg ArchiveConfig) (string, string, bool, error) {
	name := strings.TrimSpace(cfg.Name)
	if name == model.ArchivedWorkflowDirName {
		return "", "", false, fmt.Errorf("archive target cannot be %s", model.ArchivedWorkflowDirName)
	}
	if target, ok := namedWorkPackageTarget(name); ok {
		return "", "", false, WorkPackageRootOnlyError{Target: target}
	}

	resolvedTarget, rootDir, specificTarget, slug, err := resolveArchiveSelection(cfg, name)
	if err != nil {
		return "", "", false, err
	}
	if err := validateArchiveTarget(ctx, resolvedTarget, rootDir, slug, specificTarget); err != nil {
		return "", "", false, err
	}
	return resolvedTarget, rootDir, specificTarget, nil
}

func archiveSlugForTarget(name string, target string) string {
	if trimmed := strings.TrimSpace(name); trimmed != "" {
		return trimmed
	}
	return filepath.Base(strings.TrimSpace(target))
}

func resolveArchiveSelection(
	cfg ArchiveConfig,
	name string,
) (target string, rootDir string, specificTarget bool, slug string, err error) {
	if countArchiveSelectors(cfg) > 1 {
		return "", "", false, "", fmt.Errorf("archive accepts only one of --name or --tasks-dir")
	}

	rootDir = strings.TrimSpace(cfg.RootDir)
	if rootDir == "" {
		rootDir = model.TasksBaseDirForWorkspace(cfg.WorkspaceRoot)
	}
	rootDir, err = filepath.Abs(rootDir)
	if err != nil {
		return "", "", false, "", fmt.Errorf("resolve archive root: %w", err)
	}

	target = rootDir
	switch {
	case strings.TrimSpace(cfg.TasksDir) != "":
		target = strings.TrimSpace(cfg.TasksDir)
		specificTarget = true
	case name != "":
		target = filepath.Join(rootDir, name)
		specificTarget = true
	}

	target, err = filepath.Abs(target)
	if err != nil {
		return "", "", false, "", fmt.Errorf("resolve archive target: %w", err)
	}
	if specificTarget {
		rootDir = filepath.Dir(target)
	}
	return target, rootDir, specificTarget, archiveSlugForTarget(name, target), nil
}

func countArchiveSelectors(cfg ArchiveConfig) int {
	selectors := 0
	if strings.TrimSpace(cfg.Name) != "" {
		selectors++
	}
	if strings.TrimSpace(cfg.TasksDir) != "" {
		selectors++
	}
	return selectors
}

func validateArchiveTarget(
	ctx context.Context,
	target string,
	rootDir string,
	slug string,
	specificTarget bool,
) error {
	if pathContainsArchivedComponent(target) {
		return fmt.Errorf("archive target cannot be inside %s", model.ArchivedWorkflowDirName)
	}

	info, err := os.Stat(target)
	if err == nil {
		if !info.IsDir() {
			return fmt.Errorf("archive target is not a directory: %s", target)
		}
		return nil
	}

	if specificTarget && errors.Is(err, os.ErrNotExist) {
		archiveRoot := model.ArchivedTasksDir(rootDir)
		if archivedWorkflowExists(archiveRoot, slug) || archivedWorkflowIdentityExists(ctx, rootDir, slug) {
			return globaldb.WorkflowArchivedError{Slug: slug}
		}
	}
	return fmt.Errorf("stat archive target: %w", err)
}

func archivedWorkflowIdentityExists(ctx context.Context, rootDir string, slug string) bool {
	if strings.TrimSpace(rootDir) == "" || strings.TrimSpace(slug) == "" {
		return false
	}

	db, workspace, err := openWorkflowGlobalDB(ctx, rootDir)
	if err != nil {
		return false
	}
	defer func() {
		_ = db.Close()
	}()

	_, err = db.GetLatestArchivedWorkflowBySlug(ctx, workspace.ID, slug)
	return err == nil
}

func archivedWorkflowExists(archiveRoot string, slug string) bool {
	entries, err := os.ReadDir(strings.TrimSpace(archiveRoot))
	if err != nil {
		return false
	}

	suffix := "-" + strings.TrimSpace(slug)
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		if strings.HasSuffix(entry.Name(), suffix) {
			return true
		}
	}
	return false
}

func archiveWorkflow(
	ctx context.Context,
	db *globaldb.GlobalDB,
	workspace globaldb.Workspace,
	tasksDir string,
	force bool,
	result *ArchiveResult,
	conflictOnSkip bool,
) error {
	if result == nil {
		return errors.New("archive result is required")
	}
	if workflowRoot, ok := workspaceWorkflowRoot(workspace.RootDir, tasksDir); ok {
		target, resolveErr := (workpackages.TargetResolver{}).Resolve(ctx, workspace.RootDir, workflowRoot)
		if resolveErr != nil {
			return fmt.Errorf("resolve archive workflow %s: %w", tasksDir, resolveErr)
		}
		if target.Mode == workpackages.TargetModeInitiative {
			return archiveWorkPackageInitiative(ctx, db, workspace, target, force, result, conflictOnSkip)
		}
	}

	result.WorkflowsScanned++

	slug := filepath.Base(tasksDir)
	eligibility, skipArchive, err := loadArchiveEligibility(
		ctx,
		db,
		workspace.ID,
		slug,
		tasksDir,
		result,
		conflictOnSkip,
	)
	if err != nil {
		return err
	}
	if skipArchive {
		return nil
	}

	eligibility, skipArchive, err = prepareArchiveWorkflow(
		ctx,
		db,
		workspace,
		tasksDir,
		force,
		result,
		conflictOnSkip,
		eligibility,
	)
	if err != nil {
		return err
	}
	if skipArchive {
		return nil
	}

	return persistArchivedWorkflow(ctx, db, tasksDir, result, slug, eligibility.Workflow.ID)
}

func workspaceWorkflowRoot(workspaceRoot string, tasksDir string) (string, bool) {
	tasksRoot := canonicalWorkflowScopePath(model.TasksBaseDirForWorkspace(workspaceRoot))
	target := canonicalWorkflowScopePath(tasksDir)
	relative, err := filepath.Rel(tasksRoot, target)
	if err != nil || relative == "." || strings.Contains(relative, string(filepath.Separator)) {
		return "", false
	}
	if !model.IsActiveWorkflowDirName(relative) {
		return "", false
	}
	return relative, true
}

type initiativeArchiveState struct {
	Parent          globaldb.Workflow
	Children        map[string]globaldb.WorkflowArchiveEligibility
	PendingPackages []string
	MissingPackages []string
}

func archiveWorkPackageInitiative(
	ctx context.Context,
	db *globaldb.GlobalDB,
	workspace globaldb.Workspace,
	target workpackages.Target,
	force bool,
	result *ArchiveResult,
	conflictOnSkip bool,
) error {
	result.WorkflowsScanned++
	latestTarget, state, err := refreshInitiativeArchiveState(ctx, db, workspace, target.Ref.Initiative)
	if err != nil {
		return err
	}
	populateInitiativeArchiveResult(result, state)
	if len(state.MissingPackages) > 0 {
		return resolveInitiativeArchiveConflict(
			result,
			latestTarget.InitiativeDir,
			conflictOnSkip,
			state,
			"declared work package directories are missing: "+strings.Join(state.MissingPackages, ", "),
		)
	}
	if activeErr := activeInitiativeRunConflict(state); activeErr != nil {
		return activeErr
	}
	if initiativeArchiveBlocked(state) {
		if !force {
			return resolveInitiativeArchiveConflict(
				result,
				latestTarget.InitiativeDir,
				conflictOnSkip,
				state,
				initiativeArchiveReason(state),
			)
		}
		completedTasks, resolvedReviewIssues, forceErr := forceArchiveInitiative(ctx, workspace.RootDir, latestTarget)
		if forceErr != nil {
			return forceErr
		}
		result.Forced = true
		result.CompletedTasks += completedTasks
		result.ResolvedReviewIssues += resolvedReviewIssues
		latestTarget, state, err = refreshInitiativeArchiveState(ctx, db, workspace, target.Ref.Initiative)
		if err != nil {
			return err
		}
		populateInitiativeArchiveResult(result, state)
		if activeErr := activeInitiativeRunConflict(state); activeErr != nil {
			return activeErr
		}
		if initiativeChildStateBlocked(state) {
			return resolveInitiativeArchiveConflict(
				result,
				latestTarget.InitiativeDir,
				true,
				state,
				initiativeArchiveReason(state),
			)
		}
	}

	return persistArchivedWorkflowHierarchy(
		ctx,
		db,
		latestTarget.InitiativeDir,
		result,
		latestTarget.Ref.Initiative,
		state.Parent.ID,
	)
}

func refreshInitiativeArchiveState(
	ctx context.Context,
	db *globaldb.GlobalDB,
	workspace globaldb.Workspace,
	initiative string,
) (workpackages.Target, initiativeArchiveState, error) {
	resolver := workpackages.TargetResolver{}
	target, err := resolver.Resolve(ctx, workspace.RootDir, initiative)
	if err != nil {
		return workpackages.Target{}, initiativeArchiveState{}, fmt.Errorf("reload work package archive plan: %w", err)
	}
	if _, err := SyncWithDB(ctx, db, workspace, SyncConfig{
		WorkspaceRoot: workspace.RootDir,
		TasksDir:      target.InitiativeDir,
	}); err != nil {
		return workpackages.Target{}, initiativeArchiveState{}, fmt.Errorf(
			"refresh work package archive state: %w",
			err,
		)
	}
	target, err = resolver.Resolve(ctx, workspace.RootDir, initiative)
	if err != nil {
		return workpackages.Target{}, initiativeArchiveState{}, fmt.Errorf("reload work package archive plan: %w", err)
	}
	state, err := loadInitiativeArchiveState(ctx, db, workspace, target)
	if err != nil {
		return workpackages.Target{}, initiativeArchiveState{}, err
	}
	return target, state, nil
}

func populateInitiativeArchiveResult(result *ArchiveResult, state initiativeArchiveState) {
	result.PendingWorkPackages = append([]string(nil), state.PendingPackages...)
	result.WorkPackageChildIDs = result.WorkPackageChildIDs[:0]
	for packageID := range state.Children {
		result.WorkPackageChildIDs = append(result.WorkPackageChildIDs, state.Children[packageID].Workflow.ID)
	}
	sort.Strings(result.WorkPackageChildIDs)
}

func loadInitiativeArchiveState(
	ctx context.Context,
	db *globaldb.GlobalDB,
	workspace globaldb.Workspace,
	target workpackages.Target,
) (initiativeArchiveState, error) {
	parent, err := db.GetActiveWorkflowBySlug(ctx, workspace.ID, target.Ref.Initiative)
	if err != nil {
		return initiativeArchiveState{}, err
	}
	children, err := db.ListChildWorkflows(ctx, parent.ID, false)
	if err != nil {
		return initiativeArchiveState{}, err
	}
	childByPackageID := make(map[string]globaldb.Workflow, len(children))
	for childIndex := range children {
		child := &children[childIndex]
		childByPackageID[child.PackageID] = *child
	}
	declaredChildren := make([]globaldb.Workflow, 0, len(target.Plan.Packages))
	state := initiativeArchiveState{
		Parent:   parent,
		Children: make(map[string]globaldb.WorkflowArchiveEligibility, len(target.Plan.Packages)),
	}
	for packageIndex := range target.Plan.Packages {
		pkg := &target.Plan.Packages[packageIndex]
		child, exists := childByPackageID[pkg.ID]
		if !exists {
			state.MissingPackages = append(state.MissingPackages, pkg.ID)
			continue
		}
		declaredChildren = append(declaredChildren, child)
		if !pkg.Completed {
			state.PendingPackages = append(state.PendingPackages, pkg.ID)
		}
	}
	if len(declaredChildren) > 0 {
		eligibilityByID, eligibilityErr := db.WorkflowArchiveEligibilityByIDs(ctx, declaredChildren)
		if eligibilityErr != nil {
			return initiativeArchiveState{}, eligibilityErr
		}
		for childIndex := range declaredChildren {
			child := &declaredChildren[childIndex]
			state.Children[child.PackageID] = eligibilityByID[child.ID]
		}
	}
	sort.Strings(state.PendingPackages)
	sort.Strings(state.MissingPackages)
	return state, nil
}

func activeInitiativeRunConflict(state initiativeArchiveState) error {
	packageIDs := make([]string, 0, len(state.Children))
	for packageID := range state.Children {
		packageIDs = append(packageIDs, packageID)
	}
	sort.Strings(packageIDs)
	for _, packageID := range packageIDs {
		eligibility := state.Children[packageID]
		if eligibility.ActiveRuns > 0 {
			return eligibility.ConflictError()
		}
	}
	return nil
}

func initiativeArchiveBlocked(state initiativeArchiveState) bool {
	if len(state.PendingPackages) > 0 {
		return true
	}
	return initiativeChildStateBlocked(state)
}

func initiativeChildStateBlocked(state initiativeArchiveState) bool {
	for packageID := range state.Children {
		eligibility := state.Children[packageID]
		if eligibility.SkipReason() != "" {
			return true
		}
	}
	return false
}

func initiativeArchiveReason(state initiativeArchiveState) string {
	parts := make([]string, 0, len(state.PendingPackages)+len(state.Children))
	if len(state.PendingPackages) > 0 {
		parts = append(parts, "pending packages: "+strings.Join(state.PendingPackages, ", "))
	}
	for packageID := range state.Children {
		eligibility := state.Children[packageID]
		if reason := eligibility.SkipReason(); reason != "" {
			parts = append(parts, packageID+": "+reason)
		}
	}
	sort.Strings(parts)
	return strings.Join(parts, "; ")
}

func resolveInitiativeArchiveConflict(
	result *ArchiveResult,
	tasksDir string,
	conflictOnSkip bool,
	state initiativeArchiveState,
	reason string,
) error {
	if conflictOnSkip {
		return WorkflowArchiveForceRequiredError{
			WorkspaceID:      state.Parent.WorkspaceID,
			WorkflowID:       state.Parent.ID,
			Slug:             state.Parent.Slug,
			Reason:           reason,
			TaskTotal:        initiativeTaskTotal(state),
			TaskNonTerminal:  initiativePendingTaskTotal(state),
			ReviewTotal:      initiativeReviewTotal(state),
			ReviewUnresolved: initiativeUnresolvedReviewTotal(state),
		}
	}
	recordArchiveSkip(result, tasksDir, reason)
	return nil
}

func forceArchiveInitiative(ctx context.Context, workspaceRoot string, target workpackages.Target) (int, int, error) {
	resolver := workpackages.TargetResolver{}
	completedTasks := 0
	resolvedReviewIssues := 0
	for packageIndex := range target.Plan.Packages {
		pkg := &target.Plan.Packages[packageIndex]
		childTarget, err := resolver.Resolve(ctx, workspaceRoot, target.Ref.Initiative+"/"+pkg.ID)
		if err != nil {
			return completedTasks, resolvedReviewIssues, fmt.Errorf("resolve force archive package %s: %w", pkg.ID, err)
		}
		completed, err := tasks.CompleteNonTerminalTasks(childTarget.PackageDir)
		if err != nil {
			return completedTasks, resolvedReviewIssues, err
		}
		resolved, err := reviews.ResolveUnresolvedIssues(childTarget.PackageDir)
		if err != nil {
			return completedTasks + completed, resolvedReviewIssues, err
		}
		completedTasks += completed
		resolvedReviewIssues += resolved
	}
	return completedTasks, resolvedReviewIssues, nil
}

func initiativeTaskTotal(state initiativeArchiveState) int {
	total := 0
	for packageID := range state.Children {
		eligibility := state.Children[packageID]
		total += eligibility.TaskTotal
	}
	return total
}

func initiativePendingTaskTotal(state initiativeArchiveState) int {
	total := 0
	for packageID := range state.Children {
		eligibility := state.Children[packageID]
		total += eligibility.PendingTasks
	}
	return total
}

func initiativeReviewTotal(state initiativeArchiveState) int {
	total := 0
	for packageID := range state.Children {
		eligibility := state.Children[packageID]
		total += eligibility.ReviewIssueTotal
	}
	return total
}

func initiativeUnresolvedReviewTotal(state initiativeArchiveState) int {
	total := 0
	for packageID := range state.Children {
		eligibility := state.Children[packageID]
		total += eligibility.UnresolvedReviewIssues
	}
	return total
}

func loadArchiveEligibility(
	ctx context.Context,
	db *globaldb.GlobalDB,
	workspaceID string,
	slug string,
	tasksDir string,
	result *ArchiveResult,
	conflictOnSkip bool,
) (globaldb.WorkflowArchiveEligibility, bool, error) {
	eligibility, err := db.GetWorkflowArchiveEligibility(ctx, strings.TrimSpace(workspaceID), slug)
	if err == nil {
		return eligibility, false, nil
	}
	if !errors.Is(err, globaldb.ErrWorkflowNotFound) {
		return globaldb.WorkflowArchiveEligibility{}, false, err
	}

	reason := workflowStateNotSyncedReason
	if conflictOnSkip {
		return globaldb.WorkflowArchiveEligibility{}, false, globaldb.WorkflowNotArchivableError{
			WorkspaceID: strings.TrimSpace(workspaceID),
			Slug:        slug,
			Reason:      reason,
		}
	}

	recordArchiveSkip(result, tasksDir, reason)
	return globaldb.WorkflowArchiveEligibility{}, true, nil
}

func prepareArchiveWorkflow(
	ctx context.Context,
	db *globaldb.GlobalDB,
	workspace globaldb.Workspace,
	tasksDir string,
	force bool,
	result *ArchiveResult,
	conflictOnSkip bool,
	eligibility globaldb.WorkflowArchiveEligibility,
) (globaldb.WorkflowArchiveEligibility, bool, error) {
	if conflictOnSkip && archiveEligibilityNeedsFilesystemRefresh(eligibility) {
		// Never-synced workflows still follow workflowStateNotSyncedReason; this
		// refresh is only for visible DB rows whose zero-artifact counts are stale.
		updatedEligibility, err := refreshArchiveEligibility(ctx, db, workspace, tasksDir, eligibility.Workflow.Slug)
		if err != nil {
			return globaldb.WorkflowArchiveEligibility{}, false, err
		}
		eligibility = updatedEligibility
	}
	if eligibility.SkipReason() == "" {
		return eligibility, false, nil
	}

	if !archiveForceableConflict(eligibility) {
		return resolveArchiveConflict(result, tasksDir, conflictOnSkip, eligibility)
	}

	if !force {
		return handleForceRequiredConflict(result, tasksDir, conflictOnSkip, eligibility)
	}

	updatedEligibility, completedTasks, resolvedReviewIssues, err := forceArchiveWorkflow(
		ctx,
		db,
		workspace,
		tasksDir,
		eligibility,
	)
	if err != nil {
		return globaldb.WorkflowArchiveEligibility{}, false, err
	}
	if completedTasks > 0 || resolvedReviewIssues > 0 {
		result.Forced = true
		result.CompletedTasks += completedTasks
		result.ResolvedReviewIssues += resolvedReviewIssues
	}
	return resolveArchiveConflict(result, tasksDir, conflictOnSkip, updatedEligibility)
}

func archiveEligibilityNeedsFilesystemRefresh(eligibility globaldb.WorkflowArchiveEligibility) bool {
	// Keep this aligned with WorkflowArchiveEligibility.SkipReason's empty
	// catalog branch: no active runs, no task files, and no review issues.
	return eligibility.ActiveRuns == 0 &&
		eligibility.TaskTotal == 0 &&
		eligibility.ReviewIssueTotal == 0
}

func refreshArchiveEligibility(
	ctx context.Context,
	db *globaldb.GlobalDB,
	workspace globaldb.Workspace,
	tasksDir string,
	slug string,
) (globaldb.WorkflowArchiveEligibility, error) {
	if _, err := SyncWithDB(ctx, db, workspace, SyncConfig{
		WorkspaceRoot: workspace.RootDir,
		TasksDir:      tasksDir,
	}); err != nil {
		return globaldb.WorkflowArchiveEligibility{}, fmt.Errorf(
			"refresh archive eligibility sync for workspace %s slug %q: %w",
			workspace.ID,
			slug,
			err,
		)
	}

	updatedEligibility, err := db.GetWorkflowArchiveEligibility(ctx, workspace.ID, slug)
	if err != nil {
		return globaldb.WorkflowArchiveEligibility{}, fmt.Errorf(
			"refresh archive eligibility lookup for workspace %s slug %q: %w",
			workspace.ID,
			slug,
			err,
		)
	}
	return updatedEligibility, nil
}

func handleForceRequiredConflict(
	result *ArchiveResult,
	tasksDir string,
	conflictOnSkip bool,
	eligibility globaldb.WorkflowArchiveEligibility,
) (globaldb.WorkflowArchiveEligibility, bool, error) {
	if conflictOnSkip {
		return globaldb.WorkflowArchiveEligibility{}, false, WorkflowArchiveForceRequiredError{
			WorkspaceID:      eligibility.Workflow.WorkspaceID,
			WorkflowID:       eligibility.Workflow.ID,
			Slug:             eligibility.Workflow.Slug,
			Reason:           eligibility.SkipReason(),
			TaskTotal:        eligibility.TaskTotal,
			TaskNonTerminal:  eligibility.PendingTasks,
			ReviewTotal:      eligibility.ReviewIssueTotal,
			ReviewUnresolved: eligibility.UnresolvedReviewIssues,
		}
	}

	recordArchiveSkip(result, tasksDir, eligibility.SkipReason())
	return eligibility, true, nil
}

func resolveArchiveConflict(
	result *ArchiveResult,
	tasksDir string,
	conflictOnSkip bool,
	eligibility globaldb.WorkflowArchiveEligibility,
) (globaldb.WorkflowArchiveEligibility, bool, error) {
	reason := eligibility.SkipReason()
	if reason == "" {
		return eligibility, false, nil
	}

	if conflictOnSkip {
		return globaldb.WorkflowArchiveEligibility{}, false, eligibility.ConflictError()
	}

	recordArchiveSkip(result, tasksDir, reason)
	return eligibility, true, nil
}

func persistArchivedWorkflow(
	ctx context.Context,
	db *globaldb.GlobalDB,
	tasksDir string,
	result *ArchiveResult,
	slug string,
	workflowID string,
) error {
	if err := os.MkdirAll(result.ArchiveRoot, 0o755); err != nil {
		return fmt.Errorf("mkdir archive root: %w", err)
	}

	archivedAt := time.Now().UTC()
	archivedDir := filepath.Join(
		result.ArchiveRoot,
		model.ArchivedWorkflowName(slug, workflowID, archivedAt),
	)
	if err := os.Rename(tasksDir, archivedDir); err != nil {
		return fmt.Errorf("archive workflow %s: %w", tasksDir, err)
	}

	if _, err := db.MarkWorkflowArchived(ctx, workflowID, archivedAt); err != nil {
		if rollbackErr := os.Rename(archivedDir, tasksDir); rollbackErr != nil {
			return errors.Join(
				fmt.Errorf("persist archived workflow state %s: %w", workflowID, err),
				fmt.Errorf("rollback archived workflow rename %s: %w", archivedDir, rollbackErr),
			)
		}
		return fmt.Errorf("persist archived workflow state %s: %w", workflowID, err)
	}

	result.Archived++
	result.ArchivedAt = &archivedAt
	result.ArchivedPaths = append(result.ArchivedPaths, archivedDir)
	return nil
}

func persistArchivedWorkflowHierarchy(
	ctx context.Context,
	db *globaldb.GlobalDB,
	tasksDir string,
	result *ArchiveResult,
	slug string,
	parentWorkflowID string,
) error {
	if err := os.MkdirAll(result.ArchiveRoot, 0o755); err != nil {
		return fmt.Errorf("mkdir archive root: %w", err)
	}

	archivedAt := time.Now().UTC()
	archivedDir := filepath.Join(
		result.ArchiveRoot,
		model.ArchivedWorkflowName(slug, parentWorkflowID, archivedAt),
	)
	if err := os.Rename(tasksDir, archivedDir); err != nil {
		return fmt.Errorf("archive workflow hierarchy %s: %w", tasksDir, err)
	}

	if _, err := db.MarkWorkflowHierarchyArchived(ctx, parentWorkflowID, archivedAt); err != nil {
		if rollbackErr := os.Rename(archivedDir, tasksDir); rollbackErr != nil {
			return errors.Join(
				fmt.Errorf("persist archived workflow hierarchy %s: %w", parentWorkflowID, err),
				fmt.Errorf("rollback archived workflow hierarchy rename %s: %w", archivedDir, rollbackErr),
			)
		}
		return fmt.Errorf("persist archived workflow hierarchy %s: %w", parentWorkflowID, err)
	}

	result.Archived++
	result.ArchivedAt = &archivedAt
	result.ArchivedPaths = append(result.ArchivedPaths, archivedDir)
	return nil
}

func archiveForceableConflict(eligibility globaldb.WorkflowArchiveEligibility) bool {
	return eligibility.ActiveRuns == 0 &&
		(eligibility.PendingTasks > 0 || eligibility.UnresolvedReviewIssues > 0)
}

func forceArchiveWorkflow(
	ctx context.Context,
	db *globaldb.GlobalDB,
	workspace globaldb.Workspace,
	tasksDir string,
	eligibility globaldb.WorkflowArchiveEligibility,
) (globaldb.WorkflowArchiveEligibility, int, int, error) {
	completedTasks, err := tasks.CompleteNonTerminalTasks(tasksDir)
	if err != nil {
		return globaldb.WorkflowArchiveEligibility{}, 0, 0, err
	}

	resolvedReviewIssues, err := reviews.ResolveUnresolvedIssues(tasksDir)
	if err != nil {
		return globaldb.WorkflowArchiveEligibility{}, completedTasks, 0, err
	}

	if _, err := SyncWithDB(ctx, db, workspace, SyncConfig{
		WorkspaceRoot: workspace.RootDir,
		TasksDir:      tasksDir,
	}); err != nil {
		return globaldb.WorkflowArchiveEligibility{}, completedTasks, resolvedReviewIssues, err
	}

	updatedEligibility, err := db.GetWorkflowArchiveEligibility(ctx, workspace.ID, eligibility.Workflow.Slug)
	if err != nil {
		return globaldb.WorkflowArchiveEligibility{}, completedTasks, resolvedReviewIssues, err
	}
	if reason := updatedEligibility.SkipReason(); reason != "" {
		return updatedEligibility, completedTasks, resolvedReviewIssues, updatedEligibility.ConflictError()
	}
	return updatedEligibility, completedTasks, resolvedReviewIssues, nil
}

func recordArchiveSkip(result *ArchiveResult, tasksDir string, reason string) {
	if result == nil {
		return
	}
	result.Skipped++
	result.SkippedPaths = append(result.SkippedPaths, tasksDir)
	result.SkippedReasons[tasksDir] = reason
}

func pathContainsArchivedComponent(path string) bool {
	cleaned := filepath.Clean(path)
	for {
		if filepath.Base(cleaned) == model.ArchivedWorkflowDirName {
			return true
		}
		parent := filepath.Dir(cleaned)
		if parent == cleaned {
			return false
		}
		cleaned = parent
	}
}

func sortArchiveResult(result *ArchiveResult) {
	if result == nil {
		return
	}
	sort.Strings(result.ArchivedPaths)
	sort.Strings(result.SkippedPaths)
}
