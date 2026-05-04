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
	"github.com/compozy/compozy/internal/store/globaldb"
)

const workflowStateNotSyncedReason = "workflow state not synced"

var ErrWorkflowForceRequired = errors.New("core: workflow force required")

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

	if singleWorkflow {
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

func resolveArchiveTarget(ctx context.Context, cfg ArchiveConfig) (string, string, bool, error) {
	name := strings.TrimSpace(cfg.Name)
	if name == model.ArchivedWorkflowDirName {
		return "", "", false, fmt.Errorf("archive target cannot be %s", model.ArchivedWorkflowDirName)
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
