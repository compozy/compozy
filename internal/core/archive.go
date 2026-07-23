package core

import (
	"bytes"
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
	"github.com/compozy/compozy/internal/core/taskgroups"
	"github.com/compozy/compozy/internal/core/tasks"
	"github.com/compozy/compozy/internal/store/globaldb"
)

const (
	workflowStateNotSyncedReason = "workflow state not synced"
	archiveMetadataFileName      = "_meta.md"
)

var (
	ErrWorkflowForceRequired      = errors.New("core: workflow force required")
	ErrTaskGroupRootOnly          = errors.New("core: task group sync or archive requires initiative root")
	ErrArchiveDatabaseRequired    = errors.New("core: archive database is required")
	ErrArchiveWorkspaceIDRequired = errors.New("core: archive workspace id is required")
)

// TaskGroupRootOnlyError rejects a task-group-local sync or archive target.
type TaskGroupRootOnlyError struct {
	Target string
}

func (e TaskGroupRootOnlyError) Error() string {
	return fmt.Sprintf("core: task group target %q cannot be synchronized or archived independently", e.Target)
}

func (e TaskGroupRootOnlyError) Is(target error) bool {
	return target == ErrTaskGroupRootOnly
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
		if isTaskGroupOperationalDirectory(workspace.RootDir, target) {
			return result, TaskGroupRootOnlyError{Target: target}
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

func isTaskGroupOperationalDirectory(workspaceRoot string, path string) bool {
	tasksRoot := canonicalWorkflowScopePath(model.TasksBaseDirForWorkspace(workspaceRoot))
	target := canonicalWorkflowScopePath(path)
	relative, err := filepath.Rel(tasksRoot, target)
	if err != nil || relative == "." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return false
	}
	for _, component := range strings.Split(filepath.Clean(relative), string(filepath.Separator)) {
		if component == "_task_groups" {
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
	if target, ok := namedTaskGroupTarget(name); ok {
		return "", "", false, TaskGroupRootOnlyError{Target: target}
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
		target, resolveErr := (taskgroups.TargetResolver{}).Resolve(ctx, workspace.RootDir, workflowRoot)
		if resolveErr != nil {
			return fmt.Errorf("resolve archive workflow %s: %w", tasksDir, resolveErr)
		}
		if target.Mode == taskgroups.TargetModeInitiative {
			return archiveTaskGroupInitiative(ctx, db, workspace, target, force, result, conflictOnSkip)
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
	Parent            globaldb.Workflow
	ParentEligibility globaldb.WorkflowArchiveEligibility
	Children          map[string]globaldb.WorkflowArchiveEligibility
	// DirectChildren holds every non-archived direct child of the initiative,
	// including stale children retained by pruning while a run is active. It is a
	// superset of Children (plan-declared task groups) so the active-run guard covers
	// the exact hierarchy the archive mutation touches.
	DirectChildren    []globaldb.WorkflowArchiveEligibility
	PendingTaskGroups []string
	MissingTaskGroups []string
}

func archiveTaskGroupInitiative(
	ctx context.Context,
	db *globaldb.GlobalDB,
	workspace globaldb.Workspace,
	target taskgroups.Target,
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
	if len(state.MissingTaskGroups) > 0 {
		return resolveInitiativeArchiveConflict(
			result,
			latestTarget.InitiativeDir,
			conflictOnSkip,
			state,
			"declared task group directories are missing: "+strings.Join(state.MissingTaskGroups, ", "),
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
		return forceArchiveTaskGroupInitiative(ctx, db, workspace, latestTarget, result)
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

func forceArchiveTaskGroupInitiative(
	ctx context.Context,
	db *globaldb.GlobalDB,
	workspace globaldb.Workspace,
	target taskgroups.Target,
	result *ArchiveResult,
) error {
	mutation, err := forceArchiveInitiative(ctx, workspace.RootDir, target)
	if err != nil {
		return err
	}
	latestTarget, state, err := refreshInitiativeArchiveState(ctx, db, workspace, target.Ref.Initiative)
	if err != nil {
		return mutation.rollback(err)
	}
	populateInitiativeArchiveResult(result, state)
	if activeErr := activeInitiativeRunConflict(state); activeErr != nil {
		return mutation.rollback(activeErr)
	}
	if initiativeChildStateBlocked(state) {
		conflictErr := resolveInitiativeArchiveConflict(
			result,
			latestTarget.InitiativeDir,
			true,
			state,
			initiativeArchiveReason(state),
		)
		return mutation.rollback(conflictErr)
	}
	if err := persistArchivedWorkflowHierarchy(
		ctx,
		db,
		latestTarget.InitiativeDir,
		result,
		latestTarget.Ref.Initiative,
		state.Parent.ID,
	); err != nil {
		return mutation.rollback(err)
	}
	mutation.apply(result)
	return nil
}

func refreshInitiativeArchiveState(
	ctx context.Context,
	db *globaldb.GlobalDB,
	workspace globaldb.Workspace,
	initiative string,
) (taskgroups.Target, initiativeArchiveState, error) {
	resolver := taskgroups.TargetResolver{}
	target, err := resolver.Resolve(ctx, workspace.RootDir, initiative)
	if err != nil {
		return taskgroups.Target{}, initiativeArchiveState{}, fmt.Errorf("reload task group archive plan: %w", err)
	}
	if _, err := SyncWithDB(ctx, db, workspace, SyncConfig{
		WorkspaceRoot: workspace.RootDir,
		TasksDir:      target.InitiativeDir,
	}); err != nil {
		return taskgroups.Target{}, initiativeArchiveState{}, fmt.Errorf(
			"refresh task group archive state: %w",
			err,
		)
	}
	target, err = resolver.Resolve(ctx, workspace.RootDir, initiative)
	if err != nil {
		return taskgroups.Target{}, initiativeArchiveState{}, fmt.Errorf("reload task group archive plan: %w", err)
	}
	state, err := loadInitiativeArchiveState(ctx, db, workspace, target)
	if err != nil {
		return taskgroups.Target{}, initiativeArchiveState{}, err
	}
	return target, state, nil
}

func populateInitiativeArchiveResult(result *ArchiveResult, state initiativeArchiveState) {
	result.PendingTaskGroups = append([]string(nil), state.PendingTaskGroups...)
	result.TaskGroupChildIDs = result.TaskGroupChildIDs[:0]
	for taskGroupID := range state.Children {
		result.TaskGroupChildIDs = append(result.TaskGroupChildIDs, state.Children[taskGroupID].Workflow.ID)
	}
	sort.Strings(result.TaskGroupChildIDs)
}

func loadInitiativeArchiveState(
	ctx context.Context,
	db *globaldb.GlobalDB,
	workspace globaldb.Workspace,
	target taskgroups.Target,
) (initiativeArchiveState, error) {
	parent, err := db.GetActiveWorkflowBySlug(ctx, workspace.ID, target.Ref.Initiative)
	if err != nil {
		return initiativeArchiveState{}, err
	}
	children, err := db.ListChildWorkflows(ctx, parent.ID, false)
	if err != nil {
		return initiativeArchiveState{}, err
	}
	// Resolve eligibility for the parent row and every non-archived direct child so
	// the active-run guard matches the archive mutation's exact scope
	// (id = parent OR parent_workflow_id = parent), not just plan-declared task groups.
	// An ordinary workflow promoted to an initiative in place keeps its runs on the
	// parent row, and pruning retains a removed child while its run is active; both
	// live outside target.Plan.TaskGroups.
	scope := make([]globaldb.Workflow, 0, len(children)+1)
	scope = append(scope, parent)
	scope = append(scope, children...)
	eligibilityByID, err := db.WorkflowArchiveEligibilityByIDs(ctx, scope)
	if err != nil {
		return initiativeArchiveState{}, err
	}
	childByTaskGroupID := make(map[string]globaldb.Workflow, len(children))
	directChildren := make([]globaldb.WorkflowArchiveEligibility, 0, len(children))
	for childIndex := range children {
		child := &children[childIndex]
		childByTaskGroupID[child.TaskGroupID] = *child
		directChildren = append(directChildren, eligibilityByID[child.ID])
	}
	state := initiativeArchiveState{
		Parent:            parent,
		ParentEligibility: eligibilityByID[parent.ID],
		Children:          make(map[string]globaldb.WorkflowArchiveEligibility, len(target.Plan.TaskGroups)),
		DirectChildren:    directChildren,
	}
	for taskGroupIndex := range target.Plan.TaskGroups {
		taskGroup := &target.Plan.TaskGroups[taskGroupIndex]
		child, exists := childByTaskGroupID[taskGroup.ID]
		// A durable child flagged Missing has no directory on disk: sync retains its
		// completed task/review projection but marks the row missing (see
		// appendMissingTaskGroupPlaceholders). Because refreshInitiativeArchiveState
		// syncs before loading state, such a task group always owns a row, so the !exists
		// branch alone can never catch it. Treat Missing as a missing declared task group
		// so archive refuses before any normal or forced mutation, matching the daemon
		// read model (transportTaskGroupSummary), which likewise makes a Missing row
		// archive-ineligible.
		if !exists || child.Missing {
			state.MissingTaskGroups = append(state.MissingTaskGroups, taskGroup.ID)
			continue
		}
		if !taskGroup.Completed {
			state.PendingTaskGroups = append(state.PendingTaskGroups, taskGroup.ID)
		}
		state.Children[child.TaskGroupID] = eligibilityByID[child.ID]
	}
	sort.Strings(state.PendingTaskGroups)
	sort.Strings(state.MissingTaskGroups)
	return state, nil
}

func activeInitiativeRunConflict(state initiativeArchiveState) error {
	// The archive mutation moves the filesystem root and marks the parent plus
	// every non-archived direct child archived. Reject if any of those rows still
	// has an active run, covering the parent (an ordinary workflow converted in
	// place keeps its runs) and stale children retained by pruning — neither of
	// which appears in the plan-declared state.Children map.
	if state.ParentEligibility.ActiveRuns > 0 {
		return state.ParentEligibility.ConflictError()
	}
	children := append([]globaldb.WorkflowArchiveEligibility(nil), state.DirectChildren...)
	sort.Slice(children, func(i, j int) bool {
		return children[i].Workflow.ID < children[j].Workflow.ID
	})
	for childIndex := range children {
		if children[childIndex].ActiveRuns > 0 {
			return children[childIndex].ConflictError()
		}
	}
	return nil
}

func initiativeArchiveBlocked(state initiativeArchiveState) bool {
	if len(state.PendingTaskGroups) > 0 {
		return true
	}
	return initiativeChildStateBlocked(state)
}

func initiativeChildStateBlocked(state initiativeArchiveState) bool {
	for taskGroupID := range state.Children {
		eligibility := state.Children[taskGroupID]
		if eligibility.SkipReason() != "" {
			return true
		}
	}
	return false
}

func initiativeArchiveReason(state initiativeArchiveState) string {
	parts := make([]string, 0, len(state.PendingTaskGroups)+len(state.Children))
	if len(state.PendingTaskGroups) > 0 {
		parts = append(parts, "pending task groups: "+strings.Join(state.PendingTaskGroups, ", "))
	}
	for taskGroupID := range state.Children {
		eligibility := state.Children[taskGroupID]
		if reason := eligibility.SkipReason(); reason != "" {
			parts = append(parts, taskGroupID+": "+reason)
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

type archiveArtifactState struct {
	content []byte
	mode    os.FileMode
}

type initiativeArtifactTransaction struct {
	roots  []string
	before map[string]archiveArtifactState
	after  map[string]archiveArtifactState
}

type initiativeArchiveMutation struct {
	completedTasks       int
	resolvedReviewIssues int
	artifacts            *initiativeArtifactTransaction
}

func (m initiativeArchiveMutation) apply(result *ArchiveResult) {
	result.Forced = true
	result.CompletedTasks += m.completedTasks
	result.ResolvedReviewIssues += m.resolvedReviewIssues
}

func (m initiativeArchiveMutation) rollback(cause error) error {
	if m.artifacts == nil {
		return cause
	}
	if err := m.artifacts.rollback(); err != nil {
		return errors.Join(cause, fmt.Errorf("rollback forced initiative artifacts: %w", err))
	}
	return cause
}

var forceArchiveInitiative = forceArchiveInitiativeArtifacts

func forceArchiveInitiativeArtifacts(
	ctx context.Context,
	workspaceRoot string,
	target taskgroups.Target,
) (initiativeArchiveMutation, error) {
	resolver := taskgroups.TargetResolver{}
	childDirs := make([]string, 0, len(target.Plan.TaskGroups))
	for taskGroupIndex := range target.Plan.TaskGroups {
		taskGroup := &target.Plan.TaskGroups[taskGroupIndex]
		childTarget, err := resolver.Resolve(ctx, workspaceRoot, target.Ref.Initiative+"/"+taskGroup.ID)
		if err != nil {
			return initiativeArchiveMutation{}, fmt.Errorf(
				"resolve force archive task group %s: %w",
				taskGroup.ID,
				err,
			)
		}
		childDirs = append(childDirs, childTarget.TaskGroupDir)
	}

	artifacts, err := newInitiativeArtifactTransaction(childDirs)
	if err != nil {
		return initiativeArchiveMutation{}, err
	}
	mutation := initiativeArchiveMutation{artifacts: artifacts}
	for childIndex := range childDirs {
		childDir := childDirs[childIndex]
		completed, err := tasks.CompleteNonTerminalTasks(childDir)
		if err != nil {
			return initiativeArchiveMutation{}, artifacts.rollbackAfter(err)
		}
		resolved, err := reviews.ResolveUnresolvedIssues(childDir)
		if err != nil {
			return initiativeArchiveMutation{}, artifacts.rollbackAfter(err)
		}
		mutation.completedTasks += completed
		mutation.resolvedReviewIssues += resolved
	}
	if err := artifacts.seal(); err != nil {
		return initiativeArchiveMutation{}, artifacts.rollbackAfter(err)
	}
	return mutation, nil
}

func newInitiativeArtifactTransaction(roots []string) (*initiativeArtifactTransaction, error) {
	transaction := &initiativeArtifactTransaction{roots: append([]string(nil), roots...)}
	before, err := captureForceMutableArtifacts(transaction.roots)
	if err != nil {
		return nil, fmt.Errorf("snapshot forced initiative artifacts: %w", err)
	}
	transaction.before = before
	return transaction, nil
}

func (t *initiativeArtifactTransaction) seal() error {
	after, err := captureForceMutableArtifacts(t.roots)
	if err != nil {
		return fmt.Errorf("capture forced initiative changes: %w", err)
	}
	t.after = after
	return nil
}

func (t *initiativeArtifactTransaction) rollbackAfter(cause error) error {
	if t.after == nil {
		if err := t.seal(); err != nil {
			return errors.Join(cause, err)
		}
	}
	if err := t.rollback(); err != nil {
		return errors.Join(cause, fmt.Errorf("rollback forced initiative artifacts: %w", err))
	}
	return cause
}

func (t *initiativeArtifactTransaction) rollback() error {
	if t.after == nil {
		return errors.New("forced initiative artifact transaction is not sealed")
	}
	paths := make(map[string]struct{}, len(t.before)+len(t.after))
	for path := range t.before {
		paths[path] = struct{}{}
	}
	for path := range t.after {
		paths[path] = struct{}{}
	}
	orderedPaths := make([]string, 0, len(paths))
	for path := range paths {
		orderedPaths = append(orderedPaths, path)
	}
	sort.Strings(orderedPaths)

	var rollbackErr error
	for _, path := range orderedPaths {
		before, existedBefore := t.before[path]
		after, existedAfter := t.after[path]
		if archiveArtifactStatesEqual(before, existedBefore, after, existedAfter) {
			continue
		}
		current, currentExists, err := readArchiveArtifact(path)
		if err != nil {
			rollbackErr = errors.Join(rollbackErr, err)
			continue
		}
		if !archiveArtifactStatesEqual(current, currentExists, after, existedAfter) {
			rollbackErr = errors.Join(
				rollbackErr,
				fmt.Errorf("artifact changed concurrently: %s", path),
			)
			continue
		}
		if !existedBefore {
			if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
				rollbackErr = errors.Join(rollbackErr, fmt.Errorf("remove created artifact %s: %w", path, err))
			}
			continue
		}
		if err := os.WriteFile(path, before.content, before.mode.Perm()); err != nil {
			rollbackErr = errors.Join(rollbackErr, fmt.Errorf("restore artifact %s: %w", path, err))
		}
	}
	return rollbackErr
}

func captureForceMutableArtifacts(roots []string) (map[string]archiveArtifactState, error) {
	artifacts := make(map[string]archiveArtifactState)
	for _, root := range roots {
		rootFS, err := os.OpenRoot(root)
		if err != nil {
			return nil, fmt.Errorf("open artifact root %s: %w", root, err)
		}
		walkErr := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() || !forceArchiveMutableArtifact(entry.Name()) {
				return nil
			}
			info, err := entry.Info()
			if err != nil {
				return err
			}
			if !info.Mode().IsRegular() {
				return nil
			}
			relativePath, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			content, err := rootFS.ReadFile(relativePath)
			if err != nil {
				return err
			}
			artifacts[path] = archiveArtifactState{content: content, mode: info.Mode()}
			return nil
		})
		closeErr := rootFS.Close()
		if walkErr != nil || closeErr != nil {
			return nil, fmt.Errorf("scan %s: %w", root, errors.Join(walkErr, closeErr))
		}
	}
	return artifacts, nil
}

func forceArchiveMutableArtifact(name string) bool {
	return name == archiveMetadataFileName ||
		tasks.ExtractTaskNumber(name) > 0 ||
		reviews.ExtractIssueNumber(name) > 0
}

func readArchiveArtifact(path string) (archiveArtifactState, bool, error) {
	info, err := os.Stat(path)
	if errors.Is(err, os.ErrNotExist) {
		return archiveArtifactState{}, false, nil
	}
	if err != nil {
		return archiveArtifactState{}, false, fmt.Errorf("stat artifact %s: %w", path, err)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return archiveArtifactState{}, false, fmt.Errorf("read artifact %s: %w", path, err)
	}
	return archiveArtifactState{content: content, mode: info.Mode()}, true, nil
}

func archiveArtifactStatesEqual(
	left archiveArtifactState,
	leftExists bool,
	right archiveArtifactState,
	rightExists bool,
) bool {
	return leftExists == rightExists &&
		(!leftExists || left.mode.Perm() == right.mode.Perm() && bytes.Equal(left.content, right.content))
}

func initiativeTaskTotal(state initiativeArchiveState) int {
	total := 0
	for taskGroupID := range state.Children {
		eligibility := state.Children[taskGroupID]
		total += eligibility.TaskTotal
	}
	return total
}

func initiativePendingTaskTotal(state initiativeArchiveState) int {
	total := 0
	for taskGroupID := range state.Children {
		eligibility := state.Children[taskGroupID]
		total += eligibility.PendingTasks
	}
	return total
}

func initiativeReviewTotal(state initiativeArchiveState) int {
	total := 0
	for taskGroupID := range state.Children {
		eligibility := state.Children[taskGroupID]
		total += eligibility.ReviewIssueTotal
	}
	return total
}

func initiativeUnresolvedReviewTotal(state initiativeArchiveState) int {
	total := 0
	for taskGroupID := range state.Children {
		eligibility := state.Children[taskGroupID]
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
